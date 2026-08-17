package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/serverpanel/agent/internal/audit"
	"github.com/OWNER/serverpanel/agent/internal/auth"
	"github.com/OWNER/serverpanel/agent/internal/config"
	"github.com/OWNER/serverpanel/agent/internal/system"
)

const sessionCookie = "sp_session"

type session struct {
	User      auth.PublicUser
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type loginWindow struct {
	Attempts []time.Time
}

type Server struct {
	config   config.Config
	users    *auth.Store
	docker   system.Docker
	audit    *audit.Log
	sessions map[string]session
	limits   map[string]loginWindow
	mu       sync.Mutex
	handler  http.Handler
}

func New(cfg config.Config, users *auth.Store, auditLog *audit.Log) *Server {
	server := &Server{
		config: cfg, users: users, docker: system.Docker{ActionsEnabled: cfg.EnableDockerActions},
		audit: auditLog, sessions: make(map[string]session), limits: make(map[string]loginWindow),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("POST /api/auth/login", server.login)
	mux.HandleFunc("POST /api/auth/logout", server.logout)
	mux.HandleFunc("GET /api/session", server.currentSession)
	mux.HandleFunc("GET /api/metrics", server.metrics)
	mux.HandleFunc("GET /api/containers", server.containers)
	mux.HandleFunc("POST /api/containers/{id}/{action}", server.containerAction)
	mux.HandleFunc("GET /api/diagnostics", server.diagnostics)
	mux.HandleFunc("GET /api/audit", server.auditEntries)
	server.handler = server.middleware(mux)
	return server
}

func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	remoteIP := clientIP(r)
	key := remoteIP + "|" + strings.ToLower(strings.TrimSpace(input.Username))
	if !s.allowLogin(key) {
		writeError(w, http.StatusTooManyRequests, "Too many sign-in attempts. Try again in 15 minutes.")
		return
	}
	user, ok := s.users.Authenticate(input.Username, input.Password)
	if !ok {
		_ = s.audit.Append(audit.Entry{Actor: strings.TrimSpace(input.Username), Action: "auth.login", Result: "denied", RemoteIP: remoteIP})
		writeError(w, http.StatusUnauthorized, "Invalid username or password.")
		return
	}
	token, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create session.")
		return
	}
	csrf, err := randomToken(24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create session.")
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	s.sessions[token] = session{User: user, CSRFToken: csrf, CreatedAt: now, ExpiresAt: now.Add(s.config.SessionTTL)}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: s.config.SecureCookies, SameSite: http.SameSiteStrictMode, MaxAge: int(s.config.SessionTTL.Seconds())})
	_ = s.audit.Append(audit.Entry{Actor: user.Username, Action: "auth.login", Result: "allowed", RemoteIP: remoteIP})
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "csrfToken": csrf, "expiresAt": now.Add(s.config.SessionTTL)})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	current, token, ok := s.requireSession(w, r)
	if !ok || !s.requireCSRF(w, r, current) {
		return
	}
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: s.config.SecureCookies, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	_ = s.audit.Append(audit.Entry{Actor: current.User.Username, Action: "auth.logout", Result: "allowed", RemoteIP: clientIP(r)})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) currentSession(w http.ResponseWriter, r *http.Request) {
	current, _, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": current.User, "csrfToken": current.CSRFToken, "expiresAt": current.ExpiresAt})
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireSession(w, r); !ok {
		return
	}
	metrics, err := system.ReadMetrics()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"metrics": metrics, "usage": system.UsageSummary(metrics)})
}

func (s *Server) containers(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireSession(w, r); !ok {
		return
	}
	containers, err := s.docker.List(r.Context())
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, system.ErrDockerUnavailable) {
			status = http.StatusNotImplemented
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": containers, "actionsEnabled": s.config.EnableDockerActions})
}

func (s *Server) containerAction(w http.ResponseWriter, r *http.Request) {
	current, _, ok := s.requireSession(w, r)
	if !ok || !s.requireCSRF(w, r, current) {
		return
	}
	if current.User.Role != "admin" && current.User.Role != "operator" {
		writeError(w, http.StatusForbidden, "This role cannot control containers.")
		return
	}
	if time.Since(current.CreatedAt) > 15*time.Minute {
		writeError(w, http.StatusPreconditionRequired, "Sign in again before a privileged action.")
		return
	}
	id, action := r.PathValue("id"), r.PathValue("action")
	err := s.docker.Action(r.Context(), id, action)
	entry := audit.Entry{Actor: current.User.Username, Action: "docker." + action, Target: id, RemoteIP: clientIP(r), Result: "allowed"}
	if err != nil {
		entry.Result = "denied"
		entry.Detail = err.Error()
		_ = s.audit.Append(entry)
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	_ = s.audit.Append(entry)
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireSession(w, r); !ok {
		return
	}
	findings, err := system.Diagnose(r.Context(), s.docker)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"findings": findings})
}

func (s *Server) auditEntries(w http.ResponseWriter, r *http.Request) {
	current, _, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if current.User.Role != "admin" {
		writeError(w, http.StatusForbidden, "Only administrators can view the audit log.")
		return
	}
	entries, err := s.audit.Tail(100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not read audit log.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) (session, string, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "Authentication required.")
		return session{}, "", false
	}
	s.mu.Lock()
	current, ok := s.sessions[cookie.Value]
	if ok && time.Now().After(current.ExpiresAt) {
		delete(s.sessions, cookie.Value)
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusUnauthorized, "Session expired.")
		return session{}, "", false
	}
	return current, cookie.Value, true
}

func (s *Server) requireCSRF(w http.ResponseWriter, r *http.Request, current session) bool {
	if token := r.Header.Get("X-CSRF-Token"); token == "" || token != current.CSRFToken {
		writeError(w, http.StatusForbidden, "Invalid CSRF token.")
		return false
	}
	return true
}

func (s *Server) allowLogin(key string) bool {
	now := time.Now()
	cutoff := now.Add(-15 * time.Minute)
	s.mu.Lock()
	defer s.mu.Unlock()
	window := s.limits[key]
	kept := window.Attempts[:0]
	for _, attempt := range window.Attempts {
		if attempt.After(cutoff) {
			kept = append(kept, attempt)
		}
	}
	if len(kept) >= 5 {
		window.Attempts = kept
		s.limits[key] = window
		return false
	}
	window.Attempts = append(kept, now)
	s.limits[key] = window
	return true
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("request panic", "error", recovered, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "Internal server error.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request.")
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func randomToken(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func ListenAddress(cfg config.Config) string {
	return fmt.Sprintf("http://%s", cfg.Listen)
}

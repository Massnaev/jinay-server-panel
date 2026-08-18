package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Massnaev/jinay-server-panel/agent/internal/audit"
	"github.com/Massnaev/jinay-server-panel/agent/internal/auth"
	"github.com/Massnaev/jinay-server-panel/agent/internal/config"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	directory := t.TempDir()
	users, err := auth.Open(filepath.Join(directory, "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := users.Add("admin", "admin", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	return New(config.Config{Version: "test-version", SecureCookies: false, DataDir: directory, SessionTTL: time.Hour}, users, audit.New(filepath.Join(directory, "audit.jsonl")))
}

func TestProtectedRouteNeedsAuthentication(t *testing.T) {
	server := testServer(t)
	for _, path := range []string{"/api/metrics", "/api/history?range=1h"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s, got %d", path, response.Code)
		}
	}
}

func TestWebUIOnlyServesExportedPublicFiles(t *testing.T) {
	directory := t.TempDir()
	webRoot := filepath.Join(directory, "web")
	if err := os.MkdirAll(filepath.Join(webRoot, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("<!doctype html><title>Jinay</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	users, err := auth.Open(filepath.Join(directory, "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := New(config.Config{WebRoot: webRoot, DataDir: directory, SessionTTL: time.Hour}, users, audit.New(filepath.Join(directory, "audit.jsonl")))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte("Jinay")) {
		t.Fatalf("expected exported UI, got %d: %s", response.Code, response.Body.String())
	}
	if csp := response.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("expected UI CSP, got %q", csp)
	}

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/private.txt", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unexpected file path should be 404, got %d", response.Code)
	}
}

func TestLoginSessionAndCSRF(t *testing.T) {
	server := testServer(t)
	body := bytes.NewBufferString(`{"username":"admin","password":"correct horse battery staple"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var login struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) == 0 || !cookies[0].HttpOnly {
		t.Fatal("secure session cookie was not returned")
	}

	action := httptest.NewRequest(http.MethodPost, "/api/containers/example/restart", nil)
	action.AddCookie(cookies[0])
	actionResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(actionResponse, action)
	if actionResponse.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF should be forbidden, got %d", actionResponse.Code)
	}

	action = httptest.NewRequest(http.MethodPost, "/api/containers/example/restart", nil)
	action.AddCookie(cookies[0])
	action.Header.Set("X-CSRF-Token", login.CSRFToken)
	actionResponse = httptest.NewRecorder()
	server.Handler().ServeHTTP(actionResponse, action)
	if actionResponse.Code != http.StatusForbidden {
		t.Fatalf("disabled Docker action should be forbidden, got %d", actionResponse.Code)
	}

	metricsRequest := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	metricsRequest.AddCookie(cookies[0])
	metricsResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(metricsResponse, metricsRequest)
	if metricsResponse.Code != http.StatusOK {
		t.Fatalf("expected metrics response, got %d: %s", metricsResponse.Code, metricsResponse.Body.String())
	}
	var metricsBody struct {
		AgentVersion string `json:"agentVersion"`
	}
	if err := json.Unmarshal(metricsResponse.Body.Bytes(), &metricsBody); err != nil {
		t.Fatal(err)
	}
	if metricsBody.AgentVersion != "test-version" {
		t.Fatalf("expected synchronized agent version, got %q", metricsBody.AgentVersion)
	}

	historyRequest := httptest.NewRequest(http.MethodGet, "/api/history?range=1h", nil)
	historyRequest.AddCookie(cookies[0])
	historyResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(historyResponse, historyRequest)
	if historyResponse.Code != http.StatusOK || !strings.Contains(historyResponse.Body.String(), `"points":[{`) {
		t.Fatalf("expected persisted history point, got %d: %s", historyResponse.Code, historyResponse.Body.String())
	}

	invalidHistory := httptest.NewRequest(http.MethodGet, "/api/history?range=week", nil)
	invalidHistory.AddCookie(cookies[0])
	invalidHistoryResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(invalidHistoryResponse, invalidHistory)
	if invalidHistoryResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid history range should be rejected, got %d", invalidHistoryResponse.Code)
	}
}

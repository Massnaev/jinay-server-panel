package api

import (
	"bytes"
	"context"
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
	"github.com/Massnaev/jinay-server-panel/agent/internal/powercontrol"
)

type fakePowerController struct {
	available bool
	result    powercontrol.Result
	err       error
	calls     []string
}

func (f *fakePowerController) Available() bool { return f.available }
func (f *fakePowerController) Apply(_ context.Context, profile string) (powercontrol.Result, error) {
	f.calls = append(f.calls, profile)
	return f.result, f.err
}

func loginAs(t *testing.T, server *Server, username, password string) (*http.Cookie, string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("login failed: %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return response.Result().Cookies()[0], payload.CSRFToken
}

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
	for _, test := range []struct{ method, path string }{{http.MethodGet, "/api/metrics"}, {http.MethodGet, "/api/history?range=1h"}, {http.MethodPost, "/api/power/profile"}} {
		request := httptest.NewRequest(test.method, test.path, nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s, got %d", test.path, response.Code)
		}
	}
}

func TestPowerProfileRequiresCSRFAndAdminRole(t *testing.T) {
	server := testServer(t)
	if err := server.users.Add("operator", "operator", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	controller := &fakePowerController{available: true}
	server.config.EnablePowerActions = true
	server.power = controller

	adminCookie, _ := loginAs(t, server, "admin", "correct horse battery staple")
	request := httptest.NewRequest(http.MethodPost, "/api/power/profile", strings.NewReader(`{"profile":"eco"}`))
	request.AddCookie(adminCookie)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF should be forbidden, got %d", response.Code)
	}

	operatorCookie, operatorCSRF := loginAs(t, server, "operator", "correct horse battery staple")
	request = httptest.NewRequest(http.MethodPost, "/api/power/profile", strings.NewReader(`{"profile":"eco"}`))
	request.AddCookie(operatorCookie)
	request.Header.Set("X-CSRF-Token", operatorCSRF)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || len(controller.calls) != 0 {
		t.Fatalf("operator must not control power: status=%d calls=%v", response.Code, controller.calls)
	}
}

func TestPowerProfileValidationSuccessAndAudit(t *testing.T) {
	server := testServer(t)
	controller := &fakePowerController{available: true, result: powercontrol.Result{Profile: "eco", Governor: "schedutil", MaximumFrequencyMHz: 2340, TurboAllowed: false, PoliciesChanged: 32}}
	server.config.EnablePowerActions = true
	server.power = controller
	cookie, csrf := loginAs(t, server, "admin", "correct horse battery staple")

	request := httptest.NewRequest(http.MethodPost, "/api/power/profile", strings.NewReader(`{"profile":"custom"}`))
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || len(controller.calls) != 0 {
		t.Fatalf("invalid profile must be rejected before helper: status=%d calls=%v", response.Code, controller.calls)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/power/profile", strings.NewReader(`{"profile":"eco"}`))
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(controller.calls) != 1 || controller.calls[0] != "eco" {
		t.Fatalf("expected applied eco profile: status=%d calls=%v body=%s", response.Code, controller.calls, response.Body.String())
	}
	entries, err := server.audit.Tail(10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.Action == "power.profile" && entry.Target == "eco" && entry.Result == "allowed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("successful power action was not audited: %+v", entries)
	}

	server.mu.Lock()
	stale := server.sessions[cookie.Value]
	stale.CreatedAt = time.Now().Add(-16 * time.Minute)
	server.sessions[cookie.Value] = stale
	server.mu.Unlock()
	request = httptest.NewRequest(http.MethodPost, "/api/power/profile", strings.NewReader(`{"profile":"balanced"}`))
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusPreconditionRequired || len(controller.calls) != 1 {
		t.Fatalf("stale session must reauthenticate: status=%d calls=%v", response.Code, controller.calls)
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

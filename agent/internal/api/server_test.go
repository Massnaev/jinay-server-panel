package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/serverpanel/agent/internal/audit"
	"github.com/OWNER/serverpanel/agent/internal/auth"
	"github.com/OWNER/serverpanel/agent/internal/config"
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
	return New(config.Config{SecureCookies: false, DataDir: directory, SessionTTL: time.Hour}, users, audit.New(filepath.Join(directory, "audit.jsonl")))
}

func TestProtectedRouteNeedsAuthentication(t *testing.T) {
	server := testServer(t)
	request := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
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
}

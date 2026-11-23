package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIAdminsLifecycle(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, "admin")

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/admins", `{"name":"bob","role":"viewer"}`, tok, csrf))
	if rr.Code != http.StatusCreated {
		t.Fatalf("создание: %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		User     adminUser `json:"user"`
		Password string    `json:"password"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil || created.Password == "" {
		t.Fatalf("ответ: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/admins", "", tok, ""))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "bob") {
		t.Fatalf("список: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PATCH", "/api/admins/bob", `{"role":"admin"}`, tok, csrf))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"role":"admin"`) {
		t.Fatalf("роль: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/admins/bob/password", `{}`, tok, csrf))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "password") {
		t.Fatalf("пароль: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE", "/api/admins/bob", "", tok, csrf))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("удаление: %d %s", rr.Code, rr.Body.String())
	}
}

func TestAPILastAdminGuard(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, "admin")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PATCH", "/api/admins/user-admin", `{"role":"viewer"}`, tok, csrf))
	if rr.Code != http.StatusConflict {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
}

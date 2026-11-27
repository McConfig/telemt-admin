package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestServicePathAllowed(t *testing.T) {
	cases := []struct {
		scope string
		path  string
		want  bool
	}{
		{"read", "/api/users", true},
		{"read", "/api/users/main", true},
		{"read", "/api/summary", true},
		{"read", "/api/settings", false},
		{"read", "/api/account/sessions", false},
		{"read", "/api/services", false},
		{"write", "/api/users/import", true},
		{"write", "/api/settings", false},
		{"write", "/api/admins", false},
		{"write", "/api/services", false},
		{"admin", "/api/settings", true},
		{"admin", "/api/admins", false},
		{"admin", "/api/audit", true},
		{"admin", "/api/audit.csv", true},
		{"admin", "/api/diagnostics", true},
		{"admin", "/api/account/sessions", false},
		{"admin", "/api/services", false},
	}
	for _, tc := range cases {
		if got := servicePathAllowed(tc.scope, tc.path); got != tc.want {
			t.Fatalf("%s %s: %v, ожидалось %v", tc.scope, tc.path, got, tc.want)
		}
	}
}

func seedServiceToken(t *testing.T, s *Server, scope store.ServiceScope) string {
	t.Helper()
	ctx := context.Background()
	name := "svc-" + string(scope)
	if err := s.store.CreateService(ctx, store.ServiceAccount{
		Name: name, Scope: scope, CreatedBy: "admin",
	}); err != nil {
		t.Fatal(err)
	}
	full, _, err := s.store.CreateServiceToken(ctx, name, "", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	return full
}

func bearerRequest(method, path, token, body string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestBearerScopes(t *testing.T) {
	s, _, _ := coverageServer(t)
	readTok := seedServiceToken(t, s, store.ScopeRead)

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodGet, "/api/users", readTok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("read users: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodGet, "/api/settings", readTok, ""))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("read settings: %d", rr.Code)
	}

	writeTok := seedServiceToken(t, s, store.ScopeWrite)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodPost, "/api/users", writeTok,
		`{"name":"from-token"}`))
	if rr.Code != http.StatusCreated {
		t.Fatalf("write create: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodGet, "/api/settings", writeTok, ""))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("write settings: %d", rr.Code)
	}

	adminTok := seedServiceToken(t, s, store.ScopeAdmin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodGet, "/api/settings", adminTok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("admin settings: %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodGet, "/api/account/sessions", adminTok, ""))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("admin account: %d", rr.Code)
	}
}

func TestBearerInvalidAndCookieFallback(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodGet, "/api/users", "tgadm_bad_bad", ""))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("неверный токен: %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, http.MethodGet, "/api/users", "", tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("cookie fallback: %d", rr.Code)
	}
}

func TestBearerWinsOverCookie(t *testing.T) {
	s, sessionTok, _ := coverageServer(t)
	serviceTok := seedServiceToken(t, s, store.ScopeRead)
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: sessionTok})
	req.Header.Set("Authorization", "Bearer "+serviceTok)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("read-токен получил настройки: %d", rr.Code)
	}
}

func TestBearerSkipsCSRF(t *testing.T) {
	s, _, _ := coverageServer(t)
	writeTok := seedServiceToken(t, s, store.ScopeWrite)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodPost, "/api/users", writeTok,
		`{"name":"csrf-free"}`))
	if rr.Code != http.StatusCreated {
		t.Fatalf("Bearer CSRF: %d %s", rr.Code, rr.Body.String())
	}
}

func TestServiceMeAndAuditActor(t *testing.T) {
	s, _, _ := coverageServer(t)
	writeTok := seedServiceToken(t, s, store.ScopeWrite)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodGet, "/api/me", writeTok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("me: %d %s", rr.Code, rr.Body.String())
	}
	var me struct {
		User    struct{ Name, Role string }
		Service bool
		Scope   string
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if !me.Service || me.Scope != "write" || me.User.Name != "service:svc-write" {
		t.Fatalf("me: %+v", me)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodPost, "/api/users", writeTok,
		`{"name":"audited"}`))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d", rr.Code)
	}
	events, _ := s.store.RecentAudit(context.Background(), 5)
	if len(events) == 0 || events[0].Actor != "service:svc-write" {
		t.Fatalf("аудит: %+v", events)
	}
}

func TestBasicAuthFallsBackToCookie(t *testing.T) {
	s, tok, _ := coverageServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Basic + cookie: %d", rr.Code)
	}
}

func TestRoleForScopeSafeDefault(t *testing.T) {
	if roleForScope("broken") != store.RoleViewer {
		t.Fatal("неизвестный scope не должен давать admin")
	}
	if roleForScope(store.ScopeWrite) != store.RoleAdmin || roleForScope(store.ScopeAdmin) != store.RoleAdmin {
		t.Fatal("write/admin должны давать admin")
	}
}

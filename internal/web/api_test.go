package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

const testLink = "tg://proxy?server=proxy.example.com&port=443&secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"

func testServerWithTelemt(t *testing.T) *Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users":
			_, _ = w.Write([]byte(`{"ok":true,"data":[
			 {"username":"main","enabled":true,"in_runtime":true,"current_connections":2,"active_unique_ips":1,"active_unique_ips_list":["192.0.2.10"],"total_octets":2048,"links":{"tls":["` + testLink + `"]}},
			 {"username":"other","enabled":false,"in_runtime":true,"links":{"tls":[]}}
			]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/main":
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"main","enabled":true,"in_runtime":true,"current_connections":2,"active_unique_ips":1,"active_unique_ips_list":["192.0.2.10"],"total_octets":2048,"links":{"tls":["` + testLink + `"]}}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/users/main":
			_, _ = w.Write([]byte(`{"ok":true,"data":{"deleted":true}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/users/other":
			_, _ = w.Write([]byte(`{"ok":true,"data":{"deleted":true}}`))
		default:
			http.Error(w, "no route", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	s := testServer(t)
	s.telemt = telemt.New(srv.URL, "", 5*time.Second)
	return s
}

func TestAPILoginMeLogout(t *testing.T) {
	s := testServer(t)
	hash, _ := auth.Hash("long password 123")
	if err := s.store.CreateUser(context.Background(), store.User{
		Name: "admin", Hash: hash, Role: store.RoleAdmin, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/login", `{"username":"admin","password":"long password 123"}`, "", ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rr.Code, rr.Body.String())
	}
	var me meResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &me); err != nil || me.User.Name != "admin" || me.CSRF == "" {
		t.Fatalf("me: %+v, %v", me, err)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("нет cookie")
	}
	tok := cookies[0].Value

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/me", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("me код %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/logout", "", tok, me.CSRF))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/me", "", tok, ""))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("после выхода: %d", rr.Code)
	}
}

func TestAPIUsersRoleBasedLink(t *testing.T) {
	s := testServerWithTelemt(t)

	for _, tc := range []struct {
		role     store.Role
		wantLink bool
	}{
		{store.RoleAdmin, true},
		{store.RoleViewer, false},
	} {
		tok, csrf := seedSession(t, s, tc.role)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/users", "", tok, ""))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", tc.role, rr.Code, rr.Body.String())
		}
		var users []apiUser
		if err := json.Unmarshal(rr.Body.Bytes(), &users); err != nil {
			t.Fatal(err)
		}
		if len(users) != 2 {
			t.Fatalf("%s: пользователей %d", tc.role, len(users))
		}
		if (users[0].Link != "") != tc.wantLink {
			t.Fatalf("%s: ссылка %q", tc.role, users[0].Link)
		}
		_ = csrf
	}
}

func TestAPIUserStatsAndHistory(t *testing.T) {
	s := testServerWithTelemt(t)
	if err := s.store.RecordIPObservation(context.Background(), "main",
		[]string{"192.0.2.10", "192.0.2.11"}, 3000, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	tok, _ := seedSession(t, s, store.RoleAdmin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/users/main", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var u apiUser
	if err := json.Unmarshal(rr.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}
	if u.Ips24h != 2 || len(u.IPHistory) != 2 {
		t.Fatalf("статистика: %+v", u)
	}
}

func TestAPICSRFRequired(t *testing.T) {
	s := testServerWithTelemt(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/users", `{"name":"x"}`, tok, ""))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("код %d", rr.Code)
	}
}

func testServerWithSingleUser(t *testing.T) *Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/users" {
			_, _ = w.Write([]byte(`{"ok":true,"data":[{"username":"main","enabled":true,"links":{"tls":["` + testLink + `"]}}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	s := testServer(t)
	s.telemt = telemt.New(srv.URL, "", 5*time.Second)
	return s
}

func TestAPILastUserGuard(t *testing.T) {
	s := testServerWithSingleUser(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE", "/api/users/main", `{"confirm":"main"}`, tok, csrf))
	if rr.Code != http.StatusConflict {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAPITelemtDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	s := testServer(t)
	s.telemt = telemt.New(srv.URL, "", 5*time.Second)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/users", "", tok, ""))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("код %d", rr.Code)
	}
	var env errorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil || env.Error.Code != "telemt_unavailable" {
		t.Fatalf("ответ: %s", rr.Body.String())
	}
}

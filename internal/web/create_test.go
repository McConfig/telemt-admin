package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

func testServerWithMutableTelemt(t *testing.T) *Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/users":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ok":true,"data":{"user":{"username":"carol","enabled":true}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/carol":
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"carol","enabled":true,"links":{"tls":["tg://proxy?secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	s := testServer(t)
	s.telemt = telemt.New(srv.URL, "", 5*time.Second)
	return s
}

func adminToken(t *testing.T, s *Server) string {
	t.Helper()
	hash, _ := auth.Hash("long password 123")
	if err := s.store.CreateUser(context.Background(), store.User{
		Name: "admin", Hash: hash, Role: store.RoleAdmin, CreatedAt: time.Now(),
	}); err != nil && !errors.Is(err, store.ErrExists) {
		t.Fatal(err)
	}
	tok, _ := s.store.CreateSession(context.Background(), "admin", "192.0.2.10", "test", time.Hour)
	return tok
}

func csrfFor(t *testing.T, s *Server) string {
	t.Helper()
	sess, err := s.store.GetSession(context.Background(), adminToken(t, s))
	if err != nil {
		t.Fatal(err)
	}
	return sess.CSRF
}

func adminAuth(t *testing.T, s *Server) (token, csrf string) {
	t.Helper()
	tok := adminToken(t, s)
	sess, err := s.store.GetSession(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}
	return tok, sess.CSRF
}

func TestCreateUser(t *testing.T) {
	s := testServerWithMutableTelemt(t)
	tok, csrf := adminAuth(t, s)
	form := url.Values{"csrf": {csrf}, "name": {"carol"}, "max_unique_ips": {"3"}}
	req := httptest.NewRequest("POST", "/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Location") != "/users/carol" {
		t.Fatalf("редирект %q", rr.Header().Get("Location"))
	}
}

func TestCreateUserRejectsBadName(t *testing.T) {
	s := testServerWithMutableTelemt(t)
	tok, csrf := adminAuth(t, s)
	form := url.Values{"csrf": {csrf}, "name": {"плохое имя"}}
	req := httptest.NewRequest("POST", "/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("код %d", rr.Code)
	}
}

func TestCreateUserRejectsBadSecret(t *testing.T) {
	s := testServerWithMutableTelemt(t)
	tok, csrf := adminAuth(t, s)
	form := url.Values{"csrf": {csrf}, "name": {"carol"}, "secret": {"zz"}}
	req := httptest.NewRequest("POST", "/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("код %d", rr.Code)
	}
}

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
)

func seedAdmin(t *testing.T, s *Server) {
	t.Helper()
	hash, err := auth.Hash("long password 123")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.CreateUser(context.Background(), store.User{
		Name: "admin", Hash: hash, Role: store.RoleAdmin, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLoginSuccessAndLogout(t *testing.T) {
	s := testServer(t)
	seedAdmin(t, s)
	form := url.Values{"username": {"admin"}, "password": {"long password 123"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
		t.Fatalf("код входа %d, тело %s", rr.Code, rr.Body.String())
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != cookieName || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("cookie: %+v", cookies)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	s := testServer(t)
	seedAdmin(t, s)
	form := url.Values{"username": {"admin"}, "password": {"wrong password 12"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("код %d", rr.Code)
	}
}

func TestExpiredSessionRedirectsToLogin(t *testing.T) {
	s, mr := testServerWithMR(t)
	seedAdmin(t, s)
	tok, err := s.store.CreateSession(context.Background(), "admin", "192.0.2.10", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mr.FastForward(2 * time.Hour)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Fatalf("код %d, location %q", rr.Code, rr.Header().Get("Location"))
	}
}

func TestLoginWithoutUsersShowsHint(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/login", nil)
	req.RemoteAddr = "192.0.2.10:5555"
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "user add admin") {
		t.Fatalf("нет подсказки: %s", rr.Body.String())
	}
}

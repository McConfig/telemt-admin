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

func testServerWithSingleUser(t *testing.T) *Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/users" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"ok":true,"data":[{"username":"only","enabled":true}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	s := testServer(t)
	s.telemt = telemt.New(srv.URL, "", 5*time.Second)
	return s
}

func viewerAuth(t *testing.T, s *Server) (token, csrf string) {
	t.Helper()
	hash, _ := auth.Hash("long password 123")
	if err := s.store.CreateUser(context.Background(), store.User{
		Name: "viewer", Hash: hash, Role: store.RoleViewer, CreatedAt: time.Now(),
	}); err != nil && !errors.Is(err, store.ErrExists) {
		t.Fatal(err)
	}
	tok, _ := s.store.CreateSession(context.Background(), "viewer", "192.0.2.10", "test", time.Hour)
	sess, err := s.store.GetSession(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}
	return tok, sess.CSRF
}

func TestViewerCannotDisable(t *testing.T) {
	s := testServerWithMutableTelemt(t)
	tok, csrf := viewerAuth(t, s)
	form := url.Values{"csrf": {csrf}}
	req := httptest.NewRequest("POST", "/users/carol/disable", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer получил код %d", rr.Code)
	}
}

func TestBadCSRFRejected(t *testing.T) {
	s := testServerWithMutableTelemt(t)
	tok, _ := adminAuth(t, s)
	form := url.Values{"csrf": {"wrong"}, "confirm": {"carol"}}
	req := httptest.NewRequest("POST", "/users/carol/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("код %d", rr.Code)
	}
}

func TestDeleteRequiresNameConfirmation(t *testing.T) {
	s := testServerWithMutableTelemt(t)
	tok, csrf := adminAuth(t, s)
	form := url.Values{"csrf": {csrf}, "confirm": {"не то"}}
	req := httptest.NewRequest("POST", "/users/carol/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("код %d", rr.Code)
	}
}

func TestDeleteLastUserBlocked(t *testing.T) {
	s := testServerWithSingleUser(t)
	tok, csrf := adminAuth(t, s)
	form := url.Values{"csrf": {csrf}, "confirm": {"only"}}
	req := httptest.NewRequest("POST", "/users/only/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
}

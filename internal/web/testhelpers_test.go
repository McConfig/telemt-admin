package web

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/alicebob/miniredis/v2"
)

func newTestStoreMR(t *testing.T) (*store.Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, mr
}

func testServerWithMR(t *testing.T) (*Server, *miniredis.Miniredis) {
	t.Helper()
	st, mr := newTestStoreMR(t)
	cfg := &config.Config{
		Listen: ":0", LinkMode: "tls", SessionTTL: time.Hour, CookieSecure: true,
		BaseURL:      mustURL("https://tgadmin.example.com"),
		AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
	}
	return New(cfg, st, nil, nil), mr
}

func testServer(t *testing.T) *Server {
	t.Helper()
	s, _ := testServerWithMR(t)
	return s
}

func seedSession(t *testing.T, s *Server, role store.Role) (token, csrf string) {
	t.Helper()
	hash, err := auth.Hash("long password 123")
	if err != nil {
		t.Fatal(err)
	}
	name := "user-" + string(role)
	if err := s.store.CreateUser(context.Background(), store.User{
		Name: name, Hash: hash, Role: role, CreatedAt: time.Now(),
	}); err != nil && err != store.ErrExists {
		t.Fatal(err)
	}
	tok, err := s.store.CreateSession(context.Background(), name, "192.0.2.10", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := s.store.GetSession(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}
	return tok, sess.CSRF
}

func apiRequest(t *testing.T, method, path, body, token, csrfHeader string) *http.Request {
	t.Helper()
	var r *http.Request
	if body == "" {
		r, _ = http.NewRequest(method, path, nil)
	} else {
		r, _ = http.NewRequest(method, path, strings.NewReader(body))
	}
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	}
	if csrfHeader != "" {
		r.Header.Set("X-CSRF-Token", csrfHeader)
	}
	r.RemoteAddr = "192.0.2.10:5555"
	return r
}

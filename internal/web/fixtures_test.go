package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func testServerWithTelemt(t *testing.T) *Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/users":
			_, _ = w.Write([]byte(`{"ok":true,"data":[
			 {"username":"alice","enabled":true,"in_runtime":true,"current_connections":1,"active_unique_ips":1,"total_octets":1024},
			 {"username":"bob","enabled":false,"in_runtime":false}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	s := testServer(t)
	s.telemt = telemt.New(srv.URL, "", 5*time.Second)
	return s
}

func testServerWithBrokenTelemt(t *testing.T) *Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	s := testServer(t)
	s.telemt = telemt.New(srv.URL, "", 5*time.Second)
	return s
}

func authRequest(t *testing.T, s *Server, method, path string, body io.Reader) *http.Request {
	t.Helper()
	tok := seedViewerSession(t, s)
	req := httptest.NewRequest(method, path, body)
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	return req
}

func seedViewerSession(t *testing.T, s *Server) string {
	t.Helper()
	hash, _ := auth.Hash("long password 123")
	if err := s.store.CreateUser(context.Background(), store.User{
		Name: "admin", Hash: hash, Role: store.RoleAdmin, CreatedAt: time.Now(),
	}); err != nil && !errors.Is(err, store.ErrExists) {
		t.Fatal(err)
	}
	tok, err := s.store.CreateSession(context.Background(), "admin", "192.0.2.10", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func authRequestAs(t *testing.T, s *Server, role store.Role, method, path string) *http.Request {
	t.Helper()
	hash, _ := auth.Hash("long password 123")
	name := "user-" + string(role)
	if err := s.store.CreateUser(context.Background(), store.User{
		Name: name, Hash: hash, Role: role, CreatedAt: time.Now(),
	}); err != nil && !errors.Is(err, store.ErrExists) {
		t.Fatal(err)
	}
	tok, _ := s.store.CreateSession(context.Background(), name, "192.0.2.10", "test", time.Hour)
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	return req
}

func testServerWithTelemtDetail(t *testing.T) *Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/users/alice" {
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"alice","enabled":true,"in_runtime":true,
			 "current_connections":2,"active_unique_ips":1,"active_unique_ips_list":["192.0.2.10"],
			 "total_octets":2048,"links":{"classic":[],"secure":[],"tls":["tg://proxy?server=proxy.example.com&port=443&secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"]}}}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	s := testServer(t)
	s.telemt = telemt.New(srv.URL, "", 5*time.Second)
	return s
}

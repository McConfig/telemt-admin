package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	s, _ := testServerWithMR(t)
	return s
}

func TestAllowlist(t *testing.T) {
	s := testServer(t)
	h := s.allowlist(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	inside := httptest.NewRequest("GET", "/", nil)
	inside.RemoteAddr = "192.0.2.10:5555"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, inside)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("из LAN код %d", rr.Code)
	}
	outside := httptest.NewRequest("GET", "/", nil)
	outside.RemoteAddr = "198.51.100.10:5555"
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, outside)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("снаружи код %d", rr.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	s := testServer(t)
	h := s.securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("заголовки: %v", rr.Header())
	}
}

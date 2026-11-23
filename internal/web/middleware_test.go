package web

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
)

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

func TestForwardedForFromTrustedProxy(t *testing.T) {
	st, _ := newTestStoreMR(t)
	cfg := &config.Config{
		Listen: ":0", LinkMode: "tls", SessionTTL: time.Hour, CookieSecure: true,
		BaseURL:        mustURL("https://tgadmin.example.com"),
		AllowedCIDRs:   []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
		TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.1/32")},
	}
	s := New(cfg, st, nil, nil)
	h := s.allowlist(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	inside := httptest.NewRequest("GET", "/", nil)
	inside.RemoteAddr = "10.0.0.1:5555"
	inside.Header.Set("X-Forwarded-For", "10.0.0.50")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, inside)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("LAN через прокси: %d", rr.Code)
	}

	outside := httptest.NewRequest("GET", "/", nil)
	outside.RemoteAddr = "10.0.0.1:5555"
	outside.Header.Set("X-Forwarded-For", "203.0.113.7")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, outside)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("WAN через прокси: %d", rr.Code)
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
	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("нет CSP")
	}
}

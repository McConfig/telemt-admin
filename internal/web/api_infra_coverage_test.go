package web

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

func TestCSRFRejectsForeignOrigin(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	req := apiRequest(t, "POST", "/api/logout", "", tok, csrf)
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("чужой Origin: %d %s", rr.Code, rr.Body.String())
	}
}

func TestCSRFWithoutSession(t *testing.T) {
	s := testServer(t)
	h := s.apiCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/users", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("без сессии: %d %s", rr.Code, rr.Body.String())
	}
}

func TestIsAdminWithoutSession(t *testing.T) {
	s := testServer(t)
	if s.isAdmin(httptest.NewRequest("GET", "/", nil)) {
		t.Fatal("без сессии isAdmin должен быть false")
	}
}

func TestTelemtAPIErrorMapping(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/users", nil)
	cases := []struct {
		err    *telemt.APIError
		status int
	}{
		{&telemt.APIError{Status: http.StatusBadRequest, Message: "плохо"}, http.StatusBadRequest},
		{&telemt.APIError{Status: http.StatusConflict, Message: "конфликт"}, http.StatusConflict},
		{&telemt.APIError{Status: http.StatusInternalServerError, Message: "сбой"}, http.StatusBadGateway},
	}
	for _, c := range cases {
		rr := httptest.NewRecorder()
		s.telemtAPIError(rr, req, c.err)
		if rr.Code != c.status {
			t.Errorf("статус %d: получен %d %s", c.err.Status, rr.Code, rr.Body.String())
		}
	}
}

func TestReadyzStoreUnavailable(t *testing.T) {
	s, mr := testServerWithMR(t)
	mr.SetError("valkey недоступен")
	rr := httptest.NewRecorder()
	s.readyz(rr, httptest.NewRequest("GET", "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz: %d %s", rr.Code, rr.Body.String())
	}
}

func TestClientAddrBranches(t *testing.T) {
	st, _ := newTestStoreMR(t)
	cfg := &config.Config{
		Listen: ":0", LinkMode: "tls", SessionTTL: time.Hour, CookieSecure: true,
		BaseURL:        mustURL("https://tgadmin.example.com"),
		AllowedCIDRs:   []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		TrustedProxies: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/32")},
	}
	s := New(cfg, st, nil, nil)

	plain := httptest.NewRequest("GET", "/", nil)
	plain.RemoteAddr = "192.0.2.1:8080"
	addr, ok := s.clientAddr(plain)
	if !ok || addr.String() != "192.0.2.1" {
		t.Fatalf("без XFF: %v %v", addr, ok)
	}

	bad := httptest.NewRequest("GET", "/", nil)
	bad.RemoteAddr = "192.0.2.1:8080"
	bad.Header.Set("X-Forwarded-For", "не-адрес")
	addr, ok = s.clientAddr(bad)
	if !ok || addr.String() != "192.0.2.1" {
		t.Fatalf("битый XFF: %v %v", addr, ok)
	}

	good := httptest.NewRequest("GET", "/", nil)
	good.RemoteAddr = "192.0.2.1:8080"
	good.Header.Set("X-Forwarded-For", "198.51.100.5")
	addr, ok = s.clientAddr(good)
	if !ok || addr.String() != "198.51.100.5" {
		t.Fatalf("XFF от прокси: %v %v", addr, ok)
	}
}

func TestParseRemoteVariants(t *testing.T) {
	if _, ok := parseRemote(&http.Request{RemoteAddr: "мусор"}); ok {
		t.Fatal("мусор должен отклоняться")
	}
	if addr, ok := parseRemote(&http.Request{RemoteAddr: "192.0.2.10"}); !ok || addr.String() != "192.0.2.10" {
		t.Fatalf("адрес без порта: %v %v", addr, ok)
	}
}

func TestSPAHandlerMethodsAndAssets(t *testing.T) {
	s := testServer(t)

	post := httptest.NewRequest("POST", "/users", nil)
	post.RemoteAddr = "192.0.2.10:5555"
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, post)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("POST на SPA: %d", rr.Code)
	}

	get := httptest.NewRequest("GET", "/assets/index-CSrbO81L.js", nil)
	get.RemoteAddr = "192.0.2.10:5555"
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, get)
	if rr.Code != http.StatusOK {
		t.Fatalf("ассет: %d", rr.Code)
	}
}

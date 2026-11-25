package web

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/collector"
	"net/netip"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/McConfig/telemt-admin/internal/totp"
)

func failingTelemtServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	st, _ := newTestStoreMR(t)
	cfg := &config.Config{
		Listen: ":0", LinkMode: "tls", SessionTTL: time.Hour, CookieSecure: true,
		BaseURL:      mustURL("https://tgadmin.example.com"),
		AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
	}
	s := New(cfg, st, telemt.New("http://127.0.0.1:9", "", time.Second), nil)
	token, csrf := seedSession(t, s, store.RoleAdmin)
	return s, token, csrf
}

func TestAPITelemtErrors(t *testing.T) {
	s, token, csrf := failingTelemtServer(t)
	cases := []struct{ method, path, body string }{
		{"GET", "/api/users", ""},
		{"GET", "/api/users/main", ""},
		{"POST", "/api/users", `{"name":"new"}`},
		{"PATCH", "/api/users/main", `{"maxUniqueIps":3}`},
		{"POST", "/api/users/main/rotate", `{"confirm":"main"}`},
		{"POST", "/api/users/main/enable", `{}`},
		{"POST", "/api/users/main/disable", `{}`},
		{"POST", "/api/users/main/rename", `{"newName":"main2"}`},
		{"POST", "/api/users/main/reset-quota", `{}`},
		{"GET", "/api/summary", ""},
	}
	for _, c := range cases {
		rr := call(t, s, c.method, c.path, c.body, token, csrf)
		if rr.Code < 500 {
			t.Errorf("%s %s: %d %s", c.method, c.path, rr.Code, rr.Body.String())
		}
	}
}

func TestAPIReadyzAndMetrics(t *testing.T) {
	s, token, csrf := coverageServer(t)
	if rr := call(t, s, "GET", "/readyz", "", "", ""); rr.Code != 200 {
		t.Fatalf("readyz: %d %s", rr.Code, rr.Body.String())
	}
	rr := httptest.NewRecorder()
	WriteHTTPMetrics(rr)
	if rr.Code == 0 {
		t.Fatal("метрики не записаны")
	}
	s.SetCollectorStatus(func() collector.Status {
		return collector.Status{Up: true, LastSuccess: time.Now(), GeoIPLoaded: true, GeoIPBuilt: 1_700_000_000}
	})
	if rr := call(t, s, "GET", "/api/summary", "", token, csrf); rr.Code != 200 {
		t.Fatalf("summary: %d", rr.Code)
	}
}

func TestAPISPA(t *testing.T) {
	s, _, _ := coverageServer(t)
	for _, path := range []string{"/", "/users/main", "/assets/nope.js"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = "192.0.2.10:5555"
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != 200 && rr.Code != 404 {
			t.Errorf("%s: %d", path, rr.Code)
		}
	}
}

func TestAPIAccountActions(t *testing.T) {
	s, token, csrf := coverageServer(t)
	other, _ := s.store.CreateSession(context.Background(), "user-admin", "192.0.2.9", "other", time.Hour)
	rr := call(t, s, "GET", "/api/account/sessions", "", token, csrf)
	if rr.Code != 200 {
		t.Fatalf("sessions: %d", rr.Code)
	}
	var sessions []sessionItem
	_ = json.Unmarshal(rr.Body.Bytes(), &sessions)
	for _, item := range sessions {
		if !item.Current {
			if rr := call(t, s, "POST", "/api/account/sessions/"+item.ID+"/revoke", "{}", token, csrf); rr.Code != 204 {
				t.Errorf("revoke: %d %s", rr.Code, rr.Body.String())
			}
		}
	}
	_ = s.store.DeleteSession(context.Background(), other)

	body := `{"current":"wrong","new":"Another-long-password-1"}`
	if rr := call(t, s, "POST", "/api/account/password", body, token, csrf); rr.Code != 403 {
		t.Errorf("неверный пароль: %d", rr.Code)
	}
	body = `{"current":"long password 123","new":"Another-long-password-1"}`
	if rr := call(t, s, "POST", "/api/account/password", body, token, csrf); rr.Code != 204 {
		t.Errorf("смена пароля: %d %s", rr.Code, rr.Body.String())
	}
}

func TestAPITOTPRecoveryAndStatus(t *testing.T) {
	s, token, csrf := coverageServer(t)
	setup := totpSetup(t, s, token, csrf)
	codes := totpEnable(t, s, token, csrf, setup.Secret)

	rr := call(t, s, "GET", "/api/account/totp", "", token, csrf)
	if rr.Code != 200 || !json.Valid(rr.Body.Bytes()) {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}
	_ = s.store.SetTOTPLastCounter(context.Background(), "user-admin", 0)
	code, _ := totpCodeNow(setup.Secret)
	body, _ := json.Marshal(map[string]string{"password": "long password 123", "code": code})
	rr = call(t, s, "POST", "/api/account/totp/recovery", string(body), token, csrf)
	if rr.Code != 200 {
		t.Fatalf("recovery: %d %s", rr.Code, rr.Body.String())
	}
	var out recoveryBody
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if len(out.RecoveryCodes) != 8 || out.RecoveryCodes[0] == codes[0] {
		t.Fatalf("новые коды: %+v", out)
	}
}

func TestAPIAuditFilters(t *testing.T) {
	s, token, csrf := coverageServer(t)
	_ = s.store.Audit(context.Background(), store.AuditEvent{
		Actor: "admin", Action: "user.create", Target: "main", Result: "ok", IP: "192.0.2.1",
	})
	filters := []string{
		"?action=user.create", "?actor=admin", "?target=main", "?q=192.0.2",
		"?from=2020-01-01&to=2030-01-01", "?limit=1&offset=1",
		"?sort=-at,actor", "?sort=bad",
	}
	for _, f := range filters {
		if rr := call(t, s, "GET", "/api/audit"+f, "", token, csrf); rr.Code != 200 {
			t.Errorf("%s: %d", f, rr.Code)
		}
	}
}

func totpCodeNow(secret string) (string, error) {
	return totp.Code(secret, time.Now())
}

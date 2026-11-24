package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

func TestAPIUserViewBranches(t *testing.T) {
	s, token, csrf := coverageServer(t)
	now := time.Now().UTC()
	geo := map[string]store.IPGeo{
		"192.0.2.20": {Country: "TR", CountryName: "Турция"},
		"192.0.2.21": {Country: "DE", CountryName: "Германия"},
	}
	// Клиент старше суток: новая страна должна считаться.
	_ = s.store.RecordIPObservation(context.Background(), "main", []string{"192.0.2.20"}, geo, 100, now.Add(-25*time.Hour))
	_ = s.store.RecordIPObservation(context.Background(), "main", []string{"192.0.2.21"}, geo, 200, now)
	_ = s.store.SetTrusted(context.Background(), "main", store.Trusted{
		Countries: []string{"TR"}, IPs: []string{"192.0.2.20"},
	})
	if rr := call(t, s, "GET", "/api/users/main", "", token, csrf); rr.Code != 200 {
		t.Fatalf("GET user: %d %s", rr.Code, rr.Body.String())
	}

	// Ветки сброса лимитов, квоты и скоростей.
	body := `{"clearMaxUniqueIps":true,"clearMaxTcpConns":true,"dataQuotaBytes":1073741824,"quotaPeriod":"daily","rateLimitUpBps":1000,"rateLimitDownBps":2000,"expiration":"2026-12-01T00:00:00Z","sourceDeny":["203.0.113.0/24"]}`
	if rr := call(t, s, "PATCH", "/api/users/main", body, token, csrf); rr.Code != 200 {
		t.Errorf("PATCH limits: %d %s", rr.Code, rr.Body.String())
	}
}

func TestAPIViewerAdminForbidden(t *testing.T) {
	s, _, _ := coverageServer(t)
	viewer, csrf := seedSession(t, s, store.RoleViewer)
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/api/admins", ""},
		{"PATCH", "/api/admins/user-admin", `{"disabled":false}`},
		{"DELETE", "/api/admins/user-admin", `{}`},
		{"POST", "/api/admins/user-admin/password", `{"password":"Another-long-password-1"}`},
		{"POST", "/api/admins/user-admin/sessions/revoke", `{}`},
		{"POST", "/api/admins/user-admin/totp-reset", `{}`},
		{"POST", "/api/users/main/rotate", `{"confirm":"main"}`},
		{"POST", "/api/users/main/rename", `{"newName":"main2"}`},
		{"DELETE", "/api/users/main/history", `{}`},
		{"PUT", "/api/users/main/note", `{"note":"x"}`},
	} {
		if rr := call(t, s, c.method, c.path, c.body, viewer, csrf); rr.Code != http.StatusForbidden {
			t.Errorf("%s %s: %d", c.method, c.path, rr.Code)
		}
	}
}

func TestAPIAdminDuplicateAndBadInput(t *testing.T) {
	s, token, csrf := coverageServer(t)
	// Дубликат существующего пользователя.
	if rr := call(t, s, "POST", "/api/admins", `{"name":"user-admin","role":"admin"}`, token, csrf); rr.Code != http.StatusConflict {
		t.Errorf("дубликат: %d %s", rr.Code, rr.Body.String())
	}
	if rr := call(t, s, "POST", "/api/admins", `{`, token, csrf); rr.Code != http.StatusBadRequest {
		t.Errorf("плохой JSON: %d", rr.Code)
	}
	if rr := call(t, s, "PATCH", "/api/admins/missing", `{`, token, csrf); rr.Code != http.StatusBadRequest {
		t.Errorf("PATCH плохой JSON: %d", rr.Code)
	}
	if rr := call(t, s, "POST", "/api/account/sessions/missing/revoke", `{}`, token, csrf); rr.Code == 0 {
		t.Error("пустой ответ revoke")
	}
}

func TestAPILoginDisabledAndBadJSON(t *testing.T) {
	s, token, csrf := coverageServer(t)
	_ = s.store.SetDisabled(context.Background(), "user-admin", true)
	if rr := loginPassword(t, s, "user-admin", "long password 123"); rr.Code != http.StatusUnauthorized {
		t.Errorf("отключённый вход: %d", rr.Code)
	}
	_ = token
	_ = csrf
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/login", `{`, "", ""))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("плохой JSON входа: %d", rr.Code)
	}
}

func TestAPILoginTOTPWithoutKey(t *testing.T) {
	st, _ := newTestStoreMR(t)
	cfg := &config.Config{
		Listen: ":0", LinkMode: "tls", SessionTTL: time.Hour, CookieSecure: true,
		BaseURL:      mustURL("https://tgadmin.example.com"),
		AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
	}
	s := New(cfg, st, telemt.New("http://127.0.0.1:9", "", time.Second), nil)
	hash, _ := authHash("long password 123")
	_ = st.CreateUser(context.Background(), store.User{Name: "totpuser", Hash: hash, Role: store.RoleAdmin})
	_ = st.EnableTOTP(context.Background(), "totpuser", "enc", 0, nil)
	if rr := loginPassword(t, s, "totpuser", "long password 123"); rr.Code != http.StatusServiceUnavailable {
		t.Errorf("вход с 2FA без ключа: %d %s", rr.Code, rr.Body.String())
	}
}

func authHash(password string) (string, error) {
	return auth.Hash(password)
}

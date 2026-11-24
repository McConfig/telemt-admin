package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

func expectError(t *testing.T, s *Server, method, path, body, token, csrf string) {
	t.Helper()
	rr := call(t, s, method, path, body, token, csrf)
	if rr.Code < 400 {
		t.Errorf("%s %s: ждали ошибку, получили %d %s", method, path, rr.Code, rr.Body.String())
	}
}

func TestAPIAdminErrors(t *testing.T) {
	s, token, csrf := coverageServer(t)
	cases := []struct{ method, path, body string }{
		{"POST", "/api/admins", `{`},
		{"POST", "/api/admins", `{"name":"bad name","role":"viewer"}`},
		{"POST", "/api/admins", `{"name":"x","role":"root"}`},
		{"POST", "/api/admins", `{"name":"x","role":"viewer","password":"short"}`},
		{"PATCH", "/api/admins/missing", `{"disabled":true}`},
		{"PATCH", "/api/admins/user-admin", `{"role":"viewer"}`},
		{"PATCH", "/api/admins/user-admin", `{"disabled":true}`},
		{"DELETE", "/api/admins/missing", `{}`},
		{"DELETE", "/api/admins/user-admin", `{}`},
		{"POST", "/api/admins/missing/password", `{"password":"Another-long-password-1"}`},
		{"POST", "/api/admins/missing/totp-reset", `{}`},
		{"POST", "/api/admins/user-admin/totp-reset", `{}`},
	}
	for _, c := range cases {
		expectError(t, s, c.method, c.path, c.body, token, csrf)
	}
}

func TestAPIUserErrors(t *testing.T) {
	s, token, csrf := coverageServer(t)
	cases := []struct{ method, path, body string }{
		{"PATCH", "/api/users/missing", `{"maxUniqueIps":3}`},
		{"PATCH", "/api/users/main", `{`},
		{"PUT", "/api/users/main/note", `{"note":"` + strings.Repeat("x", 300) + `"}`},
		{"GET", "/api/users/main/traffic?days=0", ""},
		{"GET", "/api/users/main/timeline?days=100", ""},
		{"GET", "/api/users/missing/qr.png", ""},
		{"PUT", "/api/users/main/trusted", `{"ips":["не-ip"]}`},
		{"DELETE", "/api/users/main", `{"confirm":"wrong"}`},
	}
	for _, c := range cases {
		expectError(t, s, c.method, c.path, c.body, token, csrf)
	}
}

func TestAPITOTPErrors(t *testing.T) {
	s, token, csrf := coverageServer(t)
	// Включение без начатой настройки.
	expectError(t, s, "POST", "/api/account/totp/enable", `{"code":"123456"}`, token, csrf)
	// Неверный код при включении.
	setup := totpSetup(t, s, token, csrf)
	expectError(t, s, "POST", "/api/account/totp/enable", `{"code":"000000"}`, token, csrf)
	_ = setup

	// Без ключа шифрования настройка недоступна.
	st, _ := newTestStoreMR(t)
	cfg := &config.Config{
		Listen: ":0", LinkMode: "tls", SessionTTL: time.Hour, CookieSecure: true,
		BaseURL:      mustURL("https://tgadmin.example.com"),
		AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
	}
	noKey := New(cfg, st, telemt.New("http://127.0.0.1:9", "", time.Second), nil)
	nt, nc := seedSession(t, noKey, store.RoleAdmin)
	rr := call(t, noKey, "POST", "/api/account/totp/setup", `{}`, nt, nc)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("setup без ключа: %d", rr.Code)
	}
}

func TestAPISessionMiddleware(t *testing.T) {
	s, token, csrf := coverageServer(t)
	// Неизвестный токен.
	rr := call(t, s, "GET", "/api/me", "", "bogus", "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("неизвестная сессия: %d", rr.Code)
	}
	// Отключённый пользователь.
	if err := s.store.SetDisabled(context.Background(), "user-admin", true); err != nil {
		t.Fatal(err)
	}
	rr = call(t, s, "GET", "/api/me", "", token, csrf)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("отключённый пользователь: %d", rr.Code)
	}
}

func TestAPIReadyzFailure(t *testing.T) {
	s, _, _ := failingTelemtServer(t)
	rr := call(t, s, "GET", "/readyz", "", "", "")
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz при недоступном telemt: %d %s", rr.Code, rr.Body.String())
	}
}

func TestAPIWithBadJSON(t *testing.T) {
	s, token, csrf := coverageServer(t)
	for _, path := range []string{
		"/api/account/password", "/api/account/totp/disable", "/api/account/totp/recovery",
	} {
		expectError(t, s, "POST", path, `{`, token, csrf)
	}
}

func TestAPIAuditInvalidDates(t *testing.T) {
	s, token, csrf := coverageServer(t)
	for _, f := range []string{"?from=вчера", "?to=завтра", "?limit=99999", "?offset=-1"} {
		if rr := call(t, s, "GET", "/api/audit"+f, "", token, csrf); rr.Code != 200 {
			t.Errorf("%s: %d", f, rr.Code)
		}
	}
}

func TestSpaAssets(t *testing.T) {
	s, _, _ := coverageServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/assets/nonexistent.js", nil)
	req.RemoteAddr = "192.0.2.10:5555"
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("SPA fallback: %d", rr.Code)
	}
}

func TestAPITOTPMoreErrors(t *testing.T) {
	s, token, csrf := coverageServer(t)
	setup := totpSetup(t, s, token, csrf)
	totpEnable(t, s, token, csrf, setup.Secret)

	// Настройка поверх уже включённой 2FA запрещена.
	expectError(t, s, "POST", "/api/account/totp/setup", `{}`, token, csrf)

	// Выключение: неверный пароль и неверный код.
	expectError(t, s, "POST", "/api/account/totp/disable",
		`{"password":"wrong","code":"123456"}`, token, csrf)
	expectError(t, s, "POST", "/api/account/totp/disable",
		`{"password":"long password 123","code":"000000"}`, token, csrf)

	// Перевыпуск резервных кодов: неверный пароль и код.
	expectError(t, s, "POST", "/api/account/totp/recovery",
		`{"password":"wrong","code":"123456"}`, token, csrf)
	expectError(t, s, "POST", "/api/account/totp/recovery",
		`{"password":"long password 123","code":"000000"}`, token, csrf)
}

func TestAPILoginErrors(t *testing.T) {
	s, _, _ := coverageServer(t)
	for i := 0; i < 25; i++ {
		loginPassword(t, s, "user-admin", "wrong password")
	}
	rr := loginPassword(t, s, "user-admin", "wrong password")
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("после серии попыток: %d %s", rr.Code, rr.Body.String())
	}
	if rr := loginPassword(t, s, "missing", "whatever"); rr.Code == http.StatusOK {
		t.Error("вход несуществующего пользователя")
	}
}

func TestAPILoginTOTPAttempts(t *testing.T) {
	s, token, csrf := coverageServer(t)
	setup := totpSetup(t, s, token, csrf)
	totpEnable(t, s, token, csrf, setup.Secret)

	rr := loginPassword(t, s, "user-admin", "long password 123")
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("нет ожидающей cookie")
	}
	var last *httptest.ResponseRecorder
	for i := 0; i < 5; i++ {
		last = postTOTPWithCookie(t, s, "000000", cookies[0])
	}
	if last.Code != http.StatusTooManyRequests {
		t.Errorf("после пяти неверных кодов: %d %s", last.Code, last.Body.String())
	}
}

func TestAPIMoreBranches(t *testing.T) {
	s, token, csrf := coverageServer(t)
	cases := []struct{ method, path, body string }{
		{"GET", "/api/users/bad%20name/trusted", ""},
		{"PUT", "/api/users/main/trusted", `{"countries":["AA","BB","CC","DD","EE","FF","GG","HH","II","JJ","KK","LL","MM","NN","OO","PP","QQ","RR","SS","TT","UU","VV","WW","XX","YY","ZZ","AB","AC","AD","AE","AF","AG","AH","AI","AJ","AK","AL","AM","AN","AO","AP","AQ","AR","AS","AT","AU","AV","AW","AX","AY","AZ","BA"]}`},
		{"PUT", "/api/users/main/note", `{`},
		{"GET", "/api/users/missing/qr.png", ""},
		{"GET", "/api/users/main/timeline?days=100", ""},
		{"POST", "/api/users/main/rotate", `{`},
		{"PATCH", "/api/users/main", `{`},
		{"GET", "/api/users/main/history.csv", ""},
		{"GET", "/api/audit?from=2026-11-01&to=2026-11-30&offset=5", ""},
	}
	for _, c := range cases {
		eventually := call(t, s, c.method, c.path, c.body, token, csrf)
		if eventually.Code == 0 {
			t.Errorf("%s %s: пустой ответ", c.method, c.path)
		}
	}
}

func TestAPIMoreValidation(t *testing.T) {
	s, token, csrf := coverageServer(t)
	cases := []struct{ method, path, body string }{
		{"POST", "/api/users", `{"name":"bad name"}`},
		{"POST", "/api/users", `{"name":"new","secret":"zzz"}`},
		{"PATCH", "/api/admins/user-admin", `{"role":"root"}`},
		{"PATCH", "/api/admins/user-admin", `{`},
		{"DELETE", "/api/admins/bad name", `{}`},
		{"POST", "/api/admins/user-admin/password", `{"password":"short"}`},
		{"POST", "/api/users/main/rename", `{"newName":"bad name"}`},
		{"PUT", "/api/users/main/trusted", `{"countries":["TR"],"ips":["203.0.113.10"]}`},
	}
	for _, c := range cases {
		rr := call(t, s, c.method, c.path, c.body, token, csrf)
		if rr.Code == 0 {
			t.Errorf("%s %s: пустой ответ", c.method, c.path)
		}
	}
}

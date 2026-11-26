package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/totp"
)

func TestLoginTOTPRequestErrors(t *testing.T) {
	s := testServer(t)
	if rr := call(t, s, "POST", "/api/login/totp", `{`, "", ""); rr.Code != http.StatusBadRequest {
		t.Fatalf("битый JSON: %d %s", rr.Code, rr.Body.String())
	}
	if rr := call(t, s, "POST", "/api/login/totp", `{"code":"123456"}`, "", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("без cookie: %d %s", rr.Code, rr.Body.String())
	}
}

func TestLoginTOTPDisabledUser(t *testing.T) {
	s := testServer(t)
	ctx := context.Background()
	token, csrf := seedSession(t, s, store.RoleAdmin)
	setup := totpSetup(t, s, token, csrf)
	totpEnable(t, s, token, csrf, setup.Secret)

	rr := loginPassword(t, s, "user-admin", "long password 123")
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("нет ожидающей cookie")
	}
	if err := s.store.SetDisabled(ctx, "user-admin", true); err != nil {
		t.Fatal(err)
	}
	rr = postTOTPWithCookie(t, s, "123456", cookies[0])
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("отключённый пользователь: %d %s", rr.Code, rr.Body.String())
	}
}

func TestLoginTOTPTooManyAttempts(t *testing.T) {
	s := testServer(t)
	ctx := context.Background()
	token, csrf := seedSession(t, s, store.RoleAdmin)
	setup := totpSetup(t, s, token, csrf)
	totpEnable(t, s, token, csrf, setup.Secret)

	rr := loginPassword(t, s, "user-admin", "long password 123")
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("нет ожидающей cookie")
	}
	for i := 0; i < 10; i++ {
		if err := s.store.RecordLoginFailure(ctx, "192.0.2.10", "user-admin"); err != nil {
			t.Fatal(err)
		}
	}
	rr = postTOTPWithCookie(t, s, "123456", cookies[0])
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("слишком много попыток: %d %s", rr.Code, rr.Body.String())
	}
}

func TestCheckTOTPCodeBranches(t *testing.T) {
	s := testServer(t)
	ctx := context.Background()

	broken := &store.User{Name: "broken", TOTPEnabled: true, TOTPSecret: "не-шифротекст"}
	if ok, _, recovery := s.checkTOTPCode(ctx, broken, "123456"); ok || recovery {
		t.Fatal("битый секрет не должен приниматься")
	}
	if ok, _, _ := s.checkTOTPCode(ctx, broken, ""); ok {
		t.Fatal("пустой код не должен приниматься")
	}
	if isDigits("12a4") {
		t.Fatal("буквы не должны считаться цифрами")
	}
	if !isDigits("123456") {
		t.Fatal("цифры должны распознаваться")
	}
}

func TestTOTPEnableBadRequests(t *testing.T) {
	s := testServer(t)
	token, csrf := seedSession(t, s, store.RoleAdmin)

	// Без начатой настройки включение отклоняется.
	if rr := call(t, s, "POST", "/api/account/totp/enable", `{"code":"123456"}`, token, csrf); rr.Code != http.StatusBadRequest {
		t.Fatalf("без настройки: %d", rr.Code)
	}
	totpSetup(t, s, token, csrf)
	if rr := call(t, s, "POST", "/api/account/totp/enable", `{`, token, csrf); rr.Code != http.StatusBadRequest {
		t.Fatalf("битый JSON: %d %s", rr.Code, rr.Body.String())
	}
}

func TestTOTPEnableCorruptedPending(t *testing.T) {
	s := testServer(t)
	ctx := context.Background()
	token, csrf := seedSession(t, s, store.RoleAdmin)
	if err := s.store.SetTOTPPending(ctx, "user-admin", "испорченный секрет"); err != nil {
		t.Fatal(err)
	}
	rr := call(t, s, "POST", "/api/account/totp/enable", `{"code":"123456"}`, token, csrf)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("битый pending: %d %s", rr.Code, rr.Body.String())
	}
}

func TestTOTPManageBlockedByRequire2FA(t *testing.T) {
	s := testServer(t)
	ctx := context.Background()
	token, csrf := seedSession(t, s, store.RoleAdmin)
	setup := totpSetup(t, s, token, csrf)
	totpEnable(t, s, token, csrf, setup.Secret)

	if _, _, err := s.settings.Save(ctx, 0, map[string]json.RawMessage{
		"security.require_2fa": json.RawMessage("true"),
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetTOTPLastCounter(ctx, "user-admin", 0); err != nil {
		t.Fatal(err)
	}
	code, err := totp.Code(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"password": "long password 123", "code": code})

	for _, path := range []string{"/api/account/totp/disable", "/api/account/totp/recovery"} {
		if rr := call(t, s, "POST", path, string(body), token, csrf); rr.Code != http.StatusBadRequest {
			t.Errorf("%s при обязательной 2FA: %d %s", path, rr.Code, rr.Body.String())
		}
	}
}

func TestVerifyCurrentTOTPBrokenSecret(t *testing.T) {
	s := testServer(t)
	u := &store.User{Name: "broken", TOTPEnabled: true, TOTPSecret: "не-шифротекст"}
	if counter, ok := s.verifyCurrentTOTP(u, "123456"); ok || counter != 0 {
		t.Fatalf("битый секрет: counter=%d ok=%v", counter, ok)
	}
}

func TestResetAdminTOTPValidation(t *testing.T) {
	s := testServer(t)
	ctx := context.Background()
	token, csrf := seedSession(t, s, store.RoleAdmin)

	if rr := call(t, s, "POST", "/api/admins/bad%20name/totp-reset", `{}`, token, csrf); rr.Code != http.StatusBadRequest {
		t.Fatalf("недопустимое имя: %d %s", rr.Code, rr.Body.String())
	}

	hash, err := auth.Hash("long password 123")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.CreateUser(ctx, store.User{Name: "plain", Hash: hash, Role: store.RoleViewer}); err != nil {
		t.Fatal(err)
	}
	if rr := call(t, s, "POST", "/api/admins/plain/totp-reset", `{}`, token, csrf); rr.Code != http.StatusBadRequest {
		t.Fatalf("без 2FA: %d %s", rr.Code, rr.Body.String())
	}
}

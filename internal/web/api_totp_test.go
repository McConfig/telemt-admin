package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/totp"
)

type totpSetupBody struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
	QR     string `json:"qr"`
}

type recoveryBody struct {
	RecoveryCodes []string `json:"recoveryCodes"`
}

func totpSetup(t *testing.T, s *Server, token, csrf string) totpSetupBody {
	t.Helper()
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/account/totp/setup", "{}", token, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("setup код %d: %s", rr.Code, rr.Body.String())
	}
	var out totpSetupBody
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Secret == "" || out.URL == "" || !strings.HasPrefix(out.QR, "data:image/png;base64,") {
		t.Fatalf("setup ответ: %+v", out)
	}
	return out
}

func totpEnable(t *testing.T, s *Server, token, csrf, secret string) []string {
	t.Helper()
	code, _ := totp.Code(secret, time.Now().Add(-30*time.Second))
	body, _ := json.Marshal(map[string]string{"code": code})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/account/totp/enable", string(body), token, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("enable код %d: %s", rr.Code, rr.Body.String())
	}
	var out recoveryBody
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.RecoveryCodes) != 8 {
		t.Fatalf("резервные коды: %+v", out)
	}
	return out.RecoveryCodes
}

func loginPassword(t *testing.T, s *Server, user, password string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": user, "password": password})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/login", string(body), "", ""))
	return rr
}

func postTOTPWithCookie(t *testing.T, s *Server, code string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"code": code})
	rr := httptest.NewRecorder()
	req := apiRequest(t, "POST", "/api/login/totp", string(body), "", "")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	s.Handler().ServeHTTP(rr, req)
	return rr
}

func TestTOTPEnableAndLogin(t *testing.T) {
	s := testServer(t)
	token, csrf := seedSession(t, s, store.RoleAdmin)
	setup := totpSetup(t, s, token, csrf)
	codes := totpEnable(t, s, token, csrf, setup.Secret)

	u, err := s.store.GetUser(t.Context(), "user-admin")
	if err != nil || !u.TOTPEnabled || len(u.RecoveryCodes) != 8 {
		t.Fatalf("пользователь после включения: %+v %v", u, err)
	}

	rr := loginPassword(t, s, "user-admin", "long password 123")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"totpRequired":true`) {
		t.Fatalf("вход по паролю: %d %s", rr.Code, rr.Body.String())
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("нет cookie ожидающей сессии")
	}

	code, _ := totp.Code(setup.Secret, time.Now().Add(30*time.Second))
	rr = postTOTPWithCookie(t, s, code, cookies[0])
	if rr.Code != http.StatusOK {
		t.Fatalf("вход по коду: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"name":"user-admin"`) {
		t.Fatalf("ответ входа: %s", rr.Body.String())
	}

	// Повторное использование того же шага не проходит.
	rr = loginPassword(t, s, "user-admin", "long password 123")
	rr = postTOTPWithCookie(t, s, code, rr.Result().Cookies()[0])
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("повтор кода: %d %s", rr.Code, rr.Body.String())
	}

	// Резервный код работает один раз.
	rr = loginPassword(t, s, "user-admin", "long password 123")
	rr = postTOTPWithCookie(t, s, codes[0], rr.Result().Cookies()[0])
	if rr.Code != http.StatusOK {
		t.Fatalf("вход по резервному коду: %d %s", rr.Code, rr.Body.String())
	}
	rr = loginPassword(t, s, "user-admin", "long password 123")
	rr = postTOTPWithCookie(t, s, codes[0], rr.Result().Cookies()[0])
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("повтор резервного кода: %d %s", rr.Code, rr.Body.String())
	}
}

func TestTOTPWrongCodeAndDisable(t *testing.T) {
	s := testServer(t)
	token, csrf := seedSession(t, s, store.RoleAdmin)
	setup := totpSetup(t, s, token, csrf)
	totpEnable(t, s, token, csrf, setup.Secret)

	rr := loginPassword(t, s, "user-admin", "long password 123")
	rr = postTOTPWithCookie(t, s, "000000", rr.Result().Cookies()[0])
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("неверный код: %d %s", rr.Code, rr.Body.String())
	}

	_ = s.store.SetTOTPLastCounter(t.Context(), "user-admin", 0)
	code, _ := totp.Code(setup.Secret, time.Now())
	body, _ := json.Marshal(map[string]string{"password": "long password 123", "code": code})
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/account/totp/disable", string(body), token, csrf))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("отключение: %d %s", rr.Code, rr.Body.String())
	}
	u, _ := s.store.GetUser(t.Context(), "user-admin")
	if u.TOTPEnabled || u.TOTPSecret != "" {
		t.Fatalf("2FA не выключена: %+v", u)
	}
}

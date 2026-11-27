package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
	wa "github.com/McConfig/telemt-admin/internal/webauthn"
	"github.com/go-webauthn/webauthn/protocol"
	lib "github.com/go-webauthn/webauthn/webauthn"
)

type fakeWA struct{}

func (fakeWA) BeginRegistration(lib.User) (*protocol.CredentialCreation, *lib.SessionData, error) {
	return &protocol.CredentialCreation{}, &lib.SessionData{Challenge: "c"}, nil
}
func (fakeWA) FinishRegistration(lib.User, lib.SessionData, *http.Request) (*lib.Credential, error) {
	return &lib.Credential{ID: []byte{1, 2, 3}}, nil
}
func (fakeWA) BeginLogin(lib.User) (*protocol.CredentialAssertion, *lib.SessionData, error) {
	return &protocol.CredentialAssertion{}, &lib.SessionData{Challenge: "c"}, nil
}
func (fakeWA) FinishLogin(lib.User, lib.SessionData, *http.Request) (*lib.Credential, error) {
	return &lib.Credential{ID: []byte{1, 2, 3}}, nil
}

func TestPasskeyAPI(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	s.passkeys = wa.NewWithProtocol(s.store, fakeWA{})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/account/webauthn/register/begin",
		`{"password":"long password 123","name":"MacBook"}`, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("begin: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/account/webauthn/register/finish", `{}`, tok, csrf))
	if rr.Code != http.StatusCreated {
		t.Fatalf("finish: %d %s", rr.Code, rr.Body.String())
	}
	var created passkeyView
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	if created.ID == "" || created.Name != "MacBook" {
		t.Fatalf("created: %+v", created)
	}
	_ = s.store.UpdatePasskeyUsage(context.Background(), created.ID, 2, time.Now().UTC())
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/account/webauthn", "", tok, ""))
	if rr.Code != http.StatusOK || !json.Valid(rr.Body.Bytes()) {
		t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE", "/api/account/webauthn/"+created.ID,
		`{"password":"long password 123"}`, tok, csrf))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body.String())
	}
}

func TestPasskeyLoginFlow(t *testing.T) {
	s, _, _ := coverageServer(t)
	s.passkeys = wa.NewWithProtocol(s.store, fakeWA{})
	if err := s.store.CreatePasskey(context.Background(), store.Passkey{
		ID: "0123456789abcdef", User: "user-admin", Name: "MacBook",
		Credential: lib.Credential{ID: []byte{1, 2, 3}}, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/login",
		`{"username":"user-admin","password":"long password 123"}`, "", ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"mfaRequired":true`) || !strings.Contains(rr.Body.String(), `"passkey":true`) {
		t.Fatalf("ответ: %s", rr.Body.String())
	}
	cookie := rr.Result().Cookies()[0]

	req := apiRequest(t, "POST", "/api/login/webauthn/begin", `{}`, "", "")
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("begin: %d %s", rr.Code, rr.Body.String())
	}
	req = apiRequest(t, "POST", "/api/login/webauthn/finish", `{}`, "", "")
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("finish: %d %s", rr.Code, rr.Body.String())
	}
}

func TestPasskeyAPIErrorPaths(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	s.passkeys = wa.NewWithProtocol(s.store, fakeWA{})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/account/webauthn/register/begin",
		`{"password":"wrong","name":"x"}`, tok, csrf))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("wrong password: %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/account/webauthn/register/finish", `{}`, tok, csrf))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("no challenge: %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE", "/api/account/webauthn/unknown",
		`{"password":"long password 123"}`, tok, csrf))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown passkey: %d", rr.Code)
	}

	serviceToken := seedServiceToken(t, s, store.ScopeAdmin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodPost, "/api/account/webauthn/register/begin",
		serviceToken, `{"password":"x","name":"x"}`))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("service token: %d", rr.Code)
	}

	s.passkeys = nil
	for _, path := range []string{
		"/api/account/webauthn/register/begin",
		"/api/account/webauthn/register/finish",
	} {
		rr = httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, apiRequest(t, "POST", path, `{}`, tok, csrf))
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: %d", path, rr.Code)
		}
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/login/webauthn/begin", `{}`, "", ""))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("login begin unavailable: %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/login/webauthn/finish", `{}`, "", ""))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("login finish unavailable: %d", rr.Code)
	}
}

func TestPasskeyLoginWithoutPending(t *testing.T) {
	s, _, _ := coverageServer(t)
	s.passkeys = wa.NewWithProtocol(s.store, fakeWA{})
	for _, path := range []string{"/api/login/webauthn/begin", "/api/login/webauthn/finish"} {
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, apiRequest(t, "POST", path, `{}`, "", ""))
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("%s: %d", path, rr.Code)
		}
	}
}

type fakeWAErr struct{}

func (fakeWAErr) BeginRegistration(lib.User) (*protocol.CredentialCreation, *lib.SessionData, error) {
	return nil, nil, errors.New("fail")
}
func (fakeWAErr) FinishRegistration(lib.User, lib.SessionData, *http.Request) (*lib.Credential, error) {
	return nil, errors.New("fail")
}
func (fakeWAErr) BeginLogin(lib.User) (*protocol.CredentialAssertion, *lib.SessionData, error) {
	return nil, nil, errors.New("fail")
}
func (fakeWAErr) FinishLogin(lib.User, lib.SessionData, *http.Request) (*lib.Credential, error) {
	return nil, errors.New("fail")
}

func TestPasskeyMoreErrors(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	s.passkeys = wa.NewWithProtocol(s.store, fakeWA{})
	if err := s.store.CreatePasskey(context.Background(), store.Passkey{
		ID: "0123456789abcdef", User: "user-admin", Name: "MacBook",
		Credential: lib.Credential{ID: []byte{1, 2, 3}},
	}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE", "/api/account/webauthn/0123456789abcdef",
		`{"password":"wrong"}`, tok, csrf))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("wrong delete password: %d", rr.Code)
	}

	pending, err := s.store.CreatePendingTOTP(context.Background(), "nopass", "192.0.2.10", "test", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req := apiRequest(t, "POST", "/api/login/webauthn/begin", `{}`, "", "")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: pending})
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("begin without passkeys: %d", rr.Code)
	}

	pending2, _ := s.store.CreatePendingTOTP(context.Background(), "nopass", "192.0.2.10", "test", time.Minute)
	req = apiRequest(t, "POST", "/api/login/webauthn/finish", `{}`, "", "")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: pending2})
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("finish without challenge: %d", rr.Code)
	}
	s.passkeys = wa.NewWithProtocol(s.store, fakeWAErr{})
	req = apiRequest(t, "POST", "/api/login/webauthn/finish", `{}`, "", "")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: pending})
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("finish with fake error: %d", rr.Code)
	}
}

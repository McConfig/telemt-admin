package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestProxyConfigRequiresSession(t *testing.T) {
	s, _, _ := coverageServer(t)
	rr := call(t, s, "GET", "/api/proxy/config", "", "", "")
	if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusForbidden {
		t.Fatalf("%d", rr.Code)
	}
}

func TestProxySchemaAndConfig(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "GET", "/api/proxy/schema", "", tok, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("schema: %d %s", rr.Code, rr.Body.String())
	}
	rr = call(t, s, "GET", "/api/proxy/config", "", tok, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("config: %d %s", rr.Code, rr.Body.String())
	}
	_ = csrf
}

func TestProxyValidateAndApply(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "POST", "/api/proxy/validate",
		`{"patch":{"censorship":{"tls_domain":"new.example"}}}`, tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("validate: %d %s", rr.Code, rr.Body.String())
	}
	rr = call(t, s, "POST", "/api/proxy/apply",
		`{"patch":{"censorship":{"tls_domain":"new.example"}},"confirm":true}`, tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rr.Code, rr.Body.String())
	}
	rr = call(t, s, "GET", "/api/proxy/reload/7", "", tok, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("reload: %d %s", rr.Code, rr.Body.String())
	}
}

func TestProxyApplyRequiresConfirmForDangerousPath(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "POST", "/api/proxy/apply",
		`{"patch":{"censorship":{"tls_domain":"new.example"}}}`, tok, csrf)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestProxyServiceTokenForbidden(t *testing.T) {
	s, _, _ := coverageServer(t)
	tok := seedServiceToken(t, s, store.ScopeAdmin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest("GET", "/api/proxy/config", tok, ""))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("сервисный токен: %d %s", rr.Code, rr.Body.String())
	}
}

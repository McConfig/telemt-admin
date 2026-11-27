package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
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

func TestProxyValidateRejectsAccess(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "POST", "/api/proxy/validate",
		`{"patch":{"access":{"users":{}}}}`, tok, csrf)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "proxy_invalid") {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestProxyUnavailable(t *testing.T) {
	s, tok, _ := coverageServer(t)
	s.telemt = telemt.New("http://127.0.0.1:1", "", 200*time.Millisecond)
	rr := call(t, s, "GET", "/api/proxy/config", "", tok, "")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestProxyConflictMapped(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/config/validate":
			_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/config":
			_, _ = w.Write([]byte(`{"ok":true,"revision":"rev1","data":{}}`))
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"revision_conflict","message":"stale"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	s.telemt = telemt.New(srv.URL, "", time.Second)
	rr := call(t, s, "POST", "/api/proxy/apply",
		`{"patch":{"censorship":{"tls_domain":"new"}},"confirm":true}`, tok, csrf)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "proxy_conflict") {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestProxySchemaCached(t *testing.T) {
	s, tok, _ := coverageServer(t)
	for i := 0; i < 2; i++ {
		rr := call(t, s, "GET", "/api/proxy/schema", "", tok, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("схема %d: %d %s", i, rr.Code, rr.Body.String())
		}
	}
}

func TestProxyValidateUnavailable(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	s.telemt = telemt.New("http://127.0.0.1:1", "", 200*time.Millisecond)
	rr := call(t, s, "POST", "/api/proxy/validate", `{"patch":{"censorship":{"mask":true}}}`, tok, csrf)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestProxyReloadBadID(t *testing.T) {
	s, tok, _ := coverageServer(t)
	rr := call(t, s, "GET", "/api/proxy/reload/abc", "", tok, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestProxyApplyEmptyPatch(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "POST", "/api/proxy/apply", `{"patch":{},"confirm":true}`, tok, csrf)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestProxyUnavailableReads(t *testing.T) {
	s, tok, _ := coverageServer(t)
	s.telemt = nil
	for _, path := range []string{"/api/proxy/schema", "/api/proxy/config"} {
		rr := call(t, s, "GET", path, "", tok, "")
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: %d %s", path, rr.Code, rr.Body.String())
		}
	}
}

func TestProxyApplyInvalidPatch(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "POST", "/api/proxy/apply",
		`{"patch":{"access":{"users":{}}},"confirm":true}`, tok, csrf)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestProxyValidateEmptyPatch(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "POST", "/api/proxy/validate", `{"patch":{}}`, tok, csrf)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

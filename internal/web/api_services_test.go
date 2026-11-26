package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func mustServiceAPI(t *testing.T, s *Server, tok, csrf, name, scope string) {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"description":"test","scope":%q}`, name, scope)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services", body, tok, csrf))
	if rr.Code != http.StatusCreated {
		t.Fatalf("сервис: %d %s", rr.Code, rr.Body.String())
	}
}

func TestServiceAPICRUD(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	mustServiceAPI(t, s, tok, csrf, "monitoring", "read")

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/services", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
	}
	var list []serviceView
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "monitoring" || list[0].Tokens != 0 {
		t.Fatalf("list: %+v", list)
	}

	body := `{"description":"new","scope":"write","disabled":true}`
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PATCH", "/api/services/monitoring", body, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rr.Code, rr.Body.String())
	}
	var svc serviceView
	_ = json.Unmarshal(rr.Body.Bytes(), &svc)
	if svc.Scope != "write" || !svc.Disabled || svc.Description != "new" {
		t.Fatalf("patch: %+v", svc)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE", "/api/services/monitoring",
		`{"confirm":"wrong"}`, tok, csrf))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("confirm: %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE", "/api/services/monitoring",
		`{"confirm":"monitoring"}`, tok, csrf))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body.String())
	}
}

func TestServiceAPIValidationAndConflicts(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	mustServiceAPI(t, s, tok, csrf, "monitoring", "read")

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services",
		`{"name":"monitoring","scope":"read"}`, tok, csrf))
	if rr.Code != http.StatusConflict {
		t.Fatalf("конфликт: %d", rr.Code)
	}
	for name, body := range map[string]string{
		"имя":   `{"name":"Bad Name","scope":"read"}`,
		"права": `{"name":"ok","scope":"root"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services", body, tok, csrf))
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PATCH", "/api/services/monitoring", `{}`, tok, csrf))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("пустой patch: %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/services/missing/tokens", "", tok, ""))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing: %d", rr.Code)
	}
}

func TestIssueTokenOnlyOnceAndRevoke(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	mustServiceAPI(t, s, tok, csrf, "monitoring", "read")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services/monitoring/tokens",
		`{"label":"ci","expiresInDays":0}`, tok, csrf))
	if rr.Code != http.StatusCreated {
		t.Fatalf("issue: %d %s", rr.Code, rr.Body.String())
	}
	var issued issuedTokenView
	if err := json.Unmarshal(rr.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(issued.Token, "tgadm_") || issued.ID == "" || issued.ExpiresAt != "" {
		t.Fatalf("токен: %+v", issued)
	}
	if strings.Contains(rr.Body.String(), "secret_hash") {
		t.Fatal("хеш попал в ответ")
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/services/monitoring/tokens", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("list: %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), issued.Token) || strings.Contains(rr.Body.String(), "secret_hash") {
		t.Fatal("секрет попал в список")
	}
	var tokens []tokenView
	_ = json.Unmarshal(rr.Body.Bytes(), &tokens)
	if len(tokens) != 1 || tokens[0].Prefix != "tgadm_"+issued.ID {
		t.Fatalf("tokens: %+v", tokens)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE",
		"/api/services/monitoring/tokens/"+issued.ID, "", tok, csrf))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE",
		"/api/services/monitoring/tokens/"+issued.ID, "", tok, csrf))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("повторный отзыв: %d", rr.Code)
	}

	events, _ := s.store.RecentAudit(context.Background(), 20)
	for _, e := range events {
		if strings.Contains(e.Result, issued.Token) || strings.Contains(e.Result, "secret_hash") {
			t.Fatalf("секрет в аудите: %+v", e)
		}
	}
}

func TestIssueTokenDefaultsAndExpiry(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	mustServiceAPI(t, s, tok, csrf, "monitoring", "read")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services/monitoring/tokens", `{}`, tok, csrf))
	if rr.Code != http.StatusCreated {
		t.Fatalf("issue: %d %s", rr.Code, rr.Body.String())
	}
	var issued issuedTokenView
	_ = json.Unmarshal(rr.Body.Bytes(), &issued)
	if issued.ExpiresAt == "" {
		t.Fatal("по умолчанию срок 90 дней")
	}
	for _, body := range []string{`{"expiresInDays":-1}`, `{"expiresInDays":3651}`, `{"label":"` + strings.Repeat("x", 65) + `"}`} {
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services/monitoring/tokens", body, tok, csrf))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("валидация %s: %d", body, rr.Code)
		}
	}
}

func TestServiceAPIPermissionsAndLimit(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	mustServiceAPI(t, s, tok, csrf, "monitoring", "read")

	viewer, viewerCSRF := seedSession(t, s, store.RoleViewer)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/services", "", viewer, viewerCSRF))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer: %d", rr.Code)
	}

	adminTok := seedServiceToken(t, s, store.ScopeAdmin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, bearerRequest(http.MethodGet, "/api/services", adminTok, ""))
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), "session_required") {
		t.Fatalf("admin token: %d %s", rr.Code, rr.Body.String())
	}

	for i := 0; i < store.MaxServiceTokens; i++ {
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services/monitoring/tokens", `{}`, tok, csrf))
		if rr.Code != http.StatusCreated {
			t.Fatalf("токен %d: %d", i, rr.Code)
		}
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services/monitoring/tokens", `{}`, tok, csrf))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("лимит: %d %s", rr.Code, rr.Body.String())
	}
}

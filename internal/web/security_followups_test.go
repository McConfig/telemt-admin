package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestViewerCannotReadNotes(t *testing.T) {
	s, _, _ := coverageServer(t)
	seedCoverageData(t, s)
	viewer, vcsrf := seedSession(t, s, store.RoleViewer)

	rr := call(t, s, "GET", "/api/users/main", "", viewer, vcsrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("карточка: %d %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), `"note"`) {
		t.Fatalf("viewer видит заметку: %s", rr.Body.String())
	}

	rr = call(t, s, "GET", "/api/users/main/note", "", viewer, vcsrf)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer note: %d %s", rr.Code, rr.Body.String())
	}

	admin, acsrf := seedSession(t, s, store.RoleAdmin)
	rr = call(t, s, "GET", "/api/users/main", "", admin, acsrf)
	if !strings.Contains(rr.Body.String(), `"note":"телефон"`) {
		t.Fatalf("admin не видит заметку: %s", rr.Body.String())
	}
}

func TestUpdateServiceStoreErrorIs500(t *testing.T) {
	s, mr := testServerWithMR(t)
	if err := s.store.CreateService(context.Background(), store.ServiceAccount{Name: "svc", Scope: store.ScopeRead}); err != nil {
		t.Fatal(err)
	}
	sess, err := s.store.CreateSession(context.Background(), "user-admin", "192.0.2.10", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mr.Close()
	req := httptest.NewRequest(http.MethodPut, "/api/services/svc", strings.NewReader(`{"disabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), principalCtxKey, &principal{
		Name: "user-admin", Kind: "session", Role: store.RoleAdmin, Session: &store.Session{ID: sess, User: "user-admin"},
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "svc")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	s.apiUpdateService(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestRevokeMalformedTokenIDIs404(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "DELETE", "/api/services/svc/tokens/not-a-token-id", ``, tok, csrf)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestAdminBearerAllowedRoutes(t *testing.T) {
	s, _, _ := coverageServer(t)
	token := seedServiceToken(t, s, store.ScopeAdmin)
	paths := []string{
		"/api/settings", "/api/settings/templates", "/api/settings/backups",
		"/api/settings/system", "/api/settings/history", "/api/diagnostics",
		"/api/audit", "/api/users", "/api/users/main", "/api/summary",
	}
	for _, path := range paths {
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, bearerRequest(http.MethodGet, path, token, ""))
		if rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden || rr.Code >= 500 {
			t.Fatalf("%s: %d %s", path, rr.Code, rr.Body.String())
		}
	}
}

func TestInvalidBearerDoesNotFallBackToCookie(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	req := apiRequest(t, "GET", "/api/users", "", tok, csrf)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteServiceCascadeViaAPI(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	ctx := context.Background()
	if err := s.store.CreateService(ctx, store.ServiceAccount{Name: "cascade", Scope: store.ScopeRead}); err != nil {
		t.Fatal(err)
	}
	full, _, err := s.store.CreateServiceToken(ctx, "cascade", "test", "user-admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := call(t, s, "DELETE", "/api/services/cascade", `{"confirm":"cascade"}`, tok, csrf)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body.String())
	}
	if _, err := s.store.AuthenticateServiceToken(ctx, full); err == nil {
		t.Fatal("старый токен всё ещё работает")
	}
	rr = call(t, s, "GET", "/api/services/cascade/tokens", "", tok, "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("tokens: %d %s", rr.Code, rr.Body.String())
	}
}

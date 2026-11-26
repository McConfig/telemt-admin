package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestAdminTokenCannotMintCredentials(t *testing.T) {
	s, _, _ := coverageServer(t)
	token := seedServiceToken(t, s, store.ScopeAdmin)
	cases := []struct{ method, path, body string }{
		{http.MethodPost, "/api/admins", `{"name":"newadmin","role":"admin"}`},
		{http.MethodPost, "/api/admins/user-admin/password", `{}`},
		{http.MethodPost, "/api/admins/user-admin/totp-reset", ``},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, bearerRequest(tc.method, tc.path, token, tc.body))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("%s %s: код %d: %s", tc.method, tc.path, rr.Code, rr.Body.String())
		}
	}
}

func TestAdminTokenCannotManageUIAdmins(t *testing.T) {
	s, _, _ := coverageServer(t)
	token := seedServiceToken(t, s, store.ScopeAdmin)
	cases := []struct{ method, path, body string }{
		{http.MethodGet, "/api/admins", ``},
		{http.MethodPatch, "/api/admins/user-admin", `{"disabled":true}`},
		{http.MethodPatch, "/api/admins/user-admin", `{"role":"admin"}`},
		{http.MethodPost, "/api/admins/user-admin/sessions/revoke", ``},
		{http.MethodDelete, "/api/admins/user-admin", ``},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, bearerRequest(tc.method, tc.path, token, tc.body))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("%s %s: код %d: %s", tc.method, tc.path, rr.Code, rr.Body.String())
		}
	}
}

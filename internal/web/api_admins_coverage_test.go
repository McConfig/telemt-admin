package web

import (
	"net/http"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestAdminDisableEnableFlow(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)

	rr := call(t, s, "POST", "/api/admins",
		`{"name":"op1","role":"admin","password":"long password 123"}`, tok, csrf)
	if rr.Code != http.StatusCreated {
		t.Fatalf("создание: %d %s", rr.Code, rr.Body.String())
	}

	rr = call(t, s, "PATCH", "/api/admins/op1", `{"disabled":true}`, tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("отключение: %d %s", rr.Code, rr.Body.String())
	}
	u, err := s.store.GetUser(t.Context(), "op1")
	if err != nil || !u.Disabled {
		t.Fatalf("после отключения: %+v %v", u, err)
	}

	rr = call(t, s, "PATCH", "/api/admins/op1", `{"disabled":false}`, tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("включение: %d %s", rr.Code, rr.Body.String())
	}
	u, err = s.store.GetUser(t.Context(), "op1")
	if err != nil || u.Disabled {
		t.Fatalf("после включения: %+v %v", u, err)
	}

	// Смена роли другого администратора на просмотрщика.
	rr = call(t, s, "PATCH", "/api/admins/op1", `{"role":"viewer"}`, tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("смена роли: %d %s", rr.Code, rr.Body.String())
	}
	u, err = s.store.GetUser(t.Context(), "op1")
	if err != nil || u.Role != store.RoleViewer {
		t.Fatalf("роль после смены: %+v %v", u, err)
	}
}

func TestAdminPasswordBadJSON(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	if rr := call(t, s, "POST", "/api/admins/user-admin/password", `{`, tok, csrf); rr.Code != http.StatusBadRequest {
		t.Fatalf("битый JSON: %d %s", rr.Code, rr.Body.String())
	}
}

func TestAdminRevokeOwnSessionsRejected(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	rr := call(t, s, "POST", "/api/admins/user-admin/sessions/revoke", `{}`, tok, csrf)
	if rr.Code != http.StatusConflict {
		t.Fatalf("свои сессии: %d %s", rr.Code, rr.Body.String())
	}
}

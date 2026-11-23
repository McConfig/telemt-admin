package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestChangePasswordRejectsShort(t *testing.T) {
	s := testServer(t)
	tok, csrf := adminAuth(t, s)
	form := url.Values{"csrf": {csrf}, "current": {"long password 123"}, "new": {"short"}}
	req := httptest.NewRequest("POST", "/account/password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("код %d", rr.Code)
	}
}

func TestAuditPage(t *testing.T) {
	s := testServer(t)
	_ = s.store.Audit(context.Background(), store.AuditEvent{
		Actor: "admin", Action: "user.create", Target: "alice", Result: "ok",
	})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, authRequest(t, s, "GET", "/audit", nil))
	if rr.Code != http.StatusOK || !contains(rr.Body.String(), "user.create") {
		t.Fatalf("аудит: %d %s", rr.Code, rr.Body.String())
	}
}

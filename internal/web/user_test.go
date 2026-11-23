package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestUserPageAdminSeesLink(t *testing.T) {
	s := testServerWithTelemtDetail(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, authRequest(t, s, "GET", "/users/alice", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d", rr.Code)
	}
	if !contains(rr.Body.String(), "tg://proxy?") {
		t.Fatalf("ссылки нет: %s", rr.Body.String())
	}
}

func TestUserPageViewerHidesLink(t *testing.T) {
	s := testServerWithTelemtDetail(t)
	rr := httptest.NewRecorder()
	req := authRequestAs(t, s, store.RoleViewer, "GET", "/users/alice")
	s.Handler().ServeHTTP(rr, req)
	if contains(rr.Body.String(), "tg://proxy?") {
		t.Fatalf("viewer увидел ссылку: %s", rr.Body.String())
	}
	if !contains(rr.Body.String(), "Ссылка скрыта") {
		t.Fatalf("нет пометки: %s", rr.Body.String())
	}
}

func TestQRCodePNG(t *testing.T) {
	s := testServerWithTelemtDetail(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, authRequest(t, s, "GET", "/users/alice/qr", nil))
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("QR: %d %s", rr.Code, rr.Header().Get("Content-Type"))
	}
	if rr.Body.Len() < 100 {
		t.Fatalf("QR слишком короткий: %d", rr.Body.Len())
	}
}

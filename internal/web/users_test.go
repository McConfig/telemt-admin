package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestUsersPageShowsTelemtUsers(t *testing.T) {
	s := testServerWithTelemt(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, authRequest(t, s, "GET", "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !contains(body, "alice") || !contains(body, "Заблокирован") {
		t.Fatalf("нет ожидаемых данных: %s", body)
	}
}

func TestUsersPageFilter(t *testing.T) {
	s := testServerWithTelemt(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, authRequest(t, s, "GET", "/?q=bob", nil))
	body := rr.Body.String()
	if contains(body, ">alice<") || !contains(body, ">bob<") {
		t.Fatalf("фильтр не работает: %s", body)
	}
}

func TestUsersPageViewerHasNoCreateLink(t *testing.T) {
	s := testServerWithTelemtDetail(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, authRequestAs(t, s, store.RoleViewer, "GET", "/"))
	if contains(rr.Body.String(), "Создать пользователя") {
		t.Fatalf("viewer видит ссылку создания: %s", rr.Body.String())
	}
}

func TestUsersPageNoStore(t *testing.T) {
	s := testServerWithTelemt(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, authRequest(t, s, "GET", "/", nil))
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", rr.Header().Get("Cache-Control"))
	}
}

func TestUsersPageTelemtDown(t *testing.T) {
	s := testServerWithBrokenTelemt(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, authRequest(t, s, "GET", "/", nil))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("код %d", rr.Code)
	}
	if !contains(rr.Body.String(), "telemt не отвечает") {
		t.Fatalf("нет сообщения: %s", rr.Body.String())
	}
}

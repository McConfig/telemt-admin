package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestAuditFilterBranches(t *testing.T) {
	s := testServer(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	ctx := context.Background()
	now := time.Now().UTC()
	events := []store.AuditEvent{
		{At: now.Add(-2 * time.Hour), Actor: "alice", Action: "user.create", Target: "main", Result: "ok", IP: "192.0.2.1"},
		{At: now.Add(-time.Hour), Actor: "bob", Action: "user.delete", Target: "other", Result: "ok", IP: "192.0.2.2"},
	}
	for _, e := range events {
		if err := s.store.Audit(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	filters := []string{
		"?actor=nobody", "?action=no.such", "?target=nobody", "?q=zzzz",
		"?from=2030-01-01", "?to=2000-01-01",
	}
	for _, f := range filters {
		rr := call(t, s, "GET", "/api/audit"+f, "", tok, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", f, rr.Code, rr.Body.String())
		}
		var items []auditItem
		if err := json.Unmarshal(rr.Body.Bytes(), &items); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if len(items) != 0 {
			t.Errorf("%s: ожидалась пустая выборка, получено %+v", f, items)
		}
	}

	// Положительный фильтр с временем в формате RFC3339.
	rrPositive := call(t, s, "GET", "/api/audit?from=2020-01-01T00:00:00Z&to=2030-01-01T00:00:00Z", "", tok, "")
	var positive []auditItem
	if err := json.Unmarshal(rrPositive.Body.Bytes(), &positive); err != nil {
		t.Fatal(err)
	}
	if len(positive) != len(events) {
		t.Fatalf("фильтр по времени: %+v", positive)
	}

	// Срез по лимиту срабатывает при нескольких подходящих событиях.
	rr := call(t, s, "GET", "/api/audit?limit=1", "", tok, "")
	var limited []auditItem
	if err := json.Unmarshal(rr.Body.Bytes(), &limited); err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 {
		t.Fatalf("лимит: %+v", limited)
	}

	// CSV-выгрузка обходит все события.
	rr = call(t, s, "GET", "/api/audit.csv", "", tok, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("csv: %d", rr.Code)
	}
	if lines := countLines(rr.Body.String()); lines < 3 {
		t.Fatalf("в CSV мало строк: %d\n%s", lines, rr.Body.String())
	}
}

func countLines(s string) int {
	n := 0
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}

func TestSortAuditFields(t *testing.T) {
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	events := []store.AuditEvent{
		{At: base.Add(3 * time.Hour), Actor: "carol", Action: "z.action", Target: "t3", Result: "ok", IP: "10.0.0.3"},
		{At: base.Add(time.Hour), Actor: "alice", Action: "a.action", Target: "t1", Result: "bad", IP: "10.0.0.1"},
		{At: base.Add(2 * time.Hour), Actor: "bob", Action: "m.action", Target: "t2", Result: "ok", IP: "10.0.0.2"},
		{At: base, Actor: "bob", Action: "b.action", Target: "t0", Result: "ok", IP: "10.0.0.4"},
	}
	specs := []string{
		"at", "-at", "actor", "-actor", "action", "target", "result",
		"ip", "-ip", "at,,actor", "unknown", ",", "actor,action", "actor,-action",
	}
	for _, spec := range specs {
		cp := append([]store.AuditEvent(nil), events...)
		sortAudit(cp, spec) // проверяем отсутствие паники и корректный размер
		if len(cp) != len(events) {
			t.Fatalf("%q: размер %d", spec, len(cp))
		}
	}

	cp := append([]store.AuditEvent(nil), events...)
	sortAudit(cp, "-at")
	if !cp[0].At.Equal(base.Add(3 * time.Hour)) {
		t.Fatalf("сортировка по времени: %+v", cp[0])
	}
	cp = append([]store.AuditEvent(nil), events...)
	sortAudit(cp, "actor,action")
	if cp[0].Actor != "alice" || cp[1].Actor != "bob" || cp[1].Action != "b.action" {
		t.Fatalf("сортировка по двум полям: %+v", cp)
	}
}

func TestChangePasswordPolicyRejectsShort(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	rr := call(t, s, "POST", "/api/account/password",
		`{"current":"long password 123","new":"short"}`, tok, csrf)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("короткий пароль: %d %s", rr.Code, rr.Body.String())
	}
}

func TestRevokeCurrentSessionRejected(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	rr := call(t, s, "GET", "/api/account/sessions", "", tok, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("сессии: %d", rr.Code)
	}
	var items []sessionItem
	if err := json.Unmarshal(rr.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	current := ""
	for _, item := range items {
		if item.Current {
			current = item.ID
		}
	}
	if current == "" {
		t.Fatal("текущая сессия не найдена")
	}
	rr = call(t, s, "POST", "/api/account/sessions/"+current+"/revoke", "", tok, csrf)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("отзыв текущей сессии: %d %s", rr.Code, rr.Body.String())
	}
}

func TestCookieValueWithoutCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if got := cookieValue(req); got != "" {
		t.Fatalf("ожидалась пустая cookie, получено %q", got)
	}
}

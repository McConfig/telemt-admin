package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func riskServer(t *testing.T) *Server {
	t.Helper()
	s := testServerWithTelemt(t)
	s.cfg.RiskIPsWarn = 3
	s.cfg.RiskIPsHigh = 5
	s.cfg.RiskDistinct7d = 15
	return s
}

func TestSetRiskAndUserView(t *testing.T) {
	s := riskServer(t)
	ctx := context.Background()
	tok, csrf := seedSession(t, s, store.RoleAdmin)

	body := `{"warnIps":4,"highIps":6,"distinct7d":12,"newCountries":false}`
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/users/main/risk", body, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		RiskThresholds apiRisk         `json:"riskThresholds"`
		RiskDefaults   apiRiskDefaults `json:"riskDefaults"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if resp.RiskThresholds.WarnIPs == nil || *resp.RiskThresholds.WarnIPs != 4 {
		t.Fatalf("warn в ответе: %+v", resp.RiskThresholds.WarnIPs)
	}
	if resp.RiskDefaults.HighIPs != 6 {
		t.Fatalf("действующий high: %+v", resp.RiskDefaults)
	}

	saved, ok, err := s.store.GetRisk(ctx, "main")
	if err != nil || !ok {
		t.Fatalf("GetRisk: ok=%v err=%v", ok, err)
	}
	if saved.NewCountries == nil || *saved.NewCountries {
		t.Fatalf("newCountries: %+v", saved.NewCountries)
	}

	events, err := s.store.RecentAudit(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Action == "user.risk" && e.Target == "main" && e.Result == "ok" {
			found = true
		}
	}
	if !found {
		t.Fatal("в аудите нет user.risk")
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/users/main", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET user: %d %s", rr.Code, rr.Body.String())
	}
	var view apiUser
	if err := json.Unmarshal(rr.Body.Bytes(), &view); err != nil {
		t.Fatalf("разбор клиента: %v", err)
	}
	if view.RiskThresholds.WarnIPs == nil || *view.RiskThresholds.WarnIPs != 4 {
		t.Fatalf("пороги в карточке: %+v", view.RiskThresholds)
	}
	if view.RiskDefaults.WarnIPs != 4 || view.RiskDefaults.HighIPs != 6 || view.RiskDefaults.Distinct7d != 12 {
		t.Fatalf("действующие пороги: %+v", view.RiskDefaults)
	}
	if view.RiskDefaults.NewCountries {
		t.Fatal("новая страна должна быть выключена")
	}
	if view.Risk != 0 {
		t.Fatalf("риск: получено %d, ожидалось 0", view.Risk)
	}

	// Пустой запрос снимает переопределения.
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/users/main/risk", `{}`, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("сброс: %d %s", rr.Code, rr.Body.String())
	}
	if _, ok, err := s.store.GetRisk(ctx, "main"); err != nil || ok {
		t.Fatalf("после сброса: ok=%v err=%v", ok, err)
	}
}

func TestSetRiskValidation(t *testing.T) {
	s := riskServer(t)
	admin, adminCSRF := seedSession(t, s, store.RoleAdmin)
	viewer, viewerCSRF := seedSession(t, s, store.RoleViewer)

	cases := []struct {
		name string
		body string
		tok  string
		csrf string
		code int
	}{
		{name: "viewer", body: `{"warnIps":4}`, tok: viewer, csrf: viewerCSRF, code: http.StatusForbidden},
		{name: "битый JSON", body: `{`, tok: admin, csrf: adminCSRF, code: http.StatusBadRequest},
		{name: "warn больше high", body: `{"warnIps":6,"highIps":6}`, tok: admin, csrf: adminCSRF, code: http.StatusBadRequest},
		{name: "warn отрицательный", body: `{"warnIps":-1}`, tok: admin, csrf: adminCSRF, code: http.StatusBadRequest},
		{name: "high слишком большой", body: `{"highIps":1001}`, tok: admin, csrf: adminCSRF, code: http.StatusBadRequest},
		{name: "distinct слишком большой", body: `{"distinct7d":10001}`, tok: admin, csrf: adminCSRF, code: http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/users/main/risk", c.body, c.tok, c.csrf))
			if rr.Code != c.code {
				t.Fatalf("код %d, ожидался %d: %s", rr.Code, c.code, rr.Body.String())
			}
		})
	}
}

func TestRiskWithOverrides(t *testing.T) {
	s := riskServer(t)
	warn, high, distinct := 4, 6, 6
	no := false
	cfg := store.Risk{WarnIPs: &warn, HighIPs: &high, Distinct7d: &distinct, NewCountries: &no}

	stats := store.UserIPStats{Distinct7d: 6, NewCountries24h: 1}
	cases := []struct {
		current int64
		risk    int
	}{
		{current: 4, risk: 0},
		{current: 5, risk: 1},
		{current: 7, risk: 2},
	}
	for _, c := range cases {
		if got := s.risk(c.current, stats, cfg); got != c.risk {
			t.Fatalf("current %d: риск %d, ожидался %d", c.current, got, c.risk)
		}
	}

	// Глобальный порог новой страны возвращает предупреждение.
	if got := s.risk(4, stats, store.Risk{}); got != 1 {
		t.Fatalf("новая страна по умолчанию: %d, ожидалось 1", got)
	}
}

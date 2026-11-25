package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	stgs "github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
)

func settingsField(t *testing.T, cat stgs.Catalog, key string) stgs.Field {
	t.Helper()
	for _, section := range cat.Sections {
		for _, field := range section.Settings {
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("поле %s не найдено", key)
	return stgs.Field{}
}

func TestSettingsCatalog(t *testing.T) {
	s := testServer(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/settings", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var cat stgs.Catalog
	if err := json.Unmarshal(rr.Body.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	if cat.Version != 0 || len(cat.Sections) == 0 {
		t.Fatalf("каталог: %+v", cat)
	}
	warn := settingsField(t, cat, "risk.warn_ips")
	if warn.Status != stgs.Active || warn.Value.(float64) != 3 || warn.Source != "env" {
		t.Fatalf("warn: %+v", warn)
	}
	interval := settingsField(t, cat, "collect.interval")
	if interval.Restart {
		t.Fatalf("интервал сбора не должен требовать перезапуска: %+v", interval)
	}
	activeLogin := settingsField(t, cat, "login.max_failures")
	if activeLogin.Status != stgs.Active || activeLogin.Value.(float64) != 10 {
		t.Fatalf("login.max_failures: %+v", activeLogin)
	}
	planned := settingsField(t, cat, "security.require_2fa")
	if planned.Status != stgs.Planned || planned.PlannedRelease != "1.5.0" {
		t.Fatalf("planned: %+v", planned)
	}
	readonly := settingsField(t, cat, "network.allowed_cidrs")
	if readonly.Status != stgs.Readonly || readonly.Value == "" {
		t.Fatalf("readonly: %+v", readonly)
	}
	secret := settingsField(t, cat, "totp.key")
	if !secret.Secret || secret.Value != nil || !secret.SecretSet {
		t.Fatalf("secret: %+v", secret)
	}
}

func TestSettingsUpdateAndConflict(t *testing.T) {
	s := testServer(t)
	ctx := context.Background()
	tok, csrf := seedSession(t, s, store.RoleAdmin)

	body := `{"version":0,"values":{"risk.warn_ips":4,"risk.high_ips":8,"risk.new_countries":false,"session.ttl":"24h","links.mode":"secure"}}`
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/settings", body, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var cat stgs.Catalog
	if err := json.Unmarshal(rr.Body.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	if cat.Version != 1 {
		t.Fatalf("версия %d", cat.Version)
	}
	if field := settingsField(t, cat, "risk.warn_ips"); field.Value.(float64) != 4 || field.Source != "settings" {
		t.Fatalf("warn: %+v", field)
	}
	if field := settingsField(t, cat, "session.ttl"); field.Value != "24h0m0s" {
		t.Fatalf("ttl: %+v", field)
	}

	events, err := s.store.RecentAudit(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Action == "settings.update" && event.Target == "risk.warn_ips" && event.Result == "3 -> 4" {
			found = true
		}
	}
	if !found {
		t.Fatal("в аудите нет settings.update")
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/settings", body, tok, csrf))
	if rr.Code != http.StatusConflict {
		t.Fatalf("конфликт: код %d: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/settings",
		`{"version":1,"values":{"security.require_2fa":true}}`, tok, csrf))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("planned: код %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/settings",
		`{"version":1,"values":{"risk.high_ips":2}}`, tok, csrf))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("валидация: код %d", rr.Code)
	}

	viewer, viewerCSRF := seedSession(t, s, store.RoleViewer)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/settings", `{"version":1,"values":{}}`, viewer, viewerCSRF))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer: код %d", rr.Code)
	}
}

func TestSettingsTemplatesAPI(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/settings/templates",
		`{"name":"router","maxIps":5,"maxTcpConns":20,"quotaPeriod":"monthly","riskWarnIps":4,"riskHighIps":6}`,
		tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("создание: код %d: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/settings/templates/default",
		`{"name":"router"}`, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("default: код %d: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/settings/templates", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("список: код %d", rr.Code)
	}
	var list struct {
		Default   string                `json:"default"`
		Templates []store.LimitTemplate `json:"templates"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Default != "router" || len(list.Templates) != 1 || *list.Templates[0].MaxIps != 5 {
		t.Fatalf("список: %+v", list)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/settings/templates", `{"name":"плохое"}`, tok, csrf))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("имя: код %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE", "/api/settings/templates/router", "", tok, csrf))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("удаление: код %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSettingsBackupsAndSystem(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/settings/backups", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("бэкапы: код %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"valkey"`) {
		t.Fatalf("ответ без valkey: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/settings/backups/snapshot", "", tok, csrf))
	if rr.Code != http.StatusAccepted && rr.Code != http.StatusBadGateway {
		t.Fatalf("снимок: код %d: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/settings/system", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("система: код %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"counts"`) {
		t.Fatalf("система без counts: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/settings/history", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("история: код %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMetaDefaultChartDays(t *testing.T) {
	s := testServer(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/meta", "", "", ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("meta: %d", rr.Code)
	}
	var meta struct {
		DefaultChartDays int `json:"defaultChartDays"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.DefaultChartDays != 30 {
		t.Fatalf("по умолчанию: %d", meta.DefaultChartDays)
	}
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	if _, _, err := s.settings.Save(context.Background(), 0, map[string]json.RawMessage{
		"ui.default_chart_days": json.RawMessage("14"),
	}, nil); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/meta", "", tok, csrf))
	if err := json.Unmarshal(rr.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.DefaultChartDays != 14 {
		t.Fatalf("после настройки: %d", meta.DefaultChartDays)
	}
}

func TestSettingsPasswordPolicyApplies(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	if _, _, err := s.settings.Save(context.Background(), 0, map[string]json.RawMessage{
		"password.min_length": json.RawMessage("20"),
	}, nil); err != nil {
		t.Fatal(err)
	}
	short := `{"name":"newadmin","role":"admin","password":"123456789012"}`
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/admins", short, tok, csrf))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("короткий пароль: код %d: %s", rr.Code, rr.Body.String())
	}
	long := `{"name":"newadmin","role":"admin","password":"12345678901234567890"}`
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/admins", long, tok, csrf))
	if rr.Code != http.StatusCreated {
		t.Fatalf("длинный пароль: код %d: %s", rr.Code, rr.Body.String())
	}
}

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/collector"
	"github.com/McConfig/telemt-admin/internal/store"
)

// failingNotifier всегда возвращает ошибку отправки.
type failingNotifier struct{}

func (f *failingNotifier) Emit(context.Context, string, map[string]any) {}

func (f *failingNotifier) SendNow(context.Context, string, map[string]any) (int, error) {
	return 0, errors.New("вебхук недоступен")
}

func TestSettingsViewerGuards(t *testing.T) {
	s := testServer(t)
	admin, adminCSRF := seedSession(t, s, store.RoleAdmin)
	viewer, viewerCSRF := seedSession(t, s, store.RoleViewer)

	// Администратору доступны все точки чтения.
	for _, path := range []string{
		"/api/settings", "/api/settings/templates", "/api/settings/backups",
		"/api/settings/system", "/api/settings/history",
	} {
		if rr := call(t, s, "GET", path, "", admin, ""); rr.Code != http.StatusOK {
			t.Fatalf("админ GET %s: %d %s", path, rr.Code, rr.Body.String())
		}
	}

	cases := []struct{ method, path, body string }{
		{"GET", "/api/settings", ""},
		{"GET", "/api/settings/templates", ""},
		{"POST", "/api/settings/templates", `{"name":"x"}`},
		{"DELETE", "/api/settings/templates/x", ""},
		{"PUT", "/api/settings/templates/default", `{"name":"x"}`},
		{"GET", "/api/settings/backups", ""},
		{"POST", "/api/settings/backups/snapshot", ""},
		{"GET", "/api/settings/system", ""},
		{"POST", "/api/settings/webhook/test", ""},
		{"GET", "/api/settings/history", ""},
	}
	for _, c := range cases {
		if rr := call(t, s, c.method, c.path, c.body, viewer, viewerCSRF); rr.Code != http.StatusForbidden {
			t.Errorf("просмотрщик %s %s: %d %s", c.method, c.path, rr.Code, rr.Body.String())
		}
	}
	_ = adminCSRF
}

func TestSettingsValidationBranches(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)

	if rr := call(t, s, "PUT", "/api/settings", `{`, tok, csrf); rr.Code != http.StatusBadRequest {
		t.Fatalf("битый JSON настроек: %d", rr.Code)
	}
	if rr := call(t, s, "POST", "/api/settings/templates", `{`, tok, csrf); rr.Code != http.StatusBadRequest {
		t.Fatalf("битый JSON шаблона: %d", rr.Code)
	}
	// Неизвестный шаблон нельзя назначить по умолчанию.
	if rr := call(t, s, "PUT", "/api/settings/templates/default", `{"name":"nope"}`, tok, csrf); rr.Code != http.StatusNotFound {
		t.Fatalf("неизвестный шаблон: %d %s", rr.Code, rr.Body.String())
	}
	// Пустое имя шаблона по умолчанию отклоняется.
	if rr := call(t, s, "PUT", "/api/settings/templates/default", `{}`, tok, csrf); rr.Code != http.StatusBadRequest {
		t.Fatalf("пустое имя: %d", rr.Code)
	}
	if rr := call(t, s, "PUT", "/api/settings/templates/default", `{`, tok, csrf); rr.Code != http.StatusBadRequest {
		t.Fatalf("битый JSON default: %d", rr.Code)
	}
	// Имя из пути перекрывает имя в теле.
	if rr := call(t, s, "PUT", "/api/settings/templates/pathname", `{"maxIps":3}`, tok, csrf); rr.Code != http.StatusOK {
		t.Fatalf("шаблон через путь: %d %s", rr.Code, rr.Body.String())
	}
	listed, err := s.store.ListTemplates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Name != "pathname" {
		t.Fatalf("шаблоны: %+v", listed)
	}
}

func TestSettingsTemplateLimit(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("tpl%02d", i)
		if err := s.store.SaveTemplate(context.Background(), store.LimitTemplate{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	rr := call(t, s, "POST", "/api/settings/templates", `{"name":"overflow"}`, tok, csrf)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("лимит шаблонов: %d %s", rr.Code, rr.Body.String())
	}
}

func TestSettingsHistoryFiltering(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	ctx := context.Background()

	for i, warn := range []int{4, 3} {
		body := fmt.Sprintf(`{"version":%d,"values":{"risk.warn_ips":%d}}`, i, warn)
		if rr := call(t, s, "PUT", "/api/settings", body, tok, csrf); rr.Code != http.StatusOK {
			t.Fatalf("сохранение %d: %d %s", i, rr.Code, rr.Body.String())
		}
	}
	// Постороннее событие должно быть отфильтровано.
	if err := s.store.Audit(ctx, store.AuditEvent{
		Actor: "admin", Action: "user.create", Target: "main", Result: "ok", IP: "192.0.2.1",
	}); err != nil {
		t.Fatal(err)
	}

	for _, limit := range []string{"1", "abc", "0", "500"} {
		if rr := call(t, s, "GET", "/api/settings/history?limit="+limit, "", tok, ""); rr.Code != http.StatusOK {
			t.Fatalf("история %s: %d %s", limit, rr.Code, rr.Body.String())
		}
	}
	rr := call(t, s, "GET", "/api/settings/history?limit=1", "", tok, "")
	var events []store.AuditEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || len(events[0].Action) < len("settings.") {
		t.Fatalf("история: %+v", events)
	}
}

func TestSettingsWebhookFailure(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)
	s.SetNotifier(&failingNotifier{})
	if rr := call(t, s, "POST", "/api/settings/webhook/test", "", tok, csrf); rr.Code != http.StatusBadGateway {
		t.Fatalf("ошибка вебхука: %d %s", rr.Code, rr.Body.String())
	}
}

func TestSettingsSystemWithStatus(t *testing.T) {
	s := testServer(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	s.SetCollectorStatus(func() collector.Status {
		return collector.Status{
			Up: true, LastSuccess: time.Now(), LastError: "none",
			GeoIPLoaded: true, GeoIPBuilt: 1_700_000_000,
		}
	})
	rr := call(t, s, "GET", "/api/settings/system", "", tok, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("система: %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{`"collector"`, `"geoip"`, `"counts"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("в ответе нет %s: %s", want, body)
		}
	}
}

func TestReadHostBackupVariants(t *testing.T) {
	if got := readHostBackup(""); got.Available {
		t.Fatalf("пустой путь: %+v", got)
	}
	if got := readHostBackup(filepath.Join(t.TempDir(), "missing.json")); got.Available {
		t.Fatalf("отсутствующий файл: %+v", got)
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readHostBackup(bad); got.Available {
		t.Fatalf("битый файл: %+v", got)
	}
	good := filepath.Join(t.TempDir(), "good.json")
	raw := `{"lastRun":"2026-01-02T03:04:05Z","result":"ok","size":42,"keepDays":7,"restoreTest":true}`
	if err := os.WriteFile(good, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	got := readHostBackup(good)
	if !got.Available || got.Size != 42 || got.KeepDays != 7 || got.Result != "ok" {
		t.Fatalf("годный файл: %+v", got)
	}

	s := testServer(t)
	s.cfg.BackupStatus = good
	status, err := s.buildBackupStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Host.Available {
		t.Fatalf("статус хоста: %+v", status)
	}
}

func TestBuildBackupStatusStoreError(t *testing.T) {
	s, mr := testServerWithMR(t)
	mr.SetError("хранилище недоступно")
	if _, err := s.buildBackupStatus(context.Background()); err == nil {
		t.Fatal("ожидалась ошибка хранилища")
	}
}

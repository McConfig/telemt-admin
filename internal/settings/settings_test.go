package settings

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/logging"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/alicebob/miniredis/v2"
)

func testService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	base, _ := url.Parse("https://tgadmin.example.com")
	cfg := &config.Config{
		Listen: ":8090", BaseURL: base, SessionTTL: 720 * time.Hour,
		LinkMode: "tls", CookieSecure: true, RiskIPsWarn: 3, RiskIPsHigh: 5,
		RiskDistinct7d: 15, HomeCountry: "RU", Retention: 720 * time.Hour,
		CollectInterval:  time.Minute,
		LoginMaxFailures: 10, LoginWindow: 15 * time.Minute,
		TOTPPendingTTL: 5 * time.Minute, TOTPMaxAttempts: 5,
		AuditMaxEvents: 10000, PasswordMinLength: 12,
		DefaultChartDays: 30, LogLevel: "info",
		AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		GeoIPCity:    "/opt/geoip/city.mmdb", GeoIPASN: "/opt/geoip/asn.mmdb",
		ValkeyURL: "redis://valkey:6379/0", TelemtURL: "http://telemt:9091",
		TOTPKey:        []byte("0123456789abcdef0123456789abcdef"),
		BackupSchedule: "04:30", BackupKeepDays: 14, BackupRestoreTest: true,
		BackupConfig: filepath.Join(t.TempDir(), "backup.json"),
	}
	svc := New(cfg, st)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return svc, st
}

func raw(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func TestCatalogStatusesAndSecrets(t *testing.T) {
	svc, _ := testService(t)
	cat := svc.Catalog(context.Background())

	statuses := map[string]Status{}
	secrets := map[string]Field{}
	for _, section := range cat.Sections {
		for _, field := range section.Settings {
			statuses[field.Key] = field.Status
			if field.Secret {
				secrets[field.Key] = field
			}
		}
	}
	for _, key := range []string{"risk.warn_ips", "session.ttl", "links.mode"} {
		if statuses[key] != Active {
			t.Fatalf("%s: статус %s", key, statuses[key])
		}
	}
	if statuses["login.max_failures"] != Active {
		t.Fatalf("login.max_failures: %s", statuses["login.max_failures"])
	}
	if statuses["security.require_2fa"] != Active {
		t.Fatalf("security.require_2fa: %s", statuses["security.require_2fa"])
	}
	if statuses["notifications.webhook_url"] != Active {
		t.Fatalf("notifications.webhook_url: %s", statuses["notifications.webhook_url"])
	}
	if statuses["backup.offsite"] != Planned {
		t.Fatalf("backup.offsite: %s", statuses["backup.offsite"])
	}
	if statuses["network.allowed_cidrs"] != Readonly {
		t.Fatalf("network.allowed_cidrs: %s", statuses["network.allowed_cidrs"])
	}
	for key, field := range secrets {
		if field.Value != nil {
			t.Fatalf("%s: секрет вернулся в значении", key)
		}
	}
	if !secrets["totp.key"].SecretSet {
		t.Fatal("totp.key должен быть помечен как заданный")
	}
	if secrets["valkey.password"].SecretSet {
		t.Fatal("пустой пароль Valkey не должен считаться заданным")
	}
}

func TestSaveAndReset(t *testing.T) {
	svc, st := testService(t)
	ctx := context.Background()

	cat, changes, err := svc.Save(ctx, 0, map[string]json.RawMessage{
		"risk.warn_ips":      raw(4),
		"risk.high_ips":      raw(8),
		"risk.home_country":  raw("de"),
		"risk.new_countries": raw(false),
		"session.ttl":        raw("24h"),
		"links.mode":         raw("secure"),
		"retention":          raw("240h"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Version != 1 {
		t.Fatalf("версия %d", cat.Version)
	}
	if len(changes) != 7 {
		t.Fatalf("изменений %d: %+v", len(changes), changes)
	}
	warn, high, distinct, newCountries, home := svc.RiskValues(ctx)
	if warn != 4 || high != 8 || distinct != 15 || newCountries || home != "DE" {
		t.Fatalf("значения риска: %d %d %d %v %s", warn, high, distinct, newCountries, home)
	}
	if svc.SessionTTL(ctx) != 24*time.Hour {
		t.Fatalf("ttl: %s", svc.SessionTTL(ctx))
	}
	if svc.LinkMode(ctx) != "secure" {
		t.Fatalf("link mode: %s", svc.LinkMode(ctx))
	}
	if st.HomeCountry() != "DE" {
		t.Fatalf("домашняя страна в store: %s", st.HomeCountry())
	}
	if st.Retention() != 240*time.Hour {
		t.Fatalf("retention в store: %s", st.Retention())
	}

	cat, changes, err = svc.Save(ctx, cat.Version, nil, []string{"risk.warn_ips", "links.mode"})
	if err != nil {
		t.Fatal(err)
	}
	if cat.Version != 2 || len(changes) != 2 {
		t.Fatalf("сброс: версия %d, изменений %d", cat.Version, len(changes))
	}
	warn, _, _, _, _ = svc.RiskValues(ctx)
	if warn != 3 {
		t.Fatalf("warn после сброса: %d", warn)
	}
	if svc.LinkMode(ctx) != "tls" {
		t.Fatalf("link mode после сброса: %s", svc.LinkMode(ctx))
	}
}

func TestValidation(t *testing.T) {
	svc, _ := testService(t)
	ctx := context.Background()

	cases := []struct {
		name  string
		key   string
		value any
	}{
		{name: "warn больше high", key: "risk.high_ips", value: 3},
		{name: "warn отрицательный", key: "risk.warn_ips", value: -1},
		{name: "страна", key: "risk.home_country", value: "RUS"},
		{name: "интервал", key: "collect.interval", value: "1s"},
		{name: "retention", key: "retention", value: "1h"},
		{name: "режим", key: "links.mode", value: "ftp"},
		{name: "тип", key: "risk.warn_ips", value: "много"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := svc.Save(ctx, 0, map[string]json.RawMessage{c.key: raw(c.value)}, nil)
			var invalid *ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("ожидалась ошибка валидации, получено %v", err)
			}
			if invalid.Key != c.key {
				t.Fatalf("ключ %s, ожидался %s", invalid.Key, c.key)
			}
		})
	}
}

func TestPlannedAndReadonlyRejected(t *testing.T) {
	svc, _ := testService(t)
	ctx := context.Background()
	for _, key := range []string{"notifications.webhook_url", "network.allowed_cidrs", "valkey.password"} {
		_, _, err := svc.Save(ctx, 0, map[string]json.RawMessage{key: raw(1)}, nil)
		var invalid *ValidationError
		if !errors.As(err, &invalid) {
			t.Fatalf("%s: ожидалась ошибка, получено %v", key, err)
		}
	}
}

func TestVersionConflict(t *testing.T) {
	svc, _ := testService(t)
	ctx := context.Background()
	if _, _, err := svc.Save(ctx, 0, map[string]json.RawMessage{"risk.warn_ips": raw(4)}, nil); err != nil {
		t.Fatal(err)
	}
	_, _, err := svc.Save(ctx, 0, map[string]json.RawMessage{"risk.distinct_7d": raw(20)}, nil)
	if !errors.Is(err, store.ErrVersionConflict) {
		t.Fatalf("ожидался конфликт, получено %v", err)
	}
}

func TestTemplateValidation(t *testing.T) {
	name := func(v string) *string { return &v }
	ips := func(v uint64) *uint64 { return &v }
	days := func(v int) *int { return &v }
	if err := ValidateTemplate(store.LimitTemplate{Name: "bad name"}); err == nil {
		t.Fatal("ожидалась ошибка имени")
	}
	if err := ValidateTemplate(store.LimitTemplate{Name: "router", MaxIps: ips(2000)}); err == nil {
		t.Fatal("ожидалась ошибка лимита IP")
	}
	if err := ValidateTemplate(store.LimitTemplate{Name: "router", QuotaPeriod: name("year")}); err == nil {
		t.Fatal("ожидалась ошибка периода")
	}
	if err := ValidateTemplate(store.LimitTemplate{
		Name: "router", RiskWarnIps: days(6), RiskHighIps: days(5),
	}); err == nil {
		t.Fatal("ожидалась ошибка порогов")
	}
	if err := ValidateTemplate(store.LimitTemplate{
		Name: "router", MaxIps: ips(5), QuotaPeriod: name("monthly"),
	}); err != nil {
		t.Fatalf("валидный шаблон: %v", err)
	}
}

func TestSecuritySettingsApplied(t *testing.T) {
	svc, st := testService(t)
	ctx := context.Background()
	if _, _, err := svc.Save(ctx, 0, map[string]json.RawMessage{
		"login.max_failures":    raw(4),
		"login.window":          raw("5m"),
		"totp.pending_ttl":      raw("2m"),
		"totp.max_attempts":     raw(3),
		"audit.max_events":      raw(500),
		"password.min_length":   raw(16),
		"ui.default_chart_days": raw(14),
		"log.level":             raw("debug"),
	}, nil); err != nil {
		t.Fatal(err)
	}
	max, window := st.LoginPolicy()
	if max != 4 || window != 5*time.Minute {
		t.Fatalf("политика входа: %d %s", max, window)
	}
	if st.AuditMax() != 500 {
		t.Fatalf("аудит: %d", st.AuditMax())
	}
	if svc.TOTPPendingTTL(ctx) != 2*time.Minute || svc.TOTPMaxAttempts(ctx) != 3 {
		t.Fatalf("TOTP: %s %d", svc.TOTPPendingTTL(ctx), svc.TOTPMaxAttempts(ctx))
	}
	if svc.PasswordMinLength(ctx) != 16 || svc.DefaultChartDays(ctx) != 14 {
		t.Fatalf("пароль/графики: %d %d", svc.PasswordMinLength(ctx), svc.DefaultChartDays(ctx))
	}
	if logging.Level() != "debug" {
		t.Fatalf("лог: %s", logging.Level())
	}
	t.Cleanup(func() { _ = logging.Set("info") })
}

func TestRequire2FAState(t *testing.T) {
	svc, st := testService(t)
	ctx := context.Background()
	if _, _, err := svc.Save(ctx, 0, map[string]json.RawMessage{
		"security.require_2fa":            raw(true),
		"security.require_2fa_grace_days": raw(7),
	}, nil); err != nil {
		t.Fatal(err)
	}
	enabled, deadline, days := svc.Require2FA(ctx)
	if !enabled || deadline.IsZero() || days < 6 || days > 7 {
		t.Fatalf("2FA: %v %s %d", enabled, deadline, days)
	}
	if _, ok, err := st.Require2FASince(ctx); err != nil || !ok {
		t.Fatalf("отметка включения: %v %v", ok, err)
	}
	if _, _, err := svc.Save(ctx, 1, map[string]json.RawMessage{
		"security.require_2fa": raw(false),
	}, nil); err != nil {
		t.Fatal(err)
	}
	if enabled, _, _ := svc.Require2FA(ctx); enabled {
		t.Fatal("2FA должна быть выключена")
	}
	if _, ok, _ := st.Require2FASince(ctx); ok {
		t.Fatal("отметка включения должна быть удалена")
	}
}

func TestWebhookAndBackupSettings(t *testing.T) {
	svc, _ := testService(t)
	ctx := context.Background()
	_, changes, err := svc.Save(ctx, 0, map[string]json.RawMessage{
		"notifications.webhook_url":    raw("https://example.com/hook"),
		"notifications.webhook_secret": raw("topsecret"),
		"notifications.events":         raw("user.create,user.delete"),
		"backup.schedule":              raw("05:15"),
		"backup.keep_days":             raw(30),
		"backup.restore_test":          raw(false),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		if change.Key == "notifications.webhook_secret" && (change.Old != "***" || change.New != "***") {
			t.Fatalf("секрет попал в аудит: %+v", change)
		}
	}
	url, secret, events, enabled := svc.Webhook(ctx)
	if !enabled || url != "https://example.com/hook" || secret != "topsecret" {
		t.Fatalf("webhook: %s %s %v", url, secret, events)
	}
	if len(events) != 2 || events[0] != "user.create" || events[1] != "user.delete" {
		t.Fatalf("events: %v", events)
	}
	schedule, keep, restore := svc.BackupPolicy(ctx)
	if schedule != "05:15" || keep != 30 || restore {
		t.Fatalf("backup: %s %d %v", schedule, keep, restore)
	}
	rawFile, err := os.ReadFile(svc.cfg.BackupConfig)
	if err != nil {
		t.Fatal(err)
	}
	var written backupPolicyFile
	if err := json.Unmarshal(rawFile, &written); err != nil {
		t.Fatal(err)
	}
	if written.Schedule != "05:15" || written.KeepDays != 30 || written.RestoreTest {
		t.Fatalf("файл: %+v", written)
	}

	cat := svc.Catalog(ctx)
	secretField := settingsFieldByKey(t, cat, "notifications.webhook_secret")
	if secretField.Value != nil || !secretField.SecretSet || secretField.Source != "settings" {
		t.Fatalf("секрет в каталоге: %+v", secretField)
	}
}

func settingsFieldByKey(t *testing.T, cat Catalog, key string) Field {
	t.Helper()
	for _, section := range cat.Sections {
		for _, field := range section.Settings {
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("поле %s не найдено", key)
	return Field{}
}

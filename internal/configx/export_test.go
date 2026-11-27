package configx

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
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
		CollectInterval: time.Minute, LoginMaxFailures: 10, LoginWindow: 15 * time.Minute,
		TOTPPendingTTL: 5 * time.Minute, TOTPMaxAttempts: 5,
		AuditMaxEvents: 10000, PasswordMinLength: 12, DefaultChartDays: 30,
		LogLevel: "info", WebhookURL: "https://hooks.example.com/x", WebhookSecret: "super-secret",
		AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		ValkeyURL:    "redis://valkey:6379/0", TelemtURL: "http://telemt:9091",
	}
	svc := settings.New(cfg, st)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"username":"main","enabled":true,"links":{"tls":["tg://proxy?server=demo.example.com&port=443&secret=ee0123456789abcdef0123456789abcdefdemo.example.com"]}}]}`))
	}))
	t.Cleanup(ts.Close)
	return New(st, svc, telemt.New(ts.URL, "", time.Second), time.Now), st
}

func seedExportData(t *testing.T, st *store.Store) {
	t.Helper()
	ctx := context.Background()
	hash, _ := hashPassword("long password 123")
	if err := st.CreateUser(ctx, store.User{Name: "demo", Hash: hash, Role: store.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{Name: "viewer", Hash: hash, Role: store.RoleViewer}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveTemplate(ctx, store.LimitTemplate{Name: "default"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateService(ctx, store.ServiceAccount{Name: "monitoring", Scope: store.ScopeRead}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.CreateServiceToken(ctx, "monitoring", "prom", "demo", nil); err != nil {
		t.Fatal(err)
	}
	if err := st.SetNote(ctx, "main", "ноутбук"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetTrusted(ctx, "main", store.Trusted{Countries: []string{"GE"}}); err != nil {
		t.Fatal(err)
	}
	warn := 4
	if err := st.SetRisk(ctx, "main", store.Risk{WarnIPs: &warn}); err != nil {
		t.Fatal(err)
	}
}

func TestExportRedactsSecretsByDefault(t *testing.T) {
	svc, st := testService(t)
	seedExportData(t, st)
	b, err := svc.Export(context.Background(), ExportOptions{
		Blocks: []Block{BlockSettings, BlockUsers, BlockClients, BlockServices}, Name: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range b.Spec.Users {
		if u.PasswordHash != "" {
			t.Fatal("хеш пароля попал в экспорт")
		}
	}
	for _, c := range b.Spec.Clients {
		if c.Secret != "" {
			t.Fatal("секрет клиента попал в экспорт")
		}
	}
	for _, s := range b.Spec.Services {
		if len(s.Tokens) != 0 {
			t.Fatal("хеш токена попал в экспорт")
		}
	}
	raw, _ := Marshal(b)
	if bytes.Contains(raw, []byte("super-secret")) || bytes.Contains(raw, []byte("hooks.example.com")) {
		t.Fatalf("секрет вебхука попал в YAML: %s", raw)
	}
}

func TestExportIncludesSecretsWhenAsked(t *testing.T) {
	svc, st := testService(t)
	seedExportData(t, st)
	b, err := svc.Export(context.Background(), ExportOptions{
		Blocks: []Block{BlockUsers, BlockServices, BlockClients, BlockSettings}, Secrets: true, Name: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.Spec.Users[0].PasswordHash == "" {
		t.Fatal("нет хеша пароля")
	}
	if b.Spec.Services[0].Tokens[0].SecretHash == "" {
		t.Fatal("нет хеша токена")
	}
	if b.Spec.Clients[0].Secret != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("секрет клиента: %q", b.Spec.Clients[0].Secret)
	}
	if b.Spec.Settings["notifications"]["webhook_secret"] != "super-secret" {
		t.Fatalf("секрет вебхука: %+v", b.Spec.Settings["notifications"])
	}
}

func TestExportClientsKeepsAdminMetadata(t *testing.T) {
	svc, st := testService(t)
	seedExportData(t, st)
	b, err := svc.Export(context.Background(), ExportOptions{Blocks: []Block{BlockClients}})
	if err != nil {
		t.Fatal(err)
	}
	c := b.Spec.Clients[0]
	if c.Note != "ноутбук" || c.Trusted == nil || c.Risk == nil {
		t.Fatalf("%+v", c)
	}
}

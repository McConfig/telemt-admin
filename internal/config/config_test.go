package config

import (
	"net/netip"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != ":8090" {
		t.Errorf("Listen = %q, хочу :8090", cfg.Listen)
	}
	if cfg.SessionTTL.Hours() != 720 {
		t.Errorf("SessionTTL = %v, хочу 720h", cfg.SessionTTL)
	}
	if len(cfg.AllowedCIDRs) != 4 {
		t.Errorf("CIDR = %v, хочу 4 префикса", cfg.AllowedCIDRs)
	}
	if cfg.LinkMode != "tls" {
		t.Errorf("LinkMode = %q, хочу tls", cfg.LinkMode)
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure по умолчанию должен быть включён")
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"TGADMIN_LISTEN":        "127.0.0.1:9000",
		"TGADMIN_ALLOWED_CIDRS": "192.0.2.0/24",
		"TGADMIN_BASE_URL":      "https://tgadmin.example.com",
		"TGADMIN_SESSION_TTL":   "24h",
		"VALKEY_URL":            "redis://127.0.0.1:6379/1",
		"TELEMT_API_URL":        "http://127.0.0.1:9091",
		"TELEMT_LINK_MODE":      "secure",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != "127.0.0.1:9000" || len(cfg.AllowedCIDRs) != 1 {
		t.Fatalf("переопределения не применились: %+v", cfg)
	}
	if cfg.BaseURL.Host != "tgadmin.example.com" {
		t.Errorf("BaseURL = %v", cfg.BaseURL)
	}
}

func TestLoadCookieSecure(t *testing.T) {
	cfg, err := Load(env(map[string]string{"TGADMIN_COOKIE_SECURE": "false"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieSecure {
		t.Fatal("CookieSecure должен быть выключен")
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	cfg, err := Load(env(map[string]string{"TGADMIN_TRUSTED_PROXIES": "10.0.0.1/32"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedProxies) != 1 || !cfg.TrustsProxy(netip.MustParseAddr("10.0.0.1")) {
		t.Fatalf("доверенные прокси: %v", cfg.TrustedProxies)
	}
}

func TestLoadRejectsBadCIDR(t *testing.T) {
	if _, err := Load(env(map[string]string{"TGADMIN_ALLOWED_CIDRS": "not-a-cidr"})); err == nil {
		t.Fatal("ожидал ошибку для неверного CIDR")
	}
}

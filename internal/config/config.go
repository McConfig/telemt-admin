package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// Config — настройки сервиса из переменных окружения.
type Config struct {
	Listen         string
	AllowedCIDRs   []netip.Prefix
	TrustedProxies []netip.Prefix
	BaseURL        *url.URL
	SessionTTL     time.Duration
	ValkeyURL      string
	ValkeyPassword string
	TelemtURL      string
	TelemtToken    string
	LinkMode       string
	CookieSecure   bool
}

// Load читает конфигурацию. getenv передаётся для тестируемости.
func Load(getenv func(string) string) (*Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}
	cfg := &Config{
		Listen:         get("TGADMIN_LISTEN", ":8090"),
		ValkeyURL:      get("VALKEY_URL", "redis://valkey:6379/0"),
		ValkeyPassword: getenv("VALKEY_PASSWORD"),
		TelemtURL:      get("TELEMT_API_URL", "http://127.0.0.1:9091"),
		TelemtToken:    getenv("TELEMT_API_TOKEN"),
		LinkMode:       get("TELEMT_LINK_MODE", "tls"),
		CookieSecure:   get("TGADMIN_COOKIE_SECURE", "true") != "false",
	}

	d, err := time.ParseDuration(get("TGADMIN_SESSION_TTL", "720h"))
	if err != nil {
		return nil, fmt.Errorf("TGADMIN_SESSION_TTL: %w", err)
	}
	cfg.SessionTTL = d

	raw := get("TGADMIN_ALLOWED_CIDRS", "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		p, err := netip.ParsePrefix(item)
		if err != nil {
			return nil, fmt.Errorf("TGADMIN_ALLOWED_CIDRS %q: %w", item, err)
		}
		cfg.AllowedCIDRs = append(cfg.AllowedCIDRs, p)
	}
	if len(cfg.AllowedCIDRs) == 0 {
		return nil, fmt.Errorf("TGADMIN_ALLOWED_CIDRS пуст")
	}

	trusted := get("TGADMIN_TRUSTED_PROXIES", "")
	for _, item := range strings.Split(trusted, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		p, err := netip.ParsePrefix(item)
		if err != nil {
			return nil, fmt.Errorf("TGADMIN_TRUSTED_PROXIES %q: %w", item, err)
		}
		cfg.TrustedProxies = append(cfg.TrustedProxies, p)
	}

	base := get("TGADMIN_BASE_URL", "https://tgadmin.example.com")
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("TGADMIN_BASE_URL %q: нужен абсолютный URL", base)
	}
	cfg.BaseURL = u

	switch cfg.LinkMode {
	case "tls", "secure", "classic":
	default:
		return nil, fmt.Errorf("TELEMT_LINK_MODE %q: допустимы tls, secure, classic", cfg.LinkMode)
	}
	return cfg, nil
}

// TrustsProxy сообщает, можно ли верить X-Forwarded-For от этого адреса.
func (c *Config) TrustsProxy(addr netip.Addr) bool {
	for _, p := range c.TrustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// Allows проверяет адрес по списку разрешённых сетей.
func (c *Config) Allows(addr netip.Addr) bool {
	for _, p := range c.AllowedCIDRs {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

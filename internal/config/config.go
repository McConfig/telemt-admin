package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/totp"
)

// Config — настройки сервиса из переменных окружения.
type Config struct {
	Listen          string
	AllowedCIDRs    []netip.Prefix
	TrustedProxies  []netip.Prefix
	BaseURL         *url.URL
	SessionTTL      time.Duration
	ValkeyURL       string
	ValkeyPassword  string
	TelemtURL       string
	TelemtToken     string
	LinkMode        string
	CookieSecure    bool
	CollectInterval time.Duration
	RiskIPsWarn     int
	RiskIPsHigh     int
	RiskDistinct7d  int
	HomeCountry     string
	Retention       time.Duration
	GeoIPCity       string
	GeoIPASN        string
	BackupStatus    string
	TOTPKey         []byte
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
		RiskIPsWarn:    getInt(get("TGADMIN_RISK_IPS_WARN", "3"), 3),
		RiskIPsHigh:    getInt(get("TGADMIN_RISK_IPS_HIGH", "5"), 5),
		RiskDistinct7d: getInt(get("TGADMIN_RISK_DISTINCT_7D", "15"), 15),
		GeoIPCity:      get("TGADMIN_GEOIP_CITY", "/opt/geoip/GeoLite2-City.mmdb"),
		GeoIPASN:       get("TGADMIN_GEOIP_ASN", "/opt/geoip/GeoLite2-ASN.mmdb"),
	}
	cfg.HomeCountry = strings.ToUpper(get("TGADMIN_HOME_COUNTRY", "RU"))
	if len(cfg.HomeCountry) != 2 || cfg.HomeCountry[0] < 'A' || cfg.HomeCountry[0] > 'Z' ||
		cfg.HomeCountry[1] < 'A' || cfg.HomeCountry[1] > 'Z' {
		return nil, fmt.Errorf("TGADMIN_HOME_COUNTRY %q: нужен код ISO из двух букв", cfg.HomeCountry)
	}
	cfg.BackupStatus = get("TGADMIN_BACKUP_STATUS", "")
	collect, err := time.ParseDuration(get("TGADMIN_COLLECT_INTERVAL", "60s"))
	if err != nil {
		return nil, fmt.Errorf("TGADMIN_COLLECT_INTERVAL: %w", err)
	}
	cfg.CollectInterval = collect
	retention, err := time.ParseDuration(get("TGADMIN_RETENTION", "720h"))
	if err != nil {
		return nil, fmt.Errorf("TGADMIN_RETENTION: %w", err)
	}
	cfg.Retention = retention

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

	totpKey, err := totp.ParseKey(getenv("TGADMIN_TOTP_KEY"))
	if err != nil {
		return nil, err
	}
	cfg.TOTPKey = totpKey

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
func getInt(raw string, def int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func (c *Config) Allows(addr netip.Addr) bool {
	for _, p := range c.AllowedCIDRs {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

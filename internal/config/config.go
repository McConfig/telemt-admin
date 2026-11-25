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
	Listen              string
	AllowedCIDRs        []netip.Prefix
	TrustedProxies      []netip.Prefix
	BaseURL             *url.URL
	SessionTTL          time.Duration
	ValkeyURL           string
	ValkeyPassword      string
	TelemtURL           string
	TelemtToken         string
	LinkMode            string
	CookieSecure        bool
	CollectInterval     time.Duration
	RiskIPsWarn         int
	RiskIPsHigh         int
	RiskDistinct7d      int
	HomeCountry         string
	Retention           time.Duration
	LoginMaxFailures    int
	LoginWindow         time.Duration
	TOTPPendingTTL      time.Duration
	TOTPMaxAttempts     int
	AuditMaxEvents      int64
	PasswordMinLength   int
	DefaultChartDays    int
	LogLevel            string
	Require2FA          bool
	Require2FAGraceDays int
	WebhookURL          string
	WebhookSecret       string
	WebhookEvents       string
	BackupSchedule      string
	BackupKeepDays      int
	BackupRestoreTest   bool
	BackupConfig        string
	GeoIPCity           string
	GeoIPASN            string
	BackupStatus        string
	TOTPKey             []byte
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
	cfg.LoginMaxFailures = getInt(get("TGADMIN_LOGIN_MAX_FAILURES", "10"), 10)
	cfg.TOTPMaxAttempts = getInt(get("TGADMIN_TOTP_MAX_ATTEMPTS", "5"), 5)
	cfg.AuditMaxEvents = int64(getInt(get("TGADMIN_AUDIT_MAX_EVENTS", "10000"), 10000))
	cfg.PasswordMinLength = getInt(get("TGADMIN_PASSWORD_MIN_LENGTH", "12"), 12)
	cfg.DefaultChartDays = getInt(get("TGADMIN_DEFAULT_CHART_DAYS", "30"), 30)
	cfg.Require2FA = strings.EqualFold(get("TGADMIN_REQUIRE_2FA", "false"), "true")
	cfg.Require2FAGraceDays = getInt(get("TGADMIN_REQUIRE_2FA_GRACE_DAYS", "7"), 7)
	if cfg.Require2FAGraceDays < 0 || cfg.Require2FAGraceDays > 30 {
		return nil, fmt.Errorf("TGADMIN_REQUIRE_2FA_GRACE_DAYS: от 0 до 30")
	}
	cfg.WebhookURL = get("TGADMIN_WEBHOOK_URL", "")
	cfg.WebhookSecret = getenv("TGADMIN_WEBHOOK_SECRET")
	cfg.WebhookEvents = get("TGADMIN_WEBHOOK_EVENTS", "user.create,user.rename,user.rotate,user.enable,user.disable,user.delete,settings.update,settings.backup")
	cfg.BackupSchedule = get("TGADMIN_BACKUP_SCHEDULE", "04:30")
	cfg.BackupKeepDays = getInt(get("TGADMIN_BACKUP_KEEP_DAYS", "14"), 14)
	cfg.BackupRestoreTest = get("TGADMIN_BACKUP_RESTORE_TEST", "true") != "false"
	cfg.BackupConfig = get("TGADMIN_BACKUP_CONFIG", "")
	if cfg.BackupKeepDays < 1 || cfg.BackupKeepDays > 365 {
		return nil, fmt.Errorf("TGADMIN_BACKUP_KEEP_DAYS: от 1 до 365")
	}
	cfg.LogLevel = strings.ToLower(get("TGADMIN_LOG_LEVEL", "info"))
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return nil, fmt.Errorf("TGADMIN_LOG_LEVEL %q: debug, info, warn или error", cfg.LogLevel)
	}
	if cfg.LoginMaxFailures < 1 || cfg.LoginMaxFailures > 1000 {
		return nil, fmt.Errorf("TGADMIN_LOGIN_MAX_FAILURES: от 1 до 1000")
	}
	if cfg.TOTPMaxAttempts < 1 || cfg.TOTPMaxAttempts > 20 {
		return nil, fmt.Errorf("TGADMIN_TOTP_MAX_ATTEMPTS: от 1 до 20")
	}
	if cfg.AuditMaxEvents < 100 || cfg.AuditMaxEvents > 1_000_000 {
		return nil, fmt.Errorf("TGADMIN_AUDIT_MAX_EVENTS: от 100 до 1000000")
	}
	if cfg.PasswordMinLength < 8 || cfg.PasswordMinLength > 128 {
		return nil, fmt.Errorf("TGADMIN_PASSWORD_MIN_LENGTH: от 8 до 128")
	}
	if cfg.DefaultChartDays < 7 || cfg.DefaultChartDays > 90 {
		return nil, fmt.Errorf("TGADMIN_DEFAULT_CHART_DAYS: от 7 до 90")
	}
	loginWindow, err := time.ParseDuration(get("TGADMIN_LOGIN_WINDOW", "15m"))
	if err != nil {
		return nil, fmt.Errorf("TGADMIN_LOGIN_WINDOW: %w", err)
	}
	cfg.LoginWindow = loginWindow
	totpTTL, err := time.ParseDuration(get("TGADMIN_TOTP_PENDING_TTL", "5m"))
	if err != nil {
		return nil, fmt.Errorf("TGADMIN_TOTP_PENDING_TTL: %w", err)
	}
	cfg.TOTPPendingTTL = totpTTL
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

package collector

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/McConfig/telemt-admin/internal/geoip"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

// Snapshot — последний срез по пользователю.
type Snapshot struct {
	User         string
	Current      int64
	Distinct24h  int64
	Distinct7d   int64
	New24h       int64
	MaxToday     int64
	TrafficTotal uint64
	TrafficToday uint64
	Risk         int
	// ForeignCurrent — сколько активных адресов из других стран.
	ForeignCurrent int64
	// Foreign24h — иностранные адреса за текущие сутки.
	Foreign24h int64
	// Countries24h и Countries7d — число разных стран.
	Countries24h int64
	Countries7d  int64
	// NewCountries24h — иностранные страны, которых раньше не было.
	NewCountries24h int64
	// TrustedActive — сколько активных адресов помечено доверенными.
	TrustedActive int64
	// Countries — уникальные страны среди активных адресов, по алфавиту.
	Countries []string
	// CountryCodes — коды ISO тех же стран.
	CountryCodes []string
}

// Collector периодически снимает активные IP и трафик через API telemt.
type Collector struct {
	st         *store.Store
	tc         *telemt.Client
	geo        *geoip.Resolver
	interval   time.Duration
	warn       int64
	high       int64
	distinct7d int64

	mu           sync.RWMutex
	snap         map[string]Snapshot
	lastOK       time.Time
	last         error
	lastBackfill time.Time
}

// New создаёт сборщик.
func New(st *store.Store, tc *telemt.Client, geo *geoip.Resolver, interval time.Duration, warn, high, distinct7d int64) *Collector {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Collector{
		st: st, tc: tc, geo: geo, interval: interval,
		warn: warn, high: high, distinct7d: distinct7d,
		snap: map[string]Snapshot{},
	}
}

// Run собирает данные до отмены контекста.
func (c *Collector) Run(ctx context.Context) {
	c.collect(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *Collector) collect(ctx context.Context) {
	users, err := c.tc.ListUsers(ctx)
	if err != nil {
		c.mu.Lock()
		c.last = err
		c.mu.Unlock()
		slog.Warn("сбор статистики IP", "err", err)
		return
	}
	now := time.Now().UTC()
	next := make(map[string]Snapshot, len(users))
	for _, u := range users {
		ips := u.ActiveUniqueIPList
		geo := c.lookupIPs(ips)
		if err := c.st.RecordIPObservation(ctx, u.Username, ips, geo, u.TotalOctets, now); err != nil {
			slog.Warn("запись истории IP", "user", u.Username, "err", err)
			continue
		}
		stats, err := c.st.UserIPStats(ctx, u.Username, now)
		if err != nil {
			slog.Warn("агрегаты IP", "user", u.Username, "err", err)
			continue
		}
		trusted, err := c.st.GetTrusted(ctx, u.Username)
		if err != nil {
			slog.Warn("доверенные адреса", "user", u.Username, "err", err)
		}
		trustedCountries := map[string]bool{}
		for _, code := range trusted.Countries {
			trustedCountries[code] = true
		}
		trustedIPs := map[string]bool{}
		for _, ip := range trusted.IPs {
			trustedIPs[ip] = true
		}
		newCountries := int64(0)
		for _, code := range stats.NewCountryCodes {
			if !trustedCountries[code] {
				newCountries++
			}
		}
		seenCountry := map[string]string{}
		seenOperator := map[string]bool{}
		snap := Snapshot{
			User: u.Username, Current: int64(len(ips)),
			Distinct24h: stats.Distinct24h, Distinct7d: stats.Distinct7d,
			New24h: stats.New24h, MaxToday: stats.MaxToday,
			TrafficTotal: u.TotalOctets, TrafficToday: stats.TrafficToday,
			Foreign24h: stats.Foreign24h, Countries24h: stats.Countries24h,
			Countries7d: stats.Countries7d, NewCountries24h: newCountries,
		}
		for _, ip := range ips {
			if trustedIPs[ip] {
				snap.TrustedActive++
			}
		}
		riskCfg, _, err := c.st.GetRisk(ctx, u.Username)
		if err != nil {
			slog.Warn("пороги риска", "user", u.Username, "err", err)
		}
		snap.Risk = c.risk(snap, riskCfg)
		for _, info := range geo {
			if info.Internal {
				continue
			}
			if info.Country != "" && info.Country != "RU" {
				snap.ForeignCurrent++
			}
			if info.Country != "" {
				seenCountry[info.Country] = info.CountryName
			}
			if info.ASNOrg != "" {
				seenOperator[info.ASNOrg] = true
			}
		}
		for code, name := range seenCountry {
			snap.CountryCodes = append(snap.CountryCodes, code)
			snap.Countries = append(snap.Countries, name)
		}
		sort.Strings(snap.CountryCodes)
		sort.Strings(snap.Countries)
		operators := make([]string, 0, len(seenOperator))
		for name := range seenOperator {
			operators = append(operators, name)
		}
		sort.Strings(operators)
		if err := c.st.SaveGeoStats(ctx, u.Username, snap.ForeignCurrent, snap.CountryCodes, snap.Countries, operators); err != nil {
			slog.Warn("запись GeoIP-агрегатов", "user", u.Username, "err", err)
		}
		next[u.Username] = snap
	}
	c.mu.Lock()
	c.snap = next
	c.lastOK = time.Now().UTC()
	c.last = nil
	needBackfill := c.lastBackfill.IsZero() || time.Since(c.lastBackfill) > time.Hour
	if needBackfill {
		c.lastBackfill = time.Now()
	}
	c.mu.Unlock()

	if needBackfill {
		c.backfillGeo(ctx)
	}
}

// backfillGeo дописывает страну и оператора к адресам, которые попали в
// историю до подключения баз GeoIP. Запускается при старте и раз в час.
func (c *Collector) backfillGeo(ctx context.Context) {
	if !c.geo.Enabled() {
		return
	}
	users, err := c.tc.ListUsers(ctx)
	if err != nil {
		return
	}
	for _, u := range users {
		history, err := c.st.IPHistory(ctx, u.Username)
		if err != nil {
			continue
		}
		for _, item := range history {
			if item.Country != "" || item.Internal {
				continue
			}
			info := c.geo.Lookup(item.IP)
			if info.Empty() {
				continue
			}
			if err := c.st.SetIPGeo(ctx, u.Username, item.IP, store.IPGeo{
				Country: info.CountryCode, CountryName: info.CountryName,
				ASN: info.ASN, ASNOrg: info.ASNOrg, Internal: info.Internal,
			}); err != nil {
				slog.Warn("дозапись GeoIP", "user", u.Username, "ip", item.IP, "err", err)
			}
		}
	}
}

// lookupIPs обогащает адреса сведениями GeoIP. Без баз возвращает пустые
// сведения: история продолжает собираться, страны появятся после подключения.
func (c *Collector) lookupIPs(ips []string) map[string]store.IPGeo {
	out := make(map[string]store.IPGeo, len(ips))
	for _, ip := range ips {
		info := c.geo.Lookup(ip)
		out[ip] = store.IPGeo{
			Country: info.CountryCode, CountryName: info.CountryName,
			ASN: info.ASN, ASNOrg: info.ASNOrg, Internal: info.Internal,
		}
	}
	return out
}

func (c *Collector) risk(s Snapshot, cfg store.Risk) int {
	warn, high, distinct, newCountries := c.warn, c.high, c.distinct7d, true
	if cfg.WarnIPs != nil {
		warn = int64(*cfg.WarnIPs)
	}
	if cfg.HighIPs != nil {
		high = int64(*cfg.HighIPs)
	}
	if cfg.Distinct7d != nil {
		distinct = int64(*cfg.Distinct7d)
	}
	if cfg.NewCountries != nil {
		newCountries = *cfg.NewCountries
	}
	current := s.Current - s.TrustedActive
	switch {
	case current > high:
		return 2
	case current > warn || s.Distinct7d > distinct, newCountries && s.NewCountries24h > 0:
		return 1
	default:
		return 0
	}
}

// Status — состояние сборщика для страницы со сводкой.
type Status struct {
	Up          bool
	LastSuccess time.Time
	LastError   string
	GeoIPLoaded bool
	GeoIPBuilt  uint64
}

// Status возвращает текущее состояние сборщика.
func (c *Collector) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := Status{
		Up: c.last == nil && !c.lastOK.IsZero(), LastSuccess: c.lastOK,
		GeoIPLoaded: c.geo.Enabled(), GeoIPBuilt: c.geo.BuildEpoch(),
	}
	if c.last != nil {
		out.LastError = c.last.Error()
	}
	return out
}

// Snapshot возвращает копию последнего среза.
func (c *Collector) Snapshot() map[string]Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]Snapshot, len(c.snap))
	for k, v := range c.snap {
		out[k] = v
	}
	return out
}

// Stats возвращает срез по одному пользователю.
func (c *Collector) Stats(user string) (Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.snap[user]
	return s, ok
}

// ServeHTTP отдаёт метрики Prometheus.
func (c *Collector) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	c.mu.RLock()
	snap := make(map[string]Snapshot, len(c.snap))
	for k, v := range c.snap {
		snap[k] = v
	}
	lastOK := c.lastOK
	lastErr := c.last
	c.mu.RUnlock()

	names := make([]string, 0, len(snap))
	for name := range snap {
		names = append(names, name)
	}
	sort.Strings(names)

	writeMetric := func(name, help, typ string) {
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, typ)
	}
	writeMetric("tgadmin_user_ips_current", "Активные уникальные IP пользователя", "gauge")
	writeMetric("tgadmin_user_ips_distinct_24h", "Уникальные IP за 24 часа", "gauge")
	writeMetric("tgadmin_user_ips_distinct_7d", "Уникальные IP за 7 дней", "gauge")
	writeMetric("tgadmin_user_ips_new_24h", "Новые IP за 24 часа", "gauge")
	writeMetric("tgadmin_user_ips_max_today", "Пик одновременных IP за сутки", "gauge")
	writeMetric("tgadmin_user_traffic_bytes_total", "Суммарный трафик пользователя", "counter")
	writeMetric("tgadmin_user_traffic_today_bytes", "Трафик пользователя за сутки", "gauge")
	writeMetric("tgadmin_user_risk_level", "0 норма, 1 предупреждение, 2 высокий риск", "gauge")
	writeMetric("tgadmin_user_foreign_ips_current", "Активные адреса пользователя из других стран", "gauge")
	writeMetric("tgadmin_user_countries_current", "Стран среди активных адресов пользователя", "gauge")
	writeMetric("tgadmin_user_foreign_ips_24h", "Иностранные адреса пользователя за сутки", "gauge")
	writeMetric("tgadmin_user_countries_24h", "Стран у адресов пользователя за сутки", "gauge")
	writeMetric("tgadmin_user_countries_7d", "Стран у адресов пользователя за 7 дней", "gauge")
	writeMetric("tgadmin_user_new_countries_24h", "Новые иностранные страны за сутки", "gauge")
	for _, name := range names {
		s := snap[name]
		label := escape(name)
		_, _ = fmt.Fprintf(w, "tgadmin_user_ips_current{user=%q} %d\n", label, s.Current)
		_, _ = fmt.Fprintf(w, "tgadmin_user_ips_distinct_24h{user=%q} %d\n", label, s.Distinct24h)
		_, _ = fmt.Fprintf(w, "tgadmin_user_ips_distinct_7d{user=%q} %d\n", label, s.Distinct7d)
		_, _ = fmt.Fprintf(w, "tgadmin_user_ips_new_24h{user=%q} %d\n", label, s.New24h)
		_, _ = fmt.Fprintf(w, "tgadmin_user_ips_max_today{user=%q} %d\n", label, s.MaxToday)
		_, _ = fmt.Fprintf(w, "tgadmin_user_traffic_bytes_total{user=%q} %d\n", label, s.TrafficTotal)
		_, _ = fmt.Fprintf(w, "tgadmin_user_traffic_today_bytes{user=%q} %d\n", label, s.TrafficToday)
		_, _ = fmt.Fprintf(w, "tgadmin_user_risk_level{user=%q} %d\n", label, s.Risk)
		_, _ = fmt.Fprintf(w, "tgadmin_user_foreign_ips_current{user=%q} %d\n", label, s.ForeignCurrent)
		_, _ = fmt.Fprintf(w, "tgadmin_user_countries_current{user=%q} %d\n", label, len(s.Countries))
		_, _ = fmt.Fprintf(w, "tgadmin_user_foreign_ips_24h{user=%q} %d\n", label, s.Foreign24h)
		_, _ = fmt.Fprintf(w, "tgadmin_user_countries_24h{user=%q} %d\n", label, s.Countries24h)
		_, _ = fmt.Fprintf(w, "tgadmin_user_countries_7d{user=%q} %d\n", label, s.Countries7d)
		_, _ = fmt.Fprintf(w, "tgadmin_user_new_countries_24h{user=%q} %d\n", label, s.NewCountries24h)
	}
	writeMetric("tgadmin_geoip_loaded", "Загружены базы GeoIP", "gauge")
	geoLoaded := 0
	if c.geo.Enabled() {
		geoLoaded = 1
	}
	_, _ = fmt.Fprintf(w, "tgadmin_geoip_loaded %d\n", geoLoaded)
	writeMetric("tgadmin_geoip_build_timestamp_seconds", "Время сборки баз GeoIP", "gauge")
	if epoch := c.geo.BuildEpoch(); epoch > 0 {
		_, _ = fmt.Fprintf(w, "tgadmin_geoip_build_timestamp_seconds %d\n", epoch)
	}
	writeMetric("tgadmin_collector_up", "Сборщик получает данные", "gauge")
	up := 0
	if lastErr == nil {
		up = 1
	}
	_, _ = fmt.Fprintf(w, "tgadmin_collector_up %d\n", up)
	writeMetric("tgadmin_collector_last_success_timestamp_seconds", "Время последнего успешного сбора", "gauge")
	if !lastOK.IsZero() {
		_, _ = fmt.Fprintf(w, "tgadmin_collector_last_success_timestamp_seconds %d\n", lastOK.Unix())
	}
}

func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

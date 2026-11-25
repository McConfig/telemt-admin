// Package settings хранит реестр глобальных настроек и переопределения
// из Valkey. Переменные окружения остаются значениями по умолчанию.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
)

// Kind — тип значения настройки.
type Kind string

const (
	KindInt      Kind = "int"
	KindBool     Kind = "bool"
	KindDuration Kind = "duration"
	KindString   Kind = "string"
	KindEnum     Kind = "enum"
)

// Status — доступность настройки.
type Status string

const (
	Active   Status = "active"
	Readonly Status = "readonly"
	Planned  Status = "planned"
)

// Option — вариант enum.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Definition — описание настройки.
type Definition struct {
	Key            string
	Section        string
	Title          string
	Description    string
	Kind           Kind
	Status         Status
	PlannedRelease string
	Restart        bool
	Secret         bool
	Default        any
	Min, Max       *int
	MinDur, MaxDur *time.Duration
	Options        []Option
	ReadonlyValue  any
}

// Field — настройка с действующим значением.
type Field struct {
	Key            string   `json:"key"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Kind           Kind     `json:"kind"`
	Status         Status   `json:"status"`
	PlannedRelease string   `json:"plannedRelease,omitempty"`
	Restart        bool     `json:"restartRequired,omitempty"`
	Secret         bool     `json:"secret,omitempty"`
	Value          any      `json:"value,omitempty"`
	Default        any      `json:"default,omitempty"`
	Source         string   `json:"source,omitempty"`
	SecretSet      bool     `json:"secretSet,omitempty"`
	Min            *int     `json:"min,omitempty"`
	Max            *int     `json:"max,omitempty"`
	MinDur         string   `json:"minDuration,omitempty"`
	MaxDur         string   `json:"maxDuration,omitempty"`
	Options        []Option `json:"options,omitempty"`
}

// Section — раздел страницы.
type Section struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Settings    []Field `json:"settings"`
}

// Catalog — ответ API.
type Catalog struct {
	Version  int64     `json:"version"`
	Sections []Section `json:"sections"`
}

// Change — изменение одного поля для аудита.
type Change struct {
	Key string
	Old string
	New string
}

// ValidationError — ошибка конкретного поля.
type ValidationError struct {
	Key     string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

var countryRe = regexp.MustCompile(`^[A-Z]{2}$`)

var sectionDefs = []Section{
	{ID: "risk", Title: "Риск", Description: "Глобальные пороги и домашняя страна"},
	{ID: "collect", Title: "Сбор и хранение", Description: "Интервал сбора, срок хранения и сессии"},
	{ID: "links", Title: "Ссылки", Description: "Режим ссылок tg://"},
	{ID: "security", Title: "Безопасность", Description: "Входы, TOTP, пароли и аудит"},
	{ID: "interface", Title: "Интерфейс", Description: "Параметры отображения"},
	{ID: "logging", Title: "Логирование", Description: "Уровень логов"},
	{ID: "notifications", Title: "Уведомления", Description: "Вебхуки и события"},
	{ID: "backups", Title: "Бэкапы: будущие настройки", Description: "Расписание, хранение и проверка"},
	{ID: "infrastructure", Title: "Инфраструктура", Description: "Сеть, пути и секреты, только чтение"},
}

// Service — чтение, валидация и сохранение настроек.
type Service struct {
	cfg  *config.Config
	st   *store.Store
	defs []Definition
	by   map[string]Definition

	mu       sync.RWMutex
	override map[string]string
	version  int64
	loadedAt time.Time
}

// New создаёт сервис. Значения из Valkey подтягиваются лениво.
func New(cfg *config.Config, st *store.Store) *Service {
	s := &Service{cfg: cfg, st: st, override: map[string]string{}}
	s.build()
	return s
}

// Load перечитывает переопределения из Valkey.
func (s *Service) Load(ctx context.Context) error {
	overrides, version, err := s.st.GlobalSettings(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.override = overrides
	if s.override == nil {
		s.override = map[string]string{}
	}
	s.version = version
	s.loadedAt = time.Now()
	s.mu.Unlock()
	return nil
}

func (s *Service) ensure(ctx context.Context) {
	s.mu.RLock()
	fresh := !s.loadedAt.IsZero() && time.Since(s.loadedAt) < 2*time.Second
	s.mu.RUnlock()
	if fresh {
		return
	}
	_ = s.Load(ctx)
}

// Version возвращает текущую версию настроек.
func (s *Service) Version(ctx context.Context) int64 {
	s.ensure(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// Catalog собирает полный каталог с действующими значениями.
func (s *Service) Catalog(ctx context.Context) Catalog {
	s.ensure(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.catalogLocked()
}

func (s *Service) catalogLocked() Catalog {
	sections := make([]Section, 0, len(sectionDefs))
	for _, meta := range sectionDefs {
		section := Section{ID: meta.ID, Title: meta.Title, Description: meta.Description}
		for _, d := range s.defs {
			if d.Section != meta.ID {
				continue
			}
			field := Field{
				Key: d.Key, Title: d.Title, Description: d.Description,
				Kind: d.Kind, Status: d.Status, PlannedRelease: d.PlannedRelease,
				Restart: d.Restart, Secret: d.Secret, Min: d.Min, Max: d.Max, Options: d.Options,
			}
			if d.MinDur != nil {
				field.MinDur = d.MinDur.String()
			}
			if d.MaxDur != nil {
				field.MaxDur = d.MaxDur.String()
			}
			switch d.Status {
			case Active:
				value, source := s.effectiveLocked(d)
				field.Value = displayValue(d, value)
				field.Source = source
				field.Default = displayValue(d, d.Default)
			case Readonly:
				field.Value = displayValue(d, d.ReadonlyValue)
				field.Source = "env"
			case Planned:
				field.Default = displayValue(d, d.Default)
			}
			if d.Secret {
				field.Value = nil
				field.SecretSet = secretPresent(d.Default)
				field.Source = "env"
			}
			section.Settings = append(section.Settings, field)
		}
		sections = append(sections, section)
	}
	return Catalog{Version: s.version, Sections: sections}
}

func displayValue(d Definition, v any) any {
	if d.Kind == KindDuration {
		if dur, ok := v.(time.Duration); ok {
			return dur.String()
		}
	}
	return v
}

func secretPresent(v any) bool {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value) != ""
	case bool:
		return value
	case []byte:
		return len(value) > 0
	}
	return false
}

func (s *Service) effectiveLocked(d Definition) (any, string) {
	if raw, ok := s.override[d.Key]; ok {
		if value, err := decodeCanonical(d, raw); err == nil {
			return value, "settings"
		}
	}
	return d.Default, "env"
}

// Save проверяет и атомарно сохраняет изменения.
func (s *Service) Save(
	ctx context.Context,
	expected int64,
	values map[string]json.RawMessage,
	reset []string,
) (Catalog, []Change, error) {
	s.ensure(ctx)

	set := make(map[string]string, len(values))
	changes := make([]Change, 0, len(values)+len(reset))

	var prepareErr error
	s.mu.RLock()
	prepareErr = func() error {
		for key, raw := range values {
			d, ok := s.by[key]
			if !ok {
				return &ValidationError{Key: key, Message: "неизвестная настройка"}
			}
			if d.Status != Active {
				return &ValidationError{Key: key, Message: "настройка недоступна для изменения"}
			}
			canonical, value, err := parseRaw(d, raw)
			if err != nil {
				return &ValidationError{Key: key, Message: err.Error()}
			}
			old, _ := s.effectiveLocked(d)
			oldCanonical, _ := canonicalValue(d, old)
			set[key] = canonical
			changes = append(changes, Change{Key: key, Old: oldCanonical, New: canonicalString(d, value)})
		}
		for _, key := range reset {
			d, ok := s.by[key]
			if !ok {
				return &ValidationError{Key: key, Message: "неизвестная настройка"}
			}
			if d.Status != Active {
				return &ValidationError{Key: key, Message: "настройка недоступна для сброса"}
			}
			if _, exists := s.override[key]; !exists {
				continue
			}
			old, _ := s.effectiveLocked(d)
			oldCanonical, _ := canonicalValue(d, old)
			defaultCanonical, _ := canonicalValue(d, d.Default)
			changes = append(changes, Change{Key: key, Old: oldCanonical, New: defaultCanonical})
		}
		return s.validateRiskLocked(set, reset)
	}()
	s.mu.RUnlock()
	if prepareErr != nil {
		return Catalog{}, nil, prepareErr
	}
	if len(set) == 0 && len(reset) == 0 {
		return s.Catalog(ctx), nil, nil
	}

	version, err := s.st.SaveGlobalSettings(ctx, expected, set, reset)
	if err != nil {
		return Catalog{}, nil, err
	}

	s.mu.Lock()
	for key, value := range set {
		s.override[key] = value
	}
	for _, key := range reset {
		delete(s.override, key)
	}
	s.version = version
	s.loadedAt = time.Now()
	catalog := s.catalogLocked()
	s.mu.Unlock()

	// Домашняя страна влияет на запись новых наблюдений, поэтому применяем
	// её к store сразу, не дожидаясь перезапуска.
	homeChanged := false
	if _, ok := set["risk.home_country"]; ok {
		homeChanged = true
	}
	for _, key := range reset {
		if key == "risk.home_country" {
			homeChanged = true
		}
	}
	if homeChanged {
		s.st.SetHomeCountry(s.HomeCountry(ctx))
	}

	return catalog, changes, nil
}

func (s *Service) validateRiskLocked(set map[string]string, reset []string) error {
	warn := s.intLocked("risk.warn_ips", set, reset)
	high := s.intLocked("risk.high_ips", set, reset)
	if warn >= high {
		return &ValidationError{
			Key:     "risk.high_ips",
			Message: "порог высокого риска должен быть больше порога предупреждения",
		}
	}
	return nil
}

func (s *Service) intLocked(key string, set map[string]string, reset []string) int {
	d := s.by[key]
	for _, r := range reset {
		if r == key {
			return d.Default.(int)
		}
	}
	if raw, ok := set[key]; ok {
		value, err := decodeCanonical(d, raw)
		if err == nil {
			return value.(int)
		}
	}
	if raw, ok := s.override[key]; ok {
		if value, err := decodeCanonical(d, raw); err == nil {
			return value.(int)
		}
	}
	return d.Default.(int)
}

// RiskValues возвращает действующие значения для расчёта риска.
func (s *Service) RiskValues(ctx context.Context) (warn, high, distinct int, newCountries bool, home string) {
	s.ensure(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	warn = s.intValue("risk.warn_ips")
	high = s.intValue("risk.high_ips")
	distinct = s.intValue("risk.distinct_7d")
	newCountries = s.boolValue("risk.new_countries")
	home = s.stringValue("risk.home_country")
	return warn, high, distinct, newCountries, home
}

// SessionTTL — срок жизни новой сессии.
func (s *Service) SessionTTL(ctx context.Context) time.Duration {
	s.ensure(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.durationValue("session.ttl")
}

// LinkMode — режим ссылок по умолчанию.
func (s *Service) LinkMode(ctx context.Context) string {
	s.ensure(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stringValue("links.mode")
}

// CollectInterval — интервал сбора при старте.
func (s *Service) CollectInterval(ctx context.Context) time.Duration {
	s.ensure(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.durationValue("collect.interval")
}

// Retention — срок хранения истории при старте.
func (s *Service) Retention(ctx context.Context) time.Duration {
	s.ensure(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.durationValue("retention")
}

// HomeCountry — домашняя страна.
func (s *Service) HomeCountry(ctx context.Context) string {
	s.ensure(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stringValue("risk.home_country")
}

func (s *Service) intValue(key string) int {
	if value, ok := s.typedLocked(key).(int); ok {
		return value
	}
	return 0
}

func (s *Service) boolValue(key string) bool {
	if value, ok := s.typedLocked(key).(bool); ok {
		return value
	}
	return false
}

func (s *Service) stringValue(key string) string {
	if value, ok := s.typedLocked(key).(string); ok {
		return value
	}
	return ""
}

func (s *Service) durationValue(key string) time.Duration {
	if value, ok := s.typedLocked(key).(time.Duration); ok {
		return value
	}
	return 0
}

func (s *Service) typedLocked(key string) any {
	d := s.by[key]
	value, _ := s.effectiveLocked(d)
	return value
}

// Definitions возвращает реестр без значений.
func (s *Service) Definitions() []Definition {
	out := make([]Definition, len(s.defs))
	copy(out, s.defs)
	return out
}

// IsActive сообщает, можно ли менять ключ.
func (s *Service) IsActive(key string) bool {
	d, ok := s.by[key]
	return ok && d.Status == Active
}

func (s *Service) build() {
	cfg := s.cfg
	add := func(d Definition) {
		if d.Section == "" {
			panic("settings: у настройки нет раздела: " + d.Key)
		}
		if _, exists := s.by[d.Key]; exists {
			panic("settings: дубликат ключа: " + d.Key)
		}
		s.defs = append(s.defs, d)
		s.by[d.Key] = d
	}
	s.by = map[string]Definition{}
	intp := func(v int) *int { return &v }
	durp := func(v time.Duration) *time.Duration { return &v }

	add(Definition{
		Key: "risk.warn_ips", Section: "risk", Title: "Предупреждение, больше IP",
		Description: "Порог предупреждения по числу активных адресов",
		Kind:        KindInt, Status: Active, Default: cfg.RiskIPsWarn, Min: intp(0), Max: intp(1000),
	})
	add(Definition{
		Key: "risk.high_ips", Section: "risk", Title: "Высокий риск, больше IP",
		Description: "Порог высокого риска по числу активных адресов",
		Kind:        KindInt, Status: Active, Default: cfg.RiskIPsHigh, Min: intp(0), Max: intp(1000),
	})
	add(Definition{
		Key: "risk.distinct_7d", Section: "risk", Title: "Адресов за 7 дней",
		Description: "Порог уникальных адресов за семь дней",
		Kind:        KindInt, Status: Active, Default: cfg.RiskDistinct7d, Min: intp(0), Max: intp(10000),
	})
	add(Definition{
		Key: "risk.home_country", Section: "risk", Title: "Домашняя страна",
		Description: "Страна, адреса которой не считаются иностранными",
		Kind:        KindString, Status: Active, Default: cfg.HomeCountry,
	})
	add(Definition{
		Key: "risk.new_countries", Section: "risk", Title: "Новая страна считается риском",
		Description: "Глобальное значение по умолчанию, на клиента можно переопределить",
		Kind:        KindBool, Status: Active, Default: true,
	})
	add(Definition{
		Key: "collect.interval", Section: "collect", Title: "Интервал сбора",
		Description: "Как часто опрашивать Control API telemt", Kind: KindDuration,
		Status: Active, Default: cfg.CollectInterval, Restart: true,
		MinDur: durp(10 * time.Second), MaxDur: durp(time.Hour),
	})
	add(Definition{
		Key: "retention", Section: "collect", Title: "Срок хранения истории",
		Description: "История адресов, стран и трафика", Kind: KindDuration,
		Status: Active, Default: cfg.Retention, Restart: true,
		MinDur: durp(7 * 24 * time.Hour), MaxDur: durp(90 * 24 * time.Hour),
	})
	add(Definition{
		Key: "session.ttl", Section: "collect", Title: "Срок жизни сессии",
		Description: "Применяется к новым входам", Kind: KindDuration,
		Status: Active, Default: cfg.SessionTTL,
		MinDur: durp(time.Hour), MaxDur: durp(30 * 24 * time.Hour),
	})
	add(Definition{
		Key: "links.mode", Section: "links", Title: "Режим ссылок",
		Description: "Какая ссылка tg:// показывается первой", Kind: KindEnum,
		Status: Active, Default: cfg.LinkMode,
		Options: []Option{{Value: "tls", Label: "TLS"}, {Value: "secure", Label: "Secure"}, {Value: "classic", Label: "Classic"}},
	})

	planned := func(key, section, title, desc string, kind Kind, release string, def any) {
		add(Definition{Key: key, Section: section, Title: title, Description: desc,
			Kind: kind, Status: Planned, PlannedRelease: release, Default: def})
	}
	planned("login.max_failures", "security", "Неудачных входов до блокировки", "Сейчас жёстко 10 за 15 минут", KindInt, "1.4.0", 10)
	planned("login.window", "security", "Окно подсчёта входов", "Сейчас 15 минут", KindDuration, "1.4.0", 15*time.Minute)
	planned("totp.pending_ttl", "security", "Ожидание второго фактора", "Сейчас 5 минут", KindDuration, "1.4.0", 5*time.Minute)
	planned("totp.max_attempts", "security", "Попыток ввода TOTP", "Сейчас 5", KindInt, "1.4.0", 5)
	planned("audit.max_events", "security", "Размер аудита", "Сейчас 10 000 событий", KindInt, "1.4.0", 10000)
	planned("password.min_length", "security", "Минимальная длина пароля", "Сейчас 12 символов", KindInt, "1.4.0", 12)
	planned("security.require_2fa", "security", "Обязательный 2FA для админов", "Политика для всех администраторов", KindBool, "1.5.0", false)
	planned("ui.default_chart_days", "interface", "Период графиков по умолчанию", "Сейчас 30 дней", KindInt, "1.4.0", 30)
	planned("log.level", "logging", "Уровень логирования", "Сейчас info, нужен перезапуск", KindEnum, "1.4.0", "info")
	planned("notifications.webhook_url", "notifications", "URL вебхука", "Доставка событий во внешнюю систему", KindString, "1.5.0", "")
	planned("notifications.webhook_secret", "notifications", "Секрет вебхука", "HMAC-подпись запросов", KindString, "1.5.0", "")
	planned("notifications.events", "notifications", "События", "Какие события отправлять", KindString, "1.5.0", "")
	planned("backup.schedule", "backups", "Расписание", "Сейчас 04:30 на хосте", KindString, "1.5.0", "04:30")
	planned("backup.keep_days", "backups", "Срок хранения снимков", "Сейчас 14 дней на хосте", KindInt, "1.5.0", 14)
	planned("backup.restore_test", "backups", "Проверка восстановления", "Сейчас включена на хосте", KindBool, "1.5.0", true)
	planned("backup.offsite", "backups", "Копия вне хоста", "Копии вне хоста пока нет", KindBool, "1.6.0", false)

	readonly := func(key, section, title, desc string, kind Kind, value any) {
		add(Definition{Key: key, Section: section, Title: title, Description: desc,
			Kind: kind, Status: Readonly, ReadonlyValue: value})
	}
	readonly("network.allowed_cidrs", "infrastructure", "Разрешённые сети", "Меняется только через TGADMIN_ALLOWED_CIDRS", KindString, prefixes(cfg))
	readonly("network.trusted_proxies", "infrastructure", "Доверенные прокси", "Меняется только через TGADMIN_TRUSTED_PROXIES", KindString, trustedProxies(cfg))
	readonly("app.base_url", "infrastructure", "Base URL", "Влияет на cookie, CSRF и Origin", KindString, cfg.BaseURL.String())
	readonly("app.cookie_secure", "infrastructure", "Cookie Secure", "Меняется только через TGADMIN_COOKIE_SECURE", KindBool, cfg.CookieSecure)
	readonly("app.listen", "infrastructure", "Адрес приложения", "Меняется только через TGADMIN_LISTEN", KindString, cfg.Listen)
	readonly("geoip.city_path", "infrastructure", "Городская база GeoIP", "Меняется через env, нужен перезапуск", KindString, cfg.GeoIPCity)
	readonly("geoip.asn_path", "infrastructure", "База ASN GeoIP", "Меняется через env, нужен перезапуск", KindString, cfg.GeoIPASN)
	readonly("valkey.url", "infrastructure", "URL Valkey", "Инфраструктурный параметр", KindString, cfg.ValkeyURL)
	readonly("telemt.url", "infrastructure", "URL telemt", "Инфраструктурный параметр", KindString, cfg.TelemtURL)
	readonly("backup.status_path", "infrastructure", "Статус бэкапов", "Необязательный файл статуса хоста", KindString, cfg.BackupStatus)

	secret := func(key, title, value string) {
		add(Definition{Key: key, Section: "infrastructure", Title: title,
			Description: "Секрет, только признак «задан»", Kind: KindString,
			Status: Readonly, Secret: true, Default: value})
	}
	secret("valkey.password", "Пароль Valkey", cfg.ValkeyPassword)
	secret("telemt.token", "Токен telemt", cfg.TelemtToken)
	secret("totp.key", "Ключ TOTP", string(cfg.TOTPKey))
}

func prefixes(cfg *config.Config) string {
	parts := make([]string, 0, len(cfg.AllowedCIDRs))
	for _, p := range cfg.AllowedCIDRs {
		parts = append(parts, p.String())
	}
	return strings.Join(parts, ",")
}

func trustedProxies(cfg *config.Config) string {
	parts := make([]string, 0, len(cfg.TrustedProxies))
	for _, p := range cfg.TrustedProxies {
		parts = append(parts, p.String())
	}
	return strings.Join(parts, ",")
}

func parseRaw(d Definition, raw json.RawMessage) (string, any, error) {
	switch d.Kind {
	case KindInt:
		var n int
		if err := json.Unmarshal(raw, &n); err != nil {
			return "", nil, errors.New("нужно целое число")
		}
		if d.Min != nil && n < *d.Min {
			return "", nil, fmt.Errorf("минимум %d", *d.Min)
		}
		if d.Max != nil && n > *d.Max {
			return "", nil, fmt.Errorf("максимум %d", *d.Max)
		}
		return strconv.Itoa(n), n, nil
	case KindBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return "", nil, errors.New("нужно true или false")
		}
		return canonicalString(d, b), b, nil
	case KindDuration:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", nil, errors.New("нужна длительность, например 720h")
		}
		dur, err := time.ParseDuration(strings.TrimSpace(s))
		if err != nil {
			return "", nil, errors.New("некорректная длительность")
		}
		if d.MinDur != nil && dur < *d.MinDur {
			return "", nil, fmt.Errorf("минимум %s", d.MinDur)
		}
		if d.MaxDur != nil && dur > *d.MaxDur {
			return "", nil, fmt.Errorf("максимум %s", d.MaxDur)
		}
		return canonicalString(d, dur), dur, nil
	case KindEnum:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", nil, errors.New("нужно строковое значение")
		}
		for _, o := range d.Options {
			if o.Value == s {
				return s, s, nil
			}
		}
		return "", nil, errors.New("недопустимое значение")
	default:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", nil, errors.New("нужна строка")
		}
		s = strings.TrimSpace(s)
		if d.Key == "risk.home_country" {
			s = strings.ToUpper(s)
			if !countryRe.MatchString(s) {
				return "", nil, errors.New("нужен код страны из двух букв")
			}
		}
		return s, s, nil
	}
}

func decodeCanonical(d Definition, raw string) (any, error) {
	switch d.Kind {
	case KindInt:
		return strconv.Atoi(raw)
	case KindBool:
		return raw == "1" || raw == "true", nil
	case KindDuration:
		return time.ParseDuration(raw)
	default:
		return raw, nil
	}
}

func canonicalString(d Definition, value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "1"
		}
		return "0"
	case int:
		return strconv.Itoa(v)
	case time.Duration:
		return v.String()
	case string:
		return v
	}
	return ""
}

func canonicalValue(d Definition, value any) (string, bool) {
	if value == nil {
		return "", false
	}
	return canonicalString(d, value), true
}

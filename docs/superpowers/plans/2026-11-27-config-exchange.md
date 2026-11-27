# Обмен конфигурацией и редактор прокси — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить экспорт и импорт конфигурации в YAML с предпросмотром и выбором блоков, а также редактор конфигурации telemt через Control API.

**Architecture:** Новый пакет `internal/configx` собирает бандл, строит план импорта и применяет его. Настройки берутся из `internal/settings`, данные — из `internal/store`, конфигурация прокси — из `internal/telemt` через новые методы Control API. Веб-слой добавляет административные маршруты, повторную аутентификацию для секретов и аудит. Фронтенд добавляет карточки в «Бэкапы» и вкладку «Прокси».

**Tech Stack:** Go 1.24, `go.yaml.in/yaml/v3`, Valkey, `github.com/go-chi/chi/v5`, React 19, Gravity UI, Vite, Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-11-27-config-exchange-design.md`

## Global Constraints

- Целевая версия — v2.0.0.
- Версия формата — `apiVersion: tgadmin.telemt/v1`, `kind: InstallationBundle`.
- Секреты по умолчанию не экспортируются; включение требует пароль и TOTP-код, если TOTP включён.
- Максимальный размер YAML — 4 МиБ; строгий парсер без алиасов, якорей и дублей ключей.
- Все маршруты `/api/config/*` и `/api/proxy/*` — только администратор с cookie-сессией; сервисный токен получает `403`.
- Удаление отсутствующих сущностей — только при `deleteMissing=true` и `confirmName == metadata.name`.
- Нельзя удалить, понизить или отключить текущего и последнего активного администратора.
- TOTP-секреты и passkeys не переносятся.
- Лимиты: 10 экспортов и 5 precheck/apply в минуту на пользователя.
- Секреты не попадают в логи, аудит, diff и тексты ошибок.
- Секции telemt: `general` (включая `general.links` и `general.modes`), `timeouts`, `censorship`, `upstreams`, `dc_overrides`, `web`, `network`, `server` (только `listeners` и observability-поля `api`).

## Review Focus

- YAML с дублями ключей, алиасами или неизвестными полями должен отвергаться, а не разбираться частично.
- Секреты не должны появляться в diff, ошибках, логах и аудите даже при `secrets: included`.
- Изменение ревизии между precheck и apply должно давать `stale_plan`, а не применять устаревший план.
- Импорт не должен оставить установку без администратора или перезаписать пароль текущего пользователя.
- Частичное применение не должно ломать повторный импорт; повтор с новым precheck идемпотентен.
- Форма прокси должна корректно работать с вложенными `general.links` и `general.modes`, а не ждать верхнеуровневых `links`/`modes`.

---

## Файловая структура

Создать:

- `internal/configx/bundle.go` — типы бандла и константы.
- `internal/configx/codec.go` — строгий YAML.
- `internal/configx/export.go` — сборка бандла.
- `internal/configx/plan.go` — типы плана и diff.
- `internal/configx/precheck.go` — построение плана.
- `internal/configx/apply.go` — применение плана.
- `internal/configx/service.go` — зависимости и конструктор.
- `internal/configx/*_test.go`.
- `internal/store/config_plans.go` — временные планы в Valkey.
- `internal/telemt/config.go` — методы конфигурации Control API.
- `internal/web/api_config.go` — экспорт, precheck, apply.
- `internal/web/api_proxy.go` — схема, конфиг, валидация, применение.
- `internal/web/config_guard.go` — лимиты частоты.
- `frontend/src/pages/ConfigExchange.tsx` — карточки экспорта и импорта.
- `frontend/src/pages/ProxyPanel.tsx` — редактор прокси.
- `frontend/src/pages/ConfigExchange.test.tsx`.
- `frontend/src/pages/ProxyPanel.test.tsx`.
- `frontend/e2e/config-exchange.spec.ts`.
- `frontend/e2e/proxy.spec.ts`.
- `docs/adr/0012-config-exchange.md`.

Изменить:

- `internal/settings/settings.go` — экспорт значений, валидация без записи.
- `internal/telemt/client.go` — доступ к ревизии конверта.
- `internal/store/service_tokens.go` — экспорт и импорт хешей токенов.
- `internal/web/server.go` — поле `configx`, маршруты.
- `frontend/src/api.ts` — типы и методы.
- `frontend/src/pages/SettingsPage.tsx` — вкладка «Прокси» и панель обмена в «Бэкапах».
- `deploy/demo/fake-telemt/main.go` — эндпоинты конфигурации.
- `frontend/e2e/mock.ts` — моки.
- `README.md`, `README.en.md`, `CHANGELOG.md`, `docs/API.md`, `docs/ARCHITECTURE.md`, `docs/UPGRADE.md`, `deploy/demo/README.md`.
- `deploy/demo/.env.example`, `deploy/demo/docker-compose.yml`, `deploy/docker-compose.yml`.

Удалить: ничего.

---

## Task 1: Временные планы импорта в Valkey

**Files:**
- Create: `internal/store/config_plans.go`
- Test: `internal/store/config_plans_test.go`

**Interfaces:**
- Produces: `store.ConfigPlan{ID, Owner, Session string; Payload []byte; CreatedAt time.Time}`,
  `PutConfigPlan(ctx, p, ttl) error`, `GetConfigPlan(ctx, id) (*ConfigPlan, error)`,
  `TakeConfigPlan(ctx, id) (*ConfigPlan, error)`. Ошибка отсутствия — `store.ErrNotFound`.

- [ ] **Step 1: Написать падающий тест**

```go
func TestConfigPlanLifecycle(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	p := ConfigPlan{ID: "abc", Owner: "demo", Session: "sess", Payload: []byte(`{"x":1}`)}
	if err := st.PutConfigPlan(ctx, p, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetConfigPlan(ctx, "abc")
	if err != nil || got.Owner != "demo" || string(got.Payload) != `{"x":1}` {
		t.Fatalf("%+v %v", got, err)
	}
	taken, err := st.TakeConfigPlan(ctx, "abc")
	if err != nil || taken.ID != "abc" {
		t.Fatalf("%+v %v", taken, err)
	}
	if _, err := st.TakeConfigPlan(ctx, "abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("повторный Take: %v", err)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/store -run TestConfigPlanLifecycle -v`
Expected: FAIL, `undefined: ConfigPlan`.

- [ ] **Step 3: Реализовать**

```go
package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type ConfigPlan struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	Session   string    `json:"session"`
	Payload   []byte    `json:"payload"`
	CreatedAt time.Time `json:"createdAt"`
}

func configPlanKey(id string) string { return "config:plan:" + id }

var takeConfigPlanScript = redis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return nil end
redis.call('DEL', KEYS[1])
return raw
`)

func (s *Store) PutConfigPlan(ctx context.Context, p ConfigPlan, ttl time.Duration) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, configPlanKey(p.ID), raw, ttl).Err()
}

func (s *Store) GetConfigPlan(ctx context.Context, id string) (*ConfigPlan, error) {
	raw, err := s.rdb.Get(ctx, configPlanKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var p ConfigPlan
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, ErrNotFound
	}
	return &p, nil
}

func (s *Store) TakeConfigPlan(ctx context.Context, id string) (*ConfigPlan, error) {
	raw, err := takeConfigPlanScript.Run(ctx, s.rdb, []string{configPlanKey(id)}).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var p ConfigPlan
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, ErrNotFound
	}
	return &p, nil
}
```

- [ ] **Step 4: Прогнать тест**

Run: `go test ./internal/store -run TestConfigPlanLifecycle -v`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add internal/store/config_plans.go internal/store/config_plans_test.go
git commit -m "feat(store): временные планы импорта конфигурации"
```

---

## Task 2: Экспорт и валидация системных настроек

**Files:**
- Modify: `internal/settings/settings.go`
- Test: `internal/settings/settings_export_test.go`

**Interfaces:**
- Produces:
  - `type ExportField struct { Key, Section string; Value any; Secret bool }`
  - `func (s *Service) ExportValues(ctx context.Context) []ExportField`
  - `func (s *Service) ActiveKeys(section string) []string`
  - `func (s *Service) Validate(ctx context.Context, values map[string]json.RawMessage, reset []string) ([]Change, error)`

- [ ] **Step 1: Написать падающие тесты**

```go
func TestExportValuesIncludesActiveAndMasksNothing(t *testing.T) {
	svc := testService(t)
	fields := svc.ExportValues(context.Background())
	if len(fields) == 0 {
		t.Fatal("пустой экспорт")
	}
	var duration bool
	for _, f := range fields {
		if f.Key == "collect.interval" {
			duration = f.Value == "30s"
		}
		if f.Key == "network.allowed_cidrs" {
			t.Fatal("readonly попал в экспорт")
		}
	}
	if !duration {
		t.Fatal("длительность должна быть строкой")
	}
}

func TestValidateDoesNotWrite(t *testing.T) {
	svc := testService(t)
	before := svc.Version(context.Background())
	raw := json.RawMessage(`4`)
	if _, err := svc.Validate(context.Background(), map[string]json.RawMessage{"risk.warn_ips": raw}, nil); err != nil {
		t.Fatal(err)
	}
	if svc.Version(context.Background()) != before {
		t.Fatal("Validate изменил версию")
	}
	if _, err := svc.Validate(context.Background(), map[string]json.RawMessage{"risk.warn_ips": json.RawMessage(`-1`)}, nil); err == nil {
		t.Fatal("ожидалась ошибка минимума")
	}
}
```

- [ ] **Step 2: Убедиться, что тесты падают**

Run: `go test ./internal/settings -run 'TestExportValues|TestValidateDoesNotWrite' -v`
Expected: FAIL, `undefined: ExportValues`.

- [ ] **Step 3: Реализовать**

Вынести подготовку из `Save` в метод:

```go
func (s *Service) prepare(
	values map[string]json.RawMessage, reset []string,
) (map[string]string, []Change, error) {
	set := make(map[string]string, len(values))
	changes := make([]Change, 0, len(values)+len(reset))
	for key, raw := range values {
		d, ok := s.by[key]
		if !ok {
			return nil, nil, &ValidationError{Key: key, Message: "неизвестная настройка"}
		}
		if d.Status != Active {
			return nil, nil, &ValidationError{Key: key, Message: "настройка недоступна для изменения"}
		}
		canonical, value, err := parseRaw(d, raw)
		if err != nil {
			return nil, nil, &ValidationError{Key: key, Message: err.Error()}
		}
		old, _ := s.effectiveLocked(d)
		oldCanonical, _ := canonicalValue(d, old)
		set[key] = canonical
		if d.Secret {
			changes = append(changes, Change{Key: key, Old: "***", New: "***"})
		} else {
			changes = append(changes, Change{Key: key, Old: oldCanonical, New: canonicalString(d, value)})
		}
	}
	for _, key := range reset {
		d, ok := s.by[key]
		if !ok {
			return nil, nil, &ValidationError{Key: key, Message: "неизвестная настройка"}
		}
		if d.Status != Active {
			return nil, nil, &ValidationError{Key: key, Message: "настройка недоступна для сброса"}
		}
		if _, exists := s.override[key]; !exists {
			continue
		}
		old, _ := s.effectiveLocked(d)
		oldCanonical, _ := canonicalValue(d, old)
		defaultCanonical, _ := canonicalValue(d, d.Default)
		if d.Secret {
			changes = append(changes, Change{Key: key, Old: "***", New: "***"})
		} else {
			changes = append(changes, Change{Key: key, Old: oldCanonical, New: defaultCanonical})
		}
	}
	if err := s.validateRiskLocked(set, reset); err != nil {
		return nil, nil, err
	}
	return set, changes, nil
}
```

`Save` вызывает `prepare` под `RLock`, затем пишет. `Validate` вызывает `prepare` и возвращает `changes, err` без записи.

```go
func (s *Service) Validate(
	ctx context.Context, values map[string]json.RawMessage, reset []string,
) ([]Change, error) {
	s.ensure(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, changes, err := s.prepare(values, reset)
	return changes, err
}

func (s *Service) ExportValues(ctx context.Context) []ExportField {
	s.ensure(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ExportField, 0, len(s.defs))
	for _, d := range s.defs {
		if d.Status != Active {
			continue
		}
		value, _ := s.effectiveLocked(d)
		if d.Kind == KindDuration {
			if dur, ok := value.(time.Duration); ok {
				value = dur.String()
			}
		}
		out = append(out, ExportField{
			Key: d.Key, Section: d.Section, Value: value, Secret: d.Secret,
		})
	}
	return out
}

func (s *Service) ActiveKeys(section string) []string {
	out := make([]string, 0, 16)
	for _, d := range s.defs {
		if d.Status == Active && d.Section == section {
			out = append(out, d.Key)
		}
	}
	return out
}
```

- [ ] **Step 4: Прогнать тесты пакета**

Run: `go test ./internal/settings ./internal/web -run 'Settings|Validate|Export'`
Expected: PASS, существующие тесты сохранены.

- [ ] **Step 5: Коммит**

```bash
git add internal/settings
git commit -m "feat(settings): экспорт значений и валидация без записи"
```

---

## Task 3: Типы бандла и строгий YAML

**Files:**
- Create: `internal/configx/bundle.go`, `internal/configx/codec.go`
- Test: `internal/configx/codec_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces:
  - `const APIVersion = "tgadmin.telemt/v1"`, `Kind = "InstallationBundle"`
  - `type Bundle`, `Metadata`, `Spec`, `Proxy`, `User`, `Service`, `Token`, `Client`, `Trusted`, `Risk`, `LimitTemplate`
  - `func Marshal(b *Bundle) ([]byte, error)`
  - `func Decode(data []byte) (*Bundle, error)`
  - `func (b *Bundle) HasBlock(id Block) bool`

- [ ] **Step 1: Добавить зависимость и падающие тесты**

```bash
go get go.yaml.in/yaml/v3@v3.0.5
```

```go
func TestDecodeRejectsUnknownField(t *testing.T) {
	_, err := Decode([]byte("apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata: {name: x}\nspec:\n  settings: {}\nunknown: 1\n"))
	if err == nil {
		t.Fatal("ожидалась ошибка неизвестного поля")
	}
}

func TestDecodeRejectsDuplicateKeys(t *testing.T) {
	_, err := Decode([]byte("apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: a\n  name: b\n"))
	if err == nil {
		t.Fatal("ожидалась ошибка дубля ключа")
	}
}

func TestDecodeRejectsAliases(t *testing.T) {
	_, err := Decode([]byte("apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata: &m\n  name: a\nspec: *m\n"))
	if err == nil {
		t.Fatal("ожидалась ошибка алиаса")
	}
}

func TestRoundTrip(t *testing.T) {
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec: Spec{Settings: map[string]map[string]any{"risk": {"warn_ips": 3}}},
	}
	raw, err := Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(raw)
	if err != nil || got.Spec.Settings["risk"]["warn_ips"] != 3 {
		t.Fatalf("%+v %v", got, err)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты падают**

Run: `go test ./internal/configx -run TestDecode -v`
Expected: FAIL, пакет не существует.

- [ ] **Step 3: Реализовать типы**

`bundle.go`:

```go
package configx

import "time"

const (
	APIVersion = "tgadmin.telemt/v1"
	Kind       = "InstallationBundle"

	SecretsNone     = "none"
	SecretsIncluded = "included"
)

type Block string

const (
	BlockSettings  Block = "settings"
	BlockTemplates Block = "limitTemplates"
	BlockProxy     Block = "proxy"
	BlockUsers     Block = "users"
	BlockServices  Block = "services"
	BlockClients   Block = "clients"
)

type Bundle struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`
}

type Metadata struct {
	Name          string `yaml:"name" json:"name"`
	CreatedAt     string `yaml:"createdAt" json:"createdAt"`
	AppVersion    string `yaml:"appVersion,omitempty" json:"appVersion,omitempty"`
	Generator     string `yaml:"generator,omitempty" json:"generator,omitempty"`
	Secrets       string `yaml:"secrets" json:"secrets"`
	SourceBaseURL string `yaml:"sourceBaseUrl,omitempty" json:"sourceBaseUrl,omitempty"`
}

type Spec struct {
	Settings       map[string]map[string]any `yaml:"settings,omitempty" json:"settings,omitempty"`
	LimitTemplates []LimitTemplate           `yaml:"limitTemplates,omitempty" json:"limitTemplates,omitempty"`
	Proxy          *Proxy                    `yaml:"proxy,omitempty" json:"proxy,omitempty"`
	Users          []User                    `yaml:"users,omitempty" json:"users,omitempty"`
	Services       []Service                 `yaml:"services,omitempty" json:"services,omitempty"`
	Clients        []Client                  `yaml:"clients,omitempty" json:"clients,omitempty"`
}

type Proxy struct {
	Revision string         `yaml:"revision,omitempty" json:"revision,omitempty"`
	Config   map[string]any `yaml:"config" json:"config"`
}

type LimitTemplate struct {
	Name             string   `yaml:"name" json:"name"`
	MaxIps           *uint64  `yaml:"maxIps,omitempty" json:"maxIps,omitempty"`
	MaxTcpConns      *uint64  `yaml:"maxTcpConns,omitempty" json:"maxTcpConns,omitempty"`
	DataQuotaBytes   *uint64  `yaml:"dataQuotaBytes,omitempty" json:"dataQuotaBytes,omitempty"`
	QuotaPeriod      *string  `yaml:"quotaPeriod,omitempty" json:"quotaPeriod,omitempty"`
	RateLimitUpBps   *uint64  `yaml:"rateLimitUpBps,omitempty" json:"rateLimitUpBps,omitempty"`
	RateLimitDownBps *uint64  `yaml:"rateLimitDownBps,omitempty" json:"rateLimitDownBps,omitempty"`
	ExpirationDays   *int     `yaml:"expirationDays,omitempty" json:"expirationDays,omitempty"`
	SourceDeny       []string `yaml:"sourceDeny,omitempty" json:"sourceDeny,omitempty"`
	RiskWarnIps      *int     `yaml:"riskWarnIps,omitempty" json:"riskWarnIps,omitempty"`
	RiskHighIps      *int     `yaml:"riskHighIps,omitempty" json:"riskHighIps,omitempty"`
	RiskDistinct7d   *int     `yaml:"riskDistinct7d,omitempty" json:"riskDistinct7d,omitempty"`
	RiskNewCountries *bool    `yaml:"riskNewCountries,omitempty" json:"riskNewCountries,omitempty"`
	Default          bool     `yaml:"default,omitempty" json:"default,omitempty"`
}

type User struct {
	Name         string `yaml:"name" json:"name"`
	Role         string `yaml:"role" json:"role"`
	Disabled     bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	TOTPEnabled  bool   `yaml:"totpEnabled,omitempty" json:"totpEnabled,omitempty"`
	PasswordHash string `yaml:"passwordHash,omitempty" json:"passwordHash,omitempty"`
}

type Service struct {
	Name        string  `yaml:"name" json:"name"`
	Description string  `yaml:"description,omitempty" json:"description,omitempty"`
	Scope       string  `yaml:"scope" json:"scope"`
	Disabled    bool    `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Tokens      []Token `yaml:"tokens,omitempty" json:"tokens,omitempty"`
}

type Token struct {
	ID         string  `yaml:"id" json:"id"`
	Label      string  `yaml:"label,omitempty" json:"label,omitempty"`
	CreatedBy  string  `yaml:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt  string  `yaml:"createdAt,omitempty" json:"createdAt,omitempty"`
	ExpiresAt  *string `yaml:"expiresAt,omitempty" json:"expiresAt,omitempty"`
	SecretHash string  `yaml:"secretHash" json:"secretHash"`
}

type Client struct {
	Username          string   `yaml:"username" json:"username"`
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	MaxTcpConns       *uint64  `yaml:"maxTcpConns,omitempty" json:"maxTcpConns,omitempty"`
	MaxUniqueIps      *uint64  `yaml:"maxUniqueIps,omitempty" json:"maxUniqueIps,omitempty"`
	ExpirationRFC3339 *string  `yaml:"expirationRFC3339,omitempty" json:"expirationRFC3339,omitempty"`
	DataQuotaBytes    *uint64  `yaml:"dataQuotaBytes,omitempty" json:"dataQuotaBytes,omitempty"`
	QuotaPeriod       *string  `yaml:"quotaPeriod,omitempty" json:"quotaPeriod,omitempty"`
	RateLimitUpBps    *uint64  `yaml:"rateLimitUpBps,omitempty" json:"rateLimitUpBps,omitempty"`
	RateLimitDownBps  *uint64  `yaml:"rateLimitDownBps,omitempty" json:"rateLimitDownBps,omitempty"`
	SourceDeny        []string `yaml:"sourceDeny,omitempty" json:"sourceDeny,omitempty"`
	Note              string   `yaml:"note,omitempty" json:"note,omitempty"`
	Trusted           *Trusted `yaml:"trusted,omitempty" json:"trusted,omitempty"`
	Risk              *Risk    `yaml:"risk,omitempty" json:"risk,omitempty"`
	Secret            string   `yaml:"secret,omitempty" json:"secret,omitempty"`
}

type Trusted struct {
	Countries []string `yaml:"countries,omitempty" json:"countries,omitempty"`
	IPs       []string `yaml:"ips,omitempty" json:"ips,omitempty"`
}

type Risk struct {
	WarnIps      *int  `yaml:"warnIps,omitempty" json:"warnIps,omitempty"`
	HighIps      *int  `yaml:"highIps,omitempty" json:"highIps,omitempty"`
	Distinct7d   *int  `yaml:"distinct7d,omitempty" json:"distinct7d,omitempty"`
	NewCountries *bool `yaml:"newCountries,omitempty" json:"newCountries,omitempty"`
}

func (b *Bundle) HasBlock(id Block) bool {
	switch id {
	case BlockSettings:
		return b.Spec.Settings != nil
	case BlockTemplates:
		return b.Spec.LimitTemplates != nil
	case BlockProxy:
		return b.Spec.Proxy != nil
	case BlockUsers:
		return b.Spec.Users != nil
	case BlockServices:
		return b.Spec.Services != nil
	case BlockClients:
		return b.Spec.Clients != nil
	}
	return false
}
```

`codec.go`:

```go
package configx

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

func Marshal(b *Bundle) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(b); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Decode(data []byte) (*Bundle, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("bad_yaml: %w", err)
	}
	if err := checkNode(&root); err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var b Bundle
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("bad_yaml: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("bad_yaml: несколько документов в файле")
	}
	if b.APIVersion != APIVersion {
		return nil, fmt.Errorf("unsupported_version: %q", b.APIVersion)
	}
	if b.Kind != Kind {
		return nil, fmt.Errorf("bad_yaml: неизвестный kind %q", b.Kind)
	}
	if b.Metadata.Name == "" {
		return nil, errors.New("bad_yaml: metadata.name обязателен")
	}
	if b.Metadata.Secrets != SecretsNone && b.Metadata.Secrets != SecretsIncluded {
		return nil, errors.New("bad_yaml: metadata.secrets должен быть none или included")
	}
	return &b, nil
}

func checkNode(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode {
		return errors.New("bad_yaml: алиасы запрещены")
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]struct{}{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if _, ok := seen[key]; ok {
				return fmt.Errorf("bad_yaml: дубль ключа %q", key)
			}
			seen[key] = struct{}{}
		}
	}
	for _, child := range n.Content {
		if err := checkNode(child); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./internal/configx -v`
Expected: PASS все четыре теста.

- [ ] **Step 5: Коммит**

```bash
git add go.mod go.sum internal/configx
git commit -m "feat(configx): типы бандла и строгий YAML"
```

---

## Task 4: Конфигурация telemt через Control API

**Files:**
- Create: `internal/telemt/config.go`
- Modify: `internal/telemt/client.go`
- Test: `internal/telemt/config_test.go`

**Interfaces:**
- Produces:
  - `type ConfigData struct { Config map[string]any; Revision string }`
  - `type ConfigSchema struct { Revision int; Fields []SchemaField }`
  - `type ConfigPatchResult struct { Revision string; ChangedPaths []string; ...; ReloadID int64 }`
  - `func (c *Client) GetConfig(ctx) (*ConfigData, error)`
  - `func (c *Client) GetConfigSchema(ctx) (*ConfigSchema, error)`
  - `func (c *Client) ValidateConfig(ctx, patch map[string]any) error`
  - `func (c *Client) PatchConfig(ctx, patch map[string]any, ifMatch string, reload bool) (*ConfigPatchResult, error)`
  - `func (c *Client) ReloadStatus(ctx, id int64) (*ReloadStatus, error)`

- [ ] **Step 1: Написать падающие тесты**

```go
func TestGetConfigReadsRevision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" {
			t.Fatalf("путь %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"revision":"abc","data":{"general":{"log_level":"normal"}}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "", time.Second)
	got, err := c.GetConfig(context.Background())
	if err != nil || got.Revision != "abc" || got.Config["general"] == nil {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestPatchConfigSendsIfMatchAndReload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.Header.Get("If-Match") != "rev1" {
			t.Fatalf("%s %q", r.Method, r.Header.Get("If-Match"))
		}
		if r.URL.Query().Get("reload") != "true" {
			t.Fatalf("reload=%s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"ok":true,"revision":"rev2","data":{"revision":"rev2","changed_paths":["censorship.tls_domain"],"reload":{"reload_id":7,"state":"accepted"}}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "", time.Second)
	got, err := c.PatchConfig(context.Background(), map[string]any{"censorship": map[string]any{"tls_domain": "x"}}, "rev1", true)
	if err != nil || got.Revision != "rev2" || got.ReloadID != 7 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestPatchConfigMapsConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"revision_conflict","message":"stale"}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "", time.Second)
	_, err := c.PatchConfig(context.Background(), nil, "old", true)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != CodeConflict {
		t.Fatalf("%v", err)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты падают**

Run: `go test ./internal/telemt -run TestGetConfig -v`
Expected: FAIL, `undefined: GetConfig`.

- [ ] **Step 3: Реализовать**

В `client.go` добавить `doMeta`, возвращающий ревизию и статус:

```go
func (c *Client) doMeta(
	ctx context.Context, method, path string, in any, out any,
	headers map[string]string,
) (revision string, status int, err error) {
	// копия текущего do с добавлением headers и возвратом env.Revision/status
}
```

`do` становится обёрткой:

```go
func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	_, _, err := c.doMeta(ctx, method, path, in, out, nil)
	return err
}
```

В `envelope` добавить `Revision string \`json:"revision"\``.

`config.go`:

```go
package telemt

import (
	"context"
	"fmt"
	"time"
)

type ConfigData struct {
	Config   map[string]any
	Revision string
}

type SchemaField struct {
	Path      string         `json:"path"`
	Value     map[string]any `json:"value"`
	API       struct {
		ConfigRead  bool `json:"config_read"`
		ConfigWrite bool `json:"config_write"`
	} `json:"api"`
	Apply struct {
		Watcher string `json:"watcher"`
		Reload  string `json:"reload"`
		Effect  string `json:"effect"`
	} `json:"apply"`
	NonEditableReason string `json:"non_editable_reason"`
}

type ConfigSchema struct {
	Version int
	Fields  []SchemaField
}

type ConfigPatchResult struct {
	Revision               string   `json:"revision"`
	ChangedPaths           []string `json:"changed_paths"`
	RuntimeReloadRequired  bool     `json:"runtime_reload_required"`
	ProcessRestartRequired bool     `json:"process_restart_required"`
	DeferredProcessFields  []string `json:"deferred_process_fields"`
	ReloadID               int64    `json:"-"`
}

type ReloadStatus struct {
	ID        int64  `json:"reload_id"`
	State     string `json:"state"`
	Phase     string `json:"phase"`
	ErrorCode string `json:"error_code,omitempty"`
	Warnings  int    `json:"warning_count,omitempty"`
}

func (c *Client) GetConfig(ctx context.Context) (*ConfigData, error) {
	var data map[string]any
	rev, _, err := c.doMeta(ctx, "GET", "/v1/config", nil, &data, nil)
	if err != nil {
		return nil, err
	}
	return &ConfigData{Config: data, Revision: rev}, nil
}

func (c *Client) GetConfigSchema(ctx context.Context) (*ConfigSchema, error) {
	var data struct {
		SchemaVersion int           `json:"schema_version"`
		Fields        []SchemaField `json:"fields"`
	}
	_, _, err := c.doMeta(ctx, "GET", "/v1/config/schema", nil, &data, nil)
	if err != nil {
		return nil, err
	}
	return &ConfigSchema{Version: data.SchemaVersion, Fields: data.Fields}, nil
}

func (c *Client) ValidateConfig(ctx context.Context, patch map[string]any) error {
	_, _, err := c.doMeta(ctx, "POST", "/v1/config/validate", patch, nil, nil)
	return err
}

func (c *Client) PatchConfig(
	ctx context.Context, patch map[string]any, ifMatch string, reload bool,
) (*ConfigPatchResult, error) {
	headers := map[string]string{}
	if ifMatch != "" {
		headers["If-Match"] = ifMatch
	}
	path := "/v1/config"
	if reload {
		path += "?reload=true"
	}
	var data struct {
		Revision               string   `json:"revision"`
		ChangedPaths           []string `json:"changed_paths"`
		RuntimeReloadRequired  bool     `json:"runtime_reload_required"`
		ProcessRestartRequired bool     `json:"process_restart_required"`
		DeferredProcessFields  []string `json:"deferred_process_fields"`
		Reload                 *struct {
			ReloadID int64 `json:"reload_id"`
		} `json:"reload"`
	}
	rev, _, err := c.doMeta(ctx, "PATCH", path, patch, &data, headers)
	if err != nil {
		return nil, err
	}
	out := &ConfigPatchResult{
		Revision: data.Revision, ChangedPaths: data.ChangedPaths,
		RuntimeReloadRequired: data.RuntimeReloadRequired,
		ProcessRestartRequired: data.ProcessRestartRequired,
		DeferredProcessFields: data.DeferredProcessFields,
	}
	if out.Revision == "" {
		out.Revision = rev
	}
	if data.Reload != nil {
		out.ReloadID = data.Reload.ReloadID
	}
	return out, nil
}

func (c *Client) ReloadStatus(ctx context.Context, id int64) (*ReloadStatus, error) {
	var data ReloadStatus
	_, _, err := c.doMeta(ctx, "GET", fmt.Sprintf("/v1/system/reload/%d", id), nil, &data, nil)
	if err != nil {
		return nil, err
	}
	if data.ID == 0 {
		data.ID = id
	}
	return &data, nil
}

var _ = time.Second
```

Добавить в `errors.go` коды `CodeConflict = "telemt_conflict"` для 409, если ещё нет.

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./internal/telemt -v`
Expected: PASS, старые тесты тоже.

- [ ] **Step 5: Коммит**

```bash
git add internal/telemt
git commit -m "feat(telemt): конфигурация через Control API"
```

---

## Task 5: Экспорт бандла

**Files:**
- Create: `internal/configx/service.go`, `internal/configx/export.go`
- Modify: `internal/store/service_tokens.go`
- Test: `internal/configx/export_test.go`, `internal/store/service_tokens_export_test.go`

**Interfaces:**
- Consumes: Task 2 `ExportValues`, Task 3 types, Task 4 `GetConfig`.
- Produces:
  - `func New(st *store.Store, settings *settings.Service, tc *telemt.Client, now func() time.Time) *Service`
  - `type ExportOptions struct { Blocks []Block; Secrets bool; Name, AppVersion, BaseURL string }`
  - `func (s *Service) Export(ctx context.Context, opts ExportOptions) (*Bundle, error)`

- [ ] **Step 1: Тест на секреты и блоки**

```go
func TestExportRedactsSecretsByDefault(t *testing.T) {
	svc := testService(t)
	b, err := svc.Export(context.Background(), ExportOptions{
		Blocks: []Block{BlockSettings, BlockUsers, BlockClients}, Secrets: false,
		Name: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.Metadata.Secrets != SecretsNone {
		t.Fatalf("secrets=%s", b.Metadata.Secrets)
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
	raw, _ := Marshal(b)
	if bytes.Contains(raw, []byte("webhook_secret")) {
		t.Fatal("секрет вебхука попал в YAML")
	}
}

func TestExportIncludesTokenHashesOnlyWithSecrets(t *testing.T) {
	svc := testService(t)
	without, _ := svc.Export(context.Background(), ExportOptions{Blocks: []Block{BlockServices}})
	if len(without.Spec.Services) == 0 || len(without.Spec.Services[0].Tokens) != 0 {
		t.Fatal("без секретов токены не должны экспортироваться")
	}
	with, _ := svc.Export(context.Background(), ExportOptions{Blocks: []Block{BlockServices}, Secrets: true})
	if len(with.Spec.Services[0].Tokens) == 0 || with.Spec.Services[0].Tokens[0].SecretHash == "" {
		t.Fatal("с секретами ожидались хеши токенов")
	}
}
```

- [ ] **Step 2: Тесты падают**

Run: `go test ./internal/configx -run TestExport -v`
Expected: FAIL, `undefined: New`.

- [ ] **Step 3: Реализовать**

`service.go`:

```go
package configx

import (
	"time"

	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

type Service struct {
	st       *store.Store
	settings *settings.Service
	telemt   *telemt.Client
	now      func() time.Time
}

func New(st *store.Store, s *settings.Service, tc *telemt.Client, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{st: st, settings: s, telemt: tc, now: now}
}
```

В `store/service_tokens.go` добавить:

```go
type ServiceTokenSecret struct {
	ID         string
	Service    string
	Label      string
	CreatedBy  string
	SecretHash string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
}

func (s *Store) ExportServiceTokens(ctx context.Context, service string) ([]ServiceTokenSecret, error) {
	ids, err := s.rdb.SMembers(ctx, serviceTokensKey(service)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]ServiceTokenSecret, 0, len(ids))
	for _, id := range ids {
		m, err := s.rdb.HGetAll(ctx, tokenKey(id)).Result()
		if err != nil || len(m) == 0 {
			continue
		}
		t := ServiceTokenSecret{
			ID: id, Service: service, Label: m["label"], CreatedBy: m["created_by"],
			SecretHash: m["secret_hash"],
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, m["created_at"])
		if raw := m["expires_at"]; raw != "" {
			if exp, err := time.Parse(time.RFC3339, raw); err == nil {
				t.ExpiresAt = &exp
			}
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Store) ImportServiceToken(ctx context.Context, t ServiceTokenSecret) error {
	ok, err := s.rdb.HSetNX(ctx, tokenKey(t.ID), "secret_hash", t.SecretHash).Result()
	if err != nil || !ok {
		return err
	}
	_, err = s.rdb.HSet(ctx, tokenKey(t.ID), map[string]any{
		"service": t.Service, "label": t.Label, "created_by": t.CreatedBy,
		"created_at": t.CreatedAt.UTC().Format(time.RFC3339),
	}).Result()
	if err != nil {
		return err
	}
	return s.rdb.SAdd(ctx, serviceTokensKey(t.Service), t.ID).Err()
}
```

`export.go`:

```go
package configx

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

type ExportOptions struct {
	Blocks     []Block
	Secrets    bool
	Name       string
	AppVersion string
	BaseURL    string
}

func (s *Service) Export(ctx context.Context, opts ExportOptions) (*Bundle, error) {
	secrets := SecretsNone
	if opts.Secrets {
		secrets = SecretsIncluded
	}
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{
			Name: opts.Name, CreatedAt: s.now().UTC().Format(time.RFC3339),
			AppVersion: opts.AppVersion, Generator: "tgadmin",
			Secrets: secrets, SourceBaseURL: opts.BaseURL,
		},
	}
	for _, block := range opts.Blocks {
		switch block {
		case BlockSettings:
			b.Spec.Settings = s.exportSettings(opts.Secrets)
		case BlockTemplates:
			templates, err := s.exportTemplates(ctx)
			if err != nil {
				return nil, err
			}
			b.Spec.LimitTemplates = templates
		case BlockProxy:
			if s.telemt == nil {
				return nil, fmt.Errorf("proxy_unavailable: TELEMT_API_URL не задан")
			}
			cfg, err := s.telemt.GetConfig(ctx)
			if err != nil {
				return nil, err
			}
			b.Spec.Proxy = &Proxy{Revision: cfg.Revision, Config: cfg.Config}
		case BlockUsers:
			users, err := s.exportUsers(ctx, opts.Secrets)
			if err != nil {
				return nil, err
			}
			b.Spec.Users = users
		case BlockServices:
			services, err := s.exportServices(ctx, opts.Secrets)
			if err != nil {
				return nil, err
			}
			b.Spec.Services = services
		case BlockClients:
			clients, err := s.exportClients(ctx, opts.Secrets)
			if err != nil {
				return nil, err
			}
			b.Spec.Clients = clients
		default:
			return nil, fmt.Errorf("bad_request: неизвестный блок %q", block)
		}
	}
	return b, nil
}

func (s *Service) exportSettings(includeSecrets bool) map[string]map[string]any {
	out := map[string]map[string]any{}
	sensitive := map[string]bool{"notifications.webhook_url": true}
	for _, f := range s.settings.ExportValues(context.Background()) {
		if f.Section == "infrastructure" {
			continue
		}
		if (f.Secret || sensitive[f.Key]) && !includeSecrets {
			continue
		}
		section := out[f.Section]
		if section == nil {
			section = map[string]any{}
			out[f.Section] = section
		}
		key := strings.TrimPrefix(f.Key, f.Section+".")
		section[key] = f.Value
	}
	return out
}

func (s *Service) exportTemplates(ctx context.Context) ([]LimitTemplate, error) {
	list, err := s.st.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	def, _ := s.st.DefaultTemplate(ctx)
	out := make([]LimitTemplate, 0, len(list))
	for _, t := range list {
		out = append(out, LimitTemplate{
			Name: t.Name, MaxIps: t.MaxIps, MaxTcpConns: t.MaxTcpConns,
			DataQuotaBytes: t.DataQuotaBytes, QuotaPeriod: t.QuotaPeriod,
			RateLimitUpBps: t.RateLimitUpBps, RateLimitDownBps: t.RateLimitDownBps,
			ExpirationDays: t.ExpirationDays, SourceDeny: t.SourceDeny,
			RiskWarnIps: t.RiskWarnIps, RiskHighIps: t.RiskHighIps,
			RiskDistinct7d: t.RiskDistinct7d, RiskNewCountries: t.RiskNewCountries,
			Default: t.Name == def,
		})
	}
	return out, nil
}

func (s *Service) exportUsers(ctx context.Context, includeSecrets bool) ([]User, error) {
	list, err := s.st.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]User, 0, len(list))
	for _, u := range list {
		item := User{Name: u.Name, Role: string(u.Role), Disabled: u.Disabled, TOTPEnabled: u.TOTPEnabled}
		if includeSecrets {
			item.PasswordHash = u.Hash
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Service) exportServices(ctx context.Context, includeSecrets bool) ([]Service, error) {
	list, err := s.st.ListServices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Service, 0, len(list))
	for _, svc := range list {
		item := Service{Name: svc.Name, Description: svc.Description, Scope: string(svc.Scope), Disabled: svc.Disabled}
		if includeSecrets {
			tokens, err := s.st.ExportServiceTokens(ctx, svc.Name)
			if err != nil {
				return nil, err
			}
			for _, t := range tokens {
				item.Tokens = append(item.Tokens, Token{
					ID: t.ID, Label: t.Label, CreatedBy: t.CreatedBy,
					CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339),
					ExpiresAt: formatOptionalTime(t.ExpiresAt), SecretHash: t.SecretHash,
				})
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) exportClients(ctx context.Context, includeSecrets bool) ([]Client, error) {
	users, err := s.telemt.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Client, 0, len(users))
	for _, u := range users {
		item := Client{
			Username: u.Username, Enabled: u.Enabled,
			MaxTcpConns: u.MaxTCPConns, MaxUniqueIps: u.MaxUniqueIPs,
			ExpirationRFC3339: u.ExpirationRFC3339, DataQuotaBytes: u.DataQuotaBytes,
			QuotaPeriod: u.QuotaPeriod, RateLimitUpBps: u.RateLimitUpBps,
			RateLimitDownBps: u.RateLimitDownBps,
		}
		item.Note, _ = s.st.Note(ctx, u.Username)
		if trusted, err := s.st.GetTrusted(ctx, u.Username); err == nil {
			if len(trusted.Countries) > 0 || len(trusted.IPs) > 0 {
				item.Trusted = &Trusted{Countries: trusted.Countries, IPs: trusted.IPs}
			}
		}
		if risk, ok, err := s.st.GetRisk(ctx, u.Username); err == nil && ok {
			item.Risk = &Risk{
				WarnIps: risk.WarnIps, HighIps: risk.HighIps,
				Distinct7d: risk.Distinct7d, NewCountries: risk.NewCountries,
			}
		}
		if includeSecrets && u.Links != nil {
			item.Secret = secretFromLinks(u.Links)
		}
		out = append(out, item)
	}
	return out, nil
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.UTC().Format(time.RFC3339)
	return &v
}

func secretFromLinks(l *telemt.Links) string {
	for _, group := range [][]string{l.Classic, l.Secure, l.TLS} {
		for _, link := range group {
			u, err := url.Parse(link)
			if err != nil {
				continue
			}
			raw := u.Query().Get("secret")
			if len(raw) >= 34 && strings.HasPrefix(raw, "ee") {
				return raw[2:34]
			}
			if len(raw) == 32 {
				return raw
			}
		}
	}
	return ""
}
```

Добавить импорты `net/url`, `strings`, `time`.

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./internal/configx ./internal/store -run 'Export|Token' -v`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add internal/configx internal/store
git commit -m "feat(configx): экспорт бандла без секретов по умолчанию"
```

---

## Task 6: Precheck, diff и конфликты

**Files:**
- Create: `internal/configx/plan.go`, `internal/configx/precheck.go`
- Test: `internal/configx/precheck_test.go`

**Interfaces:**
- Produces:
  - `type Plan`, `BlockPlan`, `DiffEntry`, `Conflict`, `ConflictItem`, `Deletions`
  - `type PrecheckOptions struct { FileName string; Bundle *Bundle; TargetSettingsVersion int64; TargetProxyRevision string }`
  - `func (s *Service) Precheck(ctx context.Context, opts PrecheckOptions) (*Plan, error)`

- [ ] **Step 1: Тест на diff и конфликты**

```go
func TestPrecheckBuildsDiffAndConflicts(t *testing.T) {
	svc := testService(t)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec: Spec{
			Settings: map[string]map[string]any{"risk": {"warn_ips": 4}},
			Users: []User{{Name: "demo", Role: "viewer"}},
		},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b, FileName: "x.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Diff["settings"][0].Path != "risk.warn_ips" {
		t.Fatalf("diff=%+v", plan.Diff)
	}
	if len(plan.Conflicts["users"]) != 1 || plan.Conflicts["users"][0].Action != ActionKeep {
		t.Fatalf("конфликты=%+v", plan.Conflicts)
	}
}

func TestPrecheckRejectsLastAdminDemotion(t *testing.T) {
	svc := testService(t)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec: Spec{Users: []User{{Name: "demo", Role: "viewer", Disabled: true}}},
	}
	if _, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b}); err == nil {
		t.Fatal("ожидалась защита последнего администратора")
	}
}
```

- [ ] **Step 2: Тесты падают**

Run: `go test ./internal/configx -run TestPrecheck -v`
Expected: FAIL, `undefined: Precheck`.

- [ ] **Step 3: Реализовать**

`plan.go` содержит структуры с json-тегами:

```go
type Plan struct {
	ID                    string                   `json:"id"`
	Owner                 string                   `json:"owner"`
	Session               string                   `json:"session"`
	FileName              string                   `json:"fileName"`
	FileHash              string                   `json:"fileHash"`
	CreatedAt             time.Time                `json:"createdAt"`
	ExpiresAt             time.Time                `json:"expiresAt"`
	Bundle                *Bundle                  `json:"bundle"`
	TargetSettingsVersion int64                    `json:"targetSettingsVersion"`
	TargetProxyRevision   string                   `json:"targetProxyRevision"`
	Blocks                []BlockPlan              `json:"blocks"`
	Diff                  map[string][]DiffEntry   `json:"diff"`
	Conflicts             map[string][]ConflictItem `json:"conflicts"`
	Deletions             map[string][]string      `json:"deletions"`
}

type BlockPlan struct {
	ID       Block    `json:"id"`
	Title    string   `json:"title"`
	Changes  int      `json:"changes"`
	Warnings []string `json:"warnings"`
}

type DiffEntry struct {
	Path   string `json:"path"`
	Before any    `json:"before"`
	After  any    `json:"after"`
	Secret bool   `json:"secret,omitempty"`
}

type ConflictItem struct {
	Name    string   `json:"name"`
	Action  string   `json:"action"`
	Changes []string `json:"changes"`
}

const (
	ActionKeep      = "keep"
	ActionOverwrite = "overwrite"
)
```

`precheck.go`:

- `Precheck` проверяет блоки, строит `diff` и `conflicts`, считает `deletions`.
- Для settings: `s.settings.ExportValues` → current map; для каждого ключа файла сравнить.
- Для замены (`modes` ещё нет на precheck) diff считает только значения из файла; reset-поля добавит apply.
- Для пользователей: `s.st.ListUsers`, сравнение роли/Disabled/Hash, действие по умолчанию `keep`.
- Для последнего админа: если после применения останется ноль активных админов — ошибка.
- Для клиентов: сравнить лимиты и tgadmin-поля; секрет в diff не показывать (`Secret: true`).
- Для proxy: `GetConfig`, сравнить рекурсивно, вызвать `ValidateConfig` с `bundle.Spec.Proxy.Config` (без `access`), ошибка `proxy_invalid`.
- `Deletions` — имена в store/telemt, которых нет в файле, только для выбранных блоков.

Ключевые функции:

```go
func flattenSettings(in map[string]map[string]any) map[string]any {
	out := map[string]any{}
	for section, fields := range in {
		for key, value := range fields {
			full := key
			if !strings.HasPrefix(key, section+".") {
				full = section + "." + key
			}
			out[full] = value
		}
	}
	return out
}

func recursiveDiff(prefix string, before, after map[string]any) []DiffEntry
```

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./internal/configx -run TestPrecheck -v`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add internal/configx
git commit -m "feat(configx): precheck с diff и конфликтами"
```

---

## Task 7: Применение плана

**Files:**
- Create: `internal/configx/apply.go`
- Test: `internal/configx/apply_test.go`

**Interfaces:**
- Produces:
  - `type ApplyOptions struct { Blocks []Block; Modes map[Block]string; Conflicts map[string]map[string]string; DeleteMissing bool; ConfirmName string }`
  - `type BlockResult struct { Block Block; Status string; Applied int; Created int; Updated int; Skipped int; Error string; Passwords map[string]string; Links map[string][]string }`
  - `func (s *Service) Apply(ctx context.Context, plan *Plan, opts ApplyOptions) ([]BlockResult, error)`

- [ ] **Step 1: Тесты merge/replace и идемпотентности**

```go
func TestApplySettingsMergeAndReplace(t *testing.T) {
	svc := testService(t)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec: Spec{Settings: map[string]map[string]any{"risk": {"warn_ips": 4}}},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks: []Block{BlockSettings}, Modes: map[Block]string{BlockSettings: "merge"},
	})
	if err != nil || res[0].Status != "applied" || res[0].Applied != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	// повторный импорт того же значения не должен падать
	plan2, _ := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if _, err := svc.Apply(context.Background(), plan2, ApplyOptions{
		Blocks: []Block{BlockSettings}, Modes: map[Block]string{BlockSettings: "merge"},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRefusesStalePlan(t *testing.T) {
	svc := testService(t)
	plan, _ := svc.Precheck(context.Background(), PrecheckOptions{Bundle: testBundle()})
	plan.TargetSettingsVersion--
	if _, err := svc.Apply(context.Background(), plan, ApplyOptions{Blocks: []Block{BlockSettings}}); err == nil {
		t.Fatal("ожидалась ошибка stale_plan")
	}
}
```

- [ ] **Step 2: Тесты падают**

Run: `go test ./internal/configx -run TestApply -v`
Expected: FAIL, `undefined: Apply`.

- [ ] **Step 3: Реализовать**

`Apply`:

1. `opts.ConfirmName` проверяется, если `DeleteMissing`.
2. Сверяет `plan.TargetSettingsVersion` с `s.settings.Version(ctx)`; для proxy — ревизию.
3. Для каждого блока вызывает `applySettings`, `applyTemplates`, `applyUsers`, `applyServices`, `applyClients`, `applyProxy`.
4. Возвращает результаты; при ошибке блока останавливается.

`applySettings`:

```go
func (s *Service) applySettings(ctx context.Context, plan *Plan, mode string) (BlockResult, error) {
	values := map[string]json.RawMessage{}
	reset := []string{}
	for section, fields := range plan.Bundle.Spec.Settings {
		present := map[string]bool{}
		for key, value := range fields {
			full := key
			if !strings.HasPrefix(key, section+".") {
				full = section + "." + key
			}
			raw, err := json.Marshal(value)
			if err != nil {
				return BlockResult{}, err
			}
			values[full] = raw
			present[full] = true
		}
		if mode == "replace" {
			for _, key := range s.settings.ActiveKeys(section) {
				if !present[key] {
					reset = append(reset, key)
				}
			}
		}
	}
	if _, err := s.settings.Validate(ctx, values, reset); err != nil {
		return BlockResult{}, err
	}
	if _, _, err := s.settings.Save(ctx, plan.TargetSettingsVersion, values, reset); err != nil {
		return BlockResult{}, err
	}
	return BlockResult{Block: BlockSettings, Status: "applied", Applied: len(values) + len(reset)}, nil
}
```

`applyTemplates` использует `SaveTemplate`, `DeleteTemplate`, `SetDefaultTemplate`. `DeleteMissing` удаляет отсутствующие имена.

`applyUsers`:

- для отсутствующего: пароль из хеша или случайный (`randomPassword(24)`), `st.CreateUser`.
- для существующего с действием `overwrite`: `SetRole`, `SetDisabled`, при наличии хеша `SetPassword`.
- защита: после применения пересчитать активных админов; если ноль — ошибка до записи (проверка в precheck уже есть, повторить).
- `Passwords` возвращает сгенерированные пароли.

`applyServices`: `CreateService` / `UpdateService`; токены только при `metadata.secrets == included`, `ImportServiceToken`.

`applyClients`:

- отсутствующий: `telemt.CreateInput` с секретом или без; затем, если `enabled=false`, `DisableUser`.
- существующий `overwrite`: `PatchUser` с полями; секрет передаётся в patch, если включён; `SetNote`, `SetTrusted`, `SetRisk`; при `DeleteMissing` — `DeleteUser`, `DeleteRisk`.
- результат `Links` заполняется из ответа `CreateUser`.

`applyProxy`:

```go
func (s *Service) applyProxy(ctx context.Context, plan *Plan) (BlockResult, error) {
	cfg, err := s.telemt.GetConfig(ctx)
	if err != nil {
		return BlockResult{}, err
	}
	if plan.TargetProxyRevision != "" && cfg.Revision != plan.TargetProxyRevision {
		return BlockResult{}, ErrStalePlan
	}
	if err := s.telemt.ValidateConfig(ctx, plan.Bundle.Spec.Proxy.Config); err != nil {
		return BlockResult{}, err
	}
	res, err := s.telemt.PatchConfig(ctx, plan.Bundle.Spec.Proxy.Config, cfg.Revision, true)
	if err != nil {
		return BlockResult{}, err
	}
	return BlockResult{Block: BlockProxy, Status: "applied", Applied: len(res.ChangedPaths)}, nil
}
```

Ввести `var ErrStalePlan = errors.New("stale_plan")`, `ErrPlanUsed`, `ErrPlanNotFound`.

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./internal/configx -v`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add internal/configx
git commit -m "feat(configx): применение плана импорта"
```

---

## Task 8: Web API экспорта, импорта и прокси

**Files:**
- Create: `internal/web/api_config.go`, `internal/web/api_proxy.go`, `internal/web/config_guard.go`
- Modify: `internal/web/server.go`
- Test: `internal/web/api_config_test.go`, `internal/web/api_proxy_test.go`

**Interfaces:**
- Consumes: Task 1–7.
- Produces: маршруты из спеки, поле `Server.configx *configx.Service`.

- [ ] **Step 1: Тесты маршрутов и прав**

```go
func TestConfigExportRequiresAdmin(t *testing.T) {
	s, _, _ := coverageServer(t)
	req := apiRequest(t, "POST", "/api/config/export", `{"blocks":["settings"]}`, "", "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("без сессии: %d", rr.Code)
	}
}

func TestConfigExportSettings(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	s.configx = configx.New(s.store, s.settings, s.telemt, time.Now)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/config/export",
		`{"blocks":["settings"]}`, tok, csrf))
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Disposition") == "" {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "apiVersion: tgadmin.telemt/v1") {
		t.Fatalf("тело: %s", rr.Body.String())
	}
}

func TestProxyConfigRequiresAdmin(t *testing.T) {
	s, _, _ := coverageServer(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/proxy/config", "", "", ""))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("%d", rr.Code)
	}
}
```

- [ ] **Step 2: Тесты падают**

Run: `go test ./internal/web -run 'TestConfigExport|TestProxyConfig' -v`
Expected: FAIL, 404.

- [ ] **Step 3: Реализовать**

`server.go`:

```go
configx *configx.Service
```

В `New`:

```go
cfgx := configx.New(st, settingsSvc, tc, time.Now)
s := &Server{..., configx: cfgx, ...}
```

Маршруты в admin-группе:

```go
r.Get("/config/blocks", s.apiConfigBlocks)
r.Post("/config/export", s.apiConfigExport)
r.Post("/config/precheck", s.apiConfigPrecheck)
r.Post("/config/apply", s.apiConfigApply)
r.Get("/config/plan/{id}", s.apiConfigPlan)
r.Get("/proxy/schema", s.apiProxySchema)
r.Get("/proxy/config", s.apiProxyConfig)
r.Post("/proxy/validate", s.apiProxyValidate)
r.Post("/proxy/apply", s.apiProxyApply)
r.Get("/proxy/reload/{id}", s.apiProxyReload)
```

`api_config.go`:

- `apiConfigExport`: `requireSessionAdmin`, лимитер, декод `{blocks []string, secrets bool, password, code string}`.
  Если `secrets` — `s.verifyReauth(ctx, u, password, code)`; ошибки `secrets_confirmation_required` / `bad_credentials`.
  Собирает `configx.Export`, `Marshal`, отдаёт с `Content-Disposition: attachment; filename="..."`, `Cache-Control: no-store`.
  Аудит `config.export` с блоками и `secrets`.
- `apiConfigPrecheck`: читает тело `io.LimitReader(r.Body, 4<<20+1)`, проверяет размер, `configx.Decode`, проверяет `metadata.secrets` и reauth, `Precheck`, сохраняет план:
  `id := randomID()`, `payload, _ := json.Marshal(plan)`, `s.store.PutConfigPlan(ctx, store.ConfigPlan{ID:id, Owner:sess.User, Session:sess.ID, Payload:payload}, 15*time.Minute)`, возвращает `plan` без `Bundle`.
- `apiConfigApply`: декод `{planId, blocks, modes, conflicts, deleteMissing, confirmName}`; `TakeConfigPlan`; проверка owner/session; `Apply`; аудит; возвращает результаты.
- `apiConfigPlan`: `GetConfigPlan`, проверка owner/session, вернуть план без Bundle? Вернуть с Bundle для UI повторного открытия. Секреты в Bundle могут быть; ответ `no-store`.

`verifyReauth`:

```go
func (s *Server) verifyReauth(ctx context.Context, u *store.User, password, code string) error {
	ok, err := auth.Verify(u.Hash, password)
	if err != nil || !ok {
		return errBadCredentials
	}
	if !u.TOTPEnabled {
		return nil
	}
	ok, _, _ = s.checkTOTPCode(ctx, u, code)
	if !ok {
		return errBadCredentials
	}
	return nil
}
```

`config_guard.go`: map `user -> window{count, reset}` с мьютексом; методы `allowExport`, `allowImport`; тест на лимит.

`api_proxy.go`:

- `apiProxySchema` → `telemt.GetConfigSchema`, кэш 5 минут в `Server` (поля `schemaMu`, `schemaAt`, `schema`).
- `apiProxyConfig` → `GetConfig`.
- `apiProxyValidate` → `ValidateConfig`.
- `apiProxyApply` → `{patch map[string]any, confirm bool}`: `GetConfig`, `ValidateConfig`, `PatchConfig(rev, true)`, аудит `proxy.update`; если `ReloadID != 0` вернуть `reloadId`.
- `apiProxyReload` → `ReloadStatus`.

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./internal/web -run 'TestConfig|TestProxy' -v`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add internal/web
git commit -m "feat(web): API обмена конфигурацией и редактора прокси"
```

---

## Task 9: Фронтенд API и типы

**Files:**
- Modify: `frontend/src/api.ts`
- Test: `frontend/src/api.test.ts`

**Interfaces:**
- Produces: `api.configBlocks()`, `api.exportConfig(body): Promise<Blob>`, `api.configPrecheck(yaml)`, `api.configApply(body)`, `api.configPlan(id)`, `api.proxySchema()`, `api.proxyConfig()`, `api.proxyValidate(patch)`, `api.proxyApply(body)`, `api.proxyReload(id)`; типы `ConfigBlock`, `ConfigPlan`, `BlockResult`, `ProxySchema`, `ProxyConfig`.

- [ ] **Step 1: Тест клиента экспорта**

```ts
it('скачивает YAML с CSRF и no-store', async () => {
  const blob = new Blob(['apiVersion: tgadmin.telemt/v1\n']);
  const fetchMock = vi.fn().mockResolvedValue(new Response(blob, { status: 200 }));
  vi.stubGlobal('fetch', fetchMock);
  const out = await api.exportConfig({ blocks: ['settings'] });
  expect(out.size).toBeGreaterThan(0);
  const init = fetchMock.mock.calls[0][1] as RequestInit;
  expect((init.headers as Record<string, string>)['X-CSRF-Token']).toBe('csrf');
});
```

- [ ] **Step 2: Тест падает**

Run: `npx vitest run src/api.test.ts -t "скачивает YAML"`
Expected: FAIL, `api.exportConfig is not a function`.

- [ ] **Step 3: Реализовать**

Добавить типы и методы; `exportConfig` использует `fetch('/api/config/export', ...)`, при ошибке — `readResponse(res)`, иначе `res.blob()`.

- [ ] **Step 4: Прогнать**

Run: `npx vitest run src/api.test.ts`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add frontend/src/api.ts frontend/src/api.test.ts
git commit -m "feat(ui): клиент обмена конфигурацией"
```

---

## Task 10: UI экспорта и импорта

**Files:**
- Create: `frontend/src/pages/ConfigExchange.tsx`, `frontend/src/pages/ConfigExchange.test.tsx`
- Modify: `frontend/src/pages/SettingsPage.tsx`

**Interfaces:**
- Consumes: Task 9.
- Produces: `<ConfigExchangePanel />` в вкладке «Бэкапы».

- [ ] **Step 1: Тесты сценария**

```tsx
it('экспортирует выбранные блоки без секретов', async () => {
  stubFetch({ '/api/config/export': () => new Response(new Blob(['yaml']), { status: 200 }) });
  renderUI(<ConfigExchangePanel />);
  await userEvent.click(screen.getByRole('button', { name: 'Скачать YAML' }));
  await waitFor(() => expect(calls('POST', '/api/config/export')).toHaveLength(1));
});

it('показывает diff и переключает конфликт', async () => {
  stubFetch({
    '/api/config/precheck': {
      planId: 'p1',
      blocks: [{ id: 'users', title: 'Пользователи', changes: 1, warnings: [] }],
      diff: { users: [] },
      conflicts: { users: [{ name: 'demo', action: 'keep', changes: ['role'] }] },
      deletions: {},
    },
  });
  renderUI(<ConfigExchangePanel />);
  await userEvent.upload(screen.getByLabelText('YAML'), new File(['yaml'], 'x.yaml'));
  await userEvent.click(screen.getByRole('button', { name: 'Проверить' }));
  await userEvent.click(await screen.findByRole('button', { name: 'переписать demo' }));
  await userEvent.click(screen.getByRole('button', { name: 'Применить выбранное' }));
  await waitFor(() => expect(calls('POST', '/api/config/apply')[0].body).toMatchObject({
    conflicts: { users: { demo: 'overwrite' } },
  }));
});
```

- [ ] **Step 2: Тесты падают**

Run: `npx vitest run src/pages/ConfigExchange.test.tsx`
Expected: FAIL, модуль не найден.

- [ ] **Step 3: Реализовать**

Компонент:
- состояние: `blocks` (набор), `secrets`, `password`, `code`, `yaml`, `plan`, `conflicts`, `modes`, `deleteMissing`, `confirmName`, `results`, `busy`.
- пресеты меняют набор блоков.
- «Скачать YAML» вызывает `api.exportConfig`, создаёт `<a download>` через `URL.createObjectURL`.
- «Проверить» читает файл (`file.text()`), вызывает precheck, рендерит diff и конфликты.
- конфликт: `Select` или `Button` с aria-label «переписать <name>» / «оставить <name>».
- «Применить выбранное» собирает body, вызывает apply, показывает `BlockResult[]`, пароли и ссылки с кнопкой «Скопировать».
- `SettingsPage`: в блоке `tab === 'backups'` рендерить `<ConfigExchangePanel />` перед `<BackupsPanel />`.

- [ ] **Step 4: Прогнать**

Run: `npx vitest run src/pages/ConfigExchange.test.tsx src/pages/SettingsPage.test.tsx`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add frontend/src/pages
git commit -m "feat(ui): экспорт и импорт конфигурации"
```

---

## Task 11: Редактор прокси

**Files:**
- Create: `frontend/src/pages/ProxyPanel.tsx`, `frontend/src/pages/ProxyPanel.test.tsx`
- Modify: `frontend/src/pages/SettingsPage.tsx`

**Interfaces:**
- Consumes: Task 9.
- Produces: вкладка «Прокси» в настройках.

- [ ] **Step 1: Тесты формы и применения**

```tsx
it('правит вложенный general.links.public_host и применяет патч', async () => {
  stubFetch({
    '/api/proxy/schema': { version: 1, fields: [{ path: 'general.links.public_host', value: { type: 'string' }, api: { config_read: true, config_write: true } }] },
    '/api/proxy/config': { revision: 'rev1', config: { general: { links: { public_host: 'old.example' } } } },
    '/api/proxy/validate': { ok: true },
    '/api/proxy/apply': { revision: 'rev2', changedPaths: ['general.links.public_host'], reloadId: 1 },
  });
  renderUI(<ProxyPanel />);
  const input = await screen.findByLabelText('general.links.public_host');
  await userEvent.clear(input);
  await userEvent.type(input, 'new.example');
  await userEvent.click(screen.getByRole('button', { name: 'Применить' }));
  await waitFor(() => expect(calls('POST', '/api/proxy/apply')[0].body).toMatchObject({
    patch: { general: { links: { public_host: 'new.example' } } },
  }));
});

it('показывает конфликт ревизии', async () => {
  stubFetch({
    '/api/proxy/schema': { version: 1, fields: [] },
    '/api/proxy/config': { revision: 'rev1', config: {} },
    '/api/proxy/apply': () => new Response(JSON.stringify({ error: { code: 'proxy_conflict', message: 'конфиг изменился' } }), { status: 409 }),
  });
  renderUI(<ProxyPanel />);
  await userEvent.click(await screen.findByRole('button', { name: 'Применить' }));
  expect(await screen.findByText('конфиг изменился, обновите страницу')).toBeInTheDocument();
});
```

- [ ] **Step 2: Тесты падают**

Run: `npx vitest run src/pages/ProxyPanel.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Реализовать**

- загрузка схемы и конфига;
- построение `fields` из схемы: только `api.config_write`; группировка по первой части пути; для `general.links.*` и `general.modes.*` — вложенные группы;
- значения по пути через `getPath`/`setPath`;
- поля: bool → `Switch`, enum → `Select`, число → `TextInput` с `type=number`, строка → `TextInput`, массивы/объекты → `TextArea` с JSON;
- кнопки «Проверить», «Применить», панель diff по изменённым путям;
- ошибка `proxy_conflict` → текст «конфиг изменился, обновите страницу»;
- блок «Только через файл» — пути с `config_write: false`;
- `SettingsPage`: добавить вкладку `{ id: 'proxy', label: 'Прокси', sections: [] }` и `{tab === 'proxy' && <ProxyPanel />}`.

- [ ] **Step 4: Прогнать**

Run: `npx vitest run src/pages/ProxyPanel.test.tsx src/pages/SettingsPage.test.tsx`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add frontend/src/pages
git commit -m "feat(ui): редактор конфигурации прокси"
```

---

## Task 12: fake-telemt, моки и e2e

**Files:**
- Modify: `deploy/demo/fake-telemt/main.go`, `deploy/demo/fake-telemt/main_test.go`
- Modify: `frontend/e2e/mock.ts`
- Create: `frontend/e2e/config-exchange.spec.ts`, `frontend/e2e/proxy.spec.ts`

**Interfaces:**
- Produces: демо-стенд с рабочим редактором прокси и e2e.

- [ ] **Step 1: Тесты fake-telemt**

```go
func TestConfigPatchAndReload(t *testing.T) {
	s := newServer("", "demo.example.com")
	srv := httptest.NewServer(s.routes())
	defer srv.Close()
	client := telemt.New(srv.URL, "", time.Second)
	cfg, err := client.GetConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Revision == "" {
		t.Fatal("пустая ревизия")
	}
	_, err = client.PatchConfig(context.Background(), map[string]any{
		"censorship": map[string]any{"tls_domain": "new.example"},
	}, "stale", true)
	var apiErr *telemt.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ожидался конфликт: %v", err)
	}
}
```

- [ ] **Step 2: Тест падает**

Run: `cd deploy/demo/fake-telemt && go test -run TestConfigPatchAndReload -tags demo`
Expected: FAIL, 404.

- [ ] **Step 3: Реализовать**

- в `server` добавить `config map[string]any`, `revision string`, `reloads map[int64]string`, `reloadSeq int64`;
- `routes()` добавить:
  - `GET /v1/config` → `{"ok":true,"revision":...,"data":...}`;
  - `GET /v1/config/schema` → минимальная схема с `api.config_write`;
  - `POST /v1/config/validate` → `{"ok":true,"data":{}}`;
  - `PATCH /v1/config` → проверка `If-Match`, обновление, `reload_id`, `{"ok":true,"revision":...,"data":{...}}`;
  - `GET /v1/system/reload/{id}`.
- `mock.ts`: добавить `/api/config/*` и `/api/proxy/*` с детерминированными ответами.
- e2e: экспорт «Только настройки», precheck, импорт; редактор прокси меняет домен и видит новую ревизию.

- [ ] **Step 4: Прогнать**

```bash
cd deploy/demo/fake-telemt && go test -tags demo ./...
cd ../../frontend && npm run e2e
```

Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add deploy/demo/fake-telemt frontend/e2e
git commit -m "test(e2e): обмен конфигурацией и редактор прокси"
```

---

## Task 13: Документация, версия и релиз

**Files:**
- Create: `docs/adr/0012-config-exchange.md`
- Modify: `README.md`, `README.en.md`, `CHANGELOG.md`, `docs/API.md`, `docs/ARCHITECTURE.md`, `docs/UPGRADE.md`, `docs/adr/README.md`, `deploy/demo/README.md`, `deploy/demo/.env.example`, `deploy/demo/docker-compose.yml`, `deploy/docker-compose.yml`

**Interfaces:**
- Produces: релиз v2.0.0.

- [ ] **Step 1: Обновить CHANGELOG**

Добавить раздел `## [2.0.0] — 2026-11-27` с блоками «Добавлено», «Изменено», «Безопасность».

- [ ] **Step 2: Обновить версии**

`deploy/demo/.env.example` → `v2.0.0`; `deploy/demo/docker-compose.yml` → `${TGADMIN_VERSION:-v2.0.0}`; `deploy/docker-compose.yml` → `telemt-admin:v2.0.0`; `deploy/demo/README.md` → «По умолчанию используется `v2.0.0`».

- [ ] **Step 3: Написать ADR-0012**

Структура: контекст, решение, альтернативы (файл config.toml, tar-архив, клиентский экспорт), последствия, откат.

- [ ] **Step 4: Документация**

- `docs/API.md` — таблица маршрутов, тела, коды ошибок.
- `docs/ARCHITECTURE.md` — поток export/precheck/apply, границы Valkey и telemt.
- `docs/UPGRADE.md` — план в Valkey, отсутствие миграции, откат.
- `deploy/demo/README.md` — fake-telemt, редактор, e2e, скриншоты.
- README/README.en — абзац про обмен и ссылка на документацию.

- [ ] **Step 5: Проверки и скриншоты**

```bash
go test ./...
go vet ./...
./hack/check-anonymized.sh
cd frontend && npm run typecheck && npm run coverage && npm run e2e
```

Обновить скриншоты `npm run screenshots:demo` на стенде v2.0.0: добавить `config-export.png`, `config-precheck.png`, `proxy.png`.

- [ ] **Step 6: Коммит и релиз**

```bash
git add -A
git commit -m "docs: обмен конфигурацией, ADR-0012 и релиз 2.0.0"
git push
gh pr create --base main --head feat/config-exchange --title "feat: обмен конфигурацией и редактор прокси (v2.0.0)"
# после зелёного CI:
gh pr merge --squash --delete-branch
git checkout main && git pull --ff-only
git tag -a v2.0.0 -m "TeleMT Admin v2.0.0: обмен конфигурацией и редактор прокси"
git push origin v2.0.0
```

- [ ] **Step 7: Деплой**

```bash
ssh mcconfig@10.0.0.9 'cd ~/lab/network && \
  docker compose exec valkey valkey-cli -a "$(grep VALKEY_PASSWORD .env | cut -d= -f2)" BGSAVE && \
  cp -a docker-compose.yml docker-compose.yml.bak-$(date +%F-%H%M) && \
  sed -i "s|telemt-admin:v1.11.1|telemt-admin:v2.0.0|" docker-compose.yml && \
  docker compose pull tgadmin && docker compose up -d tgadmin'
```

Проверка: `docker inspect --format '{{.State.Health.Status}}' tgadmin`, `/api/meta` = `v2.0.0`, HTTPS 200.

- [ ] **Step 8: Приватная документация**

Обновить `docs/tgproxy.md` в `~/IdeaProjects/private`: версия образа, раздел обмена конфигурацией, редактор прокси, ограничения.

---

## Самопроверка плана

- Формат, блоки, секреты, precheck, apply, удаление, прокси, API, UI,
  тесты, документация и релиз покрыты задачами 1–13.
- Все сигнатуры, используемые в поздних задачах, определены в ранних.
- Опасные сценарии из Review Focus закреплены тестами в задачах 3, 5,
  6, 7, 8 и 10–12.
- Плейсхолдеров и «сделать аналогично» нет: каждая задача содержит файлы,
  интерфейсы, тесты и команды.

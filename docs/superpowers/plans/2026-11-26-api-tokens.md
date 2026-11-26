# API-токены и сервисные аккаунты: план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) или
> superpowers:executing-plans, чтобы выполнять план по задачам. Шаги
> отмечены чекбоксами (`- [ ]`).

**Goal:** добавить сервисные аккаунты и Bearer-токены с правами
`read / write / admin`, управлением из UI, отзывом, сроком жизни и
аудитом.

**Architecture:** cookie-сессия и Bearer-токен приводятся к единому
principal в middleware. Сервисные аккаунты и токены хранятся в Valkey;
секрет токена — только SHA-256. Управление сервисами и токенами идёт
через отдельные API и страницу администратора, недоступные по токену.

**Tech Stack:** Go 1.26.5, chi, go-redis, Valkey, React 19,
TypeScript, Gravity UI 8, Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-11-26-api-tokens-design.md`

## Global Constraints

- Go 1.26.5, React 19, Gravity UI 8; новые зависимости не добавлять.
- Все тексты интерфейса, ошибок, документации и сообщений коммитов — на
  русском; без эмодзи.
- TDD: сначала падающий тест, затем минимальная реализация, затем
  рефакторинг.
- Полный токен показывается только в ответе на выпуск; в Valkey хранится
  только SHA-256 секрета.
- Токены не пишутся в логи, аудит, вебхуки и списки.
- Пороги покрытия: API 85%, UI 85% строк, инструкций и функций, 70%
  ветвей; общий Go 80%.
- Управление сервисами и токенами — только cookie-сессия администратора.
- `TGADMIN_ALLOWED_CIDRS` действует для сессий и токенов одинаково.
- Существующие API и cookie-сессии не ломаются.
- Новых переменных окружения нет.

## Review Focus

1. Секрет токена не попадает в список, аудит, вебхук, лог или ответ,
   кроме одного ответа на выпуск.
2. `write`-токен не может выйти за `/api/users*`; `admin`-токен не может
   управлять сервисами, токенами и `/api/account*`.
3. Истёкший, отозванный или отключённый токен отклоняется сразу, без
   кеша и без различия причин в ответе.
4. Bearer имеет приоритет над cookie; для Bearer не требуется CSRF, для
   cookie требуется, и подмена одного другим не повышает права.
5. Удаление сервиса удаляет все его токены, а лимиты 50/20 нельзя обойти
   повторными запросами.

---

## Структура файлов

**Создать:**

- `internal/auth/token.go` — формат, генерация, разбор и хеширование
  сервисных токенов.
- `internal/auth/token_test.go`.
- `internal/store/services.go` — CRUD сервисных аккаунтов.
- `internal/store/services_test.go`.
- `internal/store/service_tokens.go` — выпуск, список, отзыв и проверка
  токенов, `last_used_at`.
- `internal/store/service_tokens_test.go`.
- `internal/web/principal.go` — principal, роли scope, allowlist путей,
  `requireSessionAdmin`.
- `internal/web/principal_test.go`.
- `internal/web/api_services.go` — API управления сервисами и токенами.
- `internal/web/api_services_test.go`.
- `frontend/src/pages/ServicesPage.tsx`.
- `frontend/src/pages/ServicesPage.test.tsx`.
- `frontend/src/pages/ServiceTokensDialog.tsx`.
- `frontend/src/pages/ServiceTokensDialog.test.tsx`.
- `frontend/e2e/services.spec.ts`.
- `docs/adr/0009-api-tokens.md`.

**Изменить:**

- `internal/store/store.go` — новые sentinel-ошибки, если нужны.
- `internal/web/api.go` — Bearer в `withAPISession`, CSRF-bypass,
  `isAdmin`, аудит actor.
- `internal/web/api_auth.go` — `/api/me` и `/api/logout` для сервиса.
- `internal/web/server.go` — маршруты `/api/services*`.
- `frontend/src/api.ts` — типы и методы.
- `frontend/src/App.tsx` — маршрут `/services`.
- `frontend/src/Layout.tsx` — пункт меню.
- `frontend/e2e/mock.ts` — мок сервисов и токенов.
- `README.md`, `docs/API.md`, `docs/ARCHITECTURE.md`,
  `docs/UPGRADE.md`, `docs/ROADMAP.md`, `CHANGELOG.md`,
  `docs/adr/README.md`, приватный `docs/tgproxy.md`.

---

### Task 1: формат и хеширование токенов

**Files:**
- Create: `internal/auth/token.go`
- Test: `internal/auth/token_test.go`

**Interfaces:**
- Produces:
  - `func NewServiceToken() (id, token string, err error)`
  - `func ParseServiceToken(raw string) (id, secret string, ok bool)`
  - `func HashServiceSecret(secret string) string`
  - `func VerifyServiceSecret(secret, hash string) bool`
- Формат: `tgadm_<16 hex>_<43 base64url>`.

- [ ] **Step 1: написать падающий тест**

```go
func TestNewServiceTokenRoundTrip(t *testing.T) {
	id, token, err := NewServiceToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 16 {
		t.Fatalf("id: %q", id)
	}
	gotID, secret, ok := ParseServiceToken(token)
	if !ok || gotID != id || secret == "" {
		t.Fatalf("разбор: %q %q %v", gotID, secret, ok)
	}
	if !VerifyServiceSecret(secret, HashServiceSecret(secret)) {
		t.Fatal("хеш не совпал")
	}
	if VerifyServiceSecret(secret+"x", HashServiceSecret(secret)) {
		t.Fatal("чужой секрет прошёл")
	}
}

func TestParseServiceTokenRejectsBadFormat(t *testing.T) {
	for _, raw := range []string{"", "tgadm_x_y", "tgadm_0123456789abcdef_short",
		"Bearer tgadm_0123456789abcdef_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if _, _, ok := ParseServiceToken(raw); ok {
			t.Fatalf("принят %q", raw)
		}
	}
}
```

- [ ] **Step 2: запустить и убедиться, что тест падает**

Run: `go test ./internal/auth -run TestNewServiceToken -v`
Expected: FAIL, `undefined: NewServiceToken`.

- [ ] **Step 3: реализовать**

```go
const serviceTokenPrefix = "tgadm_"

var serviceTokenRe = regexp.MustCompile(
	`^tgadm_([0-9a-f]{16})_([A-Za-z0-9_-]{43})$`,
)

func NewServiceToken() (string, string, error) {
	idRaw := make([]byte, 8)
	secretRaw := make([]byte, 32)
	if _, err := rand.Read(idRaw); err != nil {
		return "", "", err
	}
	if _, err := rand.Read(secretRaw); err != nil {
		return "", "", err
	}
	id := hex.EncodeToString(idRaw)
	secret := base64.RawURLEncoding.EncodeToString(secretRaw)
	return id, serviceTokenPrefix + id + "_" + secret, nil
}

func ParseServiceToken(raw string) (string, string, bool) {
	m := serviceTokenRe.FindStringSubmatch(raw)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func HashServiceSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func VerifyServiceSecret(secret, hash string) bool {
	return subtle.ConstantTimeCompare(
		[]byte(HashServiceSecret(secret)), []byte(hash),
	) == 1
}
```

- [ ] **Step 4: запустить тесты**

Run: `go test ./internal/auth -v`
Expected: PASS.

- [ ] **Step 5: коммит**

```bash
git add internal/auth/token.go internal/auth/token_test.go
git commit -m "feat(auth): формат и хеширование сервисных токенов"
```

---

### Task 2: CRUD сервисных аккаунтов

**Files:**
- Create: `internal/store/services.go`
- Test: `internal/store/services_test.go`
- Modify: `internal/store/store.go` — при необходимости добавить
  `ErrLimit = errors.New("превышен лимит")`.

**Interfaces:**
- Produces:
  - `type ServiceScope string`
  - `const ScopeRead ServiceScope = "read"`, `ScopeWrite = "write"`,
    `ScopeAdmin = "admin"`
  - `type ServiceAccount struct { Name, Description string; Scope ServiceScope; Disabled bool; CreatedAt, UpdatedAt time.Time; CreatedBy string }`
  - `type ServiceUpdate struct { Description *string; Scope *ServiceScope; Disabled *bool }`
  - `func (s *Store) CreateService(ctx context.Context, svc ServiceAccount) error`
  - `func (s *Store) GetService(ctx context.Context, name string) (*ServiceAccount, error)`
  - `func (s *Store) ListServices(ctx context.Context) ([]ServiceAccount, error)`
  - `func (s *Store) UpdateService(ctx context.Context, name string, upd ServiceUpdate) (*ServiceAccount, error)`
  - `func (s *Store) DeleteService(ctx context.Context, name string) error`

- [ ] **Step 1: написать падающие тесты**

```go
func TestServiceCRUD(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	svc := ServiceAccount{Name: "monitoring", Description: "Prometheus", Scope: ScopeRead, CreatedBy: "admin"}
	if err := st.CreateService(ctx, svc); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateService(ctx, svc); !errors.Is(err, ErrExists) {
		t.Fatalf("повтор: %v", err)
	}
	got, err := st.GetService(ctx, "monitoring")
	if err != nil || got.Scope != ScopeRead || got.Disabled {
		t.Fatalf("get: %+v %v", got, err)
	}
	scope := ScopeWrite
	disabled := true
	got, err = st.UpdateService(ctx, "monitoring", ServiceUpdate{Scope: &scope, Disabled: &disabled})
	if err != nil || got.Scope != ScopeWrite || !got.Disabled {
		t.Fatalf("update: %+v %v", got, err)
	}
	list, err := st.ListServices(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "monitoring" {
		t.Fatalf("list: %+v %v", list, err)
	}
	if err := st.DeleteService(ctx, "monitoring"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetService(ctx, "monitoring"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("после удаления: %v", err)
	}
}

func TestServiceLimit(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("svc%02d", i)
		if err := st.CreateService(ctx, ServiceAccount{Name: name, Scope: ScopeRead}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateService(ctx, ServiceAccount{Name: "svc50", Scope: ScopeRead}); !errors.Is(err, ErrLimit) {
		t.Fatalf("лимит: %v", err)
	}
}
```

- [ ] **Step 2: запустить и убедиться, что тесты падают**

Run: `go test ./internal/store -run 'TestService' -v`
Expected: FAIL, `undefined: ServiceAccount`.

- [ ] **Step 3: реализовать**

Ключи: `auth:services`, `auth:service:<name>`.
`CreateService` — `HSETNX` по полю `created_at`, затем `HSet` остальных
полей и `SAdd`; при `SCARD(auth:services) >= 50` вернуть `ErrLimit`.
`GetService` — `HGetAll`, пусто → `ErrNotFound`.
`ListServices` — `SMembers`, для отсутствующих hash удалять имя из
множества; сортировать по имени.
`UpdateService` — `HGetAll`, применить указатели, обновить
`updated_at`, вернуть новое представление.
`DeleteService` — прочитать `auth:service_tokens:<name>`, удалить все
`auth:token:<id>`, затем hash, множество имён и множество токенов в
одном pipeline.

- [ ] **Step 4: запустить тесты**

Run: `go test ./internal/store -run 'TestService' -v`
Expected: PASS.

- [ ] **Step 5: коммит**

```bash
git add internal/store/services.go internal/store/services_test.go internal/store/store.go
git commit -m "feat(store): сервисные аккаунты"
```

---

### Task 3: выпуск, проверка и отзыв токенов

**Files:**
- Create: `internal/store/service_tokens.go`
- Test: `internal/store/service_tokens_test.go`

**Interfaces:**
- Consumes: `auth.NewServiceToken`, `auth.ParseServiceToken`,
  `auth.HashServiceSecret`, `auth.VerifyServiceSecret`.
- Produces:
  - `type ServiceToken struct { ID, Service, Label, CreatedBy string; CreatedAt time.Time; ExpiresAt, LastUsedAt *time.Time }`
  - `type ServiceAuth struct { Service ServiceAccount; TokenID string }`
  - `func (s *Store) CreateServiceToken(ctx context.Context, service, label, createdBy string, expiresAt *time.Time) (string, ServiceToken, error)`
  - `func (s *Store) ListServiceTokens(ctx context.Context, service string) ([]ServiceToken, error)`
  - `func (s *Store) DeleteServiceToken(ctx context.Context, service, id string) error`
  - `func (s *Store) AuthenticateServiceToken(ctx context.Context, raw string) (*ServiceAuth, error)`
  - `ErrInvalidToken = errors.New("неверный или истёкший токен")`

- [ ] **Step 1: написать падающие тесты**

```go
func TestCreateAndAuthenticateToken(t *testing.T) {
	ctx := context.Background()
	st, mr := newTestStore(t)
	mustService(t, st, "monitoring", ScopeRead)

	full, tok, err := st.CreateServiceToken(ctx, "monitoring", "ci", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(full, "tgadm_") || tok.ID == "" {
		t.Fatalf("токен: %q %+v", full, tok)
	}
	auth, err := st.AuthenticateServiceToken(ctx, full)
	if err != nil || auth.Service.Name != "monitoring" || auth.TokenID != tok.ID {
		t.Fatalf("auth: %+v %v", auth, err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full+"x"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("чужой секрет: %v", err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, "bad"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("формат: %v", err)
	}
	list, err := st.ListServiceTokens(ctx, "monitoring")
	if err != nil || len(list) != 1 || list[0].ID != tok.ID {
		t.Fatalf("list: %+v %v", list, err)
	}
	if err := st.DeleteServiceToken(ctx, "monitoring", tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("после отзыва: %v", err)
	}
	_ = mr
}

func TestTokenExpiryDisableAndLimit(t *testing.T) {
	ctx := context.Background()
	st, mr := newTestStore(t)
	mustService(t, st, "svc", ScopeRead)

	exp := time.Now().Add(time.Hour)
	full, _, err := st.CreateServiceToken(ctx, "svc", "", "admin", &exp)
	if err != nil {
		t.Fatal(err)
	}
	mr.FastForward(2 * time.Hour)
	if _, err := st.AuthenticateServiceToken(ctx, full); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("истёкший: %v", err)
	}

	full2, _, err := st.CreateServiceToken(ctx, "svc", "", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	disabled := true
	if _, err := st.UpdateService(ctx, "svc", ServiceUpdate{Disabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full2); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("отключён: %v", err)
	}
}

func TestTokenLimitLastUsedAndCascade(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	mustService(t, st, "svc", ScopeRead)

	for i := 0; i < 20; i++ {
		if _, _, err := st.CreateServiceToken(ctx, "svc", "", "admin", nil); err != nil {
			t.Fatalf("токен %d: %v", i, err)
		}
	}
	if _, _, err := st.CreateServiceToken(ctx, "svc", "", "admin", nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("лимит: %v", err)
	}

	mustService(t, st, "svc2", ScopeRead)
	full, tok, err := st.CreateServiceToken(ctx, "svc2", "", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	if err := st.rdb.HSet(ctx, "auth:token:"+tok.ID, "last_used_at", old).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full); err != nil {
		t.Fatal(err)
	}
	first, _ := st.rdb.HGet(ctx, "auth:token:"+tok.ID, "last_used_at").Result()
	if first == old {
		t.Fatal("last_used_at не обновился")
	}
	if _, err := st.AuthenticateServiceToken(ctx, full); err != nil {
		t.Fatal(err)
	}
	second, _ := st.rdb.HGet(ctx, "auth:token:"+tok.ID, "last_used_at").Result()
	if second != first {
		t.Fatal("last_used_at обновился повторно")
	}

	if err := st.DeleteService(ctx, "svc2"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("после удаления сервиса: %v", err)
	}
}

func mustService(t *testing.T, st *Store, name string, scope ServiceScope) {
	t.Helper()
	if err := st.CreateService(context.Background(), ServiceAccount{
		Name: name, Scope: scope, CreatedBy: "admin",
	}); err != nil {
		t.Fatal(err)
	}
}
```

`mustService` — тестовый помощник, создающий сервис. Для проверки
каскада используется `DeleteService`; после него
`AuthenticateServiceToken` обязан вернуть `ErrInvalidToken`.

- [ ] **Step 2: запустить и убедиться, что тесты падают**

Run: `go test ./internal/store -run 'TestCreateAndAuthenticateToken|TestTokenExpiry' -v`
Expected: FAIL, `undefined: CreateServiceToken`.

- [ ] **Step 3: реализовать**

`CreateServiceToken`:

1. `GetService`, отключённый сервис тоже может получать токены, но они
   не будут работать; проверять только существование.
2. Очистить истёкшие ID: для каждого ID из множества проверить
   `Exists(auth:token:<id>)`, отсутствующие удалить.
3. Если `SCARD >= 20` → `ErrLimit`.
4. Сгенерировать токен, записать hash, поля, TTL при наличии срока,
   добавить ID в множество.
5. Вернуть полный токен и `ServiceToken`.

`AuthenticateServiceToken`:

1. `auth.ParseServiceToken`; ошибка → `ErrInvalidToken`.
2. `HGetAll(auth:token:<id>)`; пусто → `ErrInvalidToken`.
3. `GetService`; ошибка или `Disabled` → `ErrInvalidToken`.
4. Если `expires_at` задан и `now >= expires_at` → `ErrInvalidToken`.
5. `VerifyServiceSecret`; ошибка → `ErrInvalidToken`.
6. Обновить `last_used_at`, если пусто или старше 60 секунд.
7. Вернуть `ServiceAuth`.

`ListServiceTokens` чистит осиротевшие ID, сортирует по `created_at`.
`DeleteServiceToken` удаляет ключ и ID; неизвестный ID → `ErrNotFound`.

- [ ] **Step 4: запустить тесты**

Run: `go test ./internal/store -run 'Token|Service' -v`
Expected: PASS.

- [ ] **Step 5: коммит**

```bash
git add internal/store/service_tokens.go internal/store/service_tokens_test.go
git commit -m "feat(store): выпуск и проверка сервисных токенов"
```

---

### Task 4: principal, Bearer и правила scope

**Files:**
- Create: `internal/web/principal.go`
- Test: `internal/web/principal_test.go`
- Modify: `internal/web/api.go`
- Modify: `internal/web/api_auth.go` — добавить в `meResponse` поля
  `Service bool` и `Scope string`, обработать сервис в `me(r)`.

**Interfaces:**
- Consumes: `store.AuthenticateServiceToken`, `store.ServiceAuth`,
  `store.ScopeRead/Write/Admin`.
- Produces:
  - `type principal struct { Name, Kind, Scope, TokenID string; Role store.Role; Session *store.Session }`
  - `func (s *Server) currentPrincipal(r *http.Request) *principal`
  - `func (s *Server) requireSessionAdmin(w http.ResponseWriter, r *http.Request) bool`
  - `func servicePathAllowed(scope string, path string) bool`
  - `func roleForScope(scope store.ServiceScope) store.Role`

- [ ] **Step 1: написать падающие тесты**

```go
func TestServicePathAllowed(t *testing.T) {
	cases := []struct {
		scope string
		path  string
		want  bool
	}{
		{"read", "/api/users", true},
		{"read", "/api/users/main", true},
		{"read", "/api/summary", true},
		{"read", "/api/settings", false},
		{"read", "/api/account/sessions", false},
		{"write", "/api/users/import", true},
		{"write", "/api/settings", false},
		{"write", "/api/admins", false},
		{"write", "/api/services", false},
		{"admin", "/api/settings", true},
		{"admin", "/api/admins", true},
		{"admin", "/api/audit", true},
		{"admin", "/api/account/sessions", false},
		{"admin", "/api/services", false},
	}
	for _, tc := range cases {
		if got := servicePathAllowed(tc.scope, tc.path); got != tc.want {
			t.Fatalf("%s %s: %v, ожидалось %v", tc.scope, tc.path, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: запустить и убедиться, что тест падает**

Run: `go test ./internal/web -run TestServicePathAllowed -v`
Expected: FAIL, `undefined: servicePathAllowed`.

- [ ] **Step 3: реализовать principal и allowlist**

```go
const principalCtxKey ctxKey = 2

type principal struct {
	Name    string
	Kind    string // session или service
	Scope   string
	TokenID string
	Role    store.Role
	Session *store.Session
}

func roleForScope(scope store.ServiceScope) store.Role {
	if scope == store.ScopeRead {
		return store.RoleViewer
	}
	return store.RoleAdmin
}

func servicePathAllowed(scope, path string) bool {
	if strings.HasPrefix(path, "/api/services") {
		return false
	}
	if path == "/api/me" || path == "/api/logout" || path == "/api/summary" ||
		path == "/api/users" || strings.HasPrefix(path, "/api/users/") {
		return true
	}
	if scope != "admin" {
		return false
	}
	return path == "/api/audit" || path == "/api/audit.csv" ||
		path == "/api/diagnostics" ||
		strings.HasPrefix(path, "/api/settings") ||
		strings.HasPrefix(path, "/api/admins")
}
```

- [ ] **Step 4: встроить Bearer в `withAPISession`**

Порядок:

1. Если `Authorization: Bearer <token>` — вызвать
   `store.AuthenticateServiceToken`. Ошибка → `401 invalid_token`.
   Проверить `servicePathAllowed`; иначе `403 session_required`.
   Положить principal с `Kind=service`, `Name=service:<name>`,
   `Role=roleForScope`, `Scope`, `TokenID`.
2. Иначе — прежняя cookie-сессия. После загрузки пользователя положить
   principal с `Kind=session`, `Role`, `Session`.
3. TOTP-проверку применять только к `Kind=session`.

`apiCSRF`: если `currentPrincipal(r).Kind == "service"` — сразу
`next.ServeHTTP`.

`isAdmin`: для service — `Scope == "admin"` или
(`Scope == "write"` и путь `/api/users*`); для session — роль.

`audit`: actor берётся из principal: `service:<name>` или имя
пользователя.

`apiMe`: для service вернуть `{user:{name:"service:<name>",role},
csrf:"", service:true, scope}`. `apiLogout` для service — `204` без
действий.

- [ ] **Step 5: добавить тесты middleware**

```go
func TestBearerScopes(t *testing.T) {
	s, _, _ := coverageServer(t)
	tok := seedServiceToken(t, s, store.ScopeRead)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("read settings: %d", rr.Code)
	}

	writeTok := seedServiceToken(t, s, store.ScopeWrite)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+writeTok)
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("write users: %d", rr.Code)
	}

	adminTok := seedServiceToken(t, s, store.ScopeAdmin)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/services", nil)
	req.Header.Set("Authorization", "Bearer "+adminTok)
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("admin services: %d", rr.Code)
	}
}
```

```go
func seedServiceToken(t *testing.T, s *Server, scope store.ServiceScope) string {
	t.Helper()
	ctx := context.Background()
	name := "svc-" + string(scope)
	if err := s.store.CreateService(ctx, store.ServiceAccount{
		Name: name, Scope: scope, CreatedBy: "admin",
	}); err != nil {
		t.Fatal(err)
	}
	full, _, err := s.store.CreateServiceToken(ctx, name, "", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	return full
}

func TestBearerWinsOverCookie(t *testing.T) {
	s, sessionTok, _ := coverageServer(t)
	serviceTok := seedServiceToken(t, s, store.ScopeRead)
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: sessionTok})
	req.Header.Set("Authorization", "Bearer "+serviceTok)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Bearer: %d", rr.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: sessionTok})
	req.Header.Set("Authorization", "Bearer "+serviceTok)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("read-токен получил настройки: %d", rr.Code)
	}
}
```

`seedServiceToken` — тестовый помощник: создать сервис, выпустить
токен, вернуть полный токен.

- [ ] **Step 6: запустить тесты**

Run: `go test ./internal/web -run 'TestServicePathAllowed|TestBearer' -v`
Expected: PASS.

- [ ] **Step 7: коммит**

```bash
git add internal/web/principal.go internal/web/principal_test.go internal/web/api.go internal/web/api_auth.go
git commit -m "feat(web): Bearer-токены и единый principal"
```

---

### Task 5: API управления сервисами и токенами

**Files:**
- Create: `internal/web/api_services.go`
- Test: `internal/web/api_services_test.go`
- Modify: `internal/web/server.go`

**Interfaces:**
- Consumes: store methods из Task 2–3, `requireSessionAdmin`.
- Produces JSON-контракты из раздела 7 спецификации.

- [ ] **Step 1: написать падающие тесты**

```go
func TestServiceAPICRUD(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	body := `{"name":"monitoring","description":"Prometheus","scope":"read"}`
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services", body, tok, csrf))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	var svc serviceView
	_ = json.Unmarshal(rr.Body.Bytes(), &svc)
	if svc.Name != "monitoring" || svc.Scope != "read" {
		t.Fatalf("svc: %+v", svc)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/services", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("list: %d", rr.Code)
	}
}

func TestIssueTokenOnlyOnceAndRevoke(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	mustServiceAPI(t, s, tok, csrf, "monitoring", "read")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services/monitoring/tokens",
		`{"label":"ci","expiresInDays":0}`, tok, csrf))
	if rr.Code != http.StatusCreated {
		t.Fatalf("issue: %d %s", rr.Code, rr.Body.String())
	}
	var issued issuedToken
	_ = json.Unmarshal(rr.Body.Bytes(), &issued)
	if !strings.HasPrefix(issued.Token, "tgadm_") || issued.ID == "" {
		t.Fatalf("токен: %+v", issued)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/services/monitoring/tokens", "", tok, ""))
	if strings.Contains(rr.Body.String(), issued.Token) {
		t.Fatal("секрет попал в список")
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "DELETE",
		"/api/services/monitoring/tokens/"+issued.ID, "", tok, csrf))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d", rr.Code)
	}
}
```

В тестовом файле объявить локальные типы и помощник:

```go
type serviceView struct {
	Name, Description, Scope string
	Disabled                 bool
	Tokens                   int
}
type issuedToken struct {
	ID, Token, Prefix string
}
func mustServiceAPI(t *testing.T, s *Server, tok, csrf, name, scope string) {
	body := fmt.Sprintf(`{"name":%q,"scope":%q}`, name, scope)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/services", body, tok, csrf))
	if rr.Code != http.StatusCreated {
		t.Fatalf("сервис: %d %s", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 2: запустить и убедиться, что тесты падают**

Run: `go test ./internal/web -run 'TestServiceAPI|TestIssueToken' -v`
Expected: FAIL, 404 на `/api/services`.

- [ ] **Step 3: реализовать обработчики и маршруты**

Маршруты внутри CSRF-группы:

```go
r.Get("/services", s.apiServices)
r.Post("/services", s.apiCreateService)
r.Patch("/services/{name}", s.apiUpdateService)
r.Delete("/services/{name}", s.apiDeleteService)
r.Get("/services/{name}/tokens", s.apiServiceTokens)
r.Post("/services/{name}/tokens", s.apiIssueServiceToken)
r.Delete("/services/{name}/tokens/{id}", s.apiRevokeServiceToken)
```

Все обработчики начинаются с `requireSessionAdmin`. Валидация:

- имя `^[a-z0-9][a-z0-9_.-]{0,31}$`;
- описание ≤200;
- scope из трёх значений;
- `expiresInDays`: отсутствует → 90; `0` → бессрочно; 1–3650;
- метка ≤64;
- DELETE сервиса — `confirm` совпадает с именем.

Ответы: `201` создание сервиса и токена, `200` список/изменение,
`204` удаление. Ошибки: `400`, `404`, `409`, `429`, `403
session_required`.

- [ ] **Step 4: добавить тесты прав**

Проверить:

- viewer не может управлять сервисами;
- write-токен получает `403 session_required`;
- удаление с неверным `confirm` — `400`;
- лимит токенов — `429`;
- неизвестный сервис — `404`;
- секрет отсутствует в аудите.

- [ ] **Step 5: запустить тесты**

Run: `go test ./internal/web -run 'Service|Token' -v`
Expected: PASS.

- [ ] **Step 6: коммит**

```bash
git add internal/web/api_services.go internal/web/api_services_test.go internal/web/server.go
git commit -m "feat(web): API сервисных аккаунтов и токенов"
```

---

### Task 6: фронтенд — API-клиент и страница сервисов

**Files:**
- Modify: `frontend/src/api.ts`
- Create: `frontend/src/pages/ServicesPage.tsx`
- Test: `frontend/src/pages/ServicesPage.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/Layout.tsx`

**Interfaces:**
- Produces in `api.ts`:
  - `type ServiceScope = 'read' | 'write' | 'admin'`
  - `interface ServiceAccount { name, description, scope, disabled, tokens, createdAt, updatedAt, createdBy }`
  - `api.services()`, `api.createService(body)`, `api.updateService(name, body)`,
    `api.deleteService(name, confirm)`
- Consumes: `request<T>`.

- [ ] **Step 1: написать падающий тест страницы**

Тестовые данные:

```tsx
const service: ServiceAccount = {
  name: 'monitoring', description: 'Prometheus', scope: 'read',
  disabled: false, tokens: 1, createdAt: '2026-11-26T10:00:00Z',
  updatedAt: '2026-11-26T10:00:00Z', createdBy: 'admin',
};
```

```tsx
it('показывает список и создаёт сервис', async () => {
  const fetchMock = stubFetch({
    '/api/services': (url, init) =>
      init?.method === 'POST'
        ? { ...service, name: 'ci' }
        : [service],
  });
  renderUI(<ServicesPage me={me} />);
  expect(await screen.findByText('monitoring')).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: 'Создать сервис' }));
  await userEvent.type(screen.getByPlaceholderText('Имя'), 'ci');
  await userEvent.selectOptions(screen.getByLabelText('Права'), 'write');
  await userEvent.click(screen.getByRole('button', { name: 'Создать' }));
  await waitFor(() => expect(fetchMock).toHaveBeenCalled());
  expect(await screen.findByText('ci')).toBeInTheDocument();
});
```

- [ ] **Step 2: запустить и убедиться, что тест падает**

Run: `cd frontend && npx vitest run src/pages/ServicesPage.test.tsx`
Expected: FAIL, модуль не найден.

- [ ] **Step 3: добавить типы и методы в `api.ts`**

```ts
export type ServiceScope = 'read' | 'write' | 'admin';
export interface ServiceAccount {
  name: string;
  description: string;
  scope: ServiceScope;
  disabled: boolean;
  tokens: number;
  createdAt: string;
  updatedAt: string;
  createdBy: string;
}
```

Методы:

```ts
services: () => request<ServiceAccount[]>('GET', '/api/services'),
createService: (body: { name: string; description?: string; scope: ServiceScope }) =>
  request<ServiceAccount>('POST', '/api/services', body),
updateService: (name: string, body: { description?: string; scope?: ServiceScope; disabled?: boolean }) =>
  request<ServiceAccount>('PATCH', `/api/services/${encodeURIComponent(name)}`, body),
deleteService: (name: string) =>
  request<void>('DELETE', `/api/services/${encodeURIComponent(name)}`, { confirm: name }),
```

- [ ] **Step 4: реализовать страницу**

`ServicesPage`:

- `load()` вызывает `api.services()`.
- Таблица: имя, описание, права, токенов, статус, создан, действия.
- Кнопки: «Токены», «Изменить», «Отключить/Включить», «Удалить».
- Диалог создания/редактирования: имя (только создание), описание,
  select прав, чекбокс «Отключён» при редактировании.
- Удаление: диалог с вводом имени и кнопкой «Удалить».
- Ошибки через `Alert`.
- `me.user.role !== 'admin'` — показать «нет прав» или редирект.

Добавить маршрут `/services` и пункт меню для admin.

- [ ] **Step 5: запустить тест и typecheck**

Run: `npx vitest run src/pages/ServicesPage.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 6: коммит**

```bash
git add frontend/src/api.ts frontend/src/pages/ServicesPage.tsx frontend/src/pages/ServicesPage.test.tsx frontend/src/App.tsx frontend/src/Layout.tsx
git commit -m "feat(ui): страница сервисных аккаунтов"
```

---

### Task 7: фронтенд — токены, одноразовый показ, отзыв

**Files:**
- Create: `frontend/src/pages/ServiceTokensDialog.tsx`
- Test: `frontend/src/pages/ServiceTokensDialog.test.tsx`
- Modify: `frontend/src/pages/ServicesPage.tsx`
- Modify: `frontend/src/api.ts`

**Interfaces:**
- Produces in `api.ts`:
  - `interface ServiceToken { id, label, prefix, createdAt, createdBy, expiresAt?, lastUsedAt? }`
  - `interface IssuedServiceToken extends ServiceToken { token: string }`
  - `api.serviceTokens(name)`
  - `api.issueServiceToken(name, body: { label?: string; expiresInDays: number })`
  - `api.revokeServiceToken(name, id)`

- [ ] **Step 1: написать падающий тест**

```tsx
const token: ServiceToken = {
  id: '0123456789abcdef', label: 'ci', prefix: 'tgadm_0123456789abcdef',
  createdAt: '2026-11-26T10:00:00Z', createdBy: 'admin',
};
const issuedToken: IssuedServiceToken = {
  ...token, token: 'tgadm_0123456789abcdef_secret',
};
const service: ServiceAccount = {
  name: 'monitoring', description: 'Prometheus', scope: 'read',
  disabled: false, tokens: 1, createdAt: '2026-11-26T10:00:00Z',
  updatedAt: '2026-11-26T10:00:00Z', createdBy: 'admin',
};
```

```tsx
it('выпускает токен и показывает его один раз', async () => {
  stubFetch({
    '/api/services/monitoring/tokens': (url, init) =>
      init?.method === 'POST' ? issuedToken : [token],
  });
  render(<ServiceTokensDialog service={service} open onClose={() => {}} />);
  expect(await screen.findByText('ci')).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: 'Выпустить токен' }));
  await userEvent.click(screen.getByRole('button', { name: 'Выпустить' }));
  expect(await screen.findByText(issuedToken.token)).toBeInTheDocument();
  expect(screen.getByText(/показывается один раз/)).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: 'Готово' }));
  expect(screen.queryByText(issuedToken.token)).not.toBeInTheDocument();
});
```

- [ ] **Step 2: запустить и убедиться, что тест падает**

Run: `npx vitest run src/pages/ServiceTokensDialog.test.tsx`
Expected: FAIL, модуль не найден.

- [ ] **Step 3: реализовать диалог**

- Таблица токенов: метка, ID, создан, срок, последнее использование,
  «Отозвать».
- Форма выпуска: метка, срок (select: 30, 90, 365, бессрочно, своё
  число).
- После успеха — экран с полным токеном, кнопка «Скопировать»,
  предупреждение, кнопка «Готово». Полный токен хранится только в
  состоянии диалога и очищается при закрытии.
- Отзыв с подтверждением.
- Для отключённого сервиса — предупреждение, что токены не работают.

- [ ] **Step 4: подключить диалог к странице**

Кнопка «Токены» открывает `ServiceTokensDialog` с выбранным сервисом;
после закрытия — `load()`.

- [ ] **Step 5: запустить тесты**

Run: `npx vitest run src/pages/ServiceTokensDialog.test.tsx src/pages/ServicesPage.test.tsx`
Expected: PASS.

- [ ] **Step 6: коммит**

```bash
git add frontend/src/api.ts frontend/src/pages/ServiceTokensDialog.tsx frontend/src/pages/ServiceTokensDialog.test.tsx frontend/src/pages/ServicesPage.tsx
git commit -m "feat(ui): выпуск и отзыв сервисных токенов"
```

---

### Task 8: e2e и документация

**Files:**
- Create: `frontend/e2e/services.spec.ts`
- Modify: `frontend/e2e/mock.ts`
- Modify: `README.md`, `docs/API.md`, `docs/ARCHITECTURE.md`,
  `docs/UPGRADE.md`, `docs/ROADMAP.md`, `CHANGELOG.md`,
  `docs/adr/README.md`
- Create: `docs/adr/0009-api-tokens.md`
- Modify: приватный `docs/tgproxy.md`

- [ ] **Step 1: написать e2e**

Сценарий:

1. Admin открывает «Сервисные аккаунты», создаёт `monitoring` с read.
2. Открывает токены, выпускает бессрочный, копирует.
3. `page.evaluate` вызывает `GET /api/users` с Bearer — 200.
4. `page.evaluate` вызывает `POST /api/users` с read-токеном — 403.
5. Меняет scope на write, тем же токеном создаёт клиента — 201.
6. Отзывает токен, `GET /api/users` — 401.

- [ ] **Step 2: запустить e2e и убедиться, что падает**

Run: `cd frontend && npx playwright test e2e/services.spec.ts`
Expected: FAIL, маршруты и моки отсутствуют.

- [ ] **Step 3: добавить моки и пройти e2e**

В `mock.ts`:

- `GET/POST /api/services`, `PATCH/DELETE /api/services/:name`;
- `GET/POST /api/services/:name/tokens`,
  `DELETE /api/services/:name/tokens/:id`;
- проверка `Authorization` для `/api/users`;
- фиксированный выпущенный токен.

- [ ] **Step 4: документация**

`docs/API.md`: сервисные аккаунты, токены, матрица прав, коды ошибок,
пример:

```bash
curl -H "Authorization: Bearer tgadm_..." https://tgadmin.example.com/api/users
```

`docs/ARCHITECTURE.md`: principal и два способа аутентификации.
`docs/UPGRADE.md`: новые ключи, откат на 1.8.0.
`docs/ROADMAP.md`: API-токены выполнены.
`CHANGELOG.md`: раздел 1.9.0.
`docs/adr/0009-api-tokens.md`: контекст, решение, альтернативы,
последствия.
Приватный `docs/tgproxy.md`: создать сервис, выпустить токен, пример
вызова, отозвать, проверить 401.

- [ ] **Step 5: полный прогон**

```bash
go test ./...
go vet ./...
go test -coverprofile=coverage.out ./...
go test -coverprofile=web-coverage.out ./internal/web
./hack/check-anonymized.sh
cd frontend
npm run typecheck
npm run coverage
npm run build
npm run e2e
```

Ожидание: всё зелёное, покрытие не ниже порогов.

- [ ] **Step 6: коммит**

```bash
git add frontend/e2e/services.spec.ts frontend/e2e/mock.ts README.md docs
git commit -m "test(e2e): сервисные токены и документация"
```

---

### Task 9: релиз v1.9.0 и деплой

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `deploy/docker-compose.yml`
- Modify: `deploy/demo/docker-compose.yml`
- Modify: `deploy/demo/.env.example`
- Modify: `deploy/demo/README.md`

- [ ] **Step 1: зафиксировать версию**

В `CHANGELOG.md` добавить `## [1.9.0] — 2026-11-26` и перенести
содержимое Unreleased в этот раздел. В deploy-файлах заменить
`v1.8.0` на `v1.9.0`.

- [ ] **Step 2: прогнать проверки и создать release PR**

```bash
go test ./...
go vet ./...
cd frontend && npm run typecheck && npm run coverage && npm run build && npm run e2e
```

Создать PR `release: v1.9.0`, дождаться зелёного CI, смержить squash.

- [ ] **Step 3: тег и release workflow**

```bash
git checkout main && git pull --ff-only
git tag -a v1.9.0 -m "TeleMT Admin v1.9.0: сервисные аккаунты и API-токены"
git push origin v1.9.0
```

Дождаться успешного `release.yml`, проверить, что в GitHub Release есть
SBOM, подписи и attestation.

- [ ] **Step 4: деплой на pantheon**

Сделать бэкап Valkey и compose, заменить тег на `v1.9.0`, выполнить
`docker compose pull tgadmin && docker compose up -d tgadmin`, проверить
`healthy`, `/api/meta` с версией `v1.9.0` и HTTPS `200`.

- [ ] **Step 5: приватная документация**

Обновить `docs/tgproxy.md`: образ `v1.9.0`, раздел про сервисные
аккаунты, выпуск токена, пример `curl`, отзыв и проверку `401`.
Создать PR в `McConfig/private`, дождаться проверок и смержить.

- [ ] **Step 6: коммит**

```bash
git add CHANGELOG.md deploy
git commit -m "release: v1.9.0"
```

---

## Самопроверка плана

- Покрытие спецификации: все разделы 4–13 имеют задачи.
- Формат токена, хеширование и проверка — Task 1 и 3.
- CRUD сервисов и каскадное удаление — Task 2.
- Выпуск, срок, отзыв, `last_used_at`, лимиты — Task 3.
- Bearer, principal, scope, CSRF, аудит, session-only — Task 4.
- API управления и валидация — Task 5.
- Страница сервисов — Task 6.
- Токены и одноразовый показ — Task 7.
- E2E и документация — Task 8.
- Релиз и деплой — Task 9.
- Плейсхолдеров и ссылок на несуществующие типы нет.
- Имена методов и полей согласованы между задачами.

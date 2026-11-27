# WebAuthn/passkeys как второй фактор

- Статус: принято
- Дата: 2026-11-27
- Целевая версия: 1.11.0
- Подход: `go-webauthn/webauthn`, отдельные credential records в Valkey

## 1. Контекст

Сейчас вход состоит из пароля и, если включён TOTP, кода из
приложения или резервного кода. Обязательная 2FA проверяет только
`TOTPEnabled`. Нужен альтернативный второй фактор — passkey, который
удобнее TOTP и устойчивее к фишингу.

Passkey не заменяет пароль. После пароля пользователь выбирает passkey,
TOTP или резервный код. Passkey считается полноценной 2FA: проверки
`meForUser` и `totpRestricted` учитывают наличие хотя бы одного passkey
наравне с `TOTPEnabled`.

## 2. Цели

1. Добавить регистрацию нескольких passkeys в «Аккаунте».
2. Добавить вход по passkey после ввода пароля.
3. Считать наличие passkey выполнением требования обязательной 2FA.
4. Сохранить TOTP и резервные коды как резервный путь.
5. Хранить только публичные credential records; приватные ключи
   остаются у аутентификатора.
6. Проверять origin, RP ID, challenge, подпись и sign count библиотекой.
7. Вести аудит регистрации, входа и удаления.
8. Дать администратору сброс всех вторых факторов пользователя.

## 3. Не входит в задачу

- вход без пароля по discoverable passkey;
- WebAuthn для сервисных аккаунтов и Bearer-токенов;
- аттестация оборудования и проверка политик производителя;
- несколько RP ID или мультидоменный вход;
- замена пароля на passkey;
- импорт/экспорт passkeys;
- аппаратные ключи как отдельный обязательный фактор.

## 4. Зависимости

- Go: `github.com/go-webauthn/webauthn` — протокол, парсинг, проверка.
- Frontend: `@simplewebauthn/browser` — `startRegistration` и
  `startAuthentication`.
- Существующие `@gravity-ui/*` — UI.

Версии фиксируются в `go.mod` и `package-lock.json`.

## 5. Модель данных

| Ключ | Тип | Назначение |
|---|---|---|
| `auth:webauthn:<user>` | set | публичные ID passkeys пользователя |
| `auth:webauthn_cred:<id>` | hash | credential record и метаданные |
| `auth:webauthn_login:<pending>` | string | challenge входа, TTL 5 минут |
| `auth:webauthn_register:<session>` | string | challenge регистрации, TTL 5 минут |

Поля `auth:webauthn_cred:<id>`:

- `user` — имя пользователя UI;
- `credential` — JSON `webauthn.Credential`;
- `name` — имя passkey, до 64 символов;
- `created_at`, `last_used_at` — RFC3339 UTC;
- `transports` — JSON-массив строк;
- `aaguid` — строка;
- `backup_eligible`, `backup_state` — bool;
- `sign_count` — uint32.

ID passkey — 8 случайных байт в hex. ID credential из WebAuthn хранится
внутри записи и индексируется по публичному ID. WebAuthn user handle —
стабильный SHA-256 от имени пользователя, первые 16 байт; имя
пользователя в handle не попадает. Лимит — 10 passkeys на пользователя.

## 6. Конфигурация WebAuthn

RP ID и origin берутся из `TGADMIN_BASE_URL`:

- `RPID` — hostname без порта;
- `RPOrigins` — `scheme://host[:port]`;
- `RPDisplayName` — `TeleMT Admin`.

Если `TGADMIN_BASE_URL` не HTTPS и hostname не `localhost`/`127.0.0.1`,
WebAuthn выключен, а API возвращает `503 webauthn_unavailable` с
подсказкой. Это защищает от неправильной настройки RP ID.

Параметры:

- `userVerification=preferred`;
- `residentKey=discouraged`;
- timeout 5 минут;
- challenge одноразовый, хранится в Valkey и удаляется после проверки.

## 7. Регистрация passkey

1. Пользователь открывает «Аккаунт» → «Passkeys».
2. Нажимает «Добавить passkey», вводит текущий пароль и имя.
3. `POST /api/account/webauthn/register/begin` проверяет пароль, лимит 10
   и создаёт challenge.
4. Браузер вызывает `startRegistration`.
5. `POST /api/account/webauthn/register/finish` проверяет attestation и
   сохраняет credential.
6. UI показывает новый passkey в списке.

Требования:

- только cookie-сессия пользователя; сервисный токен получает
  `403 session_required`;
- текущий пароль обязателен;
- имя passkey: 1–64 символа после trim, по умолчанию «Passkey N»;
- challenge привязан к пользователю и сессии;
- аудит `passkey.create` с именем и публичным ID, без credential.

## 8. Вход

1. `POST /api/login` проверяет пароль.
2. Если у пользователя есть TOTP или passkeys, создаётся pending MFA
   сессия и ответ:

```json
{"mfaRequired":true,"passkey":true,"totp":true}
```

Для обратной совместимости при наличии TOTP остаётся `totpRequired:true`.
3. UI показывает экран второго фактора с кнопкой «Войти с passkey» и
   полем TOTP.
4. `POST /api/login/webauthn/begin` возвращает assertion options с
   allowCredentials пользователя.
5. `POST /api/login/webauthn/finish` проверяет assertion, обновляет
   sign count и `last_used_at`, создаёт полноценную сессию и возвращает
   `/api/me`.
6. TOTP и резервные коды работают как раньше.

Ошибки:

- нет pending-сессии — `401 unauthorized`;
- истёкший challenge — `401 webauthn_challenge`;
- неизвестный credential — `401 bad_passkey`;
- неверная подпись/origin — `401 bad_passkey`;
- слишком много попыток — `429 too_many_attempts`;
- WebAuthn не настроен — `503 webauthn_unavailable`.

Успешный вход: аудит `login.webauthn`.

## 9. Управление и удаление

`GET /api/account/webauthn` возвращает:

```json
[{"id":"0123456789abcdef","name":"MacBook","createdAt":"...","lastUsedAt":"..."}]
```

`DELETE /api/account/webauthn/{id}` требует тело `{"password":"..."}`.

Правила удаления:

- удалять можно только свои passkeys;
- после удаления должен остаться хотя бы один второй фактор, если
  обязательная 2FA включена и у пользователя нет TOTP;
- иначе `409 last_factor` с подсказкой включить TOTP;
- аудит `passkey.delete`.

Администраторский сброс 2FA (`POST /api/admins/{name}/totp-reset`)
дополнительно удаляет все passkeys пользователя и пишет
`admin.mfa_reset`. Название маршрута сохраняется для совместимости.

## 10. Интерфейс

«Аккаунт» → карточка «Passkeys»:

- список: имя, создан, последнее использование;
- кнопка «Добавить passkey»;
- диалог: имя и текущий пароль;
- после регистрации — сообщение и обновление списка;
- удаление с подтверждением пароля;
- ошибка, если браузер не поддерживает WebAuthn.

Экран входа:

- после пароля — «Войти с passkey», «Код из приложения», «Резервный код»;
- если passkey нет — кнопка скрыта;
- если браузер не поддерживает — кнопка скрыта.

## 11. Безопасность

- приватные ключи не покидают аутентификатор;
- в Valkey только публичные credential records;
- challenge одноразовый и с TTL;
- проверка RP ID и origin обязательна;
- sign count обновляется; уменьшение sign count не блокирует вход, но
  логируется, потому что synced passkeys могут его не увеличивать;
- passkey не обходит allowlist CIDR;
- сервисные токены не могут управлять passkeys;
- регистрация и удаление требуют пароль;
- аудит не содержит credential и публичный ключ;
- при сбросе 2FA удаляются все passkeys.

## 12. Тестирование

Store:

- CRUD credential records;
- лимит 10;
- индексы и удаление;
- обновление sign count и last_used.

Web:

- регистрация с fake verifier;
- вход с fake verifier;
- истёкший challenge;
- неверный origin/RP ID;
- пользователь без passkeys;
- запрет последнего фактора;
- сервисный токен получает `403`;
- аудит без credential.

UI:

- список, добавление, удаление;
- ошибки браузера;
- экран входа с passkey и TOTP.

E2E:

- Chromium CDP virtual authenticator;
- регистрация passkey;
- выход;
- вход по passkey;
- удаление passkey;
- fallback на TOTP.

Пороги покрытия сохраняются: API 85%, UI 85/85/85/70.

## 13. Совместимость и поставка

- Новые ключи `auth:webauthn*`; существующие `auth:user:*` не меняются.
- Новых переменных окружения нет; используется `TGADMIN_BASE_URL`.
- Откат на 1.10.x безопасен: старый код новые ключи игнорирует,
  passkey-вход недоступен, TOTP работает.
- Релиз 1.11.0, ADR-0011, README, API, UPGRADE, CHANGELOG.
- Деплой на рабочий хост, обновление приватного `docs/tgproxy.md`.

# API TeleMT Admin

Все ответы — JSON. Ошибки имеют вид:

```json
{"error":{"code":"forbidden","message":"нет прав"}}
```

Коды: `bad_request`, `unauthorized`, `forbidden`, `csrf`, `origin`,
`not_found`, `user_exists`, `too_many_attempts`, `bad_totp`,
`totp_unavailable`, `telemt_error`, `internal`.

## Аутентификация

Вход — cookie сессии `tgadmin_session`, HttpOnly и Secure. Для запросов
`POST`, `PUT`, `PATCH`, `DELETE` нужен заголовок `X-CSRF-Token` со значением
`csrf` из `/api/me` или ответа входа. Источник `Origin` должен совпадать с
`TGADMIN_BASE_URL`.

Роли:

| Роль | Права |
|---|---|
| `viewer` | чтение списка и статистики, без ссылок, полных IP и изменений |
| `admin` | всё, включая ссылки, историю адресов и управление |

Доступ дополнительно ограничен сетями `TGADMIN_ALLOWED_CIDRS`.

## Публичные

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/healthz` | liveness, всегда `ok` |
| GET | `/readyz` | проверка Valkey и telemt |
| GET | `/metrics` | метрики Prometheus |
| GET | `/api/meta` | версия и ссылка на репозиторий |
| POST | `/api/login` | вход по паролю |
| POST | `/api/login/totp` | второй фактор |

`POST /api/login`:

```json
{"username":"admin","password":"..."}
```

Ответ без 2FA:

```json
{"user":{"name":"admin","role":"admin"},"csrf":"..."}
```

Если включена 2FA:

```json
{"totpRequired":true}
```

и cookie ожидающей сессии на 5 минут. `POST /api/login/totp`:

```json
{"code":"123456"}
```

Принимает шестизначный код или резервный код вида `ABCD-EFGH`.

## Сессия

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/api/me` | текущий пользователь и CSRF |
| POST | `/api/logout` | выход |

## Клиенты telemt

| Метод | Путь | Роль | Назначение |
|---|---|---|---|
| GET | `/api/users` | обе | список клиентов |
| POST | `/api/users` | admin | создать клиента |
| GET | `/api/users/{name}` | обе | карточка; admin получает ссылку и историю |
| PATCH | `/api/users/{name}` | admin | лимиты и квоты |
| DELETE | `/api/users/{name}` | admin | удалить, тело `{"confirm":"name"}` |
| POST | `/api/users/{name}/rotate` | admin | новый секрет, тело `{"confirm":"name"}` |
| POST | `/api/users/{name}/enable` | admin | включить |
| POST | `/api/users/{name}/disable` | admin | отключить и закрыть сессии |
| POST | `/api/users/{name}/rename` | admin | `{"newName":"..."}` |
| POST | `/api/users/{name}/reset-quota` | admin | сбросить квоту |
| GET | `/api/users/{name}/qr.png` | admin | QR ссылки |
| GET | `/api/users/{name}/history.csv` | admin | история адресов |
| DELETE | `/api/users/{name}/history` | admin | удалить историю |
| GET | `/api/users/{name}/traffic?days=30` | обе | трафик по дням |
| GET | `/api/users/{name}/timeline?days=14` | обе | новые адреса и страны |
| GET | `/api/users/{name}/trusted` | admin | доверенные страны и адреса |
| PUT | `/api/users/{name}/trusted` | admin | заменить списки |
| PUT | `/api/users/{name}/risk` | admin | пороги риска клиента |
| GET | `/api/users/{name}/note` | admin | заметка |
| PUT | `/api/users/{name}/note` | admin | `{"note":"..."}` |
| GET | `/api/summary` | обе | сводка и здоровье |

`GET /api/users` возвращает массив:

```json
{
  "username":"main",
  "enabled":true,
  "currentConnections":2,
  "activeUniqueIps":1,
  "activeIps":["192.0.2.10"],
  "totalOctets":2048,
  "ips24h":1,
  "ips7d":1,
  "new24h":1,
  "maxToday":1,
  "trafficToday":1024,
  "risk":0,
  "foreignIpsCurrent":0,
  "foreignIps24h":0,
  "countries24h":1,
  "countries7d":1,
  "newCountries24h":0,
  "operators":["LeaseWeb"],
  "trustedCountries":[],
  "trustedIps":[],
  "riskThresholds":{
    "warnIps":null,
    "highIps":null,
    "distinct7d":null,
    "newCountries":null
  },
  "riskDefaults":{
    "warnIps":3,
    "highIps":5,
    "distinct7d":15,
    "newCountries":true
  }
}
```

Лимиты `PATCH /api/users/{name}`:

```json
{
  "maxUniqueIps":3,
  "maxTcpConns":10,
  "clearMaxUniqueIps":false,
  "dataQuotaBytes":10737418240,
  "quotaPeriod":"monthly",
  "rateLimitUpBps":1000000,
  "rateLimitDownBps":5000000,
  "expiration":"2026-12-31T23:59:00Z",
  "sourceDeny":["203.0.113.0/24"],
  "clearSourceDeny":false
}
```

`quotaPeriod`: `daily`, `weekly`, `monthly` или пусто.

`PUT /api/users/{name}/trusted`:

```json
{"countries":["TR","GE"],"ips":["203.0.113.10"]}
```

Доверенные страны и адреса не поднимают риск и не дают алертов.

`PUT /api/users/{name}/risk`:

```json
{"warnIps":8,"highIps":12,"distinct7d":25,"newCountries":false}
```

Поля необязательные. `null` или отсутствие поля означают «использовать
глобальное значение» из `TGADMIN_RISK_IPS_WARN`, `TGADMIN_RISK_IPS_HIGH`,
`TGADMIN_RISK_DISTINCT_7D`. Пустой объект `{}` снимает все переопределения.

Ограничения: `0 <= warnIps < highIps <= 1000`, `0 <= distinct7d <= 10000`.
`newCountries` управляет реакцией на страну, которой раньше не было.

В ответе и в карточке клиента есть два объекта: `riskThresholds` —
переопределения (могут быть `null`) и `riskDefaults` — действующие значения
после подстановки глобальных.

## Пользователи UI

Только `admin`.

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/api/admins` | список |
| POST | `/api/admins` | `{"name","role","password"?}` |
| PATCH | `/api/admins/{name}` | `{"role"?, "disabled"?}` |
| DELETE | `/api/admins/{name}` | удалить |
| POST | `/api/admins/{name}/password` | сбросить пароль |
| POST | `/api/admins/{name}/sessions/revoke` | завершить сессии |
| POST | `/api/admins/{name}/totp-reset` | сбросить 2FA |

Последнего администратора удалить или понизить нельзя, как и отключить себя.

## Аудит

| Метод | Путь | Параметры |
|---|---|---|
| GET | `/api/audit` | `q`, `actor`, `action`, `target`, `from`, `to`, `limit`, `offset` |
| GET | `/api/audit.csv` | весь журнал |

`from` и `to` — `2026-11-24` или RFC3339. `limit` до 1000, по умолчанию 200.

## Свой аккаунт

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/api/account/sessions` | активные сессии |
| POST | `/api/account/sessions/{id}/revoke` | завершить |
| POST | `/api/account/password` | `{"current","next"}` |
| GET | `/api/account/totp` | состояние 2FA |
| POST | `/api/account/totp/setup` | секрет, otpauth-ссылка, QR |
| POST | `/api/account/totp/enable` | `{"code"}`, вернёт резервные коды |
| POST | `/api/account/totp/disable` | `{"password","code"}` |
| POST | `/api/account/totp/recovery` | `{"password","code"}`, новые коды |

`GET /api/account/totp`:

```json
{"available":true,"enabled":false,"recoveryLeft":0}
```

`POST /api/account/totp/setup` возвращает `secret`, `url` и `qr` —
data URI PNG для отображения. Секрет хранится на сервере зашифрованным.

## Ограничения

- вход: ограничение попыток по адресу и имени, блокировка на 15 минут;
- второй фактор: 5 неверных кодов на одну ожидающую сессию;
- запросы к telemt: таймаут 15 секунд;
- размер JSON-тела ограничен 1 МиБ.

## Пример

```bash
curl -c cookies.txt -X POST https://tgadmin.example.com/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"..."}'

CSRF=$(curl -b cookies.txt https://tgadmin.example.com/api/me | jq -r .csrf)

curl -b cookies.txt https://tgadmin.example.com/api/users \
  -H "X-CSRF-Token: $CSRF"
```

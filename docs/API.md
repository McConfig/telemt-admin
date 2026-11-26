# API TeleMT Admin

Все ответы — JSON. Ошибки имеют вид:

```json
{"error":{"code":"forbidden","message":"нет прав","hint":"обратитесь к администратору"}}
```

`hint` может отсутствовать. Коды: `bad_request`, `unauthorized`,
`forbidden`, `csrf`, `origin`, `not_found`, `user_exists`,
`too_many_attempts`, `bad_totp`, `totp_unavailable`, `totp_required`,
`webhook_unavailable`, `webhook_error`, `telemt_*`, `internal`.

Коды `telemt_*` описаны в [DIAGNOSTICS.md](DIAGNOSTICS.md): недоступность,
таймаут, DNS, TLS, авторизация, конфликт, read-only конфигурация и другие.

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
| POST | `/api/users/{name}/reset-quota` | admin | сбросить квоту клиента |
| POST | `/api/users/reset-quota` | admin | массовый сброс квот |
| GET | `/api/users/{name}/quota-reset` | admin | расписание автосброса |
| PUT | `/api/users/{name}/quota-reset` | admin | изменить расписание автосброса |
| GET | `/api/users/{name}/qr.png` | admin | QR ссылки |
| GET | `/api/users/{name}/history.csv` | admin | история адресов |
| GET | `/api/users/{name}/risk-report?days=7` | admin | отчёт по риску |
| GET | `/api/users/{name}/risk.csv?days=30` | admin | отчёт по риску в CSV |
| DELETE | `/api/users/{name}/history` | admin | удалить историю |
| GET | `/api/users/{name}/traffic?days=30` | обе | трафик по дням |
| GET | `/api/users/{name}/timeline?days=14` | обе | новые адреса и страны |
| GET | `/api/users/{name}/trusted` | admin | доверенные страны и адреса |
| PUT | `/api/users/{name}/trusted` | admin | заменить списки |
| PUT | `/api/users/{name}/risk` | admin | пороги риска клиента |
| GET | `/api/users/{name}/note` | admin | заметка |
| PUT | `/api/users/{name}/note` | admin | `{"note":"..."}` |
| GET | `/api/summary` | обе | сводка и здоровье |

### Массовый сброс квот

`POST /api/users/reset-quota` принимает необязательное тело:

```json
{"names":["main","other"]}
```

Пустое тело или `{"names":[]}` означает всех клиентов. За один запрос
обрабатывается не больше 1000 имён; имена очищаются от пробелов,
дубликаты и недопустимые значения не отправляются в telemt. Клиенты
обрабатываются последовательно, запрос отвечает после завершения.
Каждый сброс защищён блокировкой в Valkey, поэтому ручной и плановый
сброс одного клиента не выполняются одновременно.

Ответ:

```json
{
  "total": 3,
  "reset": 2,
  "refused": 0,
  "unknown": 0,
  "notSent": 1,
  "failed": [
    {"name":"main","status":"not_sent","code":"duplicate","message":"имя повторяется"}
  ]
}
```

- `reset` — квота сброшена;
- `refused` — Control API ответил 4xx или клиент занят другим сбросом;
- `unknown` — сеть, таймаут, 5xx или неверный ответ: применился ли сброс,
  неизвестно;
- `notSent` — пустое имя, дубликат или недопустимый формат.

Массовый запрос пишет одно событие аудита `quota.reset.bulk` с итогами;
цель события — `all` или `selected`.

### Расписание автосброса

`GET /api/users/{name}/quota-reset` возвращает действующее расписание
клиента:

```json
{
  "mode":"custom",
  "days":7,
  "time":"05:00",
  "enabled":true,
  "useDays":7,
  "useTime":"05:00",
  "last":"2026-11-26T04:00:00Z",
  "next":"2026-12-03T05:00:00Z"
}
```

`mode` — `inherit` (как у всех), `off` (не сбрасывать) или `custom`
(свои `days` и `time`). `useDays` и `useTime` — действующие значения с
учётом глобальных настроек. `last` и `next` появляются после первого
запуска планировщика; время в UTC.

`PUT /api/users/{name}/quota-reset` принимает те же поля:

```json
{"mode":"custom","days":7,"time":"05:00"}
```

Для `inherit` переопределение удаляется. `days` — от 1 до 365, `time` —
`ЧЧ:ММ`, минуты кратны 5. Ответ — актуальное представление расписания.

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

`GET /api/users/{name}/risk-report?days=7`:

```json
{
  "user":"chernyh",
  "level":2,
  "custom":false,
  "thresholds":{"warnIps":3,"highIps":5,"distinct7d":15,"newCountries":true},
  "current":{
    "activeIps":6,"trustedActive":0,"current":6,
    "distinct24h":8,"distinct7d":22,"maxToday":7,
    "newIps24h":4,"newCountries":1,"foreignIps24h":5,
    "countries":[{"code":"NL","name":"Нидерланды"}]
  },
  "reasons":[
    {"code":"current_ips","severity":2,"value":6,"threshold":5,
     "message":"Активных адресов 6 при высоком пороге 5"},
    {"code":"new_country","severity":1,"value":1,"countries":["NL"],
     "message":"Новые страны за сутки: NL"}
  ],
  "days":[
    {
      "day":"2026-11-25","distinct":6,"max":7,"newIps":2,
      "newCountries":[{"code":"NL","name":"Нидерланды"}],
      "traffic":12345678,
      "ips":[
        {
          "ip":"198.51.100.7","country":"NL","countryName":"Нидерланды",
          "asn":60781,"asnOrg":"LeaseWeb",
          "firstSeen":"2026-11-20T10:00:00Z","lastSeen":"2026-11-25T12:00:00Z",
          "dayFirst":"2026-11-25T08:12:00Z","dayLast":"2026-11-25T12:00:00Z",
          "count":42,"new":true,"trusted":false
        }
      ]
    }
  ]
}
```

`days` — от 1 до 30, по умолчанию 7. Дни идут от нового к старому.
`severity` причины: 1 — предупреждение, 2 — высокий риск. Коды причин:
`current_ips`, `distinct_7d`, `new_country`. Адреса, помеченные доверенными
по IP или стране, исключаются из причин и получают `trusted: true`.
`dayFirst` и `dayLast` пустые для дней, собранных до версии 1.2.0.

`GET /api/users/{name}/risk.csv?days=30` отдаёт те же данные построчно:
`day, distinct, max, new_ips, new_countries, traffic_bytes, ip, country,
country_name, asn, asn_org, first_seen, last_seen, day_first, day_last,
observations, new, trusted`.

## Настройки

Только `admin`. Все изменяющие запросы требуют CSRF.

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/api/settings` | каталог разделов, полей, значений и версии |
| PUT | `/api/settings` | изменить активные настройки |
| GET | `/api/settings/templates` | шаблоны лимитов и шаблон по умолчанию |
| POST | `/api/settings/templates` | создать шаблон |
| PUT | `/api/settings/templates/{name}` | заменить шаблон |
| DELETE | `/api/settings/templates/{name}` | удалить шаблон |
| PUT | `/api/settings/templates/default` | `{"name":"..."}` |
| POST | `/api/settings/webhook/test` | отправить тестовое событие |
| GET | `/api/settings/backups` | статус Valkey и хоста |
| POST | `/api/settings/backups/snapshot` | запустить `BGSAVE` |
| GET | `/api/settings/system` | версия, uptime, сборщик, GeoIP, Valkey |
| GET | `/api/settings/history` | последние события `settings.*` |
| GET | `/api/diagnostics` | состояние Control API, Valkey, сборщика и предупреждения |

Каждое поле каталога содержит `key`, `title`, `description`, `kind`
(`int`, `bool`, `duration`, `string`, `enum`), `status`
(`active`, `readonly`, `planned`), `value`, `default`, `source`
(`env` или `settings`), `restartRequired`, `plannedRelease`, `secret`,
`secretSet`, `min`, `max`, `minDuration`, `maxDuration` и `options`.

`PUT /api/settings`:

```json
{
  "version": 4,
  "values": {
    "risk.warn_ips": 4,
    "risk.high_ips": 8,
    "risk.home_country": "RU",
    "risk.new_countries": true,
    "session.ttl": "720h",
    "links.mode": "tls"
  },
  "reset": ["collect.interval"]
}
```

- неизвестный ключ, `readonly` и `planned` отклоняются с `400`;
- неверный тип или диапазон — `400` с `error.key`;
- устаревшая `version` — `409` и актуальный каталог в `catalog`;
- успех — новый каталог и версия;
- каждое изменение пишется в аудит как `settings.update` с `old -> new`.

Все активные настройки применяются без перезапуска. Изменение retention
влияет на новые записи и сроки; уже истёкшие данные не восстанавливаются.

### Шаблоны лимитов

```json
{
  "name": "router",
  "maxIps": 5,
  "maxTcpConns": 20,
  "dataQuotaBytes": 10737418240,
  "quotaPeriod": "monthly",
  "rateLimitUpBps": 2000000,
  "rateLimitDownBps": 8000000,
  "expirationDays": 30,
  "sourceDeny": ["203.0.113.0/24"],
  "riskWarnIps": 4,
  "riskHighIps": 6,
  "riskDistinct7d": 20,
  "riskNewCountries": false
}
```

Имя: латиница, цифры, точка, дефис, подчёркивание, до 64 символов. Не
больше 50 шаблонов. Ноль или отсутствие поля означают «без лимита».

### Бэкапы

`GET /api/settings/backups` отдаёт `valkey` (`lastSave`, `saveInProgress`,
`lastStatus`, `keys`, `memory`) и `host` (`available`, `lastRun`, `result`,
`size`, `keepDays`, `restoreTest`). `host` заполняется из файла
`TGADMIN_BACKUP_STATUS`, если он смонтирован.

`POST /api/settings/backups/snapshot` запускает `BGSAVE` и возвращает
`202`. Копирование, сжатие, ротация и тест восстановления остаются на
хосте.

Настройки `backup.schedule`, `backup.keep_days` и `backup.restore_test`
при сохранении атомарно пишутся в файл `TGADMIN_BACKUP_CONFIG`:

```json
{"schedule":"04:30","keepDays":14,"restoreTest":true}
```

Хостовый скрипт читает файл и выполняет снимок по расписанию. После успеха
он пишет `TGADMIN_BACKUP_STATUS`, который показывается в панели «Бэкапы».

### Вебхуки

`POST /api/settings/webhook/test` отправляет событие `test` по настроенному
URL и возвращает `{"sent":true,"status":204}`. Ошибка доставки — `502`.
События аудита отправляются с заголовками `X-TeleMT-Event`,
`X-TeleMT-Delivery`, `X-TeleMT-Timestamp` и `X-TeleMT-Signature`.
Подпись — HMAC-SHA256 от `<timestamp>.<body>`.

Активные настройки: `notifications.webhook_url`,
`notifications.webhook_secret`, `notifications.events`. Секрет никогда не
возвращается в каталоге, только `secretSet`. `*` в списке событий включает
все.

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

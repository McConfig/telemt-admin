# TeleMT Admin

Веб-интерфейс для управления пользователями [telemt](https://github.com/telemt/telemt) —
MTProto-прокси для Telegram. Работает через официальный
[Control API](https://github.com/telemt/telemt/blob/main/docs/Architecture/API/API.md)
и не хранит секреты прокси.

## Возможности

- список пользователей: статус, активные IP, соединения, суммарный трафик, поиск по имени;
- создание пользователя с необязательным секретом и лимитами;
- карточка: ссылка `tg://proxy` и QR-код, только для роли `admin`;
- перевыпуск секрета с подтверждением;
- блокировка и разблокировка (активные сессии закрываются);
- переименование с сохранением ключа: ссылки на устройствах продолжают работать;
- удаление с подтверждением; последнего пользователя удалить нельзя;
- лимиты: максимум уникальных IP и максимум TCP-соединений;
- роли `admin` и `viewer`; `viewer` не видит ссылки и ничего не меняет;
- пароли Argon2id, сессии в Valkey, CSRF, ограничение попыток входа, аудит действий;
- доступ только с разрешённых сетей.

## Архитектура

```
браузер
  │ HTTPS (TLS на reverse-proxy)
  ▼
tgadmin :8090
  ├─ проверка CIDR источника
  ├─ сессия в cookie → Valkey
  ├─ шаблоны, CSRF, QR
  └─ клиент Control API → telemt :9091

Valkey ── пользователи UI, сессии, счётчики входов, аудит
telemt :8444 ── MTProto-прокси для Telegram
```

TLS завершается внешним reverse-proxy. Приложение получает обычный HTTP и
проверяет адрес источника по `TGADMIN_ALLOWED_CIDRS`.

## Требования

- Docker и Docker Compose;
- открытый порт прокси (по умолчанию 8444), если нужен доступ снаружи;
- свой домен и TLS для публикации интерфейса.

`deploy/docker-compose.yml` поднимает все три сервиса: `telemt`, `tgadmin`,
`valkey`. Если `telemt` у вас уже есть, уберите сервис `telemt` из compose и
укажите его адрес в `TELEMT_API_URL`.
- reverse-proxy с TLS для публикации (nginx, Caddy, Traefik или встроенный
  reverse-proxy роутера);
- сеть, из которой разрешён доступ.

## Готовый образ

Образы для `linux/amd64` и `linux/arm64` публикуются в GHCR:

```bash
docker pull ghcr.io/mcconfig/telemt-admin:v0.1.2
```

Тег `latest` не используется: в примере и в `deploy/docker-compose.yml`
указана конкретная версия. Обновляйтесь осознанно, меняя тег.

## Установка

1. Соберите конфигурацию telemt и секрет:

```bash
DOMAIN=proxy.example.com PORT=8444 bash deploy/telemt-init.sh
```

Скрипт создаёт `deploy/telemt/secret`, собирает `deploy/telemt/config.toml`
и печатает ссылку `tg://proxy`. Повторный запуск секрет не меняет.

2. Скопируйте пример окружения и задайте значения:

```bash
cp deploy/.env.example deploy/.env
```

Обязательно поменяйте `VALKEY_PASSWORD` и `TGADMIN_BASE_URL`.

3. Запустите сервисы:

```bash
cd deploy
docker compose up -d
```

4. Создайте первого администратора. Пароль вводится интерактивно и в
   переменные окружения не попадает:

```bash
docker compose run --rm tgadmin user add admin --role admin
```

5. Проверьте, что приложение отвечает:

```bash
curl -fsS http://127.0.0.1:8090/healthz
```

Ожидаемый ответ: `ok`.

6. Откройте интерфейс по адресу из `TGADMIN_BASE_URL` и войдите.
7. Подключите Telegram по ссылке из карточки пользователя `main` (или создайте
   отдельного пользователя).

## Переменные окружения

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `TGADMIN_LISTEN` | `:8090` | адрес HTTP-сервера |
| `TGADMIN_ALLOWED_CIDRS` | `127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16` | сети, которым разрешён доступ |
| `TGADMIN_BASE_URL` | `https://tgadmin.example.com` | внешний адрес; проверка Origin и ссылки |
| `TGADMIN_SESSION_TTL` | `720h` | срок жизни сессии |
| `TGADMIN_COOKIE_SECURE` | `true` | `Secure` у cookie; выключайте только для локальной проверки по HTTP |
| `VALKEY_URL` | `redis://valkey:6379/0` | подключение к Valkey |
| `VALKEY_PASSWORD` | — | пароль Valkey, задайте свой |
| `TELEMT_API_URL` | `http://127.0.0.1:9091` | Control API telemt |
| `TELEMT_API_TOKEN` | пусто | значение заголовка авторизации, если включён в telemt |
| `TELEMT_LINK_MODE` | `tls` | какие ссылки показывать: `tls`, `secure`, `classic` |

## Настройка telemt

### Control API

В конфигурации telemt включите API и разрешите адрес, с которого ходит
контейнер:

```toml
[server.api]
enabled = true
listen = "192.0.2.10:9091"
whitelist = ["127.0.0.1/32", "172.16.0.0/12"]
```

`172.16.0.0/12` — docker-сети. Адрес и подсеть подставьте свои. Проверка:

```bash
curl -fsS http://192.0.2.10:9091/v1/capabilities | head -c 200
```

### Ссылки

Чтобы в ссылках был ваш домен, а не определённый автоматически адрес:

```toml
[general.links]
public_host = "proxy.example.com"
public_port = 443
```

`public_host` и `public_port` влияют только на формирование ссылок.

### Пользователь для проверки

```bash
curl -fsS -X POST http://192.0.2.10:9091/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo","max_unique_ips":2}'
```

## Публикация и ограничение доступа

Пример для nginx:

```nginx
server {
    listen 443 ssl;
    server_name tgadmin.example.com;

    ssl_certificate     /etc/ssl/tgadmin.crt;
    ssl_certificate_key /etc/ssl/tgadmin.key;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

TLS можно также завершать на встроенном reverse-proxy роутера; тогда достаточно
проксировать имя на `tgadmin:8090`.

Доступ ограничивается на двух уровнях:

1. приложение отдаёт `403` всем адресам вне `TGADMIN_ALLOWED_CIDRS`;
2. при возможности добавьте правило на внешнем прокси или файрволе, чтобы
   запросы снаружи не доходили до приложения.

## Создание пользователей UI

```bash
docker compose run --rm tgadmin user add alice --role viewer
docker compose run --rm tgadmin user passwd alice
docker compose run --rm tgadmin user list
docker compose run --rm tgadmin user disable alice
docker compose run --rm tgadmin user enable alice
```

## Обновление

```bash
cd deploy
docker compose pull
docker compose up -d
```

Перед обновлением сделайте бэкап Valkey (см. ниже). Схема данных
совместима: ключи `auth:*` и `audit:events` не меняются между версиями 0.x.

## Бэкап и восстановление Valkey

Создать снимок:

```bash
docker compose exec valkey valkey-cli -a "$VALKEY_PASSWORD" BGSAVE
docker compose cp valkey:/data/dump.rdb ./backup-dump.rdb
```

Восстановить:

```bash
docker compose stop tgadmin valkey
docker compose cp ./backup-dump.rdb valkey:/data/dump.rdb
docker compose start valkey
docker compose start tgadmin
```

AOF лежит в том же томе `valkey_data`; для полного бэкапа копируйте каталог
`/data` целиком при остановленных контейнерах.

## Безопасность

- пароли только в виде Argon2id-хэшей, минимум 12 символов;
- cookie `HttpOnly`, `Secure`, `SameSite=Lax`;
- CSRF-токен на каждую форму, проверка `Origin`;
- 10 неудачных входов за 15 минут блокируют вход по IP и по имени;
- роль `viewer` не видит ссылки и не может менять пользователей;
- удаление последнего пользователя запрещено на сервере;
- секреты и ссылки не пишутся в логи и не хранятся в Valkey;
- контейнер работает без root, с `read_only`, `cap_drop: ALL` и
  `no-new-privileges`.

## Устранение неполадок

| Симптом | Что проверить |
|---|---|
| `403 доступ только из локальной сети` | адрес клиента входит в `TGADMIN_ALLOWED_CIDRS`; reverse-proxy передаёт реальный источник |
| `502 telemt не отвечает` | `TELEMT_API_URL`, доступность `:9091`, whitelist API |
| `502 telemt API: ...` | текст ошибки от telemt, права и лимиты |
| `MISSING_CREDENTIAL` в логах telemt | ключ UI не относится к telemt: проверьте `TELEMT_API_TOKEN` |
| Страница входа пишет про создание администратора | выполните `tgadmin user add` из раздела установки |
| После перезапуска все разлогинены | Valkey без персистентности: проверьте том `valkey_data` и AOF |
| Ссылка ведёт на IP, а не на домен | задайте `[general.links] public_host` в telemt |

## Разработка

```bash
go test ./...
go vet ./...
go build ./cmd/tgadmin
```

Тесты используют `miniredis` и `httptest`, внешние сервисы не нужны.

Локальная проверка по HTTP без TLS: задайте `TGADMIN_COOKIE_SECURE=false`,
иначе браузер не отправит cookie сессии. В продакшене оставляйте `true`.

## Лицензия

MIT, см. [LICENSE](LICENSE).

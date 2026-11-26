# Диагностика и ошибки Control API

Страница «Диагностика» доступна только роли `admin` и показывает состояние
всех зависимостей: Control API telemt, сборщика, GeoIP, Valkey, вебхуков,
бэкапов и требований 2FA. Кнопка «Обновить» перечитывает состояние.

## Что проверяется

| Блок | Что показывает | Как реагировать |
|---|---|---|
| Control API | доступность, задержка, версия, код ошибки | проверить адрес, токен, журнал telemt |
| Сборщик | работает ли, последний успешный сбор, последняя ошибка | проверить `/readyz`, логи и интервал |
| GeoIP | включён ли, дата сборки, возраст баз | обновить базы, проверить пути |
| Valkey | ping, задержка, число ключей, память | проверить контейнер и том |
| Вебхуки | настроены ли, секрет, события, последняя доставка | проверить URL, секрет и получатель |
| Бэкапы | последний снимок Valkey, статус хоста, срок, restore-test | проверить таймер и `backup-config` |
| Безопасность | обязательная 2FA, ключ TOTP, админы без 2FA, дедлайн | включить 2FA или задать ключ |

Предупреждения объединяются в один список. Статус `error` означает
критическую проблему: Control API или Valkey недоступны, либо обязательная
2FA включена без ключа TOTP. Статус `warn` — остальные проблемы.

## Коды ошибок Control API

Коды стабильны, тексты могут меняться. API возвращает `error.code`,
`error.message` и `error.hint`.

| Код | Причина | Что проверить |
|---|---|---|
| `telemt_unreachable` | нет соединения | адрес, порт, listener telemt |
| `telemt_timeout` | нет ответа вовремя | нагрузку, журнал, сеть |
| `telemt_dns_failed` | не разрешается имя | DNS на хосте панели |
| `telemt_tls_failed` | сертификат не проверен | имя, срок, цепочку |
| `telemt_auth_failed` | API отклонил токен | `TELEMT_API_TOKEN` |
| `telemt_not_found` | клиент не найден | имя и версию API |
| `telemt_conflict` | состояние изменилось | обновить данные и повторить |
| `telemt_bad_request` | неверные поля | ограничения telemt |
| `telemt_config_read_only` | конфиг только для чтения | режим монтирования |
| `telemt_config_permission_denied` | нет прав на конфиг | владельца и права |
| `telemt_config_storage_full` | нет места | диск и квоты |
| `telemt_config_write_failed` | ошибка записи | каталог, mount, журнал |
| `telemt_config_read_failed` | ошибка чтения | файл, include-файлы, права |
| `telemt_invalid_response` | ответ повреждён | версию API и reverse proxy |
| `telemt_api_error` | другая ошибка API | журнал и номер запроса |
| `telemt_unavailable` | API временно недоступен | повторить позже |

## API

`GET /api/diagnostics` — только admin. Ответ:

```json
{
  "status": "warn",
  "checkedAt": "2026-11-26T00:00:00Z",
  "version": "v1.6.0",
  "uptime": "2h",
  "telemt": {
    "url": "http://telemt:9091",
    "ok": true,
    "latencyMs": 4,
    "version": "3.5.17"
  },
  "collector": {"up": true, "lastSuccess": "2026-11-26T00:00:00Z"},
  "geoip": {"loaded": true, "built": "2026-11-01T00:00:00Z", "ageDays": 25},
  "valkey": {"ok": true, "latencyMs": 1, "keys": 1234, "memory": "2.1M"},
  "webhooks": {"configured": false, "events": [], "secretSet": false, "sent": 0, "failed": 0},
  "backups": {
    "valkey": {"saveInProgress": false, "lastStatus": "ok", "keys": 1234},
    "host": {"available": true, "lastRun": "2026-11-26T04:30:00Z", "keepDays": 14, "restoreTest": true}
  },
  "security": {
    "require2fa": false,
    "graceDays": 7,
    "daysLeft": 0,
    "totpKeySet": true,
    "adminsWithoutTotp": []
  },
  "warnings": []
}
```

Ошибка Control API содержит `code`, `message`, `hint`; секреты и токены
не возвращаются.

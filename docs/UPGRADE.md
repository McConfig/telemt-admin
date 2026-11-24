# Обновление TeleMT Admin

## Поддерживаемая схема

Версии `0.x` и `1.x` используют одни и те же ключи Valkey:

- `auth:*` — пользователи UI, сессии, ограничения входа, 2FA;
- `audit:events` — поток аудита;
- `iphist:*` — история адресов, страны, трафик;
- `trusted:*` — доверенные страны и адреса;
- `iphist:geo:*` — последние агрегаты GeoIP.

При переходе между версиями внутри `0.x`/`1.x` миграции не требуются.
Если схема изменится, это будет явно указано в `CHANGELOG.md` и в описании
релиза.

## Перед обновлением

1. Сделайте снимок Valkey:

   ```bash
   ssh mcconfig@10.0.0.9
   cd ~/lab/network
   sudo systemctl start valkey-backup.service
   ls -lh ~/backups/valkey/
   ```

   Ожидание: свежий `valkey-*.rdb.gz` в каталоге снимков.

2. Прочитайте `CHANGELOG.md` и описание релиза на GitHub.
3. Проверьте, что ключ `TGADMIN_TOTP_KEY` сохранён отдельно от сервера,
   если 2FA включена.

## Обновление

```bash
ssh mcconfig@10.0.0.9
cd ~/lab/network
docker compose pull tgadmin
docker compose up -d tgadmin
```

Проверка:

```bash
ssh mcconfig@10.0.0.9 \
  'curl -fsS http://10.0.0.9:8090/readyz'
```

Ожидание: `{"status":"ok"}`.

```bash
curl -fsS https://tgadmin.example.com/api/meta
```

Ожидание: версия из нового тега.

## Откат

1. Укажите в `docker-compose.yml` предыдущий тег:

   ```yaml
   image: ghcr.io/mcconfig/telemt-admin:v0.9.1
   ```

2. Поднимите контейнер:

   ```bash
   docker compose up -d tgadmin
   ```

3. Если новая версия повредила данные, остановите `tgadmin` и `valkey`,
   распакуйте последний снимок как `dump.rdb` в том `tgadmin_valkey` и
   запустите контейнеры. Инструкция — в `dev/pantheon/README.md`.

Откат образа безопасен: схема Valkey совместима, старые версии просто
игнорируют новые поля.

## Совместимость

| Версия | Go | Node для фронтенда | Valkey |
|---|---|---|---|
| 1.0.x | 1.26+ | 24+ | 8 |
| 0.9.x | 1.26+ | 24+ | 8 |
| 0.8.x | 1.26+ | 24+ | 8 |

Образ собирается под `linux/amd64` и `linux/arm64`.

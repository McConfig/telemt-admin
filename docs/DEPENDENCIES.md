# Зависимости и их обновление

## Основные зависимости

| Компонент | Версия | Назначение | Лицензия |
|---|---|---|---|
| Go | 1.26+ | backend | BSD-3-Clause |
| chi | 5.x | HTTP-роутер | MIT |
| go-redis | 9.x | Valkey | BSD-2-Clause |
| Argon2id (x/crypto) | 0.59+ | пароли | BSD-3-Clause |
| go-qrcode | 0.0.x | QR | MIT |
| maxminddb-golang | 2.x | GeoIP | ISC |
| React | 19.x | UI | MIT |
| Gravity UI | 8.x | компоненты | MIT |
| Vite | 8.x | сборка | MIT |
| Vitest | 5.x | тесты | MIT |
| Playwright | 1.65.x | e2e | Apache-2.0 |

Полный список — в `go.mod`, `frontend/package.json` и `package-lock.json`.

## Внешние сервисы

- **telemt Control API** — обязателен для работы, версия 3.5.x.
- **Valkey 8** — хранилище.
- **DB-IP Lite** — бесплатные базы GeoIP, CC BY 4.0; можно заменить на
  GeoLite2.
- **GitHub** — репозиторий, CI, релизы, Security Advisories.
- **ghcr.io** — образы.

## Политика обновления

- Dependabot проверяет Go, npm и GitHub Actions еженедельно.
- Renovate/Dependabot обновляет Docker-образы; базовые образы закреплены
  по digest.
- Патчи и минорные обновления Actions и npm мержатся после зелёного CI;
  мажорные — вручную.
- Новые зависимости добавляются только с обоснованием, проверкой лицензии
  и отсутствия известных уязвимостей.

## Мониторинг уязвимостей

- `govulncheck` для Go;
- Trivy для файловой системы и конфигов;
- CodeQL и gosec для кода;
- secret scanning и push protection GitHub для секретов;
- Dependabot alerts, security updates и weekly version updates;
- SBOM в каждом релизе.

## Обновление компонентов

1. Прочитать changelog зависимости.
2. Обновить версию и lock-файл.
3. Прогнать `go test ./...`, `go vet`, `npm run typecheck`,
   `npm run coverage`, `npm run e2e`.
4. Для мажорных обновлений проверить вручную совместимость и обновить
   документацию.
5. Выпустить релиз по SemVer.

## Ограничения

- Прокси telemt обновляется отдельно от интерфейса.
- Совместимость Control API проверяется по `go test` и `/readyz`.
- Полная воспроизводимость сборки не гарантируется: frontend-сборка
  зависит от npm-реестра, но версии фиксируются lock-файлом.

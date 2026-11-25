# Достижения проекта

Документ фиксирует проверяемые результаты, а не обещания. Ссылки ведут на
публичные артефакты.

## Версии и релизы

- Версия 1.0.0 выпущена 25 ноября 2026.
- Релизы: <https://github.com/McConfig/telemt-admin/releases>.
- История изменений: [CHANGELOG.md](../CHANGELOG.md).
- Образы: `ghcr.io/mcconfig/telemt-admin` для `linux/amd64` и `linux/arm64`.

## Качество и тесты

- Go: покрытие 81,7% операторов, порог CI 80%.
- Фронтенд: покрытие 64,3% строк, порог CI 60%.
- Vitest: API-слой, страницы и компоненты.
- Playwright: 21 e2e-сценарий, включая создание, переименование,
  перевыпуск, удаление, сортировку и мобильную вёрстку.
- CI: `test`, `security`, `e2e`, CodeQL, Scorecard, fuzz.

## Безопасность

- CodeQL, gosec, Trivy, govulncheck, Semgrep, Gitleaks.
- Argon2id, TOTP с резервными кодами, CSRF, ограничение попыток входа.
- Приватные отчёты об уязвимостях через GitHub Security Advisories.
- Образы подписаны cosign, SBOM и provenance приложены к релизам.
- OpenSSF Scorecard 7,2 на 25 ноября 2026.

## Документация

- README, README.en, API, UPGRADE, ARCHITECTURE, SECURITY, ASSURANCE,
  CODING_STANDARDS, DEPENDENCIES, ROADMAP.
- Публичная история изменений и релизов.

## Наблюдаемость

- `/healthz`, `/readyz`, `/metrics`.
- Метрики клиентов, трафика, GeoIP, HTTP и сборщика.
- Панели Grafana и алерты для риска клиента, сборщика и BGP.

## Ограничения

- Один сопровождающий, bus factor равен одному.
- Покрытие фронтенда ниже 80%.
- Независимый внешний security review пока не проводился.

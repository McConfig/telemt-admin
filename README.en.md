# TeleMT Admin

[Русская версия](README.md)

Web interface for managing [telemt](https://github.com/telemt/telemt)
MTProto proxy users. It talks to the official Control API and never stores
proxy secrets.

## Features

- list proxy clients with connections, active IPs and traffic;
- create, rename, rotate, disable and delete clients without breaking
  existing keys;
- per-client limits: traffic quota, rate, expiration and source deny;
- IP and country history with GeoIP, new-country detection and alerts;
- trusted countries and addresses to suppress false positives;
- TOTP two-factor authentication with recovery codes;
- audit log, CSV export, Prometheus metrics and Grafana dashboards;
- local-only access control, Argon2id passwords and CSRF protection.

## Quick start

See [README.md](README.md) for installation, environment variables,
reverse-proxy configuration and deployment. A short version:

```bash
cd deploy
cp .env.example .env
docker compose up -d
docker compose run --rm tgadmin user add admin --role admin
```

The interface is served on port 8090. Put it behind HTTPS and allow only
your networks.

## Documentation

- [API reference](docs/API.md)
- [Upgrade and compatibility](docs/UPGRADE.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Roadmap](docs/ROADMAP.md)
- [Coding standards](docs/CODING_STANDARDS.md)
- [Dependencies](docs/DEPENDENCIES.md)
- [Achievements](docs/ACHIEVEMENTS.md)
- [Security assurance case](docs/ASSURANCE.md)
- [Changelog](CHANGELOG.md)

## Security

Report vulnerabilities privately via
[GitHub Security Advisories](https://github.com/McConfig/telemt-admin/security/advisories/new).
See [SECURITY.md](SECURITY.md) for supported versions and response times.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Issues and review comments are
accepted in English.

## License

MIT. See [LICENSE](LICENSE).

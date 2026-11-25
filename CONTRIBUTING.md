# Contributing

Thanks for your interest in TeleMT Admin.

## Before you start

- Go 1.26 or newer.
- Node.js 24 or newer for the frontend.
- Read [README.md](README.md), [SECURITY.md](SECURITY.md) and
  [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).
- Do not open a public issue for a security problem. Use
  [GitHub Security Advisories](https://github.com/McConfig/telemt-admin/security/advisories/new).

## Workflow

1. Create a branch from `main`.
2. Make one focused change. Do not mix unrelated refactoring.
3. Add or update tests.
4. Run the checks locally:

   ```bash
   go test ./...
   go vet ./...
   go test -tags demo ./deploy/demo/... ./hack/gen-demo-geoip
   cd frontend && npm ci && npm run typecheck && npm run coverage
   cd frontend && npm run e2e
   ```

5. Open a pull request. CI must be green. Describe what changed, how it was
   tested, and any compatibility or migration notes.

`main` is protected: pull request, green `test`, `security` and `e2e`
checks, linear history. Force-push and branch deletion are disabled.

## Requirements for acceptable contributions

- New behaviour is covered by Go tests, Vitest tests or Playwright tests.
  Bug fixes should include a regression test where practical.
- Public API, environment variables and data format changes are documented
  in `README.md`, `docs/API.md`, `docs/UPGRADE.md` and `CHANGELOG.md`.
- `gofmt`, `go vet`, `golangci-lint`, `tsc --noEmit` and the project tests
  must pass.
- Do not add `continue-on-error`, `|| true` or disabled checks to hide a
  failure.
- Never commit secrets, tokens, private keys, real customer IPs, real
  `tg://` links or production host names. Use `example.com` and RFC 5737
  addresses in tests and documentation.

## Code style

- Go: standard formatting, small packages, errors wrapped with context.
- TypeScript: strict types, no `any` without a reason, Gravity UI
  components, user-facing strings in Russian.
- SQL and shell: prefer explicit, quoted paths; no destructive defaults.
- All `- name:` entries in Ansible files are written in Russian.

Detailed rules are in [docs/CODING_STANDARDS.md](docs/CODING_STANDARDS.md).

## Reviews

The maintainer reviews pull requests for correctness, security, tests and
documentation. Review comments are written in Russian or English. Security
reports are handled privately as described in [SECURITY.md](SECURITY.md).

## License

By contributing you agree that your changes are licensed under the MIT
License. There is no separate CLA.

# Governance

## Model

TeleMT Admin is a small self-hosted project maintained by one person
(`McConfig`). The model is **maintainer-led with public discussion**: anyone
can open an issue or pull request, but the maintainer makes the final
decision and is responsible for releases and security.

This document describes how decisions are made, who does what and what
happens if the maintainer becomes unavailable.

## Roles

| Role | Responsibilities |
|---|---|
| Maintainer | roadmap, architecture, code review, releases, security reports, repository and CI settings |
| Contributor | issues, pull requests, tests, documentation, review comments |
| Security reporter | private vulnerability report through GitHub Security Advisories |
| User | deployment, configuration, bug reports |

Anyone can become a contributor by opening a useful issue or pull request.
There is no separate membership process.

## Decisions

1. Small changes (bug fixes, tests, documentation, dependency updates) are
   decided in the pull request.
2. Larger changes (new public API, data format, deployment model, new
   dependency with a security impact) start as an issue or an OpenSpec
   change with a short design note.
3. The maintainer seeks lazy consensus: if no substantive objection is
   raised within a reasonable time and CI is green, the change is merged.
4. If consensus is not reached, the maintainer decides and records the
   reason in the issue or pull request.
5. Security fixes may be prepared privately and released before public
   discussion.

## Releases

- Versions follow Semantic Versioning: `MAJOR.MINOR.PATCH`.
- `main` is protected: pull request, green `test`, `security` and `e2e`
  checks, linear history, no force-push.
- A release is a signed Git tag `vX.Y.Z`. The release workflow builds
  multi-architecture images, signs the image with cosign, publishes an SBOM
  and provenance, and creates a GitHub Release.
- `CHANGELOG.md` describes user-visible changes and compatibility notes.

## Access and continuity

- Source code, issues, releases and CI live in the
  [GitHub repository](https://github.com/McConfig/telemt-admin).
- Release and registry credentials are stored in GitHub Actions secrets and
  in the maintainer's password manager, not in the repository.
- The maintainer's GitHub account uses a strong unique password and
  two-factor authentication.
- If the maintainer becomes unavailable, the repository remains public and
  forkable under the MIT license. There is currently no second maintainer,
  so the bus factor is one. Adding a trusted co-maintainer is the preferred
  way to improve continuity.
- Recovery of the deployment (Valkey snapshot, configuration, TOTP key) is
  documented separately for the operator and is not part of the public
  repository.

## Code review

Every pull request is reviewed by the maintainer before merge. Review
checks:

- correctness and failure handling, including error paths;
- security: authentication, authorization, input validation, secrets,
  CSRF, access control;
- tests: new behaviour covered by Go, Vitest or Playwright tests;
- compatibility: public API, environment variables, Valkey schema and
  upgrade notes;
- documentation and `CHANGELOG.md`;
- no secrets, real IPs, customer data or disabled checks.

Review comments are written in Russian or English. A change is merged only
after the `test`, `security` and `e2e` checks are green and the review
comments are resolved. `main` is protected and requires a pull request.

## Code of conduct

Participation is governed by [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Security

Vulnerability reports follow [SECURITY.md](SECURITY.md). The maintainer
acknowledges reports within three working days and aims to release a fix
within fourteen days for exploitable issues.

## Changes to this document

Changes are proposed by pull request and take effect after merge. The
current version is the one in `main`.

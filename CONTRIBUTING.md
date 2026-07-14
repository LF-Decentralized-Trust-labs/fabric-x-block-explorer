# Contributing to Fabric-X Block Explorer

Thank you for your interest in contributing! This project is maintained under the
[LF Decentralized Trust](https://www.lfdecentralizedtrust.org/) umbrella and follows
its community norms and the [Hyperledger Code of Conduct](https://wiki.hyperledger.org/display/HYP/Hyperledger+Code+of+Conduct).

---

## Table of Contents

- [Developer Certificate of Origin (DCO)](#developer-certificate-of-origin-dco)
- [Prerequisites](#prerequisites)
- [Building and Testing](#building-and-testing)
- [Branch and Commit Conventions](#branch-and-commit-conventions)
- [Opening a Pull Request](#opening-a-pull-request)
- [Code Review Process](#code-review-process)
- [Reporting Security Vulnerabilities](#reporting-security-vulnerabilities)

---

## Developer Certificate of Origin (DCO)

Every commit **must** be signed off. By signing off you certify that you have the
right to submit the contribution under the project's Apache 2.0 licence.

```bash
git commit --signoff -m "feat: short description"
```

The sign-off appends `Signed-off-by: Your Name <your@email.com>` to the commit
message. Commits without a `Signed-off-by` line will be rejected by CI.

See the full DCO text at <https://developercertificate.org/>.

---

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.26+ | Build the backend binary |
| Node.js | 18+ | UI development and production build |
| npm | 9+ | UI package manager |
| Docker | 28+ | Container-based workflows |
| `docker compose` | v2+ | Local stack |
| `golangci-lint` | v2.10+ | Go linting |
| `sqlc` | v1.30+ | SQL code generation |

---

## Building and Testing

### One-command local E2E (recommended)

```bash
make dev        # build + start committer/postgres/explorer + UI dev server
make dev-down   # tear everything down
```

### Backend

```bash
make build              # compile ./bin/explorer
make lint               # run golangci-lint
make test-all           # all unit tests (auto-starts Postgres)
make test-integration   # integration tests against a live committer node
make coverage           # HTML coverage report → coverage/coverage.html
```

### UI

```bash
make ui-install   # npm ci inside ui/
make ui-dev       # Next.js dev server (hot-reload)
make ui-build     # production build
make ui-lint      # ESLint
```

### SQL code generation

If you change any file under `pkg/db/queries/` or `pkg/db/migrations/`, regenerate
the Go code and commit it:

```bash
make sqlc          # regenerate
make check-sqlc    # verify it is up to date (runs in CI)
```

Run `make help` for the full list of available targets.

---

## Branch and Commit Conventions

### Branch naming

| Prefix | When to use |
|---|---|
| `feat/<short-description>` | New feature |
| `fix/<short-description>` | Bug fix |
| `chore/<short-description>` | Maintenance, deps, CI |
| `docs/<short-description>` | Documentation only |
| `refactor/<short-description>` | Code restructuring without behaviour change |
| `release/<version>` | Release preparation |

### Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<optional scope>): <short summary>

<optional body>

Signed-off-by: Your Name <your@email.com>
```

Types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`.

Keep the summary under 72 characters. Use the body to explain *why*, not *what*.

---

## Opening a Pull Request

1. Fork the repository and create a branch from `main`.
2. Make your changes; ensure all tests pass locally (`make test-all`).
3. Lint your code (`make lint` and `make ui-lint`).
4. Push your branch and open a PR targeting `main`.
5. Fill in the PR description — explain the problem, solution, and any trade-offs.
6. Ensure every commit is signed off (`git commit --signoff`).

PRs are squash-merged. The squash commit will carry your sign-off.

---

## Code Review Process

- At least **one approval** from a maintainer listed in
  [`.github/CODEOWNERS`](.github/CODEOWNERS) is required before merging.
- Address review comments by pushing new commits to the same branch (do not
  force-push unless asked).
- The CI suite (`lint`, `test`, `check-sqlc`, `integration`) must pass before
  merging.
- Maintainers reserve the right to close PRs that are stale (no activity for
  30 days) after a notice comment.

---

## Reporting Security Vulnerabilities

**Please do not open a public GitHub issue for security vulnerabilities.**

See [SECURITY.md](SECURITY.md) for the responsible disclosure process.

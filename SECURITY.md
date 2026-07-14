# Security Policy

## Supported Versions

Only the latest release of Fabric-X Block Explorer receives security fixes.
Older versions are not actively patched.

| Version | Supported |
|---|---|
| latest (`main`) | ✅ Yes |
| older releases | ❌ No |

---

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**
Public disclosure before a fix is available puts all users at risk.

### How to report

Send a description of the vulnerability by email to the maintainers listed in
[`.github/CODEOWNERS`](.github/CODEOWNERS), or use GitHub's private
[Security Advisory](../../security/advisories/new) feature:

1. Go to the **Security** tab of this repository.
2. Click **"Report a vulnerability"**.
3. Fill in the form — include reproduction steps, affected versions, and impact.

### What to include

- A clear description of the vulnerability and its potential impact.
- Steps to reproduce or a proof-of-concept (attach files if needed).
- The version(s) affected.
- Any suggested mitigations you are aware of.

### What to expect

| Timeframe | Action |
|---|---|
| Within **3 business days** | Acknowledgement of receipt |
| Within **14 days** | Initial assessment and severity triage |
| Within **90 days** | Coordinated public disclosure (may be sooner if a fix is ready) |

We follow a **coordinated disclosure** model. We will work with you to understand
the issue, develop a fix, and agree on a disclosure date. Credit will be given to
reporters in the release notes unless you prefer to remain anonymous.

---

## Scope

The following are **in scope** for this policy:

- The Explorer backend (`cmd/`, `pkg/`)
- The Next.js UI (`ui/`)
- The Docker images published to GHCR
- The CI/CD workflows (`.github/workflows/`)

The following are **out of scope**:

- The upstream [Fabric-X committer](https://github.com/hyperledger/fabric-x-committer) — report those issues to that project.
- The [PostgreSQL](https://www.postgresql.org/support/security/) image — report to the PostgreSQL Security team.
- Vulnerabilities in your own deployment infrastructure or configuration.

---

## Security Best Practices for Deployers

- **Pin image tags** — use `:<version>` tags (e.g. `:0.1.0`) rather than `:latest`
  in production to avoid unintended updates.
- **Use TLS** — configure `connection.tls.mode: mtls` in `config.docker.yaml` when
  connecting to the Fabric-X sidecar over a network boundary.
- **Rotate credentials** — change the default `POSTGRES_PASSWORD` in `.env` before
  deploying in any non-local environment.
- **Restrict network access** — expose only the ports you need (`8080` for the REST
  API, `3000` for the UI); do not expose PostgreSQL port `5432` publicly.

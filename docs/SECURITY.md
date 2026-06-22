# Security Policy

This document describes how DealSignal handles security scanning and vulnerability remediation.

## Scope

- Go backend (`apps/api`)
- Web frontend (`apps/web`)
- Infrastructure configuration and secrets

## Scanning tools

| Layer | Tool | Command | Frequency |
|-------|------|---------|-----------|
| Go dependencies | `govulncheck` | `cd apps/api && make security` | Every PR + nightly |
| Frontend dependencies | `pnpm audit` | `cd apps/web && pnpm security` | Every PR + nightly |
| Secrets | `gitleaks` | via GitHub Actions | Every PR + nightly |

## CI gate

The `.github/workflows/security.yml` workflow runs on every pull request and nightly. It must pass before merging.

## Vulnerability severity handling

- **HIGH / CRITICAL** vulnerabilities must be fixed before merging.
- If a fix is not immediately available, file a risk-acceptance issue and add it to the PR description.
- LOW / MEDIUM vulnerabilities should be evaluated and addressed in the next maintenance window.

## Secret handling

- Secrets must never be committed to the repository.
- If `gitleaks` reports a true positive, rotate the secret immediately and rewrite history if necessary.

## Dependency updates

Patch-level Go standard library and module updates that resolve HIGH/CRITICAL vulnerabilities may be committed directly as part of a security PR.

## Reporting

If you discover a security issue, please open a private security advisory on GitHub.

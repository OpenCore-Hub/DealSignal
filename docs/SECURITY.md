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
| Go / frontend dependencies | `trivy fs` | `cd apps/api && make trivy-fs` | Every PR + nightly |
| Container images | `trivy image` | via GitHub Actions | Every PR + nightly |
| Secrets | `gitleaks` | via GitHub Actions | Every PR + nightly |
| Frontend dependencies | `pnpm audit` | `cd apps/web && pnpm security` | Every PR + nightly |

## CI gate

The `.github/workflows/security.yml` workflow runs on every pull request and nightly. It must pass before merging.

## Vulnerability severity handling

- **HIGH / CRITICAL** vulnerabilities must be fixed before merging.
- If a fix is not immediately available, file a risk-acceptance issue and add it to the PR description.
- LOW / MEDIUM vulnerabilities should be evaluated and addressed in the next maintenance window.

## Accepted risks

The following risks are accepted because they originate from upstream images or components that we do not control, and a fix requires an upstream release or a managed alternative.

| Asset | Risk | Rationale | Review date |
|-------|------|-----------|-------------|
| `minio/minio:RELEASE.2025-10-15T17-29-55Z` | Potential future CVEs in the last official MinIO Community Edition image | MinIO Community Edition moved to source-only distribution after this release. We pinned the final pre-built image and will either build from source or migrate to a supported object-store alternative for production. | 2026-09-30 |
| `onlyoffice/documentserver:8.3.3` | Vulnerabilities in bundled OS/packages that ONLYOFFICE has not patched in this tag | The image is pinned to a stable release; upgrades require validating document-editor compatibility. Critical fixes will be back-ported by bumping to the next stable tag after QA. | 2026-09-30 |

## Secret handling

- Secrets must never be committed to the repository.
- If `gitleaks` reports a true positive, rotate the secret immediately and rewrite history if necessary.

## Dependency updates

Patch-level Go standard library and module updates that resolve HIGH/CRITICAL vulnerabilities may be committed directly as part of a security PR.

## Reporting

If you discover a security issue, please open a private security advisory on GitHub.

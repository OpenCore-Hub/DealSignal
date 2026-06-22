# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v2.1.2] - 2026-06-22

### Added

- **Go backend MVP**: full service scaffold with Gin, PostgreSQL (pgx), sqlc, Redis, S3-compatible storage, OpenAI, and JWT auth.
- **Auth & Workspace**: registration, login, tenant/workspace CRUD, members, invites, and subdomain/custom-domain lifecycle with SSL management.
- **Document pipeline**: upload API, signed URLs, PDF bbox/webp extraction, and OnlyOffice-based Office-to-PDF conversion.
- **Search, Evidence & Assistant**: full-text search, evidence extraction, and streaming AI assistant chat backed by real `/search` and `/assistant/chat` endpoints.
- **Smart Links & Analytics**: public/signed link creation, permissions, event tracking, heat-score algorithm, and analytics dashboard data.
- **Deal Rooms**: member management, NDA gating, folder/document permissions, and approval workflows.
- **Notifications & Integrations**: email notifications, Slack OAuth, and HubSpot CRM integration.
- **Frontend-backend integration layer**: `VITE_API_BASE_URL` switch, API adapters, BaseResponse parsing, workspace context propagation, and MSW fallback.
- **Viewer improvements**: CanvasViewer sub-component split, thumbnail strip, page navigation, zoom/rotation, and text selection enhancements.
- **Dashboard signals**: heat-score-driven signal sorting, action postpone/ignore, and real-time analytics wiring.
- **Floating AI assistant**: wired to backend chat endpoint with workspace-aware context and evidence citations.
- **Test automation**: Vitest coverage thresholds, React Testing Library tests, Playwright P0 E2E, Go race-detector tests with coverage gates.
- **Performance testing**: k6 load-test scripts for public-link, signed-url, search, and assistant-chat endpoints.
- **Security scanning**: GitHub Actions workflows for govulncheck, Trivy fs/image scan, gitleaks secret scan, and `pnpm audit`; backend runtime switched to `scratch` for zero OS-package attack surface.

### Changed

- Upgraded `github.com/jackc/pgx/v5` to `5.9.2` and `github.com/quic-go/quic-go` to `0.59.1` to resolve CVE findings.
- Updated `apps/api/Dockerfile` to use a `scratch` runtime image with a non-root numeric user.
- Refined Vitest coverage configuration to focus on core logic and tested components.

### Fixed

- `WorkspaceSwitcher` now synchronizes `currentWorkspace` from URL slug on direct navigation, preventing "No workspace selected" API errors.
- Secret-scan false positive in task documentation excluded via `.gitleaksignore`.
- `golangci-lint` compatibility with Go 1.25 by installing from source.

[unreleased]: https://github.com/OpenCore-Hub/DealSignal/compare/v2.1.2...HEAD
[v2.1.2]: https://github.com/OpenCore-Hub/DealSignal/compare/068d335...v2.1.2

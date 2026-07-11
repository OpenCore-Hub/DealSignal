# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **PII minimization & data-subject rights (COMPLIANCE-001)**:
  - New `IP_HASH_KEY` config and HMAC-SHA256 IP hashing; visitor IPs are no longer stored in plaintext.
  - Migration `062_pii_hashing` converts IP columns to `TEXT`, nullifies historical plaintext IPs, and adds `compliance_audit_log`.
  - New workspace-scoped endpoints for export, anonymize, and delete by visitor email (`/api/workspaces/:slug/compliance/data`).
  - New frontend **Settings > Compliance** page with i18n support (`en` / `zh-CN`).
- Production-hardened document sharing (P0/P1):
  - Invite tokens are now stored as HMAC-SHA256 hashes with lazy backfill for legacy tokens.
  - Security events and page views now record `tenant_id`/`workspace_id` and scroll depth.
  - HMAC-signed `/api/v1/public/files/signed` proxy with `Content-Disposition` support replaces direct MinIO presigned URLs for viewer assets and downloads.
  - Dynamic watermark text (`email | UTC timestamp | IP hash`) returned on public access.
  - Notification rule engine now evaluates `link_opened`, `page_viewed`, and security events with merge-window deduplication.
  - Heat score applies time decay based on link age.
  - New owner/public routes: link archive/renew, visitor Q&A, file requests, file-request upload/approval, AI index file, deal-room slug redirect, and SSE realtime events stream.
  - Retention cleaner, expiry reminder worker, and CRM sync/webhook scaffolding.
  - Partitioned high-volume event tables (`access_logs`, `page_views`, `security_events`) by month on `created_at`; retention is now enforced by dropping old partitions instead of row DELETE.
- Completed `useAsyncData` rollout across remaining data-fetching routes: `WorkspacesPage`, `NewDealRoomPage`, and `InsightsSuggestionsPage`.
- Added unit tests for `WorkspacesPage`, `InsightsSuggestionsPage`, `NewDealRoomPage`, `DealRoomDetailPage`, `ContactDetailPage`, `InsightsOverviewPage`, and `SettingsIntegrationsPage` (141 frontend tests total).
- Added `node` types to `apps/web/tsconfig.app.json` so test files can import Node built-ins (e.g. `node:fs`) for loading locale JSON.
- Added viewer tests for `ViewerToolbar`, `ViewerCanvas`, `useViewerDocument`, and keyboard navigation in `CanvasViewer`.
- Implemented link access request flow: public `POST /api/v1/public/links/:token/access-requests`, workspace approve/reject endpoints, automatic allow-rule creation and invitation email on approval, and per-IP per-link rate limiting (5/hour).
- Disabled placeholder switches in share dialog (file requests, index file, Q&A conversations, screenshot protection) until their backend fields are implemented.
- Polished Share / Invite / Access dialog (SHORT-006): preset overwrite confirmation, 200ms field highlight with `prefers-reduced-motion` support, save-success button state with auto-close in `LinkShareDialog`, unsaved-changes guard in `LinkShareDialog`, validation-driven primary-action disable, Resend tooltip + success toast, and full `en`/`zh-CN` i18n coverage.
- Resolved all outstanding `golangci-lint` warnings across the API, including unused mailer helpers and route wildcard conflicts.

### Changed

- Migrated settings, deal-room, contact, and insights detail pages to the shared `useAsyncData` hook with combined fetchers and `refetch`-based retry UIs.
- Moved `WorkspacesPage` single-workspace redirect into `useEffect` to avoid React setState-during-render warnings.
- Aligned `docs/API-SPEC-v2.1.0.md` with the current backend routing and response shapes:
  - API-10 heat score: added `circle` query param and camelCase `level`/`trend`/`breakdown` response.
  - API-11 suggestions: documented workspace list (`/insights/suggestions`) and link-level generate endpoints with actual response fields.
  - API-12/13 deal rooms: documented actual create/get request/response fields (`template`, `ndaEnabled`, etc.) and noted that folders/members/access_requests are not yet populated.
  - API-15/16 integrations: aligned HubSpot sync and Slack connect response examples with the backend.

### Fixed

- Playwright MSW E2E now forces `VITE_API_BASE_URL=` in `apps/web/playwright.config.ts` so `.env` cannot accidentally disable mocks.
- Stabilized Playwright E2E selectors and upload flow: updated dashboard risk-alert assertions, added `id="file-upload"` / `data-testid="upload-success"` to `Uploader`, and clicked "Upload now" in the P0 test.
- Updated `apps/api/e2e-test.sh` to parse the public token from `short_url` after the backend removed the `public_token` field.

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

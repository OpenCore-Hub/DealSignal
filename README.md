# DealSignal

Secure document sharing, deal room, and intent analytics platform.

## Prerequisites

- Node.js >= 20
- pnpm 10.x
- Docker (for local PostgreSQL)
- PostgreSQL 15+ client (`psql`)

## Project Structure

```
/apps
  /api          # Fastify backend API (Node.js + TypeScript + Drizzle ORM)
  /web          # Vite React frontend (admin + viewer)
/packages
  /shared       # Shared TypeScript types and constants
/drizzle
  /migrations   # Database migration SQL files
```

## Getting Started

### 1. Install dependencies

```bash
pnpm install
```

### 2. Start local PostgreSQL

```bash
pnpm db:start
```

This starts a PostgreSQL 16 container named `dealsignal-postgres` on port 5432 with:
- User: `dealsignal`
- Password: `dealsignal`
- Database: `dealsignal`

A `docker-compose.yml` is also included if you prefer `docker compose`.

### 3. Configure environment

```bash
cp .env.example .env
```

The default `DATABASE_URL` points to the Docker PostgreSQL instance.

Required local development variables:

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string used by the Fastify API and migration runner |
| `SESSION_SECRET` | HMAC secret for API bearer session tokens; use a strong random value outside local development |
| `STORAGE_BACKEND` | Object storage backend: `local` for development or `s3` for S3/R2-compatible storage |
| `STORAGE_BUCKET` | Private source-file bucket name stored with document metadata |
| `STORAGE_LOCAL_DIR` | Local filesystem root for development storage when `STORAGE_BACKEND=local` |
| `STORAGE_ENDPOINT` | S3/R2 endpoint URL when `STORAGE_BACKEND=s3` |
| `STORAGE_REGION` | S3/R2 signing region when `STORAGE_BACKEND=s3` |
| `STORAGE_ACCESS_KEY_ID` | S3/R2 access key when `STORAGE_BACKEND=s3` |
| `STORAGE_SECRET_ACCESS_KEY` | S3/R2 secret key when `STORAGE_BACKEND=s3` |
| `STORAGE_PROXY_SECRET` | HMAC secret for short-lived storage proxy tokens; falls back to `SESSION_SECRET` |

### 4. Run database migrations

```bash
pnpm db:migrate
```

This applies all migrations in `apps/api/drizzle/migrations/` and creates the
P0 schema including `users`, `workspaces`, `documents`, `document_versions`,
`smart_links`, `deal_rooms`, and all related tables plus `document_page_tiles`.

### 5. Start development servers

```bash
# Backend API
pnpm dev

# Frontend (in a separate terminal)
pnpm dev:web
```

- API: http://localhost:3001
- Web app: http://localhost:3000

## Useful Commands

| Command | Description |
|---|---|
| `pnpm build` | Build all packages and apps |
| `pnpm typecheck` | Run TypeScript type checks |
| `pnpm lint` | Run ESLint |
| `pnpm test` | Run package tests where test scripts exist |
| `pnpm i18n:check` | Verify English/Chinese translation parity and reject hardcoded user-facing UI strings |
| `pnpm db:migrate` | Apply database migrations |
| `pnpm db:studio` | Open Drizzle Studio |
| `pnpm db:start` | Start local PostgreSQL container (`docker run`) |
| `pnpm db:stop` | Stop and remove local PostgreSQL container |

## Database

The baseline schema is derived from `database-model.md` and `sql/schema.sql`.
Migrations are stored in `apps/api/drizzle/migrations/`:

- `0000_initial_schema.sql` — initial P0 tables and indexes
- `0001_document_page_tiles.sql` — tile pipeline metadata table
- `0002_user_credentials.sql` — password credential persistence for API auth

All tenant-scoped tables include `workspace_id` and enforce workspace isolation.

## API Authentication Contract

The v0.1 API uses bearer session tokens for local development and early product slices:

```http
Authorization: Bearer <session-token>
```

Initial auth and workspace endpoints:

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/auth/register` | Create user, workspace, owner membership, credential, and session token |
| `POST` | `/auth/login` | Verify email/password and return a session token; optionally scope to `workspaceId` |
| `GET` | `/auth/me` | Resolve the current user, workspace, and role from a bearer token |
| `GET` | `/workspaces` | List workspaces the current user belongs to |
| `POST` | `/workspaces` | Create a new workspace for the current user and return an owner-scoped session |
| `POST` | `/workspaces/switch` | Exchange the current session for a session scoped to another joined workspace |
| `POST` | `/workspaces/:workspaceId/members` | Add an existing user to the current workspace; requires owner/admin role |

User-facing API errors should expose stable `error.code` values so the web app can localize messages through i18n.

## Private Object Storage Contract

DealSignal stores private document bytes through an internal storage provider abstraction rather than public object URLs. Uploaded object metadata should persist only bucket/key fields and checksums in app data, such as `document_versions.storage_bucket`, `document_versions.storage_key`, and `document_versions.checksum_sha256`.

Storage provider capabilities:

| Method | Purpose |
|---|---|
| `putObject` | Store bytes at a private bucket/key and return checksum + size metadata, not a public URL |
| `getStream` | Retrieve a readable stream for a bucket/key after caller-side access control passes |
| `deleteObject` | Delete an object by bucket/key for rollback or retention workflows |
| `createSignedAccess` | Create short-lived signed provider access where supported by the backend |

Initial protected storage endpoints:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/storage/objects/:bucket/*` | Workspace-scoped proxy stream for a persisted bucket/key; requires auth and matching metadata |
| `POST` | `/storage/object-proxy-tokens/:bucket/*` | Issue a short-lived proxy token only after workspace metadata access passes |
| `GET` | `/storage/proxy/*` | Stream an object through the API with a short-lived proxy token |
| `GET` | `/storage/config` | Return non-secret storage backend metadata for authenticated API clients |

Storage errors expose stable `error.code` values plus actionable `error.action` guidance so upload and processing flows can show specific recovery messages.

## i18n Gate

DealSignal supports English and Chinese from the start. All user-facing frontend text must use translation keys from:

```text
apps/web/src/i18n/locales/en.json
apps/web/src/i18n/locales/zh-CN.json
```

Before submitting UI or fullstack changes, run:

```bash
pnpm i18n:check
```

This command verifies translation key parity and fails on hardcoded visible JSX strings.

## License

Proprietary — DealSignal.

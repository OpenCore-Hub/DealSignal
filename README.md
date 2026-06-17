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
| `pnpm db:migrate` | Apply database migrations |
| `pnpm db:studio` | Open Drizzle Studio |
| `pnpm db:start` | Start local PostgreSQL container (`docker run`) |
| `pnpm db:stop` | Stop and remove local PostgreSQL container |

## Database

The baseline schema is derived from `database-model.md` and `sql/schema.sql`.
Migrations are stored in `apps/api/drizzle/migrations/`:

- `0000_initial_schema.sql` — initial P0 tables and indexes
- `0001_document_page_tiles.sql` — tile pipeline metadata table

All tenant-scoped tables include `workspace_id` and enforce workspace isolation.

## License

Proprietary — DealSignal.

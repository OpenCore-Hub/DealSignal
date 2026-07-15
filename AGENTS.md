# Agent Notes

## Project layout

- `apps/api` — Go 1.25 backend (Gin, sqlc, pgx, PostgreSQL, Redis, MinIO, OnlyOffice)
- `apps/web` — React 19 + Vite 8 frontend (TypeScript, Tailwind CSS, shadcn/ui, MSW)
- `docs/` — Architecture, API spec, implementation plans

## Local backend stack

```bash
cd apps/api
cp .env.example .env
# edit .env, especially JWT_SECRET
docker-compose up --build
```

Health check: `curl http://localhost:8080/healthz`  
Readiness probe: `curl http://localhost:8080/readyz`

## Key environment variables

- `JWT_SECRET` — required. Used for signing access/refresh tokens.
- `URL_SIGNING_SECRET` — required. HMAC key for signed viewer asset URLs.
- `INVITE_TOKEN_HASH_KEY` — required. HMAC key for hashing link invitation tokens.
- `IP_HASH_KEY` — required in production. HMAC key for hashing visitor IP addresses.
- `OPENAI_API_KEY` — optional. Leave empty to disable vector search and assistant.
- `OPENAI_BASE_URL` — e.g. `https://openrouter.ai/api/v1`
- `OPENAI_REFERER` / `OPENAI_APP_TITLE` — optional headers for OpenRouter-compatible providers.

## Testing

Backend:

```bash
cd apps/api
go test ./...
go test ./internal/link -tags=integration  # requires PostgreSQL (default: localhost:5435)
./e2e-test.sh      # P0 backend E2E (no AI)
./e2e-ai.sh        # P0 + AI E2E with mock LLM
```

Frontend:

```bash
cd apps/web
pnpm lint
pnpm typecheck
pnpm test
pnpm test:e2e          # MSW mocks
./e2e-real-backend.sh  # real backend
```

## Docker notes

- Use the `docker-compose` binary (not the `docker compose` plugin) in this repo.
- MinIO and OnlyOffice run with `platform: linux/amd64` for Apple Silicon compatibility.
- The API image is `FROM alpine` and ships `poppler-utils` for PDF rendering.
- The web image is `FROM nginx:alpine` and proxies `/api` to the backend service configured via `API_HOST`.

## CI / CD

- `ci.yml` builds and pushes both `ghcr.io/<owner>/api` and `ghcr.io/<owner>/web` images on every push to `main`.
- The `deploy-staging` job is gated on the `staging` GitHub environment and these repository secrets:
  - `STAGING_HOST` — target server hostname or IP.
  - `STAGING_SSH_KEY` — private key with Docker/compose access on the host.
  - `STAGING_COMPOSE_PATH` — absolute path to `docker-compose.staging.yml` on the host.
  - `STAGING_USER` (optional, defaults to `deploy`).
- The staging host should have Docker and the Docker Compose plugin installed. `docker-compose.staging.yml` reuses `apps/api/docker-compose.yml` for infrastructure services and overrides the API/web services to use the images pushed by CI.

## Release version

- `apps/api/.env.example` and `apps/api/docker-compose.yml` should default to the current release version.
- `internal/config/config.go` default version should match the release tag.

## i18n / Internationalization (Mandatory)

All user-facing strings in `apps/web` MUST be internationalized through `apps/web/src/i18n`. Do NOT hard-code English, Chinese, or any other language directly into UI components, page titles, toasts, error messages, placeholders, labels, or empty states.

### Rules

- **MUST use `t('key')` / `Trans` / project i18n hooks** instead of literal strings.
- **MUST provide translations in both supported locales** (`en` and `zh-CN`) under `apps/web/src/i18n/locales/`.
- **MUST keep locale files in sync**: adding a key to one locale requires adding it to the other.
- **MUST NOT use user-entered data (e.g., deal room names, descriptions) as a substitute for UI labels** — the screenshot shows Chinese hard-coded text in an otherwise English page; that is not acceptable.
- **MUST run frontend checks** (`pnpm lint`, `pnpm typecheck`, `pnpm test`) after adding or changing i18n keys.

If you find existing hard-coded strings, convert them to i18n keys and update both locale files as part of the same change.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **DealSignal** (17766 symbols, 40434 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/DealSignal/context` | Codebase overview, check index freshness |
| `gitnexus://repo/DealSignal/clusters` | All functional areas |
| `gitnexus://repo/DealSignal/processes` | All execution flows |
| `gitnexus://repo/DealSignal/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->

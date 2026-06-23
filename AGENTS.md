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

## Key environment variables

- `OPENAI_API_KEY` — optional. Leave empty to disable vector search and assistant.
- `OPENAI_BASE_URL` — e.g. `https://openrouter.ai/api/v1`
- `OPENAI_REFERER` / `OPENAI_APP_TITLE` — optional headers for OpenRouter-compatible providers.

## Testing

Backend:

```bash
cd apps/api
go test ./...
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
- The API image is `FROM scratch`; a writable `/tmp` directory is copied into the final image for document ingestion.

## Release version

- `apps/api/.env.example` and `apps/api/docker-compose.yml` should default to the current release version.
- `internal/config/config.go` default version should match the release tag.

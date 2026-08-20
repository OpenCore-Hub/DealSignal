# DealSignal API

Go backend service for DealSignal.

## Local development

```bash
cd apps/api
cp .env.example .env
# adjust values as needed
go run ./cmd/server
# or: go build -o server ./cmd/server && ./server
# Both load .env from the current directory. Compose DNS hosts (`postgres`,
# `redis`, `minio`) only resolve inside docker-compose; for a host binary use
# 127.0.0.1 and the published ports.
```

## Docker Compose

```bash
cd apps/api
cp .env.example .env
# set a real JWT_SECRET
docker-compose up --build
```

> Note: this environment uses the `docker-compose` binary. If your system only has the `docker compose` plugin, replace `docker-compose` with `docker compose`.

## Health check

```bash
curl http://localhost:8080/healthz
```

## Environment variables

Key variables in `.env`:

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | yes | PostgreSQL connection string |
| `REDIS_URL` | yes | Redis connection string |
| `APP_BASE_URL` | yes | Public API origin (emails, OAuth, HMAC file proxy) |
| `JWT_SECRET` | yes | Signing secret for JWT tokens |
| `S3_BUCKET` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` | yes | MinIO / S3-compatible storage |
| `OPENAI_API_KEY` | no | OpenAI-compatible API key. Leave empty to disable LLM suggestion enrichment. Ask Docs / Diligence removed. |
| `OPENAI_BASE_URL` | no | Custom base URL, e.g. `https://openrouter.ai/api/v1` |
| `ONLYOFFICE_URL` | yes | OnlyOffice Document Server URL |

## End-to-end verification

`e2e-test.sh` exercises the core P0 backend flow without any external AI provider:

```bash
cd apps/api
./e2e-test.sh
```

Ask Docs / assistant chat E2E (`e2e-ai.sh`) was removed with the product surface; the script is a no-op skip for older CI callers.

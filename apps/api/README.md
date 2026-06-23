# DealSignal API

Go backend service for DealSignal.

## Local development

```bash
cd apps/api
cp .env.example .env
# adjust values as needed
go run ./cmd/server
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
| `JWT_SECRET` | yes | Signing secret for JWT tokens |
| `S3_BUCKET` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` | yes | MinIO / S3-compatible storage |
| `OPENAI_API_KEY` | no | OpenAI-compatible API key. Leave empty to disable vector search and assistant. |
| `OPENAI_BASE_URL` | no | Custom base URL, e.g. `https://openrouter.ai/api/v1` |
| `OPENAI_REFERER` / `OPENAI_APP_TITLE` | no | Optional headers for OpenRouter-compatible providers |
| `ONLYOFFICE_URL` | yes | OnlyOffice Document Server URL |

## End-to-end verification

`e2e-test.sh` exercises the core P0 backend flow without any external AI provider:

```bash
cd apps/api
./e2e-test.sh
```

`e2e-ai.sh` runs the same flow plus vector search and AI assistant against a local mock OpenAI-compatible server, so the AI paths can be verified without a real LLM key:

```bash
cd apps/api
./e2e-ai.sh
```

To run the AI flow against a real provider, set `OPENAI_API_KEY` and `OPENAI_BASE_URL` in `.env` and restart the API container.

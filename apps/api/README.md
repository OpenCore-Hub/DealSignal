# DealSignal API

Go backend service scaffold for DealSignal.

## Local development

```bash
cd apps/api
cp .env.example .env
# adjust values as needed
go run ./cmd/server
```

## Docker compose

```bash
cd apps/api
cp .env.example .env
# set a real JWT_SECRET
docker compose up --build
```

## Health check

```bash
curl http://localhost:8080/healthz
```

# DealSignal Web

React 19 + Vite 8 + TypeScript frontend for DealSignal.

## Local development

```bash
cd apps/web
pnpm install
pnpm dev
```

By default the dev server uses MSW mocks. To point to the real backend:

```bash
VITE_API_BASE_URL=http://localhost:8080 pnpm dev
```

## Available scripts

| Script | Purpose |
|--------|---------|
| `pnpm dev` | Start dev server with MSW mocks |
| `pnpm build` | Production build |
| `pnpm lint` | ESLint |
| `pnpm typecheck` | TypeScript |
| `pnpm test` | Vitest unit tests |
| `pnpm test:e2e` | Playwright E2E against MSW mocks |
| `pnpm test:e2e:real` | Playwright E2E against real backend |
| `pnpm security` | Dependency audit |

## Real-backend E2E

Make sure the backend stack is running (`cd apps/api && docker-compose up -d`).

Then run:

```bash
cd apps/web
./e2e-real-backend.sh
```

This script temporarily clears `OPENAI_API_KEY` in `apps/api/.env` so document ingestion succeeds without an external AI provider, runs the frontend E2E suite, and restores the original key afterwards.

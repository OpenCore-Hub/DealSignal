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
VITE_API_BASE_URL=http://127.0.0.1:8090 pnpm dev
```

The browser still calls same-origin `/api`. Vite proxies that path to the
configured origin so login cookies stay first-party (production nginx does
the same). Use the same hostname you type in the address bar (`localhost`
vs `127.0.0.1`) only for the *page*; the API origin can differ.

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

# Frontend QA Scripts

These Playwright-based scripts perform smoke tests against the local dev server.

## Prerequisites

```bash
pnpm install   # installs playwright
```

## Run

```bash
# Screenshot all routes and collect console/page errors
pnpm qa:routes

# Run interactive smoke tests (workspace switcher, theme toggle, AI assistant, create link, mobile)
pnpm qa:interactive
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `QA_BASE_URL` | `http://localhost:5173` | Dev server URL |
| `QA_WORKSPACE` | `acme-capital` | Workspace slug to use |
| `QA_OUTPUT_DIR` | `apps/web/qa-output` | Where screenshots and reports are saved |

## Outputs

- Screenshots: `qa-output/*.png`
- Route report: `qa-output/report.json`

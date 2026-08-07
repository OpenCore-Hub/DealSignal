#!/usr/bin/env bash
# Visitor Ask real-backend gates: API contract (no Vite) + optional UI smoke.
#
# Usage:
#   REAL_API_BASE_URL=http://localhost:8090 ./e2e-visitor-ask-real.sh
#   REAL_API_BASE_URL=http://localhost:8090 ./e2e-visitor-ask-real.sh --ui
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

API_BASE="${REAL_API_BASE_URL:-${VITE_API_BASE_URL:-http://localhost:8090}}"
API_BASE="${API_BASE/127.0.0.1/localhost}"
export REAL_API_BASE_URL="$API_BASE"
export VITE_API_BASE_URL="$API_BASE"

RUN_UI=false
RUN_AI=false
for arg in "$@"; do
  if [[ "$arg" == "--ui" ]]; then RUN_UI=true; fi
  if [[ "$arg" == "--ai" ]]; then RUN_AI=true; fi
done

echo "=== Visitor Ask real-backend API contract ==="
echo "API=$API_BASE"

if ! curl -fsS "$API_BASE/healthz" >/dev/null; then
  echo "ERROR: API not healthy at $API_BASE"
  exit 1
fi

pnpm exec playwright test -c playwright.api-real.config.ts \
  e2e/visitor-ask-real.spec.ts \
  --reporter=list

if [[ "$RUN_UI" == "true" ]]; then
  echo "=== Visitor Ask real-backend UI smoke ==="
  node scripts/kill-port.mjs 5173
  pnpm exec playwright test -c playwright.real.config.ts \
    e2e/visitor-ask-owner-reply-real.spec.ts \
    e2e/visitor-ask-dashboard-nav-real.spec.ts \
    e2e/visitor-ask-engage-policy-real.spec.ts \
    --reporter=list
fi

if [[ "$RUN_AI" == "true" ]]; then
  echo "=== Visitor Ask real-backend AI stream API (optional docling) ==="
  pnpm exec playwright test -c playwright.api-real.config.ts \
    e2e/visitor-ask-ai-stream-real.spec.ts \
    --reporter=list

  echo "=== Visitor Ask real-backend AI UI loop (optional docling) ==="
  node scripts/kill-port.mjs 5173
  pnpm exec playwright test -c playwright.real.config.ts \
    e2e/visitor-ask-ai-ui-real.spec.ts \
    --reporter=list
fi

echo "=== Visitor Ask real-backend gates complete ==="

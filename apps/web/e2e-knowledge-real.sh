#!/usr/bin/env bash
# Focused Playwright knowledge Q&A smoke against a running real API.
# Does NOT rebuild docker — use apps/api/e2e-knowledge.sh for API-only gate first.
#
# Usage:
#   REAL_API_BASE_URL=http://localhost:8090 ./e2e-knowledge-real.sh
#
# Prefer host "localhost" (not 127.0.0.1) so Vite origin and API cookies are
# same-site across ports.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

API_BASE="${REAL_API_BASE_URL:-${VITE_API_BASE_URL:-http://localhost:8090}}"
# Normalize loopback so browser cookies match the Vite host.
API_BASE="${API_BASE/127.0.0.1/localhost}"
export REAL_API_BASE_URL="$API_BASE"
export VITE_API_BASE_URL="$API_BASE"

echo "=== Knowledge Q&A real-backend UI smoke ==="
echo "API=$API_BASE"

if ! curl -fsS "$API_BASE/healthz" >/dev/null; then
  echo "ERROR: API not healthy at $API_BASE"
  exit 1
fi

node scripts/kill-port.mjs 5173
pnpm exec playwright test -c playwright.real.config.ts \
  e2e/deal-room-knowledge-qa-real.spec.ts \
  --reporter=list

echo "=== Knowledge Q&A real UI smoke complete ==="

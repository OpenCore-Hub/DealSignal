#!/usr/bin/env bash
set -euo pipefail

# Run frontend E2E against the real backend.
# Temporarily disables the OpenAI key so document ingestion succeeds without
# relying on an external provider.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

API_ENV="../api/.env"
ORIG_KEY=""

cleanup() {
  if [[ -n "$ORIG_KEY" ]]; then
    sed -i '' "s|^OPENAI_API_KEY=.*|OPENAI_API_KEY=$ORIG_KEY|" "$API_ENV"
    echo "[cleanup] restored OPENAI_API_KEY"
  fi
  echo "[cleanup] restarting API with restored env"
  (cd ../api && docker-compose up -d --no-deps api >/dev/null 2>&1) || true
}
trap cleanup EXIT

if [[ ! -f "$API_ENV" ]]; then
  echo "ERROR: $API_ENV not found"
  exit 1
fi

ORIG_KEY=$(grep '^OPENAI_API_KEY=' "$API_ENV" | cut -d= -f2- || true)
if [[ -z "$ORIG_KEY" ]]; then
  ORIG_KEY=""
fi

sed -i '' 's|^OPENAI_API_KEY=.*|OPENAI_API_KEY=|' "$API_ENV"
echo "[env] OPENAI_API_KEY temporarily cleared"

echo "[api] restarting to disable AI"
(cd ../api && docker-compose up -d --no-deps api >/dev/null)
sleep 5

until curl -fsS http://localhost:8080/healthz >/dev/null 2>&1; do
  sleep 1
done

echo "[frontend] running real-backend E2E"
pnpm test:e2e:real

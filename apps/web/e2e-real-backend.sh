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
    sed -e "s|^OPENAI_API_KEY=.*|OPENAI_API_KEY=$ORIG_KEY|" "$API_ENV" > "$API_ENV.tmp"
    mv "$API_ENV.tmp" "$API_ENV"
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

sed -e 's|^OPENAI_API_KEY=.*|OPENAI_API_KEY=|' "$API_ENV" > "$API_ENV.tmp"
mv "$API_ENV.tmp" "$API_ENV"
echo "[env] OPENAI_API_KEY temporarily cleared"

echo "[api] rebuilding and restarting to disable AI"
# Relax rate limits for E2E; tests register many users rapidly.
export RATE_LIMIT_AUTH_RPM=10000
export RATE_LIMIT_PUBLIC_RPM=10000
export RATE_LIMIT_WORKSPACE_RPM=10000
export RATE_LIMIT_UPLOAD_RPM=10000

# Avoid BuildKit token timeouts on macOS by using the classic builder.
export DOCKER_BUILDKIT=0

# Clear any stale rate-limit counters so the relaxed limits take effect.
docker exec api-redis-1 redis-cli FLUSHALL >/dev/null 2>&1 || true

(cd ../api && docker-compose build --no-cache api >/dev/null && \
  docker-compose up -d --no-deps --force-recreate api >/dev/null)

until curl -fsS http://localhost:8080/healthz >/dev/null 2>&1; do
  sleep 1
done
sleep 2

echo "[frontend] running ALL real-backend E2E specs"
pnpm test:e2e:real
echo ""
echo "=== Real-backend E2E complete ==="

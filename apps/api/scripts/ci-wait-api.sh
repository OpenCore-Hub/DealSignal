#!/usr/bin/env bash
# Production-grade API readiness wait for CI docker-compose stacks.
# GitHub Actions runs steps with `bash -e`; never let a failed curl abort the loop,
# and never re-curl after success (startup races can reset the second request).
set +e
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
URL="${1:-http://localhost:8080/healthz}"
ATTEMPTS="${2:-60}"
SLEEP_SECS="${3:-2}"

for i in $(seq 1 "$ATTEMPTS"); do
  body="$(curl -fsS "$URL" 2>/dev/null)"
  status=$?
  if [ "$status" -eq 0 ]; then
    echo "API ready ($URL) after ${i} attempt(s)"
    printf '%s\n' "$body"
    exit 0
  fi
  echo "wait-api: attempt ${i}/${ATTEMPTS} failed (curl exit ${status}); sleeping ${SLEEP_SECS}s"
  sleep "$SLEEP_SECS"
done

echo "API did not become ready at $URL" >&2
if command -v docker-compose >/dev/null 2>&1; then
  (cd "$ROOT" && docker-compose ps) || true
  (cd "$ROOT" && docker-compose logs --no-color --tail=200 api) || true
elif command -v docker >/dev/null 2>&1; then
  (cd "$ROOT" && docker compose ps) || true
  (cd "$ROOT" && docker compose logs --no-color --tail=200 api) || true
fi
exit 1

#!/usr/bin/env bash
# Production-grade API readiness wait for CI docker-compose stacks.
# Avoids `set -e` aborting on the first failed curl (GitHub Actions default shell).
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
URL="${1:-http://localhost:8080/healthz}"
ATTEMPTS="${2:-60}"
SLEEP_SECS="${3:-2}"

for i in $(seq 1 "$ATTEMPTS"); do
  if curl -fsS "$URL" >/dev/null; then
    echo "API ready ($URL) after ${i} attempt(s)"
    curl -fsS "$URL"
    echo
    exit 0
  fi
  echo "wait-api: attempt ${i}/${ATTEMPTS} failed; sleeping ${SLEEP_SECS}s"
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

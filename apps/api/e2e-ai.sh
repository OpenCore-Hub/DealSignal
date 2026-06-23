#!/usr/bin/env bash
set -euo pipefail

# Run the full P0 + AI end-to-end verification against a local mock OpenAI-compatible
# server so the AI flows do not depend on an external provider.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

MOCK_IMAGE="python:3.12-alpine"
MOCK_CONTAINER="dealsignal-mock-llm"
MOCK_PORT="8000"
ORIG_BASE_URL=""

cleanup() {
  echo "[cleanup] stopping mock LLM server"
  docker rm -f "$MOCK_CONTAINER" >/dev/null 2>&1 || true
  if [[ -n "$ORIG_BASE_URL" ]]; then
    sed -e "s|^OPENAI_BASE_URL=.*|OPENAI_BASE_URL=$ORIG_BASE_URL|" .env > .env.tmp
    mv .env.tmp .env
    echo "[cleanup] restored OPENAI_BASE_URL=$ORIG_BASE_URL"
  fi
  docker-compose up -d --no-deps api >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "=== DealSignal E2E P0 + AI verification (mock LLM) ==="

# Pull mock server image if missing.
if ! docker image inspect "$MOCK_IMAGE" >/dev/null 2>&1; then
  echo "[mock llm] pulling $MOCK_IMAGE"
  docker pull "$MOCK_IMAGE"
fi

# Start mock LLM server on the same Docker network as the API.
echo "[mock llm] starting container $MOCK_CONTAINER"
docker rm -f "$MOCK_CONTAINER" >/dev/null 2>&1 || true
docker run -d --rm \
  --name "$MOCK_CONTAINER" \
  --network "${COMPOSE_PROJECT_NAME:-api}_default" \
  -p "$MOCK_PORT:$MOCK_PORT" \
  -v "$SCRIPT_DIR/scripts/mock-llm-server.py:/app/server.py:ro" \
  "$MOCK_IMAGE" \
  python /app/server.py >/dev/null

# Wait for mock server health.
for i in $(seq 1 30); do
  if curl -fsS "http://localhost:$MOCK_PORT/healthz" >/dev/null 2>&1; then
    echo "[mock llm] healthy on port $MOCK_PORT"
    break
  fi
  sleep 1
  if [[ "$i" == "30" ]]; then
    echo "ERROR: mock LLM server did not become healthy"
    exit 1
  fi
done

# Point the API at the mock server.
ORIG_BASE_URL=$(grep '^OPENAI_BASE_URL=' .env | cut -d= -f2- || true)
if [[ -z "$ORIG_BASE_URL" ]]; then
  ORIG_BASE_URL=""
fi
sed -e "s|^OPENAI_BASE_URL=.*|OPENAI_BASE_URL=http://$MOCK_CONTAINER:$MOCK_PORT/v1|" .env > .env.tmp
mv .env.tmp .env
echo "[env] OPENAI_BASE_URL -> http://$MOCK_CONTAINER:$MOCK_PORT/v1"

# Restart API to pick up the new base URL.
echo "[api] restarting to use mock LLM"
docker-compose up -d --no-deps api >/dev/null
sleep 5

# Run the full P0 + AI verification.
RUN_AI=1 ./e2e-test.sh

#!/usr/bin/env bash
# Staging / production smoke verification for document category tri-state + optional full P0.
#
# Usage:
#   BASE_URL=https://staging.example.com/api ./e2e-staging-verify.sh
#   BASE_URL=https://staging.example.com/api ./e2e-staging-verify.sh --full
#   BASE_URL=https://staging.example.com/api ./e2e-staging-verify.sh --migration-check
#
# Options:
#   --category-only   Run health + auth + upload + category gate only (default)
#   --full            Run full apps/api/e2e-test.sh (P0 + category)
#   --migration-check Verify migration 128 in Postgres (requires docker-compose on host)
#
# Environment:
#   BASE_URL          API origin, no trailing slash (required)
#   PDF               Path to test PDF (default: ../web/e2e/fixtures/sample.pdf)
#   COMPOSE_DIR       Directory with docker-compose for --migration-check (default: apps/api)
#   POSTGRES_USER     Postgres user for migration check (default: dealsignal)
#   POSTGRES_DB       Postgres database (default: dealsignal)
#
# Notes:
#   - Creates a throwaway user/workspace; safe for staging smoke runs.
#   - Category gate passing implies migration 128 behavior is live (deal_room CHECK + transitions).
#   - For SSH staging host migration check, run on the server with COMPOSE_DIR pointing at compose file.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
API_DIR="$SCRIPT_DIR"
REPO_ROOT=$(cd "$API_DIR/../.." && pwd)

BASE_URL="${BASE_URL:-}"
PDF="${PDF:-$REPO_ROOT/apps/web/e2e/fixtures/sample.pdf}"
COMPOSE_DIR="${COMPOSE_DIR:-$API_DIR}"
MODE="category"

usage() {
  sed -n '2,20p' "$0" | sed 's/^# \?//'
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --full) MODE="full"; shift ;;
    --category-only) MODE="category"; shift ;;
    --migration-check) MODE="migration"; shift ;;
    -h|--help) usage 0 ;;
    *) echo "Unknown option: $1"; usage 1 ;;
  esac
done

if [[ -z "$BASE_URL" ]]; then
  echo "ERROR: BASE_URL is required (e.g. https://staging.example.com or http://localhost:8090)"
  usage 1
fi

BASE_URL="${BASE_URL%/}"

if [[ ! -f "$PDF" ]]; then
  echo "ERROR: PDF fixture not found at $PDF"
  exit 1
fi

if [[ "$MODE" == "full" ]]; then
  echo "=== Staging verify: full P0 e2e-test.sh ==="
  exec env BASE_URL="$BASE_URL" PDF="$PDF" "$API_DIR/e2e-test.sh"
fi

if [[ "$MODE" == "migration" ]]; then
  echo "=== Staging verify: migration 128 (Postgres) ==="
  if ! command -v docker-compose >/dev/null 2>&1; then
    echo "ERROR: docker-compose required for --migration-check"
    exit 1
  fi
  (
    cd "$COMPOSE_DIR"
    docker-compose exec -T postgres psql -U "${POSTGRES_USER:-dealsignal}" -d "${POSTGRES_DB:-dealsignal}" -tAc \
      "SELECT 1 FROM schema_migrations WHERE version = '128_document_category_deal_room.up.sql';" | grep -q 1
    docker-compose exec -T postgres psql -U "${POSTGRES_USER:-dealsignal}" -d "${POSTGRES_DB:-dealsignal}" -tAc \
      "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid = 'documents'::regclass AND conname = 'chk_documents_category';" \
      | grep -q deal_room
  )
  echo "migration 128: ok"
  exit 0
fi

echo "=== Staging verify: document category tri-state ==="
echo "BASE_URL=$BASE_URL"

# shellcheck source=scripts/e2e-category-tristate.sh
source "$API_DIR/scripts/e2e-category-tristate.sh"

echo -n "[healthz] "
curl -fsS "$BASE_URL/healthz" | jq -c .

echo -n "[register] "
EMAIL="staging-verify-$(date +%s)@example.com"
PASSWORD="Password123!"
COOKIE_JAR=$(mktemp)
trap 'rm -f "$COOKIE_JAR"' EXIT
REGISTER=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
echo "$REGISTER" | jq -c '{user_id: .user.id, email: .user.email}'

echo -n "[workspace create] "
SLUG="staging-$(date +%s)"
WORKSPACE=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/workspaces" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Staging Verify\",\"slug\":\"$SLUG\",\"brand_color\":\"#0055ff\"}")
WORKSPACE_SLUG=$(echo "$WORKSPACE" | jq -r '.slug')
echo "$WORKSPACE" | jq -c '{id: .id, slug: .slug}'

echo -n "[upload document] "
UPLOAD=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents" \
  -F "file=@$PDF")
DOC_ID=$(echo "$UPLOAD" | jq -r '.id')
echo "$UPLOAD" | jq -c '{id: .id, status: .status}'

echo -n "[wait ready]"
for i in $(seq 1 45); do
  sleep 1
  STATUS=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents/$DOC_ID/status" | jq -r '.status')
  echo -n " $STATUS"
  if [[ "$STATUS" == "ready" ]]; then
    echo ""
    break
  fi
  if [[ "$STATUS" == "failed" ]]; then
    echo ""
    echo "ERROR: document ingestion failed"
    exit 1
  fi
  if [[ "$i" == "45" ]]; then
    echo ""
    echo "ERROR: document did not become ready in time"
    exit 1
  fi
done

run_e2e_category_tristate

echo "=== Staging category verify complete ==="

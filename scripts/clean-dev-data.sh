#!/usr/bin/env bash
set -euo pipefail

# clean-dev-data.sh
#
# Removes uploaded documents, rendered pages/chunks, analytics events and
# empties the dev S3 bucket. Keeps users, workspaces, tenants, deal rooms,
# contacts and settings intact.
#
# Usage:
#   ./scripts/clean-dev-data.sh        # interactive confirmation
#   ./scripts/clean-dev-data.sh --yes  # skip confirmation

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${REPO_ROOT}/apps/api/.env"

if [[ -f "${ENV_FILE}" ]]; then
  # shellcheck source=/dev/null
  set -a
  # shellcheck source=/dev/null
  source "${ENV_FILE}"
  set +a
fi

POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_USER="${POSTGRES_USER:-dealsignal}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-dealsignal}"
POSTGRES_DB="${POSTGRES_DB:-dealsignal}"
S3_BUCKET="${S3_BUCKET:-dealsignal}"
MINIO_CONTAINER="${MINIO_CONTAINER:-api-minio-1}"

export PGPASSWORD="${POSTGRES_PASSWORD}"

confirm() {
  if [[ "${1:-}" == "--yes" ]]; then
    return 0
  fi
  read -r -p "This will delete all documents, pages, chunks and analytics in ${POSTGRES_DB}. Continue? [y/N] " ans
  case "${ans}" in
    [yY]*) return 0 ;;
    *) echo "Aborted."; exit 1 ;;
  esac
}

clean_database() {
  echo "Cleaning PostgreSQL database ${POSTGRES_DB}..."

  psql -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -q <<'SQL'
-- Documents cascade removes pages, chunks, chunk_boxes, links,
-- ingestion_jobs, suggestions, assistant_sessions, assistant_messages,
-- deal_room_documents and access_logs/page_views where FKs exist.
TRUNCATE documents CASCADE;

-- Explicitly clear tables that may not be linked to documents.
TRUNCATE access_logs, page_views, action_items, signals CASCADE;
SQL

  echo "Database cleaned."
}

clean_s3() {
  if ! docker ps --format '{{.Names}}' | grep -qx "${MINIO_CONTAINER}"; then
    echo "MinIO container '${MINIO_CONTAINER}' not running, skipping S3 cleanup."
    return 0
  fi

  echo "Emptying S3 bucket '${S3_BUCKET}'..."
  docker exec "${MINIO_CONTAINER}" sh -c "mc alias set local http://localhost:9000 \${MINIO_ROOT_USER:-minioadmin} \${MINIO_ROOT_PASSWORD:-minioadmin} >/dev/null && mc rm --recursive --force local/${S3_BUCKET} >/dev/null"
  echo "S3 bucket emptied."
}

main() {
  confirm "${1:-}"
  clean_database
  clean_s3
  echo "Done."
}

main "$@"

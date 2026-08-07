#!/usr/bin/env bash
# Visitor Ask AI lane smoke (requires docling-rag + synced deal-room corpus).
# Sourced by e2e-test.sh / e2e-staging-verify.sh, or run standalone:
#
#   BASE_URL=http://localhost:8080 ./scripts/e2e-visitor-ask-ai.sh
#
# Exit / return codes:
#   0  pass
#   2  skip (knowledge disabled)
#   1  fail

run_e2e_visitor_ask_ai() {
  set -euo pipefail

  local base_url="${BASE_URL:-http://localhost:8080}"
  local pdf="${PDF:-../web/e2e/fixtures/sample.pdf}"
  local tmp_dir cookie_jar visitor_jar
  tmp_dir=$(mktemp -d)
  cookie_jar=$(mktemp)
  visitor_jar=$(mktemp)
  trap 'rm -rf "$tmp_dir"; rm -f "$cookie_jar" "$visitor_jar"' RETURN

  echo "=== Visitor Ask AI lane smoke ==="
  echo "BASE_URL=$base_url"

  if [[ ! -f "$pdf" ]]; then
    echo "ERROR: PDF fixture missing at $pdf"
    return 1
  fi

  echo -n "[healthz] "
  curl -fsS "$base_url/healthz" | jq -c .

  local ts email password slug
  ts="$(date +%s)"
  email="ask-ai-smoke-${ts}@example.com"
  password="Password123!"
  slug="ask-ai-smoke-${ts}"

  echo -n "[register] "
  curl -fsS -c "$cookie_jar" -b "$cookie_jar" -X POST "$base_url/api/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\",\"password\":\"$password\"}" >/dev/null
  echo "ok"

  echo -n "[workspace] "
  local workspace ws_slug
  workspace=$(curl -fsS -c "$cookie_jar" -b "$cookie_jar" -X POST "$base_url/api/workspaces" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"Ask AI Smoke\",\"slug\":\"$slug\",\"brand_color\":\"#0055ff\"}")
  ws_slug=$(echo "$workspace" | jq -r '.slug')
  echo "$ws_slug"

  echo -n "[upload] "
  local upload doc_id status=""
  upload=$(curl -fsS -c "$cookie_jar" -b "$cookie_jar" -X POST "$base_url/api/workspaces/$ws_slug/documents" \
    -F "file=@$pdf")
  doc_id=$(echo "$upload" | jq -r '.id')

  echo -n "[wait document ready]"
  for _ in $(seq 1 60); do
    sleep 1
    status=$(curl -fsS -c "$cookie_jar" -b "$cookie_jar" \
      "$base_url/api/workspaces/$ws_slug/documents/$doc_id/status" | jq -r '.status')
    echo -n " $status"
    if [[ "$status" == "ready" ]]; then echo ""; break; fi
    if [[ "$status" == "failed" ]]; then echo ""; echo "ERROR: ingestion failed"; return 1; fi
  done
  if [[ "${status:-}" != "ready" ]]; then
    echo ""
    echo "ERROR: document not ready"
    return 1
  fi

  echo -n "[deal-room] "
  local room room_id
  room=$(curl -fsS -c "$cookie_jar" -b "$cookie_jar" -X POST "$base_url/api/workspaces/$ws_slug/deal-rooms" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"Ask AI Room\",\"slug\":\"room-${ts}\",\"template_type\":\"seed\"}")
  room_id=$(echo "$room" | jq -r '.id')
  echo "$room_id"

  echo -n "[attach doc] "
  curl -fsS -c "$cookie_jar" -b "$cookie_jar" -X POST \
    "$base_url/api/workspaces/$ws_slug/deal-rooms/$room_id/documents" \
    -H "Content-Type: application/json" \
    -d "{\"document_id\":\"$doc_id\"}" >/dev/null
  echo "ok"

  echo -n "[knowledge enabled] "
  local corpus enabled
  corpus=$(curl -fsS -c "$cookie_jar" -b "$cookie_jar" \
    "$base_url/api/workspaces/$ws_slug/deal-rooms/$room_id/knowledge")
  enabled=$(echo "$corpus" | jq -r '.enabled')
  if [[ "$enabled" != "true" ]]; then
    echo "SKIP"
    echo "SKIP: knowledge disabled (set DOCLING_RAG_BASE_URL + PLATFORM_ADMIN_KEY)"
    return 2
  fi
  echo "ok"

  echo -n "[knowledge sync] "
  local sync_code="$tmp_dir/sync.json"
  local sync_http
  sync_http=$(curl -sS -o "$sync_code" -w "%{http_code}" -c "$cookie_jar" -b "$cookie_jar" -X POST \
    "$base_url/api/workspaces/$ws_slug/deal-rooms/$room_id/knowledge/sync")
  if [[ "$sync_http" != "202" && "$sync_http" != "200" ]]; then
    echo "FAIL HTTP $sync_http"
    cat "$sync_code"
    return 1
  fi
  echo "ok"

  echo -n "[wait corpus ready]"
  local corpus_status docs_synced docs_bad
  for _ in $(seq 1 180); do
    sleep 1
    corpus=$(curl -fsS -c "$cookie_jar" -b "$cookie_jar" \
      "$base_url/api/workspaces/$ws_slug/deal-rooms/$room_id/knowledge")
    corpus_status=$(echo "$corpus" | jq -r '.status')
    docs_synced=$(echo "$corpus" | jq '[.documents[]? | select(.status=="synced")] | length')
    docs_bad=$(echo "$corpus" | jq '[.documents[]? | select(.status=="failed" or .status=="pending" or .status=="syncing")] | length')
    echo -n " ${corpus_status}(${docs_synced})"
    if [[ "$corpus_status" == "ready" && "$docs_synced" -ge 1 && "$docs_bad" -eq 0 ]]; then
      echo ""
      break
    fi
  done
  if [[ "$corpus_status" != "ready" || "$docs_synced" -lt 1 || "$docs_bad" -ne 0 ]]; then
    echo ""
    echo "ERROR: corpus not ready in time"
    return 1
  fi

  echo -n "[create link] "
  local link link_id public_token
  link=$(curl -fsS -c "$cookie_jar" -b "$cookie_jar" -X POST \
    "$base_url/api/workspaces/$ws_slug/deal-rooms/$room_id/links" \
    -H "Content-Type: application/json" \
    -d '{"name":"Ask AI Smoke Link","download_enabled":true}')
  link_id=$(echo "$link" | jq -r '.id')
  public_token=$(echo "$link" | jq -r '.public_token // .publicToken // empty')
  if [[ -z "$public_token" || "$public_token" == "null" ]]; then
    public_token=$(echo "$link" | jq -r '.shortUrl // .short_url' | sed 's|.*/l/||')
  fi
  echo "$link_id"

  echo -n "[enable ask_ai] "
  local policy
  policy=$(curl -fsS -c "$cookie_jar" -b "$cookie_jar" -X PATCH \
    "$base_url/api/workspaces/$ws_slug/links/$link_id/ask-policy" \
    -H "Content-Type: application/json" \
    -d '{"ask_ai_enabled":true}')
  if [[ "$(echo "$policy" | jq -r '.data.ask_ai_enabled')" != "true" ]]; then
    echo "FAIL"
    echo "$policy"
    return 1
  fi
  echo "ok"

  echo -n "[visitor access] "
  curl -fsS -c "$visitor_jar" -b "$visitor_jar" -X POST "$base_url/api/v1/public/links/$public_token" \
    -H "Content-Type: application/json" \
    -d '{"email":"ai-smoke-visitor@example.com"}' >/dev/null
  echo "ok"

  echo -n "[public ask AI lane] "
  local ask_body="$tmp_dir/ask.json" ask_http turn_id lane
  ask_http=$(curl -sS -o "$ask_body" -w "%{http_code}" -c "$visitor_jar" -b "$visitor_jar" \
    -X POST "$base_url/api/v1/public/links/$public_token/ask" \
    -H "Content-Type: application/json" \
    -d '{"question":"What is the valuation cap?"}')
  if [[ "$ask_http" != "201" ]]; then
    echo "FAIL HTTP $ask_http"
    cat "$ask_body"
    return 1
  fi
  turn_id=$(jq -r '.data.id' "$ask_body")
  lane=$(jq -r '.data.lane' "$ask_body")
  if [[ "$lane" != "ai" ]]; then
    echo "FAIL lane=$lane"
    cat "$ask_body"
    return 1
  fi
  echo "ok turn=$turn_id"

  echo -n "[AI stream] "
  local stream_file="$tmp_dir/stream.sse" stream_http
  stream_http=$(curl -sS -o "$stream_file" -w "%{http_code}" -c "$visitor_jar" -b "$visitor_jar" \
    "$base_url/api/v1/public/links/$public_token/ask/$turn_id/stream")
  if [[ "$stream_http" != "200" ]]; then
    echo "FAIL HTTP $stream_http"
    cat "$stream_file"
    return 1
  fi
  if ! grep -q "event:" "$stream_file"; then
    echo "FAIL missing SSE events"
    cat "$stream_file"
    return 1
  fi
  echo "ok"

  echo -n "[visitor timeline ai_answered] "
  local mine
  mine=$(curl -fsS -c "$visitor_jar" -b "$visitor_jar" \
    "$base_url/api/v1/public/links/$public_token/ask/me")
  if [[ "$(echo "$mine" | jq -r --arg id "$turn_id" '[.data[] | select(.id == $id and .status == "ai_answered")] | length')" != "1" ]]; then
    echo "FAIL"
    echo "$mine" | jq -c '.data[:3]'
    return 1
  fi
  echo "ok"

  echo -n "[owner inbox ai_handled] "
  local owner_inbox
  owner_inbox=$(curl -fsS -c "$cookie_jar" -b "$cookie_jar" \
    "$base_url/api/workspaces/$ws_slug/links/$link_id/ask?lane=ai&status=ai_answered")
  if [[ "$(echo "$owner_inbox" | jq -r --arg id "$turn_id" '[.data[] | select(.id == $id)] | length')" != "1" ]]; then
    echo "FAIL turn missing from owner ai_handled inbox"
    echo "$owner_inbox" | jq -c '.data[:3]'
    return 1
  fi
  echo "ok"

  echo "=== Visitor Ask AI lane smoke OK ==="
  return 0
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  run_e2e_visitor_ask_ai
  exit $?
fi

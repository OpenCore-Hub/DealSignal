#!/usr/bin/env bash
# Document category tri-state gate (general | agreement | deal_room).
# Sourced by e2e-test.sh and e2e-staging-verify.sh.
#
# Required env:
#   BASE_URL, COOKIE_JAR, WORKSPACE_SLUG, DOC_ID, PDF

run_e2e_category_tristate() {
  set -euo pipefail

  : "${BASE_URL:?BASE_URL required}"
  : "${COOKIE_JAR:?COOKIE_JAR required}"
  : "${WORKSPACE_SLUG:?WORKSPACE_SLUG required}"
  : "${DOC_ID:?DOC_ID required}"
  : "${PDF:?PDF required}"

  local tmp_dir
  tmp_dir=$(mktemp -d)
  trap 'rm -rf "$tmp_dir"' RETURN

  echo "=== Document category tri-state ==="

  echo -n "[category default general] "
  local doc_cat
  doc_cat=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents/$DOC_ID" | jq -r '.category // "general"')
  if [[ "$doc_cat" != "general" ]]; then
    echo "FAIL expected general, got $doc_cat"
    return 1
  fi
  echo "ok"

  echo -n "[create deal room] "
  local room_slug room room_id
  room_slug="e2e-cat-$(date +%s)"
  room=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/deal-rooms" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"E2E Category Room\",\"slug\":\"$room_slug\",\"template_type\":\"seed\"}")
  room_id=$(echo "$room" | jq -r '.id')
  echo "$room" | jq -c '{id: .id, slug: .slug}'

  echo -n "[attach doc promotes deal_room] "
  local http_attach attach_body="$tmp_dir/attach.json"
  http_attach=$(curl -sS -o "$attach_body" -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/deal-rooms/$room_id/documents" \
    -H "Content-Type: application/json" \
    -d "{\"document_id\":\"$DOC_ID\",\"folder_path\":\"/general\"}")
  if [[ "$http_attach" != "200" && "$http_attach" != "201" ]]; then
    echo "FAIL attach HTTP $http_attach"
    cat "$attach_body"
    return 1
  fi
  doc_cat=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents/$DOC_ID" | jq -r '.category')
  if [[ "$doc_cat" != "deal_room" ]]; then
    echo "FAIL expected deal_room after attach, got $doc_cat"
    return 1
  fi
  echo "ok"

  echo -n "[library list category=general excludes doc] "
  local in_general
  in_general=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents?category=general" | jq -r --arg id "$DOC_ID" '[.data[] | select(.id == $id)] | length')
  if [[ "$in_general" != "0" ]]; then
    echo "FAIL doc still listed as general"
    return 1
  fi
  echo "ok"

  echo -n "[detach last room demotes general] "
  local http_detach
  http_detach=$(curl -sS -o /dev/null -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X DELETE "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/deal-rooms/$room_id/documents/$DOC_ID")
  if [[ "$http_detach" != "200" && "$http_detach" != "204" ]]; then
    echo "FAIL detach HTTP $http_detach"
    return 1
  fi
  doc_cat=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents/$DOC_ID" | jq -r '.category')
  if [[ "$doc_cat" != "general" ]]; then
    echo "FAIL expected general after detach, got $doc_cat"
    return 1
  fi
  echo "ok"

  echo -n "[agreement blocked from deal room] "
  local nda_name agreement agreement_id astatus http_block block_body="$tmp_dir/agreement-block.json"
  nda_name="nda-e2e-$(date +%s).pdf"
  agreement=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents" \
    -F "file=@$PDF;filename=$nda_name" \
    -F "category=agreement")
  agreement_id=$(echo "$agreement" | jq -r '.id')
  for i in $(seq 1 30); do
    sleep 1
    astatus=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
      "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents/$agreement_id/status" | jq -r '.status')
    if [[ "$astatus" == "ready" ]]; then break; fi
    if [[ "$astatus" == "failed" ]]; then
      echo "FAIL agreement ingestion failed"
      return 1
    fi
  done
  http_block=$(curl -sS -o "$block_body" -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/deal-rooms/$room_id/documents" \
    -H "Content-Type: application/json" \
    -d "{\"document_id\":\"$agreement_id\",\"folder_path\":\"/general\"}")
  if [[ "$http_block" != "400" ]]; then
    echo "FAIL expected 400 for agreement attach, got $http_block"
    cat "$block_body"
    return 1
  fi
  local block_code
  block_code=$(jq -r '.code // empty' "$block_body")
  if [[ "$block_code" != "agreement_not_allowed_in_deal_room" ]]; then
    echo "FAIL expected agreement_not_allowed_in_deal_room, got $block_code"
    return 1
  fi
  echo "ok"

  echo -n "[reject POST category=deal_room] "
  local reject_name reject_http reject_body="$tmp_dir/reject-deal-room.json"
  reject_name="reject-dr-$(date +%s).pdf"
  reject_http=$(curl -sS -o "$reject_body" -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents" \
    -F "file=@$PDF;filename=$reject_name" \
    -F "category=deal_room")
  if [[ "$reject_http" != "400" ]]; then
    echo "FAIL expected 400 for deal_room upload, got $reject_http"
    cat "$reject_body"
    return 1
  fi
  if [[ "$(jq -r '.code // empty' "$reject_body")" != "category_deal_room_via_api" ]]; then
    echo "FAIL expected category_deal_room_via_api"
    cat "$reject_body"
    return 1
  fi
  echo "ok"

  echo "=== Document category tri-state OK ==="
}

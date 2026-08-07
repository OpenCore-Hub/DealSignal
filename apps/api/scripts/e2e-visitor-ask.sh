#!/usr/bin/env bash
# Visitor Ask host loop + qa_disabled boundary (deal-room links only).
# Sourced by e2e-test.sh and e2e-staging-verify.sh.
#
# Required env:
#   BASE_URL, COOKIE_JAR, WORKSPACE_SLUG, DOC_ID, PDF

run_e2e_visitor_ask() {
  set -euo pipefail

  : "${BASE_URL:?BASE_URL required}"
  : "${COOKIE_JAR:?COOKIE_JAR required}"
  : "${WORKSPACE_SLUG:?WORKSPACE_SLUG required}"
  : "${DOC_ID:?DOC_ID required}"
  : "${PDF:?PDF required}"

  local tmp_dir visitor_jar
  tmp_dir=$(mktemp -d)
  visitor_jar=$(mktemp)
  trap 'rm -rf "$tmp_dir"; rm -f "$visitor_jar"' RETURN

  echo "=== Visitor Ask (deal-room host loop) ==="

  echo -n "[create deal room for ask] "
  local room_slug room room_id
  room_slug="e2e-ask-$(date +%s)"
  room=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/deal-rooms" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"E2E Ask Room\",\"slug\":\"$room_slug\",\"template_type\":\"seed\"}")
  room_id=$(echo "$room" | jq -r '.id')
  echo "$room" | jq -c '{id: .id, slug: .slug}'

  echo -n "[attach doc to ask room] "
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
  echo "ok"

  echo -n "[create deal-room link] "
  local dr_link link_id public_token question answer turn_id host_q_id
  dr_link=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/deal-rooms/$room_id/links" \
    -H "Content-Type: application/json" \
    -d '{"name":"E2E Ask Link","download_enabled":true}')
  link_id=$(echo "$dr_link" | jq -r '.id')
  public_token=$(echo "$dr_link" | jq -r '.public_token // .publicToken // empty')
  if [[ -z "$public_token" || "$public_token" == "null" ]]; then
    public_token=$(echo "$dr_link" | jq -r '.shortUrl // .short_url' | sed 's|.*/l/||')
  fi
  echo "$dr_link" | jq -c '{id: .id, public_token: "'"$public_token"'"}'

  question="E2E shell ask: when is the next update?"
  answer="Shell gate answer: next Monday."

  echo -n "[visitor public access] "
  curl -fsS -c "$visitor_jar" -b "$visitor_jar" -X POST "$BASE_URL/api/v1/public/links/$public_token" \
    -H "Content-Type: application/json" \
    -d '{"email":"e2e-ask-visitor@example.com"}' >/dev/null
  echo "ok"

  echo -n "[public ask host lane] "
  local ask_body="$tmp_dir/ask.json" ask_http
  ask_http=$(curl -sS -o "$ask_body" -w "%{http_code}" -c "$visitor_jar" -b "$visitor_jar" \
    -X POST "$BASE_URL/api/v1/public/links/$public_token/ask" \
    -H "Content-Type: application/json" \
    -d "{\"question\":\"$question\"}")
  if [[ "$ask_http" != "201" ]]; then
    echo "FAIL ask HTTP $ask_http"
    cat "$ask_body"
    return 1
  fi
  turn_id=$(jq -r '.data.id' "$ask_body")
  host_q_id=$(jq -r '.data.host_question_id // .data.hostQuestionId // empty' "$ask_body")
  local lane status
  lane=$(jq -r '.data.lane' "$ask_body")
  status=$(jq -r '.data.status' "$ask_body")
  if [[ "$lane" != "host" || "$status" != "host_pending" ]]; then
    echo "FAIL lane=$lane status=$status"
    cat "$ask_body"
    return 1
  fi
  echo "ok turn=$turn_id"

  echo -n "[AI stream ai_not_enabled default] "
  local stream_body="$tmp_dir/stream.json" stream_http
  stream_http=$(curl -sS -o "$stream_body" -w "%{http_code}" -c "$visitor_jar" -b "$visitor_jar" \
    "$BASE_URL/api/v1/public/links/$public_token/ask/$turn_id/stream")
  if [[ "$stream_http" != "403" ]]; then
    echo "FAIL expected 403 for AI stream, got $stream_http"
    cat "$stream_body"
    return 1
  fi
  if [[ "$(jq -r '.code // empty' "$stream_body")" != "ai_not_enabled" ]]; then
    echo "FAIL expected ai_not_enabled code"
    cat "$stream_body"
    return 1
  fi
  echo "ok"

  echo -n "[PATCH ask-policy enable AI] "
  local policy_body="$tmp_dir/policy.json" policy_http
  policy_http=$(curl -sS -o "$policy_body" -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X PATCH "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/links/$link_id/ask-policy" \
    -H "Content-Type: application/json" \
    -d '{"ask_ai_enabled":true}')
  if [[ "$policy_http" != "200" ]]; then
    echo "FAIL PATCH ask-policy HTTP $policy_http"
    cat "$policy_body"
    return 1
  fi
  if [[ "$(jq -r '.data.ask_ai_enabled // .data.askAiEnabled // empty' "$policy_body")" != "true" ]]; then
    echo "FAIL PATCH response missing ask_ai_enabled=true"
    cat "$policy_body"
    return 1
  fi
  echo "ok"

  echo -n "[GET link askAiEnabled after enable] "
  local link_get
  link_get=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/links/$link_id")
  if [[ "$(echo "$link_get" | jq -r '.askAiEnabled // .ask_ai_enabled // false')" != "true" ]]; then
    echo "FAIL GET link askAiEnabled not true"
    echo "$link_get" | jq -c '{askAiEnabled, ask_ai_enabled}'
    return 1
  fi
  echo "ok"

  echo -n "[AI stream not ai_not_enabled after enable] "
  stream_http=$(curl -sS -o "$stream_body" -w "%{http_code}" -c "$visitor_jar" -b "$visitor_jar" \
    "$BASE_URL/api/v1/public/links/$public_token/ask/$turn_id/stream")
  if [[ "$stream_http" == "403" && "$(jq -r '.code // empty' "$stream_body")" == "ai_not_enabled" ]]; then
    echo "FAIL still ai_not_enabled after PATCH enable"
    cat "$stream_body"
    return 1
  fi
  echo "ok HTTP $stream_http"

  echo -n "[AI quota exceeded host fallback] "
  policy_http=$(curl -sS -o "$policy_body" -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X PATCH "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/links/$link_id/ask-policy" \
    -H "Content-Type: application/json" \
    -d '{"ask_ai_monthly_quota":0}')
  if [[ "$policy_http" != "200" ]]; then
    echo "FAIL PATCH quota HTTP $policy_http"
    cat "$policy_body"
    return 1
  fi
  ask_http=$(curl -sS -o "$ask_body" -w "%{http_code}" -c "$visitor_jar" -b "$visitor_jar" \
    -X POST "$BASE_URL/api/v1/public/links/$public_token/ask" \
    -H "Content-Type: application/json" \
    -d '{"question":"E2E shell quota fallback?"}')
  if [[ "$ask_http" != "201" ]]; then
    echo "FAIL quota ask HTTP $ask_http"
    cat "$ask_body"
    return 1
  fi
  lane=$(jq -r '.data.lane' "$ask_body")
  reason=$(jq -r '.data.route_reason // .data.routeReason // empty' "$ask_body")
  if [[ "$lane" != "host" || "$reason" != "ai_quota_exceeded" ]]; then
    echo "FAIL lane=$lane route_reason=$reason"
    cat "$ask_body"
    return 1
  fi
  echo "ok"

  echo -n "[owner link inbox] "
  local link_inbox
  link_inbox=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/links/$link_id/ask?lane=host&status=host_pending")
  if [[ "$(echo "$link_inbox" | jq -r --arg id "$turn_id" '[.data[] | select(.id == $id)] | length')" != "1" ]]; then
    echo "FAIL turn missing from link inbox"
    echo "$link_inbox" | jq -c '.data[:3]'
    return 1
  fi
  echo "ok"

  echo -n "[owner room inbox] "
  local room_inbox
  room_inbox=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/deal-rooms/$room_id/ask?link_id=$link_id&lane=host&status=host_pending")
  if [[ "$(echo "$room_inbox" | jq -r --arg id "$turn_id" '[.data[] | select(.id == $id)] | length')" != "1" ]]; then
    echo "FAIL turn missing from room inbox"
    echo "$room_inbox" | jq -c '.data[:3]'
    return 1
  fi
  echo "ok"

  echo -n "[dashboard deal_room_link_question action] "
  local action_source_id="${host_q_id:-$turn_id}"
  local stats todo_count target_id
  stats=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/dashboard/stats")
  todo_count=$(echo "$stats" | jq -r --arg sid "$action_source_id" \
    '[.actionItems[]? | select(.sourceType == "deal_room_link_question" and .sourceId == $sid and .status == "pending")] | length')
  if [[ "$todo_count" != "1" ]]; then
    echo "FAIL expected pending deal_room_link_question action, got $todo_count"
    echo "$stats" | jq -c '[.actionItems[]? | {sourceType, sourceId, targetId, status}] | .[:5]'
    return 1
  fi
  target_id=$(echo "$stats" | jq -r --arg sid "$action_source_id" \
    '[.actionItems[]? | select(.sourceType == "deal_room_link_question" and .sourceId == $sid)] | .[0].targetId // empty')
  if [[ "$target_id" != "${room_id}/${link_id}" ]]; then
    echo "FAIL targetId=$target_id expected ${room_id}/${link_id}"
    return 1
  fi
  echo "ok"

  echo -n "[host answer] "
  local answer_body="$tmp_dir/answer.json" answer_http
  answer_http=$(curl -sS -o "$answer_body" -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X PATCH "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/links/$link_id/ask/$turn_id/host-answer" \
    -H "Content-Type: application/json" \
    -d "{\"answer\":\"$answer\"}")
  if [[ "$answer_http" != "200" ]]; then
    echo "FAIL host-answer HTTP $answer_http"
    cat "$answer_body"
    return 1
  fi
  if [[ "$(jq -r '.data.status' "$answer_body")" != "host_answered" ]]; then
    echo "FAIL status not host_answered"
    cat "$answer_body"
    return 1
  fi
  echo "ok"

  echo -n "[visitor ask/me sees reply] "
  local mine
  mine=$(curl -fsS -c "$visitor_jar" -b "$visitor_jar" \
    "$BASE_URL/api/v1/public/links/$public_token/ask/me")
  if [[ "$(echo "$mine" | jq -r --arg id "$turn_id" '[.data[] | select(.id == $id and .status == "host_answered")] | length')" != "1" ]]; then
    echo "FAIL visitor timeline missing answered turn"
    echo "$mine" | jq -c '.data[:3]'
    return 1
  fi
  if [[ "$(echo "$mine" | jq -r --arg id "$turn_id" --arg ans "$answer" '[.data[] | select(.id == $id and .host_answer == $ans)] | length')" != "1" ]]; then
    echo "FAIL host_answer mismatch"
    echo "$mine" | jq -c '.data[:3]'
    return 1
  fi
  echo "ok"

  echo -n "[link analytics ask_summary] "
  local analytics_body="$tmp_dir/analytics.json"
  analytics_body=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/links/$link_id/analytics")
  if [[ "$(echo "$analytics_body" | jq -r '.ask_summary.host_answered // 0')" != "1" ]]; then
    echo "FAIL expected ask_summary.host_answered=1"
    echo "$analytics_body" | jq -c '.ask_summary // empty'
    return 1
  fi
  if [[ "$(echo "$analytics_body" | jq -r '.ask_summary.host_pending // 0')" != "0" ]]; then
    echo "FAIL expected ask_summary.host_pending=0 after host answer"
    echo "$analytics_body" | jq -c '.ask_summary // empty'
    return 1
  fi
  echo "ok"

  echo -n "[document link qa_disabled] "
  local doc_link doc_token block_body="$tmp_dir/qa-block.json" block_http
  doc_link=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/links" \
    -H "Content-Type: application/json" \
    -d "{\"document_id\":\"$DOC_ID\",\"name\":\"E2E Doc Ask Block\",\"permission_type\":\"public\",\"download_enabled\":true}")
  doc_token=$(echo "$doc_link" | jq -r '.shortUrl // .short_url' | sed 's|.*/l/||')
  curl -fsS -X POST "$BASE_URL/api/v1/public/links/$doc_token" \
    -H "Content-Type: application/json" \
    -d '{"email":"e2e-doc-ask-block@example.com"}' >/dev/null
  block_http=$(curl -sS -o "$block_body" -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/public/links/$doc_token/ask" \
    -H "Content-Type: application/json" \
    -d '{"question":"Should be blocked"}')
  if [[ "$block_http" != "403" ]]; then
    echo "FAIL expected 403, got $block_http"
    cat "$block_body"
    return 1
  fi
  if [[ "$(jq -r '.code // empty' "$block_body")" != "qa_disabled" ]]; then
    echo "FAIL expected qa_disabled code"
    cat "$block_body"
    return 1
  fi
  echo "ok"

  echo "=== Visitor Ask OK ==="
}

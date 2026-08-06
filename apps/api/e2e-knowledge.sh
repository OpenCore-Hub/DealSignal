#!/usr/bin/env bash
# Real-backend knowledge Q&A smoke (requires running API + docling-rag).
#
# Usage:
#   BASE_URL=http://localhost:8090 ./e2e-knowledge.sh
#
# Exit codes:
#   0  pass
#   2  skip (knowledge disabled / docling not configured)
#   1  fail
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8090}"
PDF="${PDF:-e2e-test.pdf}"
SYNC_TIMEOUT_S="${SYNC_TIMEOUT_S:-180}"
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$COOKIE_JAR"' EXIT

echo "=== DealSignal knowledge Q&A smoke ==="
echo "BASE_URL=$BASE_URL"

if [[ ! -f "$PDF" ]]; then
  if command -v ps2pdf >/dev/null 2>&1; then
    TMP_PS=$(mktemp)
    cat > "$TMP_PS" <<'EOF'
%!PS
/Times-Roman findfont 24 scalefont setfont
100 700 moveto
(DealSignal Knowledge Smoke Cap is ten million USD) show
showpage
EOF
    ps2pdf "$TMP_PS" "$PDF"
    rm -f "$TMP_PS"
  else
    echo "ERROR: $PDF not found and ps2pdf unavailable"
    exit 1
  fi
fi

echo -n "[healthz] "
curl -fsS "$BASE_URL/healthz" | jq -c .

TS="$(date +%s)"
EMAIL="knowledge-smoke-${TS}@example.com"
PASSWORD="Password123!"
SLUG="kqa-smoke-${TS}"

echo -n "[register] "
REGISTER=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
echo "$REGISTER" | jq -c '{user_id: .user.id, email: .user.email}'

echo -n "[workspace] "
WORKSPACE=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/workspaces" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Knowledge Smoke\",\"slug\":\"$SLUG\",\"brand_color\":\"#0055ff\"}")
echo "$WORKSPACE" | jq -c '{id: .id, slug: .slug}'
WS_SLUG=$(echo "$WORKSPACE" | jq -r '.slug')

echo -n "[upload] "
UPLOAD=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/workspaces/$WS_SLUG/documents" \
  -F "file=@$PDF")
DOC_ID=$(echo "$UPLOAD" | jq -r '.id')
echo "$UPLOAD" | jq -c '{id: .id, status: .status}'

echo -n "[wait document ready]"
for _ in $(seq 1 60); do
  sleep 1
  STATUS=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WS_SLUG/documents/$DOC_ID/status" | jq -r '.status')
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
done
if [[ "${STATUS:-}" != "ready" ]]; then
  echo ""
  echo "ERROR: document not ready in time"
  exit 1
fi

echo -n "[deal-room] "
ROOM=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Knowledge Smoke Room\",\"slug\":\"room-${TS}\",\"template_type\":\"seed\"}")
ROOM_ID=$(echo "$ROOM" | jq -r '.id')
echo "$ROOM" | jq -c '{id: .id, slug: .slug}'

echo -n "[A5 corpus not ready] "
A5_CODE=$(curl -sS -o /tmp/kqa-a5.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/sessions/query" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"Should be blocked\",\"answer\":true,\"top_k\":8,\"clientRequestId\":\"kqa-smoke-${TS}-a5\"}")
echo "HTTP $A5_CODE $(jq -c . </tmp/kqa-a5.json 2>/dev/null || cat /tmp/kqa-a5.json)"
if [[ "$A5_CODE" != "409" ]] || ! jq -e '.code=="knowledge_corpus_not_ready"' >/dev/null </tmp/kqa-a5.json; then
  echo "ERROR: expected 409 knowledge_corpus_not_ready before sync"
  exit 1
fi

echo -n "[add document to room] "
ADD_DOC=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/documents" \
  -H "Content-Type: application/json" \
  -d "{\"document_id\":\"$DOC_ID\"}")
echo "$ADD_DOC" | jq -c '{document_id: (.document_id // .documentId // .id // "ok")}'

echo -n "[knowledge get] "
CORPUS=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge")
echo "$CORPUS" | jq -c '{enabled: .enabled, status: .status, docs: (.documents|length)}'
ENABLED=$(echo "$CORPUS" | jq -r '.enabled')
if [[ "$ENABLED" != "true" ]]; then
  echo "SKIP: knowledge disabled (set DOCLING_RAG_BASE_URL + PLATFORM_ADMIN_KEY)"
  exit 2
fi

echo -n "[knowledge sync] "
SYNC_CODE=$(curl -sS -o /tmp/kqa-sync.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/sync")
echo "HTTP $SYNC_CODE $(jq -c . </tmp/kqa-sync.json 2>/dev/null || cat /tmp/kqa-sync.json)"
if [[ "$SYNC_CODE" != "202" ]]; then
  echo "ERROR: sync did not accept"
  exit 1
fi

echo -n "[wait corpus ready]"
READY=0
for _ in $(seq 1 "$SYNC_TIMEOUT_S"); do
  sleep 1
  CORPUS=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge")
  STATUS=$(echo "$CORPUS" | jq -r '.status')
  SYNCED=$(echo "$CORPUS" | jq -r '.progress.synced // 0')
  TOTAL=$(echo "$CORPUS" | jq -r '.progress.total // 0')
  FAILED=$(echo "$CORPUS" | jq -r '.progress.failed // 0')
  echo -n " ${STATUS}(${SYNCED}/${TOTAL})"
  # Align with server corpusAskReady: ready + at least one synced doc, no failed.
  DOCS_SYNCED=$(echo "$CORPUS" | jq '[.documents[]? | select(.status=="synced")] | length')
  DOCS_BAD=$(echo "$CORPUS" | jq '[.documents[]? | select(.status=="failed" or .status=="pending" or .status=="syncing")] | length')
  if [[ "$STATUS" == "ready" && "$DOCS_SYNCED" -ge 1 && "$DOCS_BAD" -eq 0 ]]; then
    echo ""
    READY=1
    break
  fi
  if [[ "$STATUS" == "failed" || "$FAILED" -gt 0 ]]; then
    echo ""
    echo "ERROR: corpus sync failed"
    echo "$CORPUS" | jq .
    exit 1
  fi
done
if [[ "$READY" != "1" ]]; then
  echo ""
  echo "ERROR: corpus not ready within ${SYNC_TIMEOUT_S}s"
  echo "$CORPUS" | jq .
  exit 1
fi

CLIENT_REQ="kqa-smoke-${TS}-ask-1"
QUESTION="What is the valuation cap?"

echo -n "[session query json] "
ASK=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/sessions/query" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"$QUESTION\",\"answer\":true,\"top_k\":8,\"clientRequestId\":\"$CLIENT_REQ\"}")
echo "$ASK" | jq -c '{sessionId, resultStatus: .turn.resultStatus, hits: (.turn.hits|length), answer: ((.turn.answer // "")|tostring|.[0:60])}'
SESSION_ID=$(echo "$ASK" | jq -r '.sessionId')
TURN_ID=$(echo "$ASK" | jq -r '.turn.id')
RESULT=$(echo "$ASK" | jq -r '.turn.resultStatus')
if [[ -z "$SESSION_ID" || "$SESSION_ID" == "null" || -z "$TURN_ID" || "$TURN_ID" == "null" ]]; then
  echo "ERROR: missing session/turn"
  echo "$ASK" | jq .
  exit 1
fi
if [[ "$RESULT" == "error" ]]; then
  echo "ERROR: turn errorSummary=$(echo "$ASK" | jq -r '.turn.errorSummary')"
  exit 1
fi

echo -n "[sessionState on ask] "
# Phase L: ask response carries post-turn session.state (desk rail).
STATE_KEYS=$(echo "$ASK" | jq -c '{open:(.sessionState.openQuestions // [])|length, ent:(.sessionState.entities // [])|length, cov:(.sessionState.coverageHints // [])|length}')
echo "$STATE_KEYS"
if ! jq -e '.sessionState|type=="object"' >/dev/null <<<"$ASK"; then
  echo "ERROR: expected sessionState object on session query response"
  echo "$ASK" | jq 'keys'
  exit 1
fi
if [[ "$RESULT" == "refused" || "$RESULT" == "no_hits" ]]; then
  OPEN_N=$(echo "$ASK" | jq '(.sessionState.openQuestions // [])|length')
  if [[ "$OPEN_N" -lt 1 ]]; then
    echo "ERROR: gap/refuse turns should seed sessionState.openQuestions"
    echo "$ASK" | jq '.sessionState'
    exit 1
  fi
fi

echo -n "[idempotent replay] "
REPLAY=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/sessions/query" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"$QUESTION\",\"answer\":true,\"top_k\":8,\"clientRequestId\":\"$CLIENT_REQ\"}")
REPLAY_TURN=$(echo "$REPLAY" | jq -r '.turn.id')
if [[ "$REPLAY_TURN" != "$TURN_ID" ]]; then
  echo "ERROR: replay created a new turn ($REPLAY_TURN != $TURN_ID)"
  exit 1
fi
echo "ok turn=$REPLAY_TURN"

echo -n "[missing clientRequestId] "
MISS_CODE=$(curl -sS -o /tmp/kqa-miss-crid.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/sessions/query" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"$QUESTION\",\"answer\":true,\"top_k\":8}")
echo "HTTP $MISS_CODE $(jq -c . </tmp/kqa-miss-crid.json)"
if [[ "$MISS_CODE" != "400" ]] || ! jq -e '.code=="invalid_input"' >/dev/null </tmp/kqa-miss-crid.json; then
  echo "ERROR: expected 400 invalid_input for missing clientRequestId"
  exit 1
fi

echo -n "[legacy answer rejected] "
LEGACY_CODE=$(curl -sS -o /tmp/kqa-legacy.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/query" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"$QUESTION\",\"answer\":true}")
echo "HTTP $LEGACY_CODE $(jq -c . </tmp/kqa-legacy.json)"
if [[ "$LEGACY_CODE" != "400" ]] || ! jq -e '.code=="answer_requires_session"' >/dev/null </tmp/kqa-legacy.json; then
  echo "ERROR: expected 400 answer_requires_session"
  exit 1
fi

echo -n "[sse stream] "
STREAM_FILE=$(mktemp)
STREAM_CODE=$(curl -sS -o "$STREAM_FILE" -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/sessions/query/stream" \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d "{\"sessionId\":\"$SESSION_ID\",\"query\":\"What about the option pool?\",\"answer\":true,\"top_k\":8,\"clientRequestId\":\"kqa-smoke-${TS}-ask-2\"}")
echo "HTTP $STREAM_CODE"
if [[ "$STREAM_CODE" != "200" ]]; then
  cat "$STREAM_FILE"
  exit 1
fi
if ! grep -q 'event: phase' "$STREAM_FILE" || ! grep -q 'event: done' "$STREAM_FILE"; then
  echo "ERROR: SSE missing phase/done"
  head -c 800 "$STREAM_FILE"
  echo ""
  exit 1
fi
if grep -q ': keepalive' "$STREAM_FILE"; then
  echo -n "(keepalive present) "
fi
echo "ok frames=$(grep -c '^event:' "$STREAM_FILE" || true)"
rm -f "$STREAM_FILE"

echo -n "[sse idempotent replay] "
REPLAY_STREAM=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/sessions/query/stream" \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d "{\"sessionId\":\"$SESSION_ID\",\"query\":\"$QUESTION\",\"answer\":true,\"top_k\":8,\"clientRequestId\":\"$CLIENT_REQ\"}")
REPLAY_STREAM_TURN=$(echo "$REPLAY_STREAM" | awk '/^data: /{sub(/^data: /,""); last=$0} END{print last}' | jq -r '.turn.id // empty')
if [[ -z "$REPLAY_STREAM_TURN" ]] || [[ "$REPLAY_STREAM_TURN" != "$TURN_ID" ]]; then
  echo "ERROR: stream replay did not return original turn (got=$REPLAY_STREAM_TURN want=$TURN_ID)"
  echo "$REPLAY_STREAM" | head -c 400
  exit 1
fi
echo "ok turn=$REPLAY_STREAM_TURN"

echo -n "[follow-ups] "
FU_CODE=$(curl -sS -o /tmp/kqa-fu.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/turns/$TURN_ID/follow-ups")
echo "HTTP $FU_CODE $(jq -c '{source, n:(.items|length), sample:(.items[0].text // "" | .[0:60])}' </tmp/kqa-fu.json 2>/dev/null || cat /tmp/kqa-fu.json)"
if [[ "$FU_CODE" != "200" ]] || ! jq -e '.items|length>=1' >/dev/null </tmp/kqa-fu.json; then
  echo "ERROR: expected follow-ups items"
  exit 1
fi
FU_SOURCE=$(jq -r '.source' </tmp/kqa-fu.json)
if [[ "$FU_SOURCE" != "llm" && "$FU_SOURCE" != "mission" && "$FU_SOURCE" != "template" ]]; then
  echo "ERROR: unexpected follow-ups source=$FU_SOURCE"
  exit 1
fi

echo -n "[mission pack] "
MP_CODE=$(curl -sS -o /tmp/kqa-mission.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/mission")
echo "HTTP $MP_CODE $(jq -c '{packId, source, n:(.items|length)}' </tmp/kqa-mission.json 2>/dev/null || cat /tmp/kqa-mission.json)"
if [[ "$MP_CODE" != "200" ]] || ! jq -e '.packId and (.items|length)>=1' >/dev/null </tmp/kqa-mission.json; then
  echo "ERROR: expected mission pack with items"
  exit 1
fi
PUT_CODE=$(curl -sS -o /tmp/kqa-mission-put.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X PUT \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/mission" \
  -H "Content-Type: application/json" \
  -d '{"packId":"ma_redflag_v1"}')
echo "PUT HTTP $PUT_CODE $(jq -c '{packId, source}' </tmp/kqa-mission-put.json 2>/dev/null || true)"
if [[ "$PUT_CODE" != "200" ]] || ! jq -e '.packId=="ma_redflag_v1" and .source=="room"' >/dev/null </tmp/kqa-mission-put.json; then
  echo "ERROR: expected room-bound ma_redflag_v1"
  exit 1
fi

echo -n "[mission progress] "
MPROG_CODE=$(curl -sS -o /tmp/kqa-mission-progress.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/mission/progress?sessionId=$SESSION_ID")
echo "HTTP $MPROG_CODE $(jq -c '{packId, covered, total, n:(.items|length)}' </tmp/kqa-mission-progress.json 2>/dev/null || cat /tmp/kqa-mission-progress.json)"
if [[ "$MPROG_CODE" != "200" ]] || ! jq -e '.packId=="ma_redflag_v1" and (.total|tonumber)>=1 and (.items|length)==.total and (.covered|type)=="number"' >/dev/null </tmp/kqa-mission-progress.json; then
  echo "ERROR: expected mission progress for room pack"
  jq . </tmp/kqa-mission-progress.json 2>/dev/null || true
  exit 1
fi

echo -n "[feedback helpful] "
FB_CODE=$(curl -sS -o /tmp/kqa-fb.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X PUT \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/turns/$TURN_ID/feedback" \
  -H "Content-Type: application/json" \
  -d '{"kind":"helpful","note":"smoke ok"}')
echo "HTTP $FB_CODE $(jq -c . </tmp/kqa-fb.json 2>/dev/null || true)"
if [[ "$FB_CODE" != "200" ]]; then
  exit 1
fi

echo -n "[feedback wrong_citation → eval candidate] "
WC_CODE=$(curl -sS -o /tmp/kqa-fb-wc.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X PUT \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/turns/$TURN_ID/feedback" \
  -H "Content-Type: application/json" \
  -d '{"kind":"wrong_citation","note":"cited wrong schedule"}')
echo "HTTP $WC_CODE $(jq -c . </tmp/kqa-fb-wc.json 2>/dev/null || true)"
if [[ "$WC_CODE" != "200" ]] || ! jq -e '.kind=="wrong_citation"' >/dev/null </tmp/kqa-fb-wc.json; then
  echo "ERROR: expected wrong_citation feedback"
  exit 1
fi
CAND_CODE=$(curl -sS -o /tmp/kqa-eval-cands.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/eval/candidates?kind=wrong_citation&status=pending")
echo "list HTTP $CAND_CODE $(jq -c '{n:(.items|length), sample:(.items[0]//{}|{id,feedbackKind,reviewStatus})}' </tmp/kqa-eval-cands.json 2>/dev/null || true)"
if [[ "$CAND_CODE" != "200" ]] || ! jq -e '(.items|length)>=1 and .items[0].feedbackKind=="wrong_citation" and .items[0].reviewStatus=="pending"' >/dev/null </tmp/kqa-eval-cands.json; then
  echo "ERROR: expected pending wrong_citation eval candidate"
  jq . </tmp/kqa-eval-cands.json 2>/dev/null || true
  exit 1
fi
CAND_ID=$(jq -r '.items[0].id' </tmp/kqa-eval-cands.json)
REV_CODE=$(curl -sS -o /tmp/kqa-eval-review.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X PATCH \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/eval/candidates/$CAND_ID" \
  -H "Content-Type: application/json" \
  -d '{"reviewStatus":"accepted"}')
echo "review HTTP $REV_CODE $(jq -c '{id,reviewStatus,expect}' </tmp/kqa-eval-review.json 2>/dev/null || true)"
if [[ "$REV_CODE" != "200" ]] || ! jq -e '.reviewStatus=="accepted" and .expect=="reject_or_rebind"' >/dev/null </tmp/kqa-eval-review.json; then
  echo "ERROR: expected accepted reject_or_rebind gold"
  exit 1
fi
EXP_CODE=$(curl -sS -o /tmp/kqa-eval-export.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/eval/candidates/export")
echo "export HTTP $EXP_CODE $(jq -c '{n:(.seeds|length)}' </tmp/kqa-eval-export.json 2>/dev/null || true)"
if [[ "$EXP_CODE" != "200" ]] || ! jq -e '(.seeds|length)>=1 and .seeds[0].expect=="reject_or_rebind"' >/dev/null </tmp/kqa-eval-export.json; then
  echo "ERROR: expected accepted seed export"
  exit 1
fi

echo -n "[active session] "
ACTIVE=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/sessions/active")
ACTIVE_TURNS=$(echo "$ACTIVE" | jq '.turns|length')
echo "turns=$ACTIVE_TURNS"
if [[ "$ACTIVE_TURNS" -lt 2 ]]; then
  echo "ERROR: expected ≥2 turns after json+stream asks"
  echo "$ACTIVE" | jq .
  exit 1
fi

echo -n "[session state machine] "
# After no_hits/refused/error turns, openQuestions must be provenanced in session.state.
STATE_OPEN=$(echo "$ACTIVE" | jq '(.session.state.openQuestions // []) | length')
STATE_ENT=$(echo "$ACTIVE" | jq '(.session.state.entities // []) | length')
STATE_COV=$(echo "$ACTIVE" | jq '(.session.state.coverageHints // []) | length')
echo "open=$STATE_OPEN entities=$STATE_ENT coverage=$STATE_COV"
HAS_GAP_TURN=$(echo "$ACTIVE" | jq '[.turns[] | select(.resultStatus=="no_hits" or .resultStatus=="refused" or .resultStatus=="error")] | length')
if [[ "$HAS_GAP_TURN" -gt 0 && "$STATE_OPEN" -lt 1 ]]; then
  echo "ERROR: gap turns require session.state.openQuestions"
  echo "$ACTIVE" | jq '.session.state'
  exit 1
fi
HAS_HIT_TURN=$(echo "$ACTIVE" | jq '[.turns[] | select((.hits|length)>0)] | length')
if [[ "$HAS_HIT_TURN" -gt 0 && "$STATE_ENT" -lt 1 && "$STATE_COV" -lt 1 ]]; then
  echo "ERROR: hit turns require session.state entities or coverageHints"
  echo "$ACTIVE" | jq '.session.state'
  exit 1
fi

echo -n "[bound claims] "
# When a turn has hits + a non-refusal answer, claims must bind at least one sentence.
BOUND_OK=$(echo "$ACTIVE" | jq '[.turns[]
  | select((.hits|length)>0 and .refused==false and ((.answer // "")|length)>0)
  | select((.claims // [])|length < 1)] | length')
# Gap / refusal turns must not invent unbound claims.
LEAK_CLAIMS=$(echo "$ACTIVE" | jq '[.turns[]
  | select((.hits|length)==0 or .refused==true)
  | select((.claims // [])|length > 0)] | length')
BOUND_SAMPLE=$(echo "$ACTIVE" | jq -c '[.turns[] | select((.claims // [])|length>0) | {id, n:(.claims|length), c0:(.claims[0].confidence // ""), hits:(.claims[0].hitIds // [])}] | .[0]')
echo "unbound_answered=$BOUND_OK gap_claim_leak=$LEAK_CLAIMS sample=$BOUND_SAMPLE"

echo -n "[multi-hop audit] "
# multiHop is optional (corpus-dependent); when present it must be a well-formed audit object.
HOP_BAD=$(echo "$ACTIVE" | jq '[.turns[]
  | select(.multiHop != null)
  | select((.multiHop.applied|type)!="boolean"
      or ((.multiHop.queries // [])|type)!="array")] | length')
HOP_N=$(echo "$ACTIVE" | jq '[.turns[] | select(.multiHop != null)] | length')
HOP_SAMPLE=$(echo "$ACTIVE" | jq -c '[.turns[] | select(.multiHop != null) | {id, applied:.multiHop.applied, nq:(.multiHop.queries|length), added:((.multiHop.addedHitIds // [])|length)}] | .[0]')
echo "present=$HOP_N bad=$HOP_BAD sample=$HOP_SAMPLE"
if [[ "$HOP_BAD" -gt 0 ]]; then
  echo "ERROR: multiHop audit shape invalid"
  echo "$ACTIVE" | jq '[.turns[] | select(.multiHop != null) | {id, multiHop}]'
  exit 1
fi

echo -n "[conflict set audit] "
# conflicts are optional (multi-source numeric disagreement); when present, require id/kind/sides.
CONFLICT_BAD=$(echo "$ACTIVE" | jq '[.turns[]
  | select((.conflicts // [])|length > 0)
  | .conflicts[]
  | select((.id|type)!="string" or (.id|length)<1
      or (.kind|type)!="string" or (.kind|length)<1
      or ((.sides // [])|type)!="array" or ((.sides // [])|length)<2
      or ([.sides[] | select((.sourceName|type)!="string" or (.sourceName|length)<1)]|length)>0)] | length')
CONFLICT_N=$(echo "$ACTIVE" | jq '[.turns[] | select((.conflicts // [])|length > 0)] | length')
CONFLICT_SAMPLE=$(echo "$ACTIVE" | jq -c '[.turns[] | select((.conflicts // [])|length > 0) | {id, n:(.conflicts|length), c0:(.conflicts[0]|{id, kind, sides:(.sides|length)})}] | .[0]')
echo "present=$CONFLICT_N bad=$CONFLICT_BAD sample=$CONFLICT_SAMPLE"
if [[ "$CONFLICT_BAD" -gt 0 ]]; then
  echo "ERROR: conflict audit shape invalid"
  echo "$ACTIVE" | jq '[.turns[] | select((.conflicts // [])|length > 0) | {id, conflicts}]'
  exit 1
fi

echo -n "[typed refusal] "
# Gap / refuse / error turns should carry a typed refusal envelope (Phase J).
REFUSAL_MISSING=$(echo "$ACTIVE" | jq '[.turns[]
  | select(.resultStatus=="refused" or .resultStatus=="no_hits" or .resultStatus=="error")
  | select(.refusal == null or ((.refusal.kind // "")|length)<1)] | length')
REFUSAL_BAD=$(echo "$ACTIVE" | jq '[.turns[]
  | select(.refusal != null)
  | select((.refusal.kind|type)!="string"
      or (.refusal.kind as $k | ($k!="ungrounded" and $k!="no_hits" and $k!="error")))] | length')
REFUSAL_N=$(echo "$ACTIVE" | jq '[.turns[] | select(.refusal != null)] | length')
REFUSAL_SAMPLE=$(echo "$ACTIVE" | jq -c '[.turns[] | select(.refusal != null) | {id, status:.resultStatus, kind:.refusal.kind, hadHits:(.refusal.hadHits // false)}] | .[0]')
echo "present=$REFUSAL_N missing=$REFUSAL_MISSING bad=$REFUSAL_BAD sample=$REFUSAL_SAMPLE"
if [[ "$REFUSAL_MISSING" -gt 0 || "$REFUSAL_BAD" -gt 0 ]]; then
  echo "ERROR: typed refusal audit missing or invalid"
  echo "$ACTIVE" | jq '[.turns[] | {id, resultStatus, refusal}]'
  exit 1
fi

echo -n "[partial judgment] "
# Answered turns should carry judgment; partial reasons must be from the allow-list.
JUDGE_MISSING=$(echo "$ACTIVE" | jq '[.turns[]
  | select(.resultStatus=="answered")
  | select(.judgment == null or ((.judgment.kind // "")|length)<1)] | length')
JUDGE_BAD=$(echo "$ACTIVE" | jq '[.turns[]
  | select(.judgment != null)
  | select(
      ((.judgment.kind as $k | ($k!="grounded" and $k!="partial")))
      or (
        .judgment.kind=="partial"
        and ((.judgment.reason // "") as $r
          | ($r!="" and $r!="weak_only" and $r!="has_unresolved" and $r!="mixed"))
      )
    )] | length')
JUDGE_N=$(echo "$ACTIVE" | jq '[.turns[] | select(.judgment != null)] | length')
JUDGE_SAMPLE=$(echo "$ACTIVE" | jq -c '[.turns[] | select(.judgment != null) | {id, kind:.judgment.kind, reason:(.judgment.reason // ""), g:.judgment.groundedClaims, w:.judgment.weakClaims}] | .[0]')
echo "present=$JUDGE_N missing=$JUDGE_MISSING bad=$JUDGE_BAD sample=$JUDGE_SAMPLE"
if [[ "$JUDGE_MISSING" -gt 0 || "$JUDGE_BAD" -gt 0 ]]; then
  echo "ERROR: judgment audit missing or invalid"
  echo "$ACTIVE" | jq '[.turns[] | {id, resultStatus, judgment}]'
  exit 1
fi
if [[ "$BOUND_OK" -gt 0 ]]; then
  echo "ERROR: answered turns with hits must include claims"
  echo "$ACTIVE" | jq '[.turns[] | {id, resultStatus, hits:(.hits|length), claims}]'
  exit 1
fi
if [[ "$LEAK_CLAIMS" -gt 0 ]]; then
  echo "ERROR: no_hits/refused turns must not carry claims"
  echo "$ACTIVE" | jq '[.turns[] | {id, resultStatus, hits:(.hits|length), claims}]'
  exit 1
fi

echo -n "[diligence export] "
EXPORT_CODE=$(curl -sS -o /tmp/kqa-export.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/sessions/$SESSION_ID/export")
echo "HTTP $EXPORT_CODE $(jq -c '{schemaVersion, sessionId, turns:(.turns|length), fp:(.corpusFingerprint // "" | .[0:16])}' </tmp/kqa-export.json 2>/dev/null || cat /tmp/kqa-export.json)"
if [[ "$EXPORT_CODE" != "200" ]] || ! jq -e '.schemaVersion=="knowledge_qa_diligence_v1" and .sessionId and (.turns|length)>=1' >/dev/null </tmp/kqa-export.json; then
  echo "ERROR: expected diligence export pack"
  exit 1
fi
# Turns written after Phase H should carry a corpus fingerprint snapshot.
FP_TURNS=$(jq '[.turns[] | select((.corpusFingerprint // "")|length>0)] | length' </tmp/kqa-export.json)
if [[ "$FP_TURNS" -lt 1 ]]; then
  echo "ERROR: expected at least one turn with corpusFingerprint"
  jq -c '[.turns[] | {id, corpusFingerprint, durationMs}]' </tmp/kqa-export.json
  exit 1
fi

echo -n "[ops summary] "
OPS_CODE=$(curl -sS -o /tmp/kqa-ops.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/ops?windowHours=24")
echo "HTTP $OPS_CODE $(jq -c '{scope, turnsTotal, avgDurationMs, p95DurationMs, costUnitsTotal, refusalsByKind, judgmentsByKind, pendingEvalCandidates, coldArchiveCount, retentionDays}' </tmp/kqa-ops.json 2>/dev/null || cat /tmp/kqa-ops.json)"
if [[ "$OPS_CODE" != "200" ]] || ! jq -e '.scope=="workspace" and (.turnsTotal|type)=="number"' >/dev/null </tmp/kqa-ops.json; then
  echo "ERROR: expected ops summary"
  exit 1
fi
# Phase M: cost / SLO attribution fields must be present.
if ! jq -e '(.p95DurationMs|type)=="number" and (.costUnitsTotal|type)=="number" and (.refusalsByKind|type)=="object" and (.judgmentsByKind|type)=="object"' >/dev/null </tmp/kqa-ops.json; then
  echo "ERROR: ops summary missing Phase M cost/SLO fields"
  jq . </tmp/kqa-ops.json
  exit 1
fi
# Phase O: gold-review queue counters.
if ! jq -e '(.pendingEvalCandidates|type)=="number" and (.evalCandidatesByStatus|type)=="object"' >/dev/null </tmp/kqa-ops.json; then
  echo "ERROR: ops summary missing Phase O eval candidate fields"
  jq . </tmp/kqa-ops.json
  exit 1
fi
# Smoke asks produce refused/no_hits turns → expect cost units and/or refusals.
COST_N=$(jq '.costUnitsTotal' </tmp/kqa-ops.json)
REF_N=$(jq '[.refusalsByKind | to_entries[] | .value] | add // 0' </tmp/kqa-ops.json)
if [[ "$COST_N" -lt 1 && "$REF_N" -lt 1 ]]; then
  echo "ERROR: expected costUnits or refusals after smoke asks"
  jq . </tmp/kqa-ops.json
  exit 1
fi

echo -n "[cold archives list] "
ARCH_CODE=$(curl -sS -o /tmp/kqa-arch.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/archives")
echo "HTTP $ARCH_CODE $(jq -c '{n:(.items|length)}' </tmp/kqa-arch.json 2>/dev/null || cat /tmp/kqa-arch.json)"
if [[ "$ARCH_CODE" != "200" ]] || ! jq -e '.items|type=="array"' >/dev/null </tmp/kqa-arch.json; then
  echo "ERROR: expected archives list"
  exit 1
fi
# Tombstones must never leak object-storage keys.
if ! jq -e '[.items[] | select((.storageKey // "") != "")] | length == 0' >/dev/null </tmp/kqa-arch.json; then
  echo "ERROR: archives list leaked storageKey"
  jq . </tmp/kqa-arch.json
  exit 1
fi

echo -n "[cold archive missing → 404] "
ARCH_MISS_CODE=$(curl -sS -o /tmp/kqa-arch-miss.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/archives/00000000-0000-4000-8000-000000000001")
echo "HTTP $ARCH_MISS_CODE"
if [[ "$ARCH_MISS_CODE" != "404" ]]; then
  echo "ERROR: expected 404 for missing archive"
  cat /tmp/kqa-arch-miss.json
  exit 1
fi

ARCH_N=$(jq '.items|length' </tmp/kqa-arch.json)
if [[ "$ARCH_N" -ge 1 ]]; then
  ARCH_ID=$(jq -r '.items[0].id' </tmp/kqa-arch.json)
  echo -n "[cold archive detail restore] "
  ARCH_D_CODE=$(curl -sS -o /tmp/kqa-arch-detail.json -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/workspaces/$WS_SLUG/deal-rooms/$ROOM_ID/knowledge/archives/$ARCH_ID")
  echo "HTTP $ARCH_D_CODE $(jq -c '{status:.archive.status, turns:(.pack.turns|length), schema:.pack.schemaVersion}' </tmp/kqa-arch-detail.json 2>/dev/null || true)"
  if [[ "$ARCH_D_CODE" != "200" ]]; then
    echo "ERROR: expected archive detail 200"
    cat /tmp/kqa-arch-detail.json
    exit 1
  fi
  if ! jq -e '
    .archive.status == "restored_readonly"
    and ((.archive.storageKey // "") == "")
    and (.pack.schemaVersion|type)=="string" and (.pack.schemaVersion|length)>0
    and (.pack.sessionId|type)=="string" and (.pack.sessionId|length)>0
    and (.pack.sessionId == .archive.sessionId)
    and (.pack.turns|type)=="array"
  ' >/dev/null </tmp/kqa-arch-detail.json; then
    echo "ERROR: archive detail contract failed (Phase W)"
    jq . </tmp/kqa-arch-detail.json
    exit 1
  fi
else
  echo "[cold archive detail] skipped (no tombstones in fresh smoke room)"
fi

echo "=== Knowledge Q&A smoke passed ==="

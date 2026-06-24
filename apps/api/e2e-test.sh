#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
PDF="${PDF:-e2e-test.pdf}"

echo "=== DealSignal E2E P0 verification ==="
echo "BASE_URL=$BASE_URL"

# Generate a minimal valid PDF fixture if one is not provided.
if [[ ! -f "$PDF" ]]; then
  if command -v ps2pdf >/dev/null 2>&1; then
    TMP_PS=$(mktemp)
    cat > "$TMP_PS" <<'EOF'
%!PS
/Times-Roman findfont 24 scalefont setfont
100 700 moveto
(Hello DealSignal E2E) show
showpage
EOF
    ps2pdf "$TMP_PS" "$PDF"
    rm -f "$TMP_PS"
  else
    echo "ERROR: $PDF not found and ps2pdf is unavailable to generate it"
    exit 1
  fi
fi

# 1. Health
echo -n "[healthz] "
curl -fsS "$BASE_URL/healthz" | jq -c .

# 2. Register
echo -n "[register] "
EMAIL="e2e-$(date +%s)@example.com"
PASSWORD="Password123!"
REGISTER=$(curl -fsS -X POST "$BASE_URL/api/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
echo "$REGISTER" | jq -c '{user_id: .user.id, email: .user.email}'
TOKEN=$(echo "$REGISTER" | jq -r '.access_token')

# 3. Create workspace
echo -n "[workspace create] "
SLUG="e2e-$(date +%s)"
WORKSPACE=$(curl -fsS -X POST "$BASE_URL/api/workspaces" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"E2E Workspace\",\"slug\":\"$SLUG\",\"brand_color\":\"#0055ff\"}")
echo "$WORKSPACE" | jq -c '{id: .id, slug: .slug}'
WORKSPACE_ID=$(echo "$WORKSPACE" | jq -r '.id')
WORKSPACE_SLUG=$(echo "$WORKSPACE" | jq -r '.slug')

# 4. Upload PDF
echo -n "[upload document] "
UPLOAD=$(curl -fsS -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@$PDF")
echo "$UPLOAD" | jq -c '{id: .id, status: .status, job_status: .ingestion_job.status}'
DOC_ID=$(echo "$UPLOAD" | jq -r '.id')

# 5. Poll until ready
echo -n "[wait ready]"
for i in $(seq 1 30); do
  sleep 1
  STATUS=$(curl -fsS -H "Authorization: Bearer $TOKEN" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents/$DOC_ID/status" | jq -r '.status')
  echo -n " $STATUS"
  if [[ "$STATUS" == "ready" ]]; then
    echo ""
    break
  fi
  if [[ "$STATUS" == "failed" ]]; then
    echo ""
    echo "ERROR: document ingestion failed"
    curl -fsS -H "Authorization: Bearer $TOKEN" \
      "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents/$DOC_ID/status" | jq .
    exit 1
  fi
  if [[ "$i" == "30" ]]; then
    echo ""
    echo "ERROR: document did not become ready in time"
    exit 1
  fi
done

# 6. List pages
echo -n "[list pages] "
PAGES=$(curl -fsS -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/documents/$DOC_ID/pages")
echo "$PAGES" | jq -c '{total: .total, pages: [.pages[].page_number]}'

# 7. Create link
echo -n "[create link] "
LINK=$(curl -fsS -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/links" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"document_id\":\"$DOC_ID\",\"name\":\"E2E Link\",\"permission_type\":\"public\",\"download_enabled\":true}")
echo "$LINK" | jq -c '{id: .id, public_token: .public_token, short_url: .short_url}'
LINK_ID=$(echo "$LINK" | jq -r '.id')
TOKEN_PUBLIC=$(echo "$LINK" | jq -r '.public_token')

# 8. Access public link
echo -n "[public access] "
ACCESS=$(curl -fsS "$BASE_URL/api/v1/public/links/$TOKEN_PUBLIC")
echo "$ACCESS" | jq -c '{visitor_id: .visitor_id, document_status: .document.status, page_count: .document.page_count}'
VISITOR_ID=$(echo "$ACCESS" | jq -r '.visitor_id')

# 9. Record page viewed event
echo -n "[record event] "
curl -fsS -X POST "$BASE_URL/api/v1/public/events" \
  -H "Content-Type: application/json" \
  -d "{\"event_type\":\"page_viewed\",\"public_token\":\"$TOKEN_PUBLIC\",\"visitor_id\":\"$VISITOR_ID\",\"page_number\":1,\"duration_seconds\":12,\"scroll_depth\":0.75}" \
  -o /dev/null -w "%{http_code}\n"

# 10. Heat score
echo -n "[heat score] "
SCORE=$(curl -fsS -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/analytics/links/$LINK_ID/score")
echo "$SCORE" | jq -c '{score: .score, level: .level, trend: .trend}'

# 11. AI search (only when RUN_AI=1)
if [[ "${RUN_AI:-0}" == "1" ]]; then
  echo -n "[ai search] "
  SEARCH=$(curl -fsS -H "Authorization: Bearer $TOKEN" \
    "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/search?q=DealSignal&limit=5")
  EVIDENCE_COUNT=$(echo "$SEARCH" | jq '.evidence | length')
  echo "evidence_count=$EVIDENCE_COUNT"
  if [[ "$EVIDENCE_COUNT" -eq 0 ]]; then
    echo "ERROR: AI search returned no evidence"
    exit 1
  fi

  echo -n "[ai assistant] "
  CHAT=$(curl -fsS -X POST "$BASE_URL/api/workspaces/$WORKSPACE_SLUG/assistant/chat" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"message":"What does the document say?"}')
  echo "$CHAT" | jq -c '{session_id: .session_id, answer: .answer, evidence_count: (.evidence | length)}'
fi

echo "=== E2E P0 verification complete ==="

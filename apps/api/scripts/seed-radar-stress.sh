#!/usr/bin/env bash
# Seed / clean Deal Radar six-product stress rows against the local API DB (:8090 stack).
#
# These rows are tagged so they never collide with real host work:
#   signals.metadata.seed = 'radar-stress'
#   action_items.source_type = 'radar_stress_gate' (diligence only)
#   titles prefix: [radar-stress]
#
# Usage:
#   ./scripts/seed-radar-stress.sh clean
#   ./scripts/seed-radar-stress.sh seed [--workspace kendiyang] [--per-product 8]
#   ./scripts/seed-radar-stress.sh reset   # clean + seed
#   ./scripts/seed-radar-stress.sh status
#
# Env (defaults match apps/api docker-compose on :5436 / API :8090):
#   DATABASE_URL or PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE
#   WORKSPACE_SLUG  PER_PRODUCT
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PGHOST="${PGHOST:-127.0.0.1}"
PGPORT="${PGPORT:-5436}"
PGUSER="${PGUSER:-dealsignal}"
PGPASSWORD="${PGPASSWORD:-dealsignal}"
PGDATABASE="${PGDATABASE:-dealsignal}"
export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE

WORKSPACE_SLUG="${WORKSPACE_SLUG:-kendiyang}"
PER_PRODUCT="${PER_PRODUCT:-8}"
API_BASE="${API_BASE:-http://localhost:8090}"

SEED_TAG="radar-stress"
GATE_SOURCE="radar_stress_gate"

psql_q() {
  psql -v ON_ERROR_STOP=1 "$@"
}

usage() {
  sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
  exit 1
}

cmd="${1:-}"
shift || true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --workspace) WORKSPACE_SLUG="$2"; shift 2 ;;
    --per-product) PER_PRODUCT="$2"; shift 2 ;;
    --api) API_BASE="$2"; shift 2 ;;
    -h|--help) usage ;;
    *) echo "unknown arg: $1" >&2; usage ;;
  esac
done

if [[ -z "$cmd" ]]; then
  usage
fi

ws_exists() {
  psql_q -tAc "SELECT 1 FROM workspaces WHERE slug = '${WORKSPACE_SLUG}' LIMIT 1" | grep -q 1
}

clean_msw_artifacts() {
  # Playwright stress UX leftovers (MSW never wrote Postgres).
  local web_root
  web_root="$(cd "$ROOT/../web" && pwd)"
  if [[ -d "$web_root/test-results" ]]; then
    find "$web_root/test-results" -maxdepth 1 -type d -name 'radar-products-stress-ux*' -exec rm -rf {} + 2>/dev/null || true
    echo "cleaned playwright test-results for radar-products-stress-ux"
  fi
}

clean_db() {
  if ! ws_exists; then
    echo "workspace slug=${WORKSPACE_SLUG} not found — nothing to clean in DB"
    return 0
  fi
  # access_logs / security_events are append-only; briefly disable triggers for stress cleanup.
  psql_q <<SQL
BEGIN;
ALTER TABLE access_logs DISABLE TRIGGER USER;
ALTER TABLE security_events DISABLE TRIGGER USER;

DELETE FROM access_logs
WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
  AND (
    visitor_id LIKE 'stress_%'
    OR user_agent = '${SEED_TAG}'
    OR link_id IN (
      SELECT id FROM links
      WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
        AND name LIKE '[${SEED_TAG}]%'
    )
  );

DELETE FROM security_events
WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
  AND (
    visitor_id LIKE 'stress_%'
    OR user_agent = '${SEED_TAG}'
    OR link_id IN (
      SELECT id FROM links
      WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
        AND name LIKE '[${SEED_TAG}]%'
    )
  );

ALTER TABLE access_logs ENABLE TRIGGER USER;
ALTER TABLE security_events ENABLE TRIGGER USER;

-- Signal-backed stress cards (action_items cascade via signal_id FK).
DELETE FROM signals
WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
  AND (
    metadata->>'seed' = '${SEED_TAG}'
    OR title LIKE '[${SEED_TAG}]%'
  );

-- Diligence operational cards (no signal_id).
DELETE FROM action_items
WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
  AND (
    source_type = '${GATE_SOURCE}'
    OR title LIKE '[${SEED_TAG}]%'
  );

-- Disposable share links created for coalesce diversity.
DELETE FROM links
WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
  AND name LIKE '[${SEED_TAG}]%';
COMMIT;
SQL
  echo "cleaned radar-stress rows for workspace=${WORKSPACE_SLUG}"
}

status_db() {
  if ! ws_exists; then
    echo "workspace slug=${WORKSPACE_SLUG} not found"
    exit 1
  fi
  psql_q <<SQL
SELECT 'signals' AS kind, count(*)::text AS n
FROM signals
WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
  AND metadata->>'seed' = '${SEED_TAG}'
UNION ALL
SELECT 'gate_actions', count(*)::text
FROM action_items
WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
  AND source_type = '${GATE_SOURCE}'
UNION ALL
SELECT 'pending_actions_all', count(*)::text
FROM action_items
WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
  AND status = 'pending'
UNION ALL
SELECT 'stress_access_logs', count(*)::text
FROM access_logs
WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
  AND visitor_id LIKE 'stress_%'
UNION ALL
SELECT 'stress_security_events', count(*)::text
FROM security_events
WHERE workspace_id = (SELECT id FROM workspaces WHERE slug = '${WORKSPACE_SLUG}')
  AND visitor_id LIKE 'stress_%';
SQL
  echo "API health: $(curl -s -o /dev/null -w '%{http_code}' "${API_BASE}/healthz" || echo fail)"
}

seed_db() {
  if ! ws_exists; then
    echo "workspace slug=${WORKSPACE_SLUG} not found" >&2
    exit 1
  fi
  if ! [[ "$PER_PRODUCT" =~ ^[0-9]+$ ]] || [[ "$PER_PRODUCT" -lt 1 ]]; then
    echo "PER_PRODUCT must be a positive integer" >&2
    exit 1
  fi

  # Idempotent: wipe prior stress batch first.
  clean_db

  psql_q <<SQL
DO \$\$
DECLARE
  v_ws uuid;
  v_tenant uuid;
  v_doc uuid;
  v_link uuid;
  v_sig uuid;
  i int;
  actor text;
  actors text[] := ARRAY[
    'lp@stress.example.com',
    'analyst@stress.example.com',
    'buyer@stress.example.com',
    'partner@stress.example.com',
    'ir@stress.example.com',
    'counsel@stress.example.com'
  ];
  deal_names text[] := ARRAY[
    'Northstar Series A',
    'Harbor Fund I',
    'Apex M&A',
    'Riverview Closing',
    'Portfolio IR Q3',
    'Enterprise Pipeline',
    'Seed Round Deck',
    'Bridge Diligence'
  ];
BEGIN
  SELECT id, tenant_id INTO v_ws, v_tenant FROM workspaces WHERE slug = '${WORKSPACE_SLUG}';
  IF v_ws IS NULL THEN
    RAISE EXCEPTION 'workspace not found';
  END IF;

  SELECT id INTO v_doc FROM documents WHERE workspace_id = v_ws ORDER BY created_at DESC LIMIT 1;
  IF v_doc IS NULL THEN
    RAISE EXCEPTION 'workspace has no documents — upload one before seeding';
  END IF;

  SELECT id INTO v_link FROM links
  WHERE workspace_id = v_ws AND document_id IS NOT NULL AND status <> 'deleted'
  ORDER BY created_at DESC LIMIT 1;
  IF v_link IS NULL THEN
    RAISE EXCEPTION 'workspace has no document share links — create one before seeding';
  END IF;

  -- Ensure enough distinct links so coalesce does not collapse everything.
  FOR i IN 1..GREATEST(${PER_PRODUCT}, 4) LOOP
    INSERT INTO links (
      tenant_id, workspace_id, document_id, public_token, name,
      permission_type, require_email, status
    )
    SELECT
      v_tenant,
      v_ws,
      v_doc,
      'stress_' || substr(md5(random()::text || clock_timestamp()::text || i::text), 1, 24),
      '[${SEED_TAG}] link ' || i || ' ' || substr(md5(i::text), 1, 6),
      'email_required',
      true,
      'active'
    WHERE NOT EXISTS (
      SELECT 1 FROM links
      WHERE workspace_id = v_ws AND name = '[${SEED_TAG}] link ' || i || ' ' || substr(md5(i::text), 1, 6)
    );
  END LOOP;

  FOR i IN 0..(${PER_PRODUCT} - 1) LOOP
    actor := actors[1 + (i % array_length(actors, 1))];

    -- buying_window
    SELECT id INTO v_link FROM links
    WHERE workspace_id = v_ws AND name LIKE '[${SEED_TAG}] link %'
    ORDER BY created_at DESC
    OFFSET (i % GREATEST(${PER_PRODUCT}, 4)) LIMIT 1;

    INSERT INTO signals (
      tenant_id, workspace_id, type, subtype, title, description, explanation, suggestion,
      document_id, link_id, priority, metadata, context, created_at
    ) VALUES (
      v_tenant, v_ws, 'hot_signal', 'hot',
      '[${SEED_TAG}] Hot window ' || i || ' — ' || deal_names[1 + (i % array_length(deal_names, 1))],
      actor || ' reopened the deck',
      'Multiple key-page views showing hot intent',
      'Follow up while intent is warm',
      v_doc, v_link, CASE WHEN i % 3 = 0 THEN 'high' ELSE 'medium' END,
      jsonb_build_object('seed', '${SEED_TAG}', 'product', 'buying_window', 'n', i),
      jsonb_build_object(
        'contactEmail', actor,
        'documentTitle', deal_names[1 + (i % array_length(deal_names, 1))],
        'opens', 4 + (i % 5),
        'uniqueVisitors', 2,
        'keyPageTitles', jsonb_build_array('Executive summary', 'Financials')
      ),
      now() - ((i * 17) || ' minutes')::interval
    ) RETURNING id INTO v_sig;

    INSERT INTO action_items (
      tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type, created_at
    ) VALUES (
      v_tenant, v_ws, v_sig,
      '[${SEED_TAG}] Email ' || actor || ' on ' || deal_names[1 + (i % array_length(deal_names, 1))],
      CASE WHEN i % 3 = 0 THEN 'high' ELSE 'medium' END,
      now() + ((2 + (i % 6)) || ' hours')::interval,
      'pending', 'email',
      now() - ((i * 17) || ' minutes')::interval
    );

    -- commitment_ask
    INSERT INTO signals (
      tenant_id, workspace_id, type, subtype, title, description, explanation, suggestion,
      document_id, link_id, priority, metadata, context, created_at
    ) VALUES (
      v_tenant, v_ws, 'follow_up', 'question',
      '[${SEED_TAG}] Ask from ' || actor || ' #' || i,
      'Visitor Ask waiting on host',
      'Commitment Ask pending reply',
      'Reply in Ask inbox',
      v_doc, v_link, 'high',
      jsonb_build_object('seed', '${SEED_TAG}', 'product', 'commitment_ask', 'n', i),
      jsonb_build_object('contactEmail', actor, 'documentTitle', deal_names[1 + (i % array_length(deal_names, 1))]),
      now() - ((i * 19) || ' minutes')::interval
    ) RETURNING id INTO v_sig;

    INSERT INTO action_items (
      tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type, created_at
    ) VALUES (
      v_tenant, v_ws, v_sig,
      '[${SEED_TAG}] Reply to Ask from ' || actor,
      'high',
      now() + ((1 + (i % 4)) || ' hours')::interval,
      'pending', 'answer',
      now() - ((i * 19) || ' minutes')::interval
    );

    -- leak_watch
    INSERT INTO signals (
      tenant_id, workspace_id, type, subtype, title, description, explanation, suggestion,
      document_id, link_id, priority, metadata, context, created_at
    ) VALUES (
      v_tenant, v_ws, 'risk_alert', 'forward',
      '[${SEED_TAG}] Forward risk #' || i,
      'Unrecognized forward pattern',
      'Possible leak / forward',
      'Review before it spreads',
      v_doc, v_link, 'high',
      jsonb_build_object('seed', '${SEED_TAG}', 'product', 'leak_watch', 'n', i),
      jsonb_build_object(
        'contactEmail', actor,
        'documentTitle', deal_names[1 + (i % array_length(deal_names, 1))],
        'forwardSignals', 2 + (i % 3)
      ),
      now() - ((i * 23) || ' minutes')::interval
    ) RETURNING id INTO v_sig;

    INSERT INTO action_items (
      tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type, created_at
    ) VALUES (
      v_tenant, v_ws, v_sig,
      '[${SEED_TAG}] Review forward risk for ' || deal_names[1 + (i % array_length(deal_names, 1))],
      'high',
      now() + ((3 + (i % 5)) || ' hours')::interval,
      'pending', 'review',
      now() - ((i * 23) || ' minutes')::interval
    );

    -- access_decay
    INSERT INTO signals (
      tenant_id, workspace_id, type, subtype, title, description, explanation, suggestion,
      document_id, link_id, priority, metadata, context, created_at
    ) VALUES (
      v_tenant, v_ws, 'risk_alert', 'expired',
      '[${SEED_TAG}] Expired access #' || i,
      'Visitor hit an expired link',
      'Access decay',
      'Renew the share',
      v_doc, v_link, 'medium',
      jsonb_build_object('seed', '${SEED_TAG}', 'product', 'access_decay', 'n', i),
      jsonb_build_object('contactEmail', actor, 'documentTitle', deal_names[1 + (i % array_length(deal_names, 1))]),
      now() - ((i * 29) || ' minutes')::interval
    ) RETURNING id INTO v_sig;

    INSERT INTO action_items (
      tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type, created_at
    ) VALUES (
      v_tenant, v_ws, v_sig,
      '[${SEED_TAG}] Renew access for ' || deal_names[1 + (i % array_length(deal_names, 1))],
      'medium',
      now() + ((5 + (i % 4)) || ' hours')::interval,
      'pending', 'renew',
      now() - ((i * 29) || ' minutes')::interval
    );

    -- abuse_guard
    INSERT INTO signals (
      tenant_id, workspace_id, type, subtype, title, description, explanation, suggestion,
      document_id, link_id, priority, metadata, context, created_at
    ) VALUES (
      v_tenant, v_ws, 'risk_alert', 'anomaly',
      '[${SEED_TAG}] Abuse / rate limit #' || i,
      'Ask or capture rate limit',
      'Abuse guard',
      'Tighten quotas',
      v_doc, v_link, 'high',
      jsonb_build_object(
        'seed', '${SEED_TAG}',
        'product', 'abuse_guard',
        'n', i,
        'eventType', 'rate_limit_exceeded',
        'ruleId', 'security_rate_limit_exceeded'
      ),
      jsonb_build_object('contactEmail', actor, 'documentTitle', deal_names[1 + (i % array_length(deal_names, 1))]),
      now() - ((i * 31) || ' minutes')::interval
    ) RETURNING id INTO v_sig;

    INSERT INTO action_items (
      tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type, created_at
    ) VALUES (
      v_tenant, v_ws, v_sig,
      '[${SEED_TAG}] Review abuse on ' || deal_names[1 + (i % array_length(deal_names, 1))],
      'high',
      now() + ((1 + (i % 3)) || ' hours')::interval,
      'pending', 'review',
      now() - ((i * 31) || ' minutes')::interval
    );

    -- diligence_gate (operational; custom source_type so SyncWorkspace will not closeStale)
    INSERT INTO action_items (
      tenant_id, workspace_id, source_type, source_id, title, impact, due_at, status, action_type, created_at
    ) VALUES (
      v_tenant, v_ws,
      '${GATE_SOURCE}',
      '${SEED_TAG}-gate-' || i,
      '[${SEED_TAG}] Approve access request from ' || actor || ' for ' || deal_names[1 + (i % array_length(deal_names, 1))],
      'high',
      now() + ((2 + (i % 3)) || ' hours')::interval,
      'pending', 'approve',
      now() - ((i * 13) || ' minutes')::interval
    );

    -- Evidence facets for Evidence rail (24h metrics + security events).
    -- visitor_id / user_agent tagged so clean can remove append-only rows.
    INSERT INTO access_logs (
      tenant_id, workspace_id, link_id, visitor_id, visitor_email, event_type, user_agent, created_at
    ) VALUES
      -- buying_window / commitment: opens + revisit
      (v_tenant, v_ws, v_link, 'stress_vis_' || i || '_a', actor, 'link_opened', '${SEED_TAG}', now() - ((20 + i) || ' minutes')::interval),
      (v_tenant, v_ws, v_link, 'stress_vis_' || i || '_a', actor, 'link_opened', '${SEED_TAG}', now() - ((15 + i) || ' minutes')::interval),
      (v_tenant, v_ws, v_link, 'stress_vis_' || i || '_b', actor, 'return_visit', '${SEED_TAG}', now() - ((12 + i) || ' minutes')::interval),
      (v_tenant, v_ws, v_link, 'stress_vis_' || i || '_b', actor, 'link_opened', '${SEED_TAG}', now() - ((11 + i) || ' minutes')::interval),
      -- leak_watch: forwards + download
      (v_tenant, v_ws, v_link, 'stress_vis_' || i || '_c', actor, 'forward_signal', '${SEED_TAG}', now() - ((9 + i) || ' minutes')::interval),
      (v_tenant, v_ws, v_link, 'stress_vis_' || i || '_d', 'unknown+' || i || '@forward.test', 'forward_signal', '${SEED_TAG}', now() - ((8 + i) || ' minutes')::interval),
      (v_tenant, v_ws, v_link, 'stress_vis_' || i || '_c', actor, 'download_attempted', '${SEED_TAG}', now() - ((7 + i) || ' minutes')::interval);

    INSERT INTO security_events (
      tenant_id, workspace_id, link_id, event_type, visitor_id, email, reason, user_agent, created_at
    ) VALUES
      -- access_decay
      (v_tenant, v_ws, v_link, 'expired_link_accessed', 'stress_vis_' || i || '_e', actor, NULL, '${SEED_TAG}', now() - ((6 + i) || ' minutes')::interval),
      -- abuse_guard
      (v_tenant, v_ws, v_link, 'rate_limit_exceeded', 'stress_vis_' || i || '_f', actor, 'ask_ai_rpm', '${SEED_TAG}', now() - ((5 + i) || ' minutes')::interval),
      (v_tenant, v_ws, v_link, 'capture_attempt', 'stress_vis_' || i || '_f', actor, 'printscreen', '${SEED_TAG}', now() - ((4 + i) || ' minutes')::interval),
      -- diligence_gate (stress) + leak corroboration
      (v_tenant, v_ws, v_link, 'security_gate_failed', 'stress_vis_' || i || '_g', actor, 'email_code_required', '${SEED_TAG}', now() - ((3 + i) || ' minutes')::interval),
      (v_tenant, v_ws, v_link, 'abnormal_access_pattern', 'stress_vis_' || i || '_c', actor, 'forward cluster', '${SEED_TAG}', now() - ((2 + i) || ' minutes')::interval);
  END LOOP;
END
\$\$;
SQL

  echo "seeded ${PER_PRODUCT}×6 radar-stress products for workspace=${WORKSPACE_SLUG}"
  status_db
  echo
  echo "Refresh Deal Radar for /${WORKSPACE_SLUG}/dashboard (API ${API_BASE})."
  echo "Expected filter chips ≈ ${PER_PRODUCT} per product (+ any real pending gates)."
}

case "$cmd" in
  clean)
    clean_msw_artifacts
    clean_db
    ;;
  seed)
    seed_db
    ;;
  reset)
    clean_msw_artifacts
    seed_db
    ;;
  status)
    status_db
    ;;
  *)
    usage
    ;;
esac

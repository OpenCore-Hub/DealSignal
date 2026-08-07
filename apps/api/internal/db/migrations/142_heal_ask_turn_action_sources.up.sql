-- Turns-only Ask (141): pending deal_room_link_question rows may still reference
-- legacy question UUIDs as source_id. Close orphans so SyncWorkspace recreates
-- actions keyed by turn id with correct target_id navigation.
UPDATE action_items ai
SET status = 'done',
    updated_at = now()
WHERE ai.status = 'pending'
  AND ai.source_type = 'deal_room_link_question'
  AND NOT EXISTS (
    SELECT 1
    FROM link_ask_turns t
    JOIN links l ON l.id = t.link_id
    WHERE t.id::text = ai.source_id
      AND t.workspace_id = ai.workspace_id
      AND t.lane IN ('host', 'hybrid')
      AND t.status IN ('host_pending', 'host_escalated')
      AND l.deal_room_id IS NOT NULL
  );

-- Document-surface link_question rows are obsolete; sync closes these each run
-- but heal once at deploy for immediate dashboard UX.
UPDATE action_items
SET status = 'done',
    updated_at = now()
WHERE status = 'pending'
  AND source_type = 'link_question';

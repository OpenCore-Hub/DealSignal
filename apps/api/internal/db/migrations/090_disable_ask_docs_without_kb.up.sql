-- Q5: turn off Ask Docs on deal-room links whose room KB is not ready/stale.
UPDATE links l
SET ai_copilot_enabled = false,
    updated_at = now()
WHERE l.deal_room_id IS NOT NULL
  AND l.ai_copilot_enabled = true
  AND l.status <> 'deleted'
  AND NOT EXISTS (
    SELECT 1
    FROM deal_room_knowledge_bases kb
    WHERE kb.room_id = l.deal_room_id
      AND kb.status IN ('ready', 'stale')
  );

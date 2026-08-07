-- Pre-split link_question rows used question id as source_id but dashboard
-- navigation expected a link id → /links/{questionId}. Close so SyncWorkspace
-- recreates deal_room_link_question rows with room/link target_id.
UPDATE action_items
SET status = 'done',
    updated_at = now()
WHERE status = 'pending'
  AND source_type = 'link_question';

-- Navigation parent for operational action items (e.g. deal_room id when
-- source_id is a share-link id). Keeps upsert/resolve identity (source_*)
-- separate from dashboard deep-link routing (target_id).
ALTER TABLE action_items
    ADD COLUMN IF NOT EXISTS target_id TEXT;

-- Heal pre-split operational action rows so the next SyncWorkspace recreates
-- surface-correct todos (document share vs deal-room share vs room membership).
--
-- 1) room_access_request / room_nda used request/member UUIDs as source_id;
--    navigation expected a deal_rooms.id → "room not found".
-- 2) link_access_request included deal-room shares; Document Library inbox
--    excludes those applicants → dead dashboard deep links.

UPDATE action_items a
SET status = 'done',
    updated_at = now()
WHERE a.status = 'pending'
  AND a.source_type IN ('room_access_request', 'room_nda')
  AND (
      a.source_id IS NULL
      OR a.source_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
      OR NOT EXISTS (
          SELECT 1
          FROM deal_rooms dr
          WHERE dr.workspace_id = a.workspace_id
            AND dr.id = a.source_id::uuid
      )
  );

UPDATE action_items a
SET status = 'done',
    updated_at = now()
WHERE a.status = 'pending'
  AND a.source_type = 'link_access_request'
  AND a.source_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  AND EXISTS (
      SELECT 1
      FROM links l
      WHERE l.workspace_id = a.workspace_id
        AND l.id = a.source_id::uuid
        AND l.deal_room_id IS NOT NULL
  );

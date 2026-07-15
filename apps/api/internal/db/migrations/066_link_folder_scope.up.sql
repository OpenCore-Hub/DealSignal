ALTER TABLE links ADD COLUMN IF NOT EXISTS folder_scope_paths TEXT[] NOT NULL DEFAULT '{}';

-- Backfill deal-room links that currently have document-level scope.
UPDATE links
SET folder_scope_paths = (
    SELECT ARRAY_AGG(DISTINCT drd.folder_path)
    FROM link_documents ld
    JOIN deal_room_documents drd
      ON drd.document_id = ld.document_id
     AND drd.room_id = links.deal_room_id
    WHERE ld.link_id = links.id
)
WHERE deal_room_id IS NOT NULL
  AND has_document_scope = TRUE;

-- Ensure unscoped deal-room links have an empty array and no NULL arrays.
UPDATE links
SET folder_scope_paths = '{}'
WHERE deal_room_id IS NOT NULL
  AND (folder_scope_paths IS NULL OR folder_scope_paths = '{NULL}');

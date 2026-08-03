-- Enforce case-insensitive unique link names at the database layer.
-- Scope matches application checks in ensureUniqueLinkName:
--   - deal-room links: unique within deal_room_id
--   - document links: unique within workspace_id (deal_room_id IS NULL)
-- Soft-deleted rows (status = 'deleted') and blank names are excluded.

-- Resolve existing deal-room duplicates before creating the unique index.
WITH ranked AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY deal_room_id, lower(btrim(name))
            ORDER BY created_at ASC NULLS LAST, id ASC
        ) AS rn
    FROM links
    WHERE deal_room_id IS NOT NULL
      AND status <> 'deleted'
      AND name IS NOT NULL
      AND btrim(name) <> ''
)
UPDATE links AS l
SET
    name = btrim(l.name) || ' (' || substring(l.id::text, 1, 8) || ')',
    updated_at = now()
FROM ranked AS r
WHERE l.id = r.id
  AND r.rn > 1;

-- Resolve existing workspace document-link duplicates.
WITH ranked AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY workspace_id, lower(btrim(name))
            ORDER BY created_at ASC NULLS LAST, id ASC
        ) AS rn
    FROM links
    WHERE deal_room_id IS NULL
      AND status <> 'deleted'
      AND name IS NOT NULL
      AND btrim(name) <> ''
)
UPDATE links AS l
SET
    name = btrim(l.name) || ' (' || substring(l.id::text, 1, 8) || ')',
    updated_at = now()
FROM ranked AS r
WHERE l.id = r.id
  AND r.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS idx_links_unique_name_deal_room
    ON links (deal_room_id, lower(btrim(name)))
    WHERE deal_room_id IS NOT NULL
      AND status <> 'deleted'
      AND name IS NOT NULL
      AND btrim(name) <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_links_unique_name_workspace
    ON links (workspace_id, lower(btrim(name)))
    WHERE deal_room_id IS NULL
      AND status <> 'deleted'
      AND name IS NOT NULL
      AND btrim(name) <> '';

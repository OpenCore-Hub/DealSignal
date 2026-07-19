-- Fix ON CONFLICT for operational action items.
-- The partial unique index created in migration 079 cannot be used as an
-- ON CONFLICT target. Replace it with a proper unique constraint.

-- 1. Deduplicate existing operational action items, keeping the newest row.
DELETE FROM action_items
WHERE id IN (
    SELECT id
    FROM (
        SELECT id,
               row_number() OVER (
                   PARTITION BY workspace_id, source_type, source_id
                   ORDER BY created_at DESC, id DESC
               ) AS rn
        FROM action_items
        WHERE source_type IS NOT NULL
    ) ranked
    WHERE ranked.rn > 1
);

-- 2. Remove the partial index that cannot serve as an ON CONFLICT target.
DROP INDEX IF EXISTS idx_action_items_source;

-- 3. Add a real unique constraint for operational action items.
--    PostgreSQL allows multiple NULL combinations, so signal-based items
--    (source_type IS NULL) remain unaffected.
ALTER TABLE action_items
    ADD CONSTRAINT action_items_workspace_source_type_source_id_unique
        UNIQUE (workspace_id, source_type, source_id);

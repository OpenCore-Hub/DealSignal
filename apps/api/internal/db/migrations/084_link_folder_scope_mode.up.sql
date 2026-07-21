-- folder_scope_mode separates legacy "empty paths = whole room" from
-- secure allowlist semantics where empty paths mean deny-all.
-- Values: 'full' | 'allowlist'
ALTER TABLE links
  ADD COLUMN IF NOT EXISTS folder_scope_mode TEXT NOT NULL DEFAULT 'full';

ALTER TABLE links
  DROP CONSTRAINT IF EXISTS links_folder_scope_mode_check;

ALTER TABLE links
  ADD CONSTRAINT links_folder_scope_mode_check
  CHECK (folder_scope_mode IN ('full', 'allowlist'));

-- Preserve legacy behavior: empty paths meant whole-room access.
-- Explicit path lists were already allowlists.
UPDATE links
SET folder_scope_mode = 'allowlist'
WHERE cardinality(folder_scope_paths) > 0;

UPDATE links
SET folder_scope_mode = 'full'
WHERE cardinality(COALESCE(folder_scope_paths, '{}')) = 0;

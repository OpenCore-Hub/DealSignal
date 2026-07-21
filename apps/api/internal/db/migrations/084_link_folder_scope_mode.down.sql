ALTER TABLE links DROP CONSTRAINT IF EXISTS links_folder_scope_mode_check;
ALTER TABLE links DROP COLUMN IF EXISTS folder_scope_mode;

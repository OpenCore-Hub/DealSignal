ALTER TABLE links
    ADD COLUMN IF NOT EXISTS require_email BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS require_password BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS require_nda BOOLEAN NOT NULL DEFAULT false;

-- Backfill existing links based on the legacy single permission type.
UPDATE links
SET require_email = permission_type IN ('email_required', 'whitelist', 'nda'),
    require_password = permission_type = 'password',
    require_nda = permission_type = 'nda'
WHERE true;

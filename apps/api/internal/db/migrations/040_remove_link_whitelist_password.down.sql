-- Re-add whitelist and password columns (data is lost).

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS allowed_emails JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS allowed_domains JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS password_hash TEXT,
    ADD COLUMN IF NOT EXISTS require_password BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE links
    DROP CONSTRAINT IF EXISTS chk_links_permission_type;

ALTER TABLE links
    ADD CONSTRAINT chk_links_permission_type
        CHECK (permission_type IN ('public', 'email_required', 'whitelist', 'password', 'nda'));

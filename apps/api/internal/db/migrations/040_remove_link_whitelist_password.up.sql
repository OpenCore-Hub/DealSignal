-- Remove whitelist and password features from links.
-- First migrate legacy permission types so they satisfy the new check constraint.
UPDATE links
SET permission_type = 'email_required'
WHERE permission_type IN ('whitelist', 'password');

ALTER TABLE links
    DROP COLUMN IF EXISTS allowed_emails,
    DROP COLUMN IF EXISTS allowed_domains,
    DROP COLUMN IF EXISTS password_hash,
    DROP COLUMN IF EXISTS require_password;

-- Update permission_type check to remove 'whitelist' and 'password'.
ALTER TABLE links
    DROP CONSTRAINT IF EXISTS chk_links_permission_type;

ALTER TABLE links
    ADD CONSTRAINT chk_links_permission_type
        CHECK (permission_type IN ('public', 'email_required', 'nda'));

ALTER TABLE links
    DROP COLUMN IF EXISTS require_email,
    DROP COLUMN IF EXISTS require_password,
    DROP COLUMN IF EXISTS require_nda;

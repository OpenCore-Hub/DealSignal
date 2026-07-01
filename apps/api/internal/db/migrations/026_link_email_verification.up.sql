ALTER TABLE links
    ADD COLUMN IF NOT EXISTS require_email_verification BOOLEAN NOT NULL DEFAULT false;

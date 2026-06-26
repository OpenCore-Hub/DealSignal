-- Re-add the original (pre-NDA) permission_type check constraint.
-- Note: this will fail if any rows with permission_type = 'nda' exist.
ALTER TABLE links
    ADD CONSTRAINT links_permission_type_check
        CHECK (permission_type IN ('public','email_required','whitelist','password'));

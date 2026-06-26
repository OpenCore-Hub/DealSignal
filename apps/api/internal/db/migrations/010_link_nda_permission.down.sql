DROP TRIGGER IF EXISTS page_views_prevent_update ON page_views;
DROP TRIGGER IF EXISTS access_logs_prevent_update ON access_logs;
DROP FUNCTION IF EXISTS prevent_event_mutation();

ALTER TABLE links
    DROP CONSTRAINT IF EXISTS chk_links_permission_type;

ALTER TABLE links
    DROP CONSTRAINT IF EXISTS links_permission_type_check;

ALTER TABLE links
    ADD CONSTRAINT links_permission_type_check
        CHECK (permission_type IN ('public','email_required','whitelist','password'));

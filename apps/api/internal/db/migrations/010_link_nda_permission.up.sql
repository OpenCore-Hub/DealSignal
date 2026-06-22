ALTER TABLE links
    DROP CONSTRAINT IF EXISTS chk_links_permission_type;

ALTER TABLE links
    ADD CONSTRAINT chk_links_permission_type
        CHECK (permission_type IN ('public','email_required','whitelist','password','nda'));

-- Enforce append-only access logs and page views.
CREATE OR REPLACE FUNCTION prevent_event_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION '%.% is append-only and cannot be modified or deleted', TG_TABLE_SCHEMA, TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS access_logs_prevent_update ON access_logs;
CREATE TRIGGER access_logs_prevent_update
    BEFORE UPDATE OR DELETE ON access_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_event_mutation();

DROP TRIGGER IF EXISTS page_views_prevent_update ON page_views;
CREATE TRIGGER page_views_prevent_update
    BEFORE UPDATE OR DELETE ON page_views
    FOR EACH ROW EXECUTE FUNCTION prevent_event_mutation();

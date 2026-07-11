-- Migration: partition high-volume event tables by month on created_at.
-- Retention is enforced by dropping old partitions instead of row DELETE,
-- avoiding table bloat and bypassing the append-only triggers.

BEGIN;

CREATE OR REPLACE FUNCTION tmp_create_monthly_partitions(table_name text, start_month date, end_month date)
RETURNS void AS $$
DECLARE
    d date := start_month;
    p text;
BEGIN
    WHILE d <= end_month LOOP
        p := table_name || '_y' || to_char(d, 'YYYY') || 'm' || to_char(d, 'MM');
        EXECUTE format('CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L);',
            p, table_name, d, (d + interval '1 month')::date);
        d := (d + interval '1 month')::date;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tmp_copy_retained_data(src text, dst text, start_month date, end_month date)
RETURNS bigint AS $$
DECLARE
    n bigint;
BEGIN
    EXECUTE format('INSERT INTO %I SELECT * FROM %I WHERE created_at >= %L AND created_at < %L;',
        dst, src, start_month, (end_month + interval '1 month')::date);
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN n;
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    start_m date := date_trunc('month', now() - interval '6 months')::date;
    end_m   date := date_trunc('month', now() + interval '3 months')::date;
    copied bigint;
BEGIN
    -- access_logs
    ALTER TABLE access_logs RENAME TO access_logs_legacy;
    -- Index names are global; drop the legacy indexes so the new table can reuse canonical names.
    DROP INDEX IF EXISTS idx_access_logs_link;
    DROP INDEX IF EXISTS idx_access_logs_visitor;
    DROP INDEX IF EXISTS idx_access_logs_event_type;
    DROP INDEX IF EXISTS idx_access_logs_link_visitor_created;

    CREATE TABLE access_logs (
        id UUID NOT NULL DEFAULT gen_random_uuid(),
        tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
        workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
        link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
        visitor_id TEXT,
        visitor_email TEXT,
        event_type TEXT NOT NULL CHECK (event_type IN ('link_opened','download_attempted')),
        ip INET,
        user_agent TEXT,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        PRIMARY KEY (id, created_at)
    ) PARTITION BY RANGE (created_at);

    PERFORM tmp_create_monthly_partitions('access_logs', start_m, end_m);

    copied := tmp_copy_retained_data('access_logs_legacy', 'access_logs', start_m, end_m);
    RAISE NOTICE 'copied % access_logs rows', copied;

    CREATE INDEX idx_access_logs_link ON access_logs(link_id);
    CREATE INDEX idx_access_logs_visitor ON access_logs(visitor_id);
    CREATE INDEX idx_access_logs_event_type ON access_logs(event_type);
    CREATE INDEX idx_access_logs_link_visitor_created ON access_logs(link_id, visitor_id, created_at DESC);

    CREATE TRIGGER access_logs_prevent_update
        BEFORE UPDATE OR DELETE ON access_logs
        FOR EACH ROW EXECUTE FUNCTION prevent_event_mutation();

    -- page_views
    ALTER TABLE page_views RENAME TO page_views_legacy;
    DROP INDEX IF EXISTS idx_page_views_link;
    DROP INDEX IF EXISTS idx_page_views_visitor;
    DROP INDEX IF EXISTS idx_page_views_link_page;
    DROP INDEX IF EXISTS idx_page_views_link_visitor_page_created;

    CREATE TABLE page_views (
        id UUID NOT NULL DEFAULT gen_random_uuid(),
        tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
        workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
        link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
        visitor_id TEXT,
        page_number INT NOT NULL CHECK (page_number > 0),
        duration_seconds INT NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
        scroll_depth DECIMAL(5,2) CHECK (scroll_depth IS NULL OR (scroll_depth >= 0 AND scroll_depth <= 1)),
        created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        PRIMARY KEY (id, created_at)
    ) PARTITION BY RANGE (created_at);

    PERFORM tmp_create_monthly_partitions('page_views', start_m, end_m);

    copied := tmp_copy_retained_data('page_views_legacy', 'page_views', start_m, end_m);
    RAISE NOTICE 'copied % page_views rows', copied;

    CREATE INDEX idx_page_views_link ON page_views(link_id);
    CREATE INDEX idx_page_views_visitor ON page_views(visitor_id);
    CREATE INDEX idx_page_views_link_page ON page_views(link_id, page_number);
    CREATE INDEX idx_page_views_link_visitor_page_created ON page_views(link_id, visitor_id, page_number, created_at DESC);

    CREATE TRIGGER page_views_prevent_update
        BEFORE UPDATE OR DELETE ON page_views
        FOR EACH ROW EXECUTE FUNCTION prevent_event_mutation();

    -- security_events
    ALTER TABLE security_events RENAME TO security_events_legacy;
    DROP INDEX IF EXISTS idx_security_events_link;
    DROP INDEX IF EXISTS idx_security_events_event_type;
    DROP INDEX IF EXISTS idx_security_events_created_at;
    DROP INDEX IF EXISTS idx_security_events_ip;
    DROP INDEX IF EXISTS idx_security_events_tenant;
    DROP INDEX IF EXISTS idx_security_events_workspace;

    CREATE TABLE security_events (
        id UUID NOT NULL DEFAULT gen_random_uuid(),
        tenant_id UUID,
        workspace_id UUID,
        link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
        event_type TEXT NOT NULL CHECK (event_type IN (
            'security_gate_failed',
            'expired_link_accessed',
            'max_access_reached',
            'revoked_link_accessed',
            'abnormal_access_pattern',
            'access_rules_updated',
            'invite_token_failed',
            'invite_token_expired',
            'invite_token_revoked',
            'invite_token_redeemed',
            'invalid_password',
            'blocked_email',
            'blocked_domain',
            'allowed_email',
            'allowed_domain',
            'not_in_allow_list',
            'no_allow_match',
            'no_rules'
        )),
        visitor_id TEXT,
        email TEXT,
        ip INET,
        user_agent TEXT,
        reason TEXT,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        PRIMARY KEY (id, created_at)
    ) PARTITION BY RANGE (created_at);

    PERFORM tmp_create_monthly_partitions('security_events', start_m, end_m);

    -- Explicit column list because legacy security_events has tenant_id/workspace_id appended
    -- at the end by migration 054, while the new table places them before link_id.
    INSERT INTO security_events (
        id, tenant_id, workspace_id, link_id, event_type, visitor_id, email, ip, user_agent, reason, created_at
    )
    SELECT
        id, tenant_id, workspace_id, link_id, event_type, visitor_id, email, ip, user_agent, reason, created_at
    FROM security_events_legacy
    WHERE created_at >= start_m AND created_at < (end_m + interval '1 month')::date;
    GET DIAGNOSTICS copied = ROW_COUNT;
    RAISE NOTICE 'copied % security_events rows', copied;

    CREATE INDEX idx_security_events_link ON security_events(link_id);
    CREATE INDEX idx_security_events_event_type ON security_events(event_type);
    CREATE INDEX idx_security_events_created_at ON security_events(created_at);
    CREATE INDEX idx_security_events_ip ON security_events(ip);
    CREATE INDEX idx_security_events_tenant ON security_events(tenant_id);
    CREATE INDEX idx_security_events_workspace ON security_events(workspace_id);

    CREATE TRIGGER security_events_prevent_update
        BEFORE UPDATE OR DELETE ON security_events
        FOR EACH ROW EXECUTE FUNCTION prevent_event_mutation();
END $$;

DROP FUNCTION IF EXISTS tmp_create_monthly_partitions(text, date, date);
DROP FUNCTION IF EXISTS tmp_copy_retained_data(text, text, date, date);

COMMIT;

-- Expand email type enum to cover the new templated categories.
ALTER TABLE email_logs DROP CONSTRAINT email_logs_email_type_check;
ALTER TABLE email_logs ADD CONSTRAINT email_logs_email_type_check CHECK (email_type IN ('verification', 'access_code', 'marketing', 'custom'));

-- Email engagement events (opens and clicks) keyed to email_logs.
CREATE TABLE email_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_log_id UUID NOT NULL REFERENCES email_logs(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('open', 'click')),
    user_agent TEXT,
    ip_address TEXT,
    link_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_email_events_email_log_id ON email_events(email_log_id);
CREATE INDEX idx_email_events_event_type ON email_events(event_type);
CREATE INDEX idx_email_events_created_at ON email_events(created_at);

CREATE TABLE IF NOT EXISTS security_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'security_gate_failed',
        'expired_link_accessed',
        'max_access_reached',
        'revoked_link_accessed',
        'abnormal_access_pattern'
    )),
    visitor_id TEXT,
    email TEXT,
    ip INET,
    user_agent TEXT,
    reason TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_security_events_link ON security_events(link_id);
CREATE INDEX IF NOT EXISTS idx_security_events_event_type ON security_events(event_type);
CREATE INDEX IF NOT EXISTS idx_security_events_created_at ON security_events(created_at);
CREATE INDEX IF NOT EXISTS idx_security_events_ip ON security_events(ip);

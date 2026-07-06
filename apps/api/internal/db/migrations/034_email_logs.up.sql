CREATE TABLE email_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient TEXT NOT NULL,
    email_type TEXT NOT NULL CHECK (email_type IN ('verification', 'access_code')),
    provider TEXT NOT NULL CHECK (provider IN ('resend', 'smtp', 'log')),
    provider_message_id TEXT,
    status TEXT NOT NULL CHECK (status IN ('queued', 'sent', 'delivered', 'bounced', 'complained', 'failed')),
    subject TEXT NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_email_logs_recipient ON email_logs(recipient);
CREATE INDEX idx_email_logs_status ON email_logs(status);
CREATE INDEX idx_email_logs_provider_message_id ON email_logs(provider_message_id);
CREATE INDEX idx_email_logs_created_at ON email_logs(created_at);

ALTER TABLE link_contacts
    ADD COLUMN IF NOT EXISTS code_send_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (code_send_status IN ('pending', 'sent', 'failed')),
    ADD COLUMN IF NOT EXISTS code_send_error TEXT;

CREATE INDEX IF NOT EXISTS idx_link_contacts_code_send_status
    ON link_contacts(code_send_status);

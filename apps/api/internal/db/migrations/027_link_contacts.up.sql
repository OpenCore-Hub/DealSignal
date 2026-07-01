CREATE TABLE IF NOT EXISTS link_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    access_code TEXT NOT NULL,
    code_sent_at TIMESTAMPTZ DEFAULT now(),
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(link_id, contact_id)
);

CREATE INDEX IF NOT EXISTS idx_link_contacts_link ON link_contacts(link_id);
CREATE INDEX IF NOT EXISTS idx_link_contacts_contact ON link_contacts(contact_id);

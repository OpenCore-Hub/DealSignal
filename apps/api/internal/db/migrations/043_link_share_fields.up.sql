-- Migration: additional link sharing fields (domain, tags, notify on access).

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS custom_domain TEXT,
    ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS notify_on_access BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_links_custom_domain ON links(custom_domain);

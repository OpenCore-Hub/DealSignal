-- Ask Docs DD visitor checklist chips (P2c); link-level, default off.

ALTER TABLE links
  ADD COLUMN IF NOT EXISTS ask_docs_dd_chips_enabled boolean NOT NULL DEFAULT false;

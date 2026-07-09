-- Migration: real backend columns for placeholder switches in AccessTab.
-- qa_enabled       - controls the Visitor Q&A panel (SHORT-008)
-- file_requests_enabled - controls inbound file requests (SHORT-009)

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS qa_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS file_requests_enabled BOOLEAN NOT NULL DEFAULT false;

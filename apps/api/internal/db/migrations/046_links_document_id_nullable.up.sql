-- Migration: allow document_id to be NULL so deal-room links can exist without
-- an associated document.
ALTER TABLE links
    ALTER COLUMN document_id DROP NOT NULL;

-- Revert: restore NOT NULL on links.document_id.
-- This will fail if any NULL values exist.
ALTER TABLE links
    ALTER COLUMN document_id SET NOT NULL;

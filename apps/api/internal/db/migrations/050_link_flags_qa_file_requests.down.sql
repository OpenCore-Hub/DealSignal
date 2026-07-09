ALTER TABLE links
    DROP COLUMN IF EXISTS qa_enabled,
    DROP COLUMN IF EXISTS file_requests_enabled;

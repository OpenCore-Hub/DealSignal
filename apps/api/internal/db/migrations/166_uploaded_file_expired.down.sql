DROP INDEX IF EXISTS idx_link_uploaded_files_pending_created_at;

UPDATE link_uploaded_files SET status = 'rejected' WHERE status = 'expired';

ALTER TABLE link_uploaded_files DROP CONSTRAINT IF EXISTS link_uploaded_files_status_check;
ALTER TABLE link_uploaded_files
    ADD CONSTRAINT link_uploaded_files_status_check
    CHECK (status IN ('pending_review', 'approved', 'rejected'));

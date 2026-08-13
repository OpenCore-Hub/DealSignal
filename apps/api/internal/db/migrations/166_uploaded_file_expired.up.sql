-- Pending file-request uploads expire after TTL. Worker sets status=expired;
-- human reject stays rejected. Both delete the object first.

ALTER TABLE link_uploaded_files DROP CONSTRAINT IF EXISTS link_uploaded_files_status_check;
ALTER TABLE link_uploaded_files
    ADD CONSTRAINT link_uploaded_files_status_check
    CHECK (status IN ('pending_review', 'approved', 'rejected', 'expired'));

CREATE INDEX IF NOT EXISTS idx_link_uploaded_files_pending_created_at
    ON link_uploaded_files (created_at)
    WHERE status = 'pending_review';

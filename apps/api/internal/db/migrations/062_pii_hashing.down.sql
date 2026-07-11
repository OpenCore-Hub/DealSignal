-- Revert COMPLIANCE-001.
-- Note: IP columns cannot be safely converted back to INET once hashed/nullified,
-- so this down migration only removes the audit table and the lookup indexes.

DROP TABLE IF EXISTS compliance_audit_log;

DROP INDEX IF EXISTS idx_security_events_ip;
DROP INDEX IF EXISTS idx_access_logs_workspace_email;
DROP INDEX IF EXISTS idx_security_events_workspace_email;
DROP INDEX IF EXISTS idx_link_nda_agreements_workspace_email;
DROP INDEX IF EXISTS idx_room_nda_agreements_email;
DROP INDEX IF EXISTS idx_link_visitor_questions_email;
DROP INDEX IF EXISTS idx_link_file_requests_email;
DROP INDEX IF EXISTS idx_link_uploaded_files_uploader_email;
DROP INDEX IF EXISTS idx_contacts_workspace_email;

-- Recreate the original INET index; column type remains TEXT.
CREATE INDEX IF NOT EXISTS idx_security_events_ip ON security_events(ip);

-- Migration: PII minimization and data-subject rights support (COMPLIANCE-001).
--
-- 1. Create an audit log for compliance operations.
-- 2. Convert IP columns from INET to TEXT so HMAC hashes can be stored.
-- 3. Remove historical plaintext IPs.
-- 4. Add indexes to support export / anonymize / delete by visitor email.

CREATE TABLE IF NOT EXISTS compliance_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('export', 'anonymize', 'delete')),
    visitor_email TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_compliance_audit_log_workspace
    ON compliance_audit_log(workspace_id, created_at DESC);

-- The existing IP index references an INET column; drop before altering type.
DROP INDEX IF EXISTS idx_security_events_ip;

-- Convert IP columns to TEXT and nullify historical plaintext values.
ALTER TABLE access_logs ALTER COLUMN ip TYPE TEXT USING NULL;
ALTER TABLE security_events ALTER COLUMN ip TYPE TEXT USING NULL;
ALTER TABLE link_nda_agreements ALTER COLUMN ip TYPE TEXT USING NULL;
ALTER TABLE room_nda_agreements ALTER COLUMN ip TYPE TEXT USING NULL;
ALTER TABLE link_uploaded_files ALTER COLUMN uploader_ip TYPE TEXT USING NULL;

-- Recreate a hash-friendly index for security anomaly checks.
CREATE INDEX IF NOT EXISTS idx_security_events_ip ON security_events(ip);

-- Indexes to speed up data-subject operations.
CREATE INDEX IF NOT EXISTS idx_access_logs_workspace_email ON access_logs(workspace_id, visitor_email);
CREATE INDEX IF NOT EXISTS idx_security_events_workspace_email ON security_events(workspace_id, email);
CREATE INDEX IF NOT EXISTS idx_link_nda_agreements_workspace_email ON link_nda_agreements(workspace_id, email);
CREATE INDEX IF NOT EXISTS idx_room_nda_agreements_email ON room_nda_agreements(email);
CREATE INDEX IF NOT EXISTS idx_link_visitor_questions_email ON link_visitor_questions(workspace_id, visitor_email);
CREATE INDEX IF NOT EXISTS idx_link_file_requests_email ON link_file_requests(workspace_id, visitor_email);
CREATE INDEX IF NOT EXISTS idx_link_uploaded_files_uploader_email ON link_uploaded_files(workspace_id, uploader_email);
CREATE INDEX IF NOT EXISTS idx_contacts_workspace_email ON contacts(workspace_id, email);

ALTER TABLE email_logs ADD COLUMN workspace_id UUID;
CREATE INDEX idx_email_logs_workspace_id ON email_logs(workspace_id);

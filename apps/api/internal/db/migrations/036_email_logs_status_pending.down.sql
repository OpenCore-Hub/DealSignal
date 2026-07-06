ALTER TABLE email_logs DROP CONSTRAINT email_logs_status_check;
ALTER TABLE email_logs ADD CONSTRAINT email_logs_status_check CHECK (status IN ('queued', 'sent', 'delivered', 'bounced', 'complained', 'failed'));

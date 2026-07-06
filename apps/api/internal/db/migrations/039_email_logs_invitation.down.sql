ALTER TABLE email_logs DROP CONSTRAINT email_logs_email_type_check;
ALTER TABLE email_logs ADD CONSTRAINT email_logs_email_type_check CHECK (email_type IN ('verification', 'access_code', 'marketing', 'custom'));

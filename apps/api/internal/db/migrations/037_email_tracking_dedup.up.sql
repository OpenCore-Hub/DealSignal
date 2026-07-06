CREATE UNIQUE INDEX idx_email_events_dedup
ON email_events (email_log_id, event_type, coalesce(ip_address, ''), date_trunc('hour', created_at AT TIME ZONE 'UTC'));

DROP INDEX IF EXISTS idx_link_contacts_code_send_status;
ALTER TABLE link_contacts
    DROP COLUMN IF EXISTS code_send_error,
    DROP COLUMN IF EXISTS code_send_status;

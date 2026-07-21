-- Persist applicant display name from access requests so approval can
-- create a workspace contact with email + name.
ALTER TABLE link_access_requests
    ADD COLUMN IF NOT EXISTS signer_name TEXT;

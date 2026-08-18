-- Stamp the bound agreement hash on member NDA signatures so a later
-- template swap does not erase what was signed.
ALTER TABLE room_nda_agreements
    ADD COLUMN IF NOT EXISTS content_sha256 TEXT NOT NULL DEFAULT '';

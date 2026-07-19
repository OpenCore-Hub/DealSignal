ALTER TABLE links
    ADD COLUMN IF NOT EXISTS nda_document_id UUID;

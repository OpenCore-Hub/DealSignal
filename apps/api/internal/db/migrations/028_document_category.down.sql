ALTER TABLE documents DROP CONSTRAINT IF EXISTS chk_documents_category;
ALTER TABLE documents DROP COLUMN IF EXISTS category;

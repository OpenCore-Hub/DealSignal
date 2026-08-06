UPDATE documents
SET category = 'general', updated_at = now()
WHERE category = 'deal_room';

ALTER TABLE documents DROP CONSTRAINT IF EXISTS chk_documents_category;
ALTER TABLE documents ADD CONSTRAINT chk_documents_category
    CHECK (category IN ('general', 'agreement'));

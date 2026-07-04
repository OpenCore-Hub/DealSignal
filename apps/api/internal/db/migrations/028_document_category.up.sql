ALTER TABLE documents ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'general';
ALTER TABLE documents ADD CONSTRAINT chk_documents_category CHECK (category IN ('general', 'agreement'));
CREATE INDEX IF NOT EXISTS idx_documents_category ON documents(workspace_id, category);

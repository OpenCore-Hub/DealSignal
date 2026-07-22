-- Ask Docs audit projection fields on assistant message turns (Q6).
ALTER TABLE assistant_messages
  ADD COLUMN IF NOT EXISTS result_status TEXT,
  ADD COLUMN IF NOT EXISTS authorized_document_ids UUID[] NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS retrieval_document_ids UUID[] NOT NULL DEFAULT '{}';

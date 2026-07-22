ALTER TABLE assistant_messages
  DROP COLUMN IF EXISTS result_status,
  DROP COLUMN IF EXISTS authorized_document_ids,
  DROP COLUMN IF EXISTS retrieval_document_ids;

-- Ask Docs assistant_messages audit fields (removed in 106). No-op when table absent.
DO $$
BEGIN
  IF to_regclass('public.assistant_messages') IS NULL THEN
    RETURN;
  END IF;

  ALTER TABLE assistant_messages
    ADD COLUMN IF NOT EXISTS result_status TEXT,
    ADD COLUMN IF NOT EXISTS authorized_document_ids UUID[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS retrieval_document_ids UUID[] NOT NULL DEFAULT '{}';
END $$;

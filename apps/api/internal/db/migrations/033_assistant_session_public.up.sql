-- Ask Docs assistant_sessions (removed in 106). No-op when table absent.
DO $$
BEGIN
  IF to_regclass('public.assistant_sessions') IS NULL THEN
    RETURN;
  END IF;

  ALTER TABLE assistant_sessions
      ALTER COLUMN user_id DROP NOT NULL,
      ADD COLUMN IF NOT EXISTS visitor_id TEXT;

  CREATE INDEX IF NOT EXISTS idx_assistant_sessions_link_visitor
      ON assistant_sessions(link_id, visitor_id);
END $$;

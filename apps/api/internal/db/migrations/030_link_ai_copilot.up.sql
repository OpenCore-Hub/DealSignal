ALTER TABLE links ADD COLUMN IF NOT EXISTS ai_copilot_enabled boolean NOT NULL DEFAULT false;

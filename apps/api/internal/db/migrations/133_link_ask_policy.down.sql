ALTER TABLE links
    DROP COLUMN IF EXISTS ask_ai_monthly_quota,
    DROP COLUMN IF EXISTS ask_ai_enabled,
    DROP COLUMN IF EXISTS ask_mode;

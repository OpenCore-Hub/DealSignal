-- Temporary snooze for Insights suggestions (radar actions already support snoozed status).
-- snoozed_until hides the suggestion from active lists until the timestamp; does not dismiss.

ALTER TABLE suggestions
    ADD COLUMN IF NOT EXISTS snoozed_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_suggestions_workspace_snooze
    ON suggestions (workspace_id, snoozed_until)
    WHERE dismissed = false;

-- Allow feedback calibration to record snooze (distinct from permanent dismiss).
ALTER TABLE suggestion_feedback DROP CONSTRAINT IF EXISTS suggestion_feedback_feedback_type_check;
ALTER TABLE suggestion_feedback
    ADD CONSTRAINT suggestion_feedback_feedback_type_check
    CHECK (feedback_type IN ('dismissed', 'acted', 'spam', 'snoozed'));

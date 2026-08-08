ALTER TABLE suggestion_feedback DROP CONSTRAINT IF EXISTS suggestion_feedback_feedback_type_check;
ALTER TABLE suggestion_feedback
    ADD CONSTRAINT suggestion_feedback_feedback_type_check
    CHECK (feedback_type IN ('dismissed', 'acted', 'spam'));

DROP INDEX IF EXISTS idx_suggestions_workspace_snooze;

ALTER TABLE suggestions DROP COLUMN IF EXISTS snoozed_until;

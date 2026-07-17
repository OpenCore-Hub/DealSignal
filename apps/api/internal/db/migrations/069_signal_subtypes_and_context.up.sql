-- Add stable subtypes, rule metadata, and trigger context to suggestions and signals.

ALTER TABLE suggestions
    ADD COLUMN IF NOT EXISTS subtype TEXT,
    ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS context   JSONB DEFAULT '{}'::jsonb;

ALTER TABLE suggestions
    ADD CONSTRAINT chk_suggestions_subtype
        CHECK (subtype IS NULL OR subtype IN (
            'hot','revisit','download','question',
            'bounce','expired','access_exhausted','access_revoked',
            'blocked_attempt','anomaly','forward'
        ));

CREATE INDEX IF NOT EXISTS idx_suggestions_subtype ON suggestions(workspace_id, subtype);

ALTER TABLE signals
    ADD COLUMN IF NOT EXISTS subtype TEXT,
    ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS context   JSONB DEFAULT '{}'::jsonb;

ALTER TABLE signals
    ADD CONSTRAINT chk_signals_subtype
        CHECK (subtype IS NULL OR subtype IN (
            'hot','revisit','download','question',
            'bounce','expired','access_exhausted','access_revoked',
            'blocked_attempt','anomaly','forward'
        ));

CREATE INDEX IF NOT EXISTS idx_signals_subtype ON signals(workspace_id, subtype);

-- Backfill legacy risk_alert rows using the same heuristics previously in analytics/handler.go.
-- Only signals has a title column; suggestions does not, so only backfill signals here.
UPDATE signals
SET subtype = CASE
    WHEN type != 'risk_alert' THEN NULL
    WHEN title ILIKE '%download%' THEN 'download'
    WHEN title ILIKE '%expir%' THEN 'expired'
    WHEN title ILIKE '%anomal%' OR title ILIKE '%suspicious%'
         OR title ILIKE '%unusual%' OR title ILIKE '%spike%' THEN 'anomaly'
    ELSE 'forward'
END
WHERE subtype IS NULL AND type = 'risk_alert';

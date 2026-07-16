DROP INDEX IF EXISTS idx_suggestions_subtype;

ALTER TABLE suggestions
    DROP CONSTRAINT IF EXISTS chk_suggestions_subtype,
    DROP COLUMN IF EXISTS subtype,
    DROP COLUMN IF EXISTS metadata,
    DROP COLUMN IF EXISTS context;

DROP INDEX IF EXISTS idx_signals_subtype;

ALTER TABLE signals
    DROP CONSTRAINT IF EXISTS chk_signals_subtype,
    DROP COLUMN IF EXISTS subtype,
    DROP COLUMN IF EXISTS metadata,
    DROP COLUMN IF EXISTS context;

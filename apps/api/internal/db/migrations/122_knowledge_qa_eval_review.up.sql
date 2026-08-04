-- Ceiling Phase O: feedback→gold review loop (snapshot + human accept/reject).

ALTER TABLE knowledge_qa_eval_candidates
    ADD COLUMN IF NOT EXISTS snapshot JSONB,
    ADD COLUMN IF NOT EXISTS corpus_fingerprint TEXT,
    ADD COLUMN IF NOT EXISTS review_status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS expect TEXT,
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'knowledge_qa_eval_candidates_review_status_check'
    ) THEN
        ALTER TABLE knowledge_qa_eval_candidates
            ADD CONSTRAINT knowledge_qa_eval_candidates_review_status_check
            CHECK (review_status IN ('pending', 'accepted', 'rejected'));
    END IF;
END $$;

-- One candidate row per turn×kind (re-feedback refreshes snapshot, resets to pending).
CREATE UNIQUE INDEX IF NOT EXISTS uq_knowledge_qa_eval_candidates_turn_kind
    ON knowledge_qa_eval_candidates (turn_id, feedback_kind);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_eval_candidates_review
    ON knowledge_qa_eval_candidates (workspace_id, review_status, created_at DESC);

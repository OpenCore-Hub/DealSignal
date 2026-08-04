DROP INDEX IF EXISTS idx_knowledge_qa_eval_candidates_review;
DROP INDEX IF EXISTS uq_knowledge_qa_eval_candidates_turn_kind;

ALTER TABLE knowledge_qa_eval_candidates
    DROP CONSTRAINT IF EXISTS knowledge_qa_eval_candidates_review_status_check;

ALTER TABLE knowledge_qa_eval_candidates
    DROP COLUMN IF EXISTS reviewed_by,
    DROP COLUMN IF EXISTS reviewed_at,
    DROP COLUMN IF EXISTS expect,
    DROP COLUMN IF EXISTS review_status,
    DROP COLUMN IF EXISTS corpus_fingerprint,
    DROP COLUMN IF EXISTS snapshot;

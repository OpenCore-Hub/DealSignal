-- Per-user feedback on knowledge Q&A audit turns (Phase C).

CREATE TABLE IF NOT EXISTS knowledge_qa_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    turn_id UUID NOT NULL REFERENCES knowledge_qa_turns(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL
        CHECK (kind IN ('helpful', 'wrong_citation', 'not_answering')),
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (turn_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_feedback_turn
    ON knowledge_qa_feedback (turn_id);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_feedback_user
    ON knowledge_qa_feedback (user_id, updated_at DESC);

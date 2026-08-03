-- Room-scoped knowledge Q&A sessions / turns (audit timeline).
-- Vectors stay in docling-rag; DealSignal persists questions, answers, hit snapshots.

CREATE TABLE IF NOT EXISTS knowledge_qa_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    title TEXT,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_turn_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_sessions_room_last_turn
    ON knowledge_qa_sessions (room_id, last_turn_at DESC NULLS LAST);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_sessions_room_status
    ON knowledge_qa_sessions (room_id, status);

CREATE TABLE IF NOT EXISTS knowledge_qa_turns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES knowledge_qa_sessions(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    sequence INT NOT NULL,
    question TEXT NOT NULL,
    answer TEXT,
    refused BOOLEAN NOT NULL DEFAULT false,
    result_status TEXT NOT NULL
        CHECK (result_status IN ('answered', 'refused', 'no_hits', 'error')),
    corpus_status_snapshot JSONB,
    hits JSONB NOT NULL DEFAULT '[]'::jsonb,
    mode TEXT,
    top_k INT,
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (session_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_turns_session_seq
    ON knowledge_qa_turns (session_id, sequence);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_turns_room_created
    ON knowledge_qa_turns (room_id, created_at DESC);

-- Ceiling Phase H: corpus fingerprint + turn latency + cold archive tombstones.

ALTER TABLE knowledge_qa_turns
    ADD COLUMN IF NOT EXISTS corpus_fingerprint TEXT;

ALTER TABLE knowledge_qa_turns
    ADD COLUMN IF NOT EXISTS duration_ms INT NOT NULL DEFAULT 0;

COMMENT ON COLUMN knowledge_qa_turns.corpus_fingerprint IS
    'Stable sha256 of room RAG doc sync generation at ask time (ceiling §3.6)';
COMMENT ON COLUMN knowledge_qa_turns.duration_ms IS
    'End-to-end ask latency in milliseconds (retrieve + audit write)';

CREATE TABLE IF NOT EXISTS knowledge_qa_session_archives (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    session_id UUID NOT NULL,
    title TEXT,
    storage_key TEXT NOT NULL,
    turn_count INT NOT NULL DEFAULT 0,
    corpus_fingerprint TEXT,
    status TEXT NOT NULL DEFAULT 'cold'
        CHECK (status IN ('cold', 'restored_readonly')),
    archived_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_qa_session_archives_session
    ON knowledge_qa_session_archives (session_id);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_session_archives_room
    ON knowledge_qa_session_archives (room_id, archived_at DESC);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_session_archives_workspace
    ON knowledge_qa_session_archives (workspace_id, archived_at DESC);

COMMENT ON TABLE knowledge_qa_session_archives IS
    'Cold-archive tombstones: diligence JSON pack in object storage; hot session rows purged';

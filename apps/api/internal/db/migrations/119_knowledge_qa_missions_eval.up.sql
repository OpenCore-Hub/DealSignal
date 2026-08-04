-- Ceiling Phase G: room mission pack binding + feedback→eval candidate pipeline.

CREATE TABLE IF NOT EXISTS knowledge_qa_room_missions (
    room_id UUID PRIMARY KEY REFERENCES deal_rooms(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    pack_id TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_room_missions_workspace
    ON knowledge_qa_room_missions (workspace_id);

CREATE TABLE IF NOT EXISTS knowledge_qa_eval_candidates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    turn_id UUID NOT NULL REFERENCES knowledge_qa_turns(id) ON DELETE CASCADE,
    feedback_kind TEXT NOT NULL
        CHECK (feedback_kind IN ('wrong_citation', 'not_answering')),
    question TEXT NOT NULL,
    answer TEXT,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_eval_candidates_created
    ON knowledge_qa_eval_candidates (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_eval_candidates_room
    ON knowledge_qa_eval_candidates (room_id, created_at DESC);

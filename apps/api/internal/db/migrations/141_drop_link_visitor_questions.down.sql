-- Recreate legacy table for rollback only; historical rows are not restored.

CREATE TABLE link_visitor_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    visitor_id TEXT NOT NULL,
    visitor_email TEXT,
    question TEXT NOT NULL,
    answer TEXT,
    answered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    intent_tag TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_link_visitor_questions_link_id ON link_visitor_questions(link_id);
CREATE INDEX idx_link_visitor_questions_visitor ON link_visitor_questions(visitor_id);

ALTER TABLE link_ask_turns
    ADD COLUMN IF NOT EXISTS host_question_id UUID REFERENCES link_visitor_questions(id) ON DELETE SET NULL;

CREATE INDEX idx_link_ask_turns_host_question ON link_ask_turns (host_question_id)
    WHERE host_question_id IS NOT NULL;

-- Unified visitor Ask timeline (Phase A): sessions + turns with host-lane dual-write
-- to link_visitor_questions for Inbox compatibility.

CREATE TABLE link_ask_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    visitor_id TEXT NOT NULL,
    visitor_email TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_link_ask_sessions_link_visitor ON link_ask_sessions (link_id, visitor_id);

CREATE TABLE link_ask_turns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES link_ask_sessions(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    visitor_id TEXT NOT NULL,
    question TEXT NOT NULL,
    lane TEXT NOT NULL CHECK (lane IN ('ai', 'host', 'hybrid')),
    status TEXT NOT NULL CHECK (status IN (
        'routing', 'ai_streaming', 'ai_answered', 'ai_refused',
        'host_pending', 'host_answered', 'failed'
    )),
    ai_payload JSONB,
    host_question_id UUID REFERENCES link_visitor_questions(id) ON DELETE SET NULL,
    host_answer TEXT,
    answered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    route_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_link_ask_turns_link_visitor ON link_ask_turns (link_id, visitor_id, created_at DESC);
CREATE INDEX idx_link_ask_turns_host_pending ON link_ask_turns (link_id, status)
    WHERE status = 'host_pending';
CREATE INDEX idx_link_ask_turns_host_question ON link_ask_turns (host_question_id)
    WHERE host_question_id IS NOT NULL;

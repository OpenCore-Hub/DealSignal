-- Ask Docs audit cold archive (B2 / US#28): sessions older than the 90-day hot window
-- are projected here, then removed from assistant_sessions (messages CASCADE).

CREATE TABLE IF NOT EXISTS ask_docs_audit_archives (
    session_id UUID PRIMARY KEY,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL,
    deal_room_id UUID,
    tenant_id UUID,
    visitor_id TEXT NOT NULL,
    question_preview TEXT NOT NULL DEFAULT '',
    result_status TEXT NOT NULL DEFAULT '',
    evidence_count INTEGER NOT NULL DEFAULT 0,
    question TEXT NOT NULL DEFAULT '',
    answer TEXT NOT NULL DEFAULT '',
    evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
    authorized_document_ids UUID[] NOT NULL DEFAULT '{}',
    retrieval_document_ids UUID[] NOT NULL DEFAULT '{}',
    messages JSONB NOT NULL DEFAULT '[]'::jsonb,
    session_created_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ask_docs_audit_archives_link_created
    ON ask_docs_audit_archives (link_id, session_created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ask_docs_audit_archives_room_created
    ON ask_docs_audit_archives (deal_room_id, session_created_at DESC)
    WHERE deal_room_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ask_docs_audit_archives_workspace_created
    ON ask_docs_audit_archives (workspace_id, session_created_at DESC);

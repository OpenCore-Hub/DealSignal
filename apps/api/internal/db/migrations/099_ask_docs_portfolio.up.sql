-- Ask Docs cross-room portfolio views (P3). Snapshot aggregation only — no cross-room RAG.

CREATE TABLE IF NOT EXISTS ask_docs_portfolio_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    pack_id TEXT NOT NULL DEFAULT 'financing_dd_v1',
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ask_docs_portfolio_views_name_nonempty CHECK (char_length(trim(name)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_ask_docs_portfolio_views_workspace
    ON ask_docs_portfolio_views (workspace_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS ask_docs_portfolio_view_rooms (
    view_id UUID NOT NULL REFERENCES ask_docs_portfolio_views(id) ON DELETE CASCADE,
    deal_room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    sort_order INT NOT NULL DEFAULT 0,
    PRIMARY KEY (view_id, deal_room_id)
);

CREATE INDEX IF NOT EXISTS idx_ask_docs_portfolio_view_rooms_room
    ON ask_docs_portfolio_view_rooms (deal_room_id);

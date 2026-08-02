-- Ask Docs DD room-level pack fork (P2.1c). Owner-editable copy of financing_dd_v1.

CREATE TABLE IF NOT EXISTS ask_docs_dd_room_packs (
    deal_room_id UUID PRIMARY KEY REFERENCES deal_rooms(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    base_pack_id TEXT NOT NULL,
    pack_version TEXT NOT NULL,
    fork_revision INT NOT NULL DEFAULT 1 CHECK (fork_revision >= 1),
    items JSONB NOT NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ask_docs_dd_room_packs_workspace
    ON ask_docs_dd_room_packs (workspace_id);

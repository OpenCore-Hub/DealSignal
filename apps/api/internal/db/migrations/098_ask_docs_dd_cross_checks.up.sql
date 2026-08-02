-- Ask Docs DD Owner dual-document cross-check (P2.2). Latest per room+pack.

CREATE TABLE IF NOT EXISTS ask_docs_dd_cross_checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    deal_room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    pack_id TEXT NOT NULL,
    pack_version TEXT NOT NULL,
    document_a_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    document_b_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    triggered_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    claims JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ask_docs_dd_cross_checks_docs_distinct CHECK (document_a_id <> document_b_id)
);

-- One latest cross-check per room+pack.
CREATE UNIQUE INDEX IF NOT EXISTS uq_ask_docs_dd_cross_checks_room_pack
    ON ask_docs_dd_cross_checks (deal_room_id, pack_id);

CREATE INDEX IF NOT EXISTS idx_ask_docs_dd_cross_checks_room_created
    ON ask_docs_dd_cross_checks (deal_room_id, created_at DESC);

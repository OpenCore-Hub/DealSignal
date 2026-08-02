-- Ask Docs DD Coverage snapshots + runs (P2 / financing_dd_v1 wedge).

CREATE TABLE IF NOT EXISTS ask_docs_dd_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    deal_room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    link_id UUID REFERENCES links(id) ON DELETE CASCADE,
    pack_id TEXT NOT NULL,
    pack_version TEXT NOT NULL,
    status TEXT NOT NULL
      CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    triggered_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    error_message TEXT NOT NULL DEFAULT '',
    kb_generation INT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ask_docs_dd_runs_room_created
    ON ask_docs_dd_runs (deal_room_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ask_docs_dd_runs_room_active
    ON ask_docs_dd_runs (deal_room_id, link_id, status)
    WHERE status IN ('queued', 'running');

CREATE TABLE IF NOT EXISTS ask_docs_dd_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    deal_room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    link_id UUID REFERENCES links(id) ON DELETE CASCADE,
    pack_id TEXT NOT NULL,
    pack_version TEXT NOT NULL,
    run_id UUID NOT NULL REFERENCES ask_docs_dd_runs(id) ON DELETE CASCADE,
    kb_generation INT,
    stale BOOLEAN NOT NULL DEFAULT false,
    coverage_rows JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One latest snapshot per room-scoped pack (link_id IS NULL).
CREATE UNIQUE INDEX IF NOT EXISTS uq_ask_docs_dd_snapshots_room_pack
    ON ask_docs_dd_snapshots (deal_room_id, pack_id)
    WHERE link_id IS NULL;

-- One latest snapshot per link-scoped pack.
CREATE UNIQUE INDEX IF NOT EXISTS uq_ask_docs_dd_snapshots_link_pack
    ON ask_docs_dd_snapshots (deal_room_id, link_id, pack_id)
    WHERE link_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ask_docs_dd_snapshots_room
    ON ask_docs_dd_snapshots (deal_room_id, updated_at DESC);

-- Tile metadata for each document page (SPEC Section 3.1)
CREATE TABLE document_page_tiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    document_version_id UUID NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
    page_number INTEGER NOT NULL CHECK (page_number > 0),
    zoom_level INTEGER NOT NULL DEFAULT 1,
    tile_size_px INTEGER NOT NULL DEFAULT 512,
    cols INTEGER NOT NULL,
    rows INTEGER NOT NULL,
    tile_manifest JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (document_version_id, page_number, zoom_level)
);

CREATE INDEX idx_document_page_tiles_version_page ON document_page_tiles(document_version_id, page_number);

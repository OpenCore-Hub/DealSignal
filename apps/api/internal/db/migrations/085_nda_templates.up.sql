-- Workspace-scoped NDA agreement templates (One-Click signing).
CREATE TABLE IF NOT EXISTS nda_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    source_document_id UUID NOT NULL REFERENCES documents(id) ON DELETE RESTRICT,
    content_sha256 TEXT NOT NULL DEFAULT '',
    require_signer_name BOOLEAN NOT NULL DEFAULT true,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, source_document_id)
);

CREATE INDEX IF NOT EXISTS idx_nda_templates_workspace
    ON nda_templates(workspace_id) WHERE status = 'active';

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS nda_template_id UUID REFERENCES nda_templates(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_links_nda_template_id ON links(nda_template_id)
    WHERE nda_template_id IS NOT NULL;

-- Enrich agreement responses for One-Click audit evidence.
ALTER TABLE link_nda_agreements
    ADD COLUMN IF NOT EXISTS nda_template_id UUID REFERENCES nda_templates(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS content_sha256 TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS signer_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS certificate_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS signed_file_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'signed';

CREATE UNIQUE INDEX IF NOT EXISTS idx_link_nda_agreements_certificate
    ON link_nda_agreements(certificate_id) WHERE certificate_id <> '';

CREATE INDEX IF NOT EXISTS idx_link_nda_agreements_template
    ON link_nda_agreements(nda_template_id) WHERE nda_template_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_link_nda_agreements_link_visitor_template
    ON link_nda_agreements(link_id, visitor_id, nda_template_id)
    WHERE visitor_id IS NOT NULL AND visitor_id <> '' AND nda_template_id IS NOT NULL;

-- Backfill templates from existing link NDA document bindings.
INSERT INTO nda_templates (
    tenant_id, workspace_id, name, source_document_id, content_sha256, created_by
)
SELECT DISTINCT ON (l.workspace_id, l.nda_document_id)
    l.tenant_id,
    l.workspace_id,
    COALESCE(NULLIF(d.title, ''), 'NDA Agreement'),
    l.nda_document_id,
    '',
    l.created_by
FROM links l
JOIN documents d ON d.id = l.nda_document_id
WHERE l.nda_document_id IS NOT NULL
ON CONFLICT (workspace_id, source_document_id) DO NOTHING;

UPDATE links l
SET nda_template_id = t.id
FROM nda_templates t
WHERE l.nda_document_id IS NOT NULL
  AND t.workspace_id = l.workspace_id
  AND t.source_document_id = l.nda_document_id
  AND l.nda_template_id IS NULL;

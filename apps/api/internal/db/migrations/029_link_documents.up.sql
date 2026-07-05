-- 029: Create link_documents join table for multi-document link bundles.
-- Each link can reference multiple documents. sort_order controls display order
-- on the recipient view. link_id + document_id is unique.

CREATE TABLE link_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id),
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(link_id, document_id)
);

CREATE INDEX idx_link_documents_link_id ON link_documents(link_id);
CREATE INDEX idx_link_documents_document_id ON link_documents(document_id);

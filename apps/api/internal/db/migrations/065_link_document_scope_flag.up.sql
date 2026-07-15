ALTER TABLE links ADD COLUMN has_document_scope BOOLEAN NOT NULL DEFAULT false;

-- Backfill existing deal-room links that already have an explicit document scope.
UPDATE links
SET has_document_scope = true
WHERE deal_room_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM link_documents WHERE link_documents.link_id = links.id);

CREATE INDEX idx_links_has_document_scope ON links(has_document_scope);

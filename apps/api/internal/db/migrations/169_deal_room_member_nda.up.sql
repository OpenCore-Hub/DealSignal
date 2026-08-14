-- Room-level NDA agreement for invited members (distinct from share-link NDA floor).
ALTER TABLE deal_rooms
    ADD COLUMN IF NOT EXISTS nda_template_id UUID REFERENCES nda_templates(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS nda_document_id UUID REFERENCES documents(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_deal_rooms_nda_template
    ON deal_rooms(nda_template_id) WHERE nda_template_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_deal_rooms_nda_document
    ON deal_rooms(nda_document_id) WHERE nda_document_id IS NOT NULL;

ALTER TABLE room_nda_agreements
    ADD COLUMN IF NOT EXISTS nda_template_id UUID REFERENCES nda_templates(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_room_nda_agreements_template
    ON room_nda_agreements(nda_template_id) WHERE nda_template_id IS NOT NULL;

DROP INDEX IF EXISTS idx_room_nda_agreements_template;
ALTER TABLE room_nda_agreements DROP COLUMN IF EXISTS nda_template_id;

DROP INDEX IF EXISTS idx_deal_rooms_nda_document;
DROP INDEX IF EXISTS idx_deal_rooms_nda_template;
ALTER TABLE deal_rooms
    DROP COLUMN IF EXISTS nda_document_id,
    DROP COLUMN IF EXISTS nda_template_id;

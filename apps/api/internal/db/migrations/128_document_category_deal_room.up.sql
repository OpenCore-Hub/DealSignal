-- Tri-state document category: general | agreement | deal_room.
-- Membership (deal_room_documents) still places a doc in a room/folder;
-- category is the library partition + lifecycle truth source.

ALTER TABLE documents DROP CONSTRAINT IF EXISTS chk_documents_category;
ALTER TABLE documents ADD CONSTRAINT chk_documents_category
    CHECK (category IN ('general', 'agreement', 'deal_room'));

-- Heal: library docs already attached to any deal room become deal_room.
-- Agreements already in rooms keep category=agreement (new attaches are blocked in app).
UPDATE documents d
SET category = 'deal_room', updated_at = now()
WHERE d.deleted_at IS NULL
  AND d.category = 'general'
  AND EXISTS (
      SELECT 1
      FROM deal_room_documents drd
      WHERE drd.document_id = d.id
  );

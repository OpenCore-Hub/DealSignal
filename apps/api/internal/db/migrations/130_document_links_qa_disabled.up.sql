-- Document-only share links: disable visitor Q&A until unified Visitor Ask ships.
-- Deal-room share links keep qa_enabled = true.

UPDATE links
SET qa_enabled = false
WHERE deal_room_id IS NULL;

ALTER TABLE links
    ALTER COLUMN qa_enabled SET DEFAULT false;

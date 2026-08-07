-- Deal-room share links default to AI lane; runtime routing still requires synced KB corpus.
UPDATE links
SET ask_ai_enabled = true
WHERE deal_room_id IS NOT NULL
  AND qa_enabled = true
  AND ask_ai_enabled = false;

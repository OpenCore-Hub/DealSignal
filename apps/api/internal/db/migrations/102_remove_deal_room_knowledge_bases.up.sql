-- Disable Ask Docs on deal-room share links, then drop the Knowledge Base product tables/columns.

UPDATE links
SET ai_copilot_enabled = false,
    ask_docs_dd_chips_enabled = false
WHERE deal_room_id IS NOT NULL
  AND (ai_copilot_enabled = true OR ask_docs_dd_chips_enabled = true);

DROP TABLE IF EXISTS deal_room_knowledge_bases;

ALTER TABLE ask_docs_dd_runs DROP COLUMN IF EXISTS kb_generation;
ALTER TABLE ask_docs_dd_snapshots DROP COLUMN IF EXISTS kb_generation;

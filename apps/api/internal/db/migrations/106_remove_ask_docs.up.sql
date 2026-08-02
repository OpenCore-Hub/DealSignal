-- Remove Ask Docs chat/audit and Diligence (DD coverage / portfolio) schema.

DROP TABLE IF EXISTS ask_docs_portfolio_view_rooms;
DROP TABLE IF EXISTS ask_docs_portfolio_views;
DROP TABLE IF EXISTS ask_docs_dd_cross_checks;
DROP TABLE IF EXISTS ask_docs_dd_room_packs;
DROP TABLE IF EXISTS ask_docs_dd_snapshots;
DROP TABLE IF EXISTS ask_docs_dd_runs;

DROP TABLE IF EXISTS ask_docs_audit_archives;
DROP TABLE IF EXISTS assistant_messages;
DROP TABLE IF EXISTS assistant_sessions;

ALTER TABLE links DROP COLUMN IF EXISTS ask_docs_dd_chips_enabled;
ALTER TABLE links DROP COLUMN IF EXISTS ai_copilot_enabled;

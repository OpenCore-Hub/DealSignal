-- Turns-only Ask: drop legacy link_visitor_questions dual-write artifacts.

DROP INDEX IF EXISTS idx_link_ask_turns_host_question_unique;
DROP INDEX IF EXISTS idx_link_ask_turns_host_question;

ALTER TABLE link_ask_turns
    DROP COLUMN IF EXISTS host_question_id;

DROP INDEX IF EXISTS idx_link_visitor_questions_workspace_status_created;
DROP INDEX IF EXISTS idx_link_visitor_questions_email;
DROP INDEX IF EXISTS idx_link_visitor_questions_link_id;
DROP INDEX IF EXISTS idx_link_visitor_questions_visitor;

DROP TABLE IF EXISTS link_visitor_questions;

-- Backfill unified Ask sessions/turns from pre-Phase-A link_visitor_questions.

INSERT INTO link_ask_sessions (tenant_id, workspace_id, link_id, visitor_id, visitor_email, created_at, updated_at)
SELECT
    q.tenant_id,
    q.workspace_id,
    q.link_id,
    q.visitor_id,
    q.visitor_email,
    MIN(q.created_at),
    MAX(GREATEST(q.updated_at, q.created_at))
FROM link_visitor_questions q
WHERE NOT EXISTS (
    SELECT 1
    FROM link_ask_sessions s
    WHERE s.link_id = q.link_id
      AND s.visitor_id = q.visitor_id
)
GROUP BY q.tenant_id, q.workspace_id, q.link_id, q.visitor_id, q.visitor_email;

INSERT INTO link_ask_turns (
    session_id,
    tenant_id,
    workspace_id,
    link_id,
    visitor_id,
    question,
    lane,
    status,
    host_question_id,
    host_answer,
    answered_by,
    route_reason,
    created_at,
    updated_at
)
SELECT
    s.id,
    q.tenant_id,
    q.workspace_id,
    q.link_id,
    q.visitor_id,
    q.question,
    'host',
    CASE WHEN q.status = 'answered' THEN 'host_answered' ELSE 'host_pending' END,
    q.id,
    q.answer,
    q.answered_by,
    'legacy_backfill',
    q.created_at,
    q.updated_at
FROM link_visitor_questions q
INNER JOIN link_ask_sessions s
    ON s.link_id = q.link_id
   AND s.visitor_id = q.visitor_id
WHERE NOT EXISTS (
    SELECT 1
    FROM link_ask_turns t
    WHERE t.host_question_id = q.id
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_link_ask_turns_host_question_unique
    ON link_ask_turns (host_question_id)
    WHERE host_question_id IS NOT NULL;

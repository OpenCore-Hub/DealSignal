-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1 LIMIT 1;

-- name: VerifyUserEmail :exec
UPDATE users
SET email_verified = TRUE
WHERE id = $1;

-- name: CreateTenant :one
INSERT INTO tenants (name, slug)
VALUES ($1, $2)
RETURNING id, name, slug, created_at;

-- name: CreateWorkspace :one
INSERT INTO workspaces (tenant_id, name, slug, brand_color)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetWorkspaceByID :one
SELECT * FROM workspaces WHERE id = $1 LIMIT 1;

-- name: GetWorkspaceBySlug :one
SELECT * FROM workspaces WHERE slug = $1 LIMIT 1;

-- name: UpdateWorkspace :one
UPDATE workspaces
SET name = $2, brand_color = $3
WHERE id = $1
RETURNING *;

-- name: UpdateWorkspaceSecurity :one
UPDATE workspaces
SET force_email_verification = $1, watermark_downloads = $2, two_factor_enabled = $3
WHERE id = $4
RETURNING *;

-- name: ListWorkspacesByUser :many
SELECT w.id, w.tenant_id, w.name, w.slug, w.brand_color, w.force_email_verification, w.watermark_downloads, w.two_factor_enabled, w.created_at, m.role
FROM workspaces w
JOIN workspace_members m ON m.workspace_id = w.id
WHERE m.user_id = $1
ORDER BY w.created_at DESC;

-- name: AddWorkspaceMember :one
INSERT INTO workspace_members (workspace_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING workspace_id, user_id, role, joined_at;

-- name: GetWorkspaceMember :one
SELECT workspace_id, user_id, role, joined_at
FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2 LIMIT 1;

-- name: ListWorkspaceMembers :many
SELECT
    wm.workspace_id,
    wm.user_id,
    wm.role,
    wm.joined_at,
    u.email
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id
WHERE wm.workspace_id = $1
ORDER BY wm.joined_at DESC;

-- name: CreateDocument :one
INSERT INTO documents (
    id, tenant_id, workspace_id, created_by, title, source_type, status, storage_key, file_size, category
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at;

-- name: GetDocumentByID :one
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL
LIMIT 1;

-- name: ListDocumentsByWorkspace :many
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: ListRecentlyAccessedDocumentsByWorkspace :many
SELECT
    d.id, d.tenant_id, d.workspace_id, d.created_by, COALESCE(d.title, ''::text) as title, d.source_type, d.status, d.storage_key, COALESCE(d.file_size, 0::bigint) as file_size, d.category, d.page_count, d.created_at, d.updated_at, d.deleted_at,
    COALESCE(MAX(al.created_at), d.created_at) as last_accessed_at
FROM documents d
LEFT JOIN links l ON l.document_id = d.id AND l.status = 'active'
LEFT JOIN access_logs al ON al.link_id = l.id
WHERE d.workspace_id = $1 AND d.deleted_at IS NULL AND d.status != 'archived'
GROUP BY d.id
HAVING MAX(al.created_at) IS NOT NULL
ORDER BY last_accessed_at DESC, d.created_at DESC;

-- name: ListPopularDocumentsByWorkspace :many
SELECT
    d.id, d.tenant_id, d.workspace_id, d.created_by, COALESCE(d.title, ''::text) as title, d.source_type, d.status, d.storage_key, COALESCE(d.file_size, 0::bigint) as file_size, d.category, d.page_count, d.created_at, d.updated_at, d.deleted_at,
    COALESCE(SUM(l.access_count), 0)::bigint as total_views
FROM documents d
LEFT JOIN links l ON l.document_id = d.id AND l.status = 'active'
WHERE d.workspace_id = $1 AND d.deleted_at IS NULL AND d.status != 'archived'
GROUP BY d.id
HAVING COALESCE(SUM(l.access_count), 0) >= 30
ORDER BY total_views DESC, d.created_at DESC;

-- name: ListUnsharedDocumentsByWorkspace :many
SELECT d.id, d.tenant_id, d.workspace_id, d.created_by, COALESCE(d.title, ''::text) as title, d.source_type, d.status, d.storage_key, COALESCE(d.file_size, 0::bigint) as file_size, d.category, d.page_count, d.created_at, d.updated_at, d.deleted_at
FROM documents d
WHERE d.workspace_id = $1 AND d.deleted_at IS NULL AND d.status != 'archived'
  AND NOT EXISTS (SELECT 1 FROM links l WHERE l.document_id = d.id AND l.status = 'active')
ORDER BY d.created_at DESC;

-- name: ListArchivedDocumentsByWorkspace :many
SELECT d.id, d.tenant_id, d.workspace_id, d.created_by, COALESCE(d.title, ''::text) as title, d.source_type, d.status, d.storage_key, COALESCE(d.file_size, 0::bigint) as file_size, d.category, d.page_count, d.created_at, d.updated_at, d.deleted_at
FROM documents d
WHERE d.workspace_id = $1 AND d.deleted_at IS NULL AND d.status = 'archived'
ORDER BY d.created_at DESC;

-- name: ListRecentDocumentsByWorkspace :many
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2;

-- name: UpdateDocumentStatus :exec
UPDATE documents
SET status = $1, page_count = $2, updated_at = now()
WHERE id = $3;

-- name: ArchiveDocument :exec
UPDATE documents
SET status = 'archived', updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND status = 'ready';

-- name: UnarchiveDocument :exec
UPDATE documents
SET status = 'ready', updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND status = 'archived';

-- name: SoftDeleteDocument :exec
UPDATE documents
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL;

-- name: CreateIngestionJob :one
INSERT INTO ingestion_jobs (tenant_id, workspace_id, document_id, status)
VALUES ($1, $2, $3, $4)
RETURNING id, tenant_id, workspace_id, document_id, status, attempts, error_message, created_at, updated_at;

-- name: GetIngestionJobByDocument :one
SELECT id, tenant_id, workspace_id, document_id, status, attempts, error_message, created_at, updated_at
FROM ingestion_jobs
WHERE document_id = $1
LIMIT 1;

-- name: ListPendingIngestionJobs :many
SELECT id, tenant_id, workspace_id, document_id, status, attempts, error_message, created_at, updated_at
FROM ingestion_jobs
WHERE status = 'queued'
   OR (status = 'failed' AND attempts < 3)
   OR (status = 'processing' AND updated_at < now() - interval '5 minutes')
ORDER BY created_at ASC
LIMIT $1;

-- name: UpdateIngestionJob :exec
UPDATE ingestion_jobs
SET status = $1, attempts = $2, error_message = $3, updated_at = now()
WHERE id = $4;

-- name: CreatePage :one
INSERT INTO pages (tenant_id, workspace_id, document_id, page_number, image_object_key, width, height, file_size, title)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, tenant_id, workspace_id, document_id, page_number, image_object_key, width, height, file_size, title, created_at;

-- name: ListPagesByDocument :many
SELECT id, tenant_id, workspace_id, document_id, page_number, image_object_key, width, height, created_at
FROM pages
WHERE document_id = $1
ORDER BY page_number;

-- name: GetPageByDocumentAndNumber :one
SELECT id, tenant_id, workspace_id, document_id, page_number, image_object_key, width, height, created_at
FROM pages
WHERE document_id = $1 AND page_number = $2
LIMIT 1;

-- name: CreateChunk :exec
INSERT INTO chunks (tenant_id, workspace_id, page_id, text, bbox)
VALUES ($1, $2, $3, $4, $5);

-- name: CreateChunkWithEmbedding :exec
INSERT INTO chunks (tenant_id, workspace_id, page_id, text, bbox, embedding, search_vector)
VALUES ($1, $2, $3, $4, $5, $6, to_tsvector('english', $4));

-- name: UpdateChunkSearchVector :exec
UPDATE chunks
SET search_vector = to_tsvector('english', text)
WHERE id = $1;

-- name: SearchChunksByVector :many
SELECT
    c.id,
    c.text,
    c.bbox,
    p.page_number,
    p.document_id,
    (c.embedding <=> sqlc.arg(embedding)::vector)::float8 AS distance
FROM chunks c
JOIN pages p ON p.id = c.page_id
WHERE c.workspace_id = $1
  AND c.embedding IS NOT NULL
ORDER BY c.embedding <=> sqlc.arg(embedding)::vector
LIMIT $2;

-- name: SearchChunksByText :many
SELECT
    c.id,
    c.text,
    c.bbox,
    p.page_number,
    p.document_id,
    ts_rank(c.search_vector, plainto_tsquery('english', sqlc.arg(query))) AS rank
FROM chunks c
JOIN pages p ON p.id = c.page_id
WHERE c.workspace_id = $1
  AND c.search_vector @@ plainto_tsquery('english', sqlc.arg(query))
ORDER BY rank DESC
LIMIT $2;

-- name: CreateAssistantSession :one
INSERT INTO assistant_sessions (workspace_id, user_id, link_id, document_id, visitor_id, title)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAssistantSession :one
SELECT *
FROM assistant_sessions
WHERE id = $1 AND workspace_id = $2 AND user_id = $3
LIMIT 1;

-- name: GetAssistantSessionByLinkAndVisitor :one
SELECT *
FROM assistant_sessions
WHERE link_id = $1 AND visitor_id = $2
ORDER BY updated_at DESC
LIMIT 1;

-- name: GetAssistantSessionByIDForPublic :one
SELECT *
FROM assistant_sessions
WHERE id = $1 AND link_id = $2 AND visitor_id = $3
LIMIT 1;

-- name: UpdateAssistantSessionTitle :exec
UPDATE assistant_sessions
SET title = $1, updated_at = now()
WHERE id = $2;

-- name: CreateAssistantMessage :one
INSERT INTO assistant_messages (session_id, role, content, evidence)
VALUES ($1, $2, $3, $4)
RETURNING id, session_id, role, content, evidence, created_at;

-- name: ListAssistantMessagesBySession :many
SELECT id, session_id, role, content, evidence, created_at
FROM assistant_messages
WHERE session_id = $1
ORDER BY created_at ASC
LIMIT $2;
-- name: CreateLink :one
INSERT INTO links (
    tenant_id, workspace_id, document_id, deal_room_id, public_token, name, permission_type, expires_at, max_access_count,
    download_enabled, watermark_enabled, status, created_by,
    require_email, require_nda, require_email_verification,
    ai_copilot_enabled, require_password, password_hash,
    qa_enabled, file_requests_enabled, index_file_enabled, screenshot_protection_enabled,
    link_type, target_folder_path,
    custom_domain, tags, notify_on_access,
    has_document_scope, folder_scope_paths
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30)
RETURNING *;

-- name: GetLinkByIDAndWorkspace :one
SELECT *
FROM links
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: GetLinkByID :one
SELECT *
FROM links
WHERE id = $1
LIMIT 1;

-- name: GetLinkByPublicToken :one
SELECT *
FROM links
WHERE public_token = $1
LIMIT 1;

-- name: IncrementLinkAccessCount :exec
UPDATE links
SET access_count = access_count + 1, updated_at = now()
WHERE id = $1;

-- name: ListLinksByWorkspace :many
SELECT *
FROM links
WHERE workspace_id = $1 AND status NOT IN ('deleted', 'disabled')
ORDER BY created_at DESC;

-- name: ListRecentLinksByWorkspace :many
SELECT *
FROM links
WHERE workspace_id = $1 AND status NOT IN ('deleted', 'disabled')
ORDER BY created_at DESC
LIMIT $2;

-- name: ListLinksByDocument :many
SELECT *
FROM links
WHERE workspace_id = $1 AND document_id = $2 AND status NOT IN ('deleted', 'disabled')
ORDER BY created_at DESC;

-- name: UpdateLinkStatus :one
UPDATE links
SET status = $1, updated_at = now()
WHERE id = $2 AND workspace_id = $3
RETURNING *;

-- name: UpdateLinkFull :one
UPDATE links SET
    name = $1,
    document_id = $2,
    deal_room_id = $3,
    permission_type = $4,
    expires_at = $5,
    max_access_count = $6,
    download_enabled = $7,
    watermark_enabled = $8,
    require_email = $9,
    require_email_verification = $10,
    require_nda = $11,
    ai_copilot_enabled = $12,
    require_password = $13,
    password_hash = $14,
    custom_domain = $15,
    tags = $16,
    notify_on_access = $17,
    qa_enabled = $18,
    file_requests_enabled = $19,
    index_file_enabled = $20,
    screenshot_protection_enabled = $21,
    link_type = $22,
    target_folder_path = $23,
    security_version = $24,
    has_document_scope = $25,
    folder_scope_paths = $26,
    updated_at = now()
WHERE id = $27 AND workspace_id = $28
RETURNING *;

-- name: DeleteLink :execrows
UPDATE links
SET status = 'deleted', updated_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: HardDeleteLink :execrows
DELETE FROM links
WHERE id = $1 AND workspace_id = $2;

-- name: ListLinksByDealRoomID :many
SELECT *
FROM links
WHERE deal_room_id = $1;

-- name: UpdateLinkFolderScopePaths :exec
UPDATE links
SET folder_scope_paths = $1, updated_at = now()
WHERE id = $2 AND workspace_id = $3;

-- name: CreateLinkNDAAgreement :one
INSERT INTO link_nda_agreements (
    tenant_id, workspace_id, link_id, visitor_id, email, ip, user_agent
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, tenant_id, workspace_id, link_id, visitor_id, email, ip, user_agent, nda_agreed, signed_at;

-- name: GetDocumentViewMetrics :many
SELECT
    d.id,
    COALESCE(d.title, ''::text) as title,
    COALESCE(SUM(l.access_count), 0)::bigint AS views
FROM documents d
LEFT JOIN links l ON l.document_id = d.id AND l.status != 'deleted'
WHERE d.workspace_id = $1 AND d.deleted_at IS NULL
GROUP BY d.id, d.title
ORDER BY views DESC, d.created_at DESC
LIMIT $2;

-- name: CreateAccessLog :exec
INSERT INTO access_logs (tenant_id, workspace_id, link_id, visitor_id, visitor_email, event_type, ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: RecordLinkOpened :execrows
WITH inc AS (
    UPDATE links
    SET access_count = access_count + 1
    WHERE links.id = $1
      AND links.status = 'active'
      AND (links.max_access_count IS NULL OR links.access_count < links.max_access_count)
    RETURNING links.id
)
INSERT INTO access_logs (tenant_id, workspace_id, link_id, visitor_id, visitor_email, event_type, ip, user_agent)
SELECT $2, $3, $4, $5, $6, 'link_opened', $7, $8
WHERE EXISTS (SELECT 1 FROM inc);

-- name: CreatePageView :exec
INSERT INTO page_views (tenant_id, workspace_id, link_id, visitor_id, page_number, duration_seconds, scroll_depth)
VALUES ($1, $2, $3, $4, $5, $6, $7::numeric);

-- name: GetLinkAccessMetrics :one
SELECT
    COUNT(*) FILTER (WHERE event_type = 'link_opened') AS opens,
    COUNT(DISTINCT visitor_id) FILTER (WHERE event_type = 'link_opened') AS unique_visitors,
    COUNT(*) FILTER (WHERE event_type = 'download_attempted') AS downloads
FROM access_logs
WHERE link_id = $1;

-- name: GetLinkAccessMetrics24h :one
-- Rolling 24-hour access metrics used by signal rules.
SELECT
    COUNT(*) FILTER (WHERE event_type = 'link_opened') AS opens,
    COUNT(DISTINCT visitor_id) FILTER (WHERE event_type = 'link_opened') AS unique_visitors,
    COUNT(*) FILTER (WHERE event_type = 'download_attempted') AS downloads
FROM access_logs
WHERE link_id = $1
  AND created_at > now() - interval '24 hours';

-- name: GetLinkLastAccessAt :one
SELECT MAX(created_at)::timestamptz AS last_access_at
FROM access_logs
WHERE link_id = $1;

-- name: CountRecentDistinctIPsByLink :one
SELECT COUNT(DISTINCT ip)::bigint AS distinct_ips
FROM access_logs
WHERE link_id = $1
  AND event_type = 'link_opened'
  AND created_at > now() - interval '1 hour';

-- name: CountRecentDownloadAttemptsByLink :one
SELECT
    COUNT(*)::bigint AS total_downloads,
    COUNT(DISTINCT al.visitor_email) FILTER (WHERE al.visitor_email IS NOT NULL AND al.visitor_email <> '')::bigint AS distinct_emails,
    COUNT(DISTINCT al.visitor_email) FILTER (
        WHERE al.visitor_email IS NOT NULL
          AND al.visitor_email <> ''
          AND NOT EXISTS (
              SELECT 1 FROM contacts c
              WHERE c.workspace_id = l.workspace_id
                AND lower(c.email) = lower(al.visitor_email)
          )
    )::bigint AS distinct_unknown_emails
FROM access_logs al
JOIN links l ON l.id = al.link_id
WHERE al.link_id = $1
  AND al.event_type = 'download_attempted'
  AND al.created_at > now() - interval '24 hours';

-- name: GetLinkPageViewMetrics :one
SELECT
    COALESCE(AVG(duration_seconds), 0)::float8 AS avg_duration_seconds,
    COUNT(*) FILTER (WHERE duration_seconds >= 3) AS engaged_page_views,
    COUNT(*) AS total_page_views,
    COALESCE(MAX(documents.title), '')::text AS document_title
FROM page_views
JOIN links ON links.id = page_views.link_id
LEFT JOIN documents ON documents.id = links.document_id
WHERE page_views.link_id = $1;

-- name: GetLinkPageViewMetrics24h :one
-- Rolling 24-hour page-view metrics used by signal rules.
SELECT
    COALESCE(AVG(duration_seconds) FILTER (WHERE created_at > now() - interval '24 hours'), 0)::float8 AS avg_duration_seconds,
    COUNT(*) FILTER (WHERE duration_seconds >= 3 AND created_at > now() - interval '24 hours') AS engaged_page_views,
    COUNT(*) FILTER (WHERE created_at > now() - interval '24 hours') AS total_page_views
FROM page_views
WHERE link_id = $1;

-- name: GetLinkKeyPageViewMetrics :one
-- Counts page views whose page title matches any of the provided keyword patterns.
-- Patterns should be lowercase SQL LIKE patterns, e.g. '%financial%'.
SELECT
    COUNT(*) FILTER (WHERE duration_seconds >= 3) AS engaged_key_page_views,
    COUNT(*) AS total_key_page_views
FROM page_views pv
JOIN links l ON l.id = pv.link_id
JOIN pages p ON p.document_id = l.document_id AND p.page_number = pv.page_number
WHERE pv.link_id = $1
  AND p.title IS NOT NULL AND p.title <> ''
  AND lower(p.title) LIKE ANY (sqlc.arg(patterns)::text[]);

-- name: GetLinkKeyPageViewMetrics24h :one
-- Rolling 24-hour key-page metrics used by signal rules.
SELECT
    COUNT(*) FILTER (WHERE duration_seconds >= 3) AS engaged_key_page_views,
    COUNT(*) AS total_key_page_views
FROM page_views pv
JOIN links l ON l.id = pv.link_id
JOIN pages p ON p.document_id = l.document_id AND p.page_number = pv.page_number
WHERE pv.link_id = $1
  AND p.title IS NOT NULL AND p.title <> ''
  AND pv.created_at > now() - interval '24 hours'
  AND lower(p.title) LIKE ANY (sqlc.arg(patterns)::text[]);

-- name: GetLinkKeyPageViewDetails :many
-- Returns the most-viewed key pages for a link, including their titles.
SELECT
    pv.page_number,
    COALESCE(NULLIF(TRIM(p.title), ''), 'Page ' || pv.page_number)::text AS title,
    COUNT(*)::bigint AS views,
    COALESCE(AVG(pv.duration_seconds), 0)::float8 AS avg_duration_seconds
FROM page_views pv
JOIN links l ON l.id = pv.link_id
JOIN pages p ON p.document_id = l.document_id AND p.page_number = pv.page_number
WHERE pv.link_id = $1
  AND p.title IS NOT NULL AND p.title <> ''
  AND lower(p.title) LIKE ANY (sqlc.arg(patterns)::text[])
GROUP BY pv.page_number, p.title
ORDER BY views DESC, avg_duration_seconds DESC
LIMIT 3;

-- name: GetWorkspaceStorageUsage :one
SELECT (
    COALESCE((
        SELECT SUM(d.file_size) FROM documents d
        WHERE d.workspace_id = $1 AND d.deleted_at IS NULL
    ), 0) + COALESCE((
        SELECT SUM(p.file_size) FROM pages p
        JOIN documents d ON p.document_id = d.id
        WHERE d.workspace_id = $1 AND d.deleted_at IS NULL
    ), 0)
)::bigint AS total_bytes;

-- name: GetLinkBounceCount :one
SELECT COUNT(*) AS bounce_count
FROM access_logs a
WHERE a.link_id = $1
  AND a.event_type = 'link_opened'
  AND a.visitor_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM page_views p
      WHERE p.link_id = $1 AND p.visitor_id = a.visitor_id
  );

-- name: GetLinkBounceCount24h :one
-- Rolling 24-hour bounce count used by signal rules.
-- A bounce is a link_opened event with no matching page_view in the same window.
SELECT COUNT(*) AS bounce_count
FROM access_logs a
WHERE a.link_id = $1
  AND a.event_type = 'link_opened'
  AND a.visitor_id IS NOT NULL
  AND a.created_at > now() - interval '24 hours'
  AND NOT EXISTS (
      SELECT 1 FROM page_views p
      WHERE p.link_id = $1
        AND p.visitor_id = a.visitor_id
        AND p.created_at > now() - interval '24 hours'
  );

-- name: ListAccessLogsByLink :many
WITH visitor_emails AS (
    SELECT al.visitor_id, MAX(al.visitor_email) AS visitor_email
    FROM access_logs al
    WHERE al.link_id = $1 AND al.visitor_email IS NOT NULL AND al.visitor_email <> ''
    GROUP BY al.visitor_id
)
SELECT
    e.id,
    e.tenant_id,
    e.workspace_id,
    e.link_id,
    e.visitor_id,
    COALESCE(ve.visitor_email, '')::text AS visitor_email,
    e.event_type,
    e.ip,
    e.user_agent,
    COALESCE(e.page_number, 0) AS page_number,
    COALESCE(e.duration_seconds, 0) AS duration_seconds,
    e.created_at
FROM (
    SELECT
        id,
        tenant_id,
        workspace_id,
        link_id,
        visitor_id,
        'page_viewed'::text AS event_type,
        NULL::text AS ip,
        NULL::text AS user_agent,
        page_number,
        duration_seconds,
        created_at
    FROM page_views
    WHERE page_views.link_id = $1
    UNION ALL
    SELECT
        id,
        tenant_id,
        workspace_id,
        link_id,
        visitor_id,
        event_type,
        ip,
        user_agent,
        NULL::int AS page_number,
        0 AS duration_seconds,
        created_at
    FROM access_logs
    WHERE access_logs.link_id = $1
) e
LEFT JOIN visitor_emails ve ON ve.visitor_id = e.visitor_id
ORDER BY e.created_at DESC
LIMIT $2;

-- name: GetLinkAnalytics :one
WITH link_access AS (
    SELECT visitor_id, created_at
    FROM access_logs
    WHERE link_id = $1 AND event_type = 'link_opened'
),
daily_views AS (
    SELECT DATE(created_at)::text AS day, COUNT(*)::bigint AS views
    FROM link_access
    WHERE created_at >= now() - interval '30 days'
    GROUP BY DATE(created_at)
    ORDER BY day
)
SELECT
    COALESCE((SELECT COUNT(*) FROM link_access), 0)::bigint AS total_views,
    COALESCE((SELECT COUNT(DISTINCT visitor_id) FROM link_access WHERE visitor_id IS NOT NULL AND visitor_id <> ''), 0)::bigint AS unique_visitors,
    COALESCE((SELECT COUNT(*) FROM access_logs al WHERE al.link_id = $1 AND al.event_type = 'download_attempted'), 0)::bigint AS download_attempts,
    (SELECT MIN(created_at)::timestamptz FROM link_access) AS first_access_at,
    (SELECT MAX(created_at)::timestamptz FROM link_access) AS last_access_at,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('day', day, 'views', views)) FROM daily_views), '[]'::jsonb)::jsonb AS views_over_time;

-- name: ListRecentVisitorsByLink :many
SELECT
    visitor_id,
    COALESCE(MAX(visitor_email), '')::text AS visitor_email,
    MIN(created_at)::timestamptz AS first_access_at,
    MAX(created_at)::timestamptz AS last_access_at,
    COUNT(*) FILTER (WHERE event_type = 'link_opened')::bigint AS total_views
FROM access_logs
WHERE link_id = $1 AND visitor_id IS NOT NULL AND visitor_id <> ''
GROUP BY visitor_id
ORDER BY last_access_at DESC
LIMIT 10;

-- name: GetAverageDurationByLink :one
SELECT COALESCE(AVG(duration_seconds), 0)::float8 AS avg_duration_seconds
FROM page_views
WHERE link_id = $1;

-- name: ListTopPagesByLink :many
SELECT
    page_number,
    COUNT(*)::bigint AS views,
    COALESCE(AVG(duration_seconds), 0)::float8 AS avg_duration_seconds
FROM page_views
WHERE link_id = $1
GROUP BY page_number
ORDER BY views DESC, avg_duration_seconds DESC
LIMIT 10;

-- name: GetVisitorSummariesByDocument :many
WITH visitor_emails AS (
    SELECT al.visitor_id, MAX(al.visitor_email) AS visitor_email
    FROM access_logs al
    WHERE al.link_id IN (SELECT l.id FROM links l WHERE l.document_id = $1 AND l.workspace_id = $2 AND l.status != 'deleted')
      AND al.workspace_id = $2
      AND al.visitor_email IS NOT NULL AND al.visitor_email <> ''
    GROUP BY al.visitor_id
)
SELECT
    pv.visitor_id,
    COALESCE(ve.visitor_email, '')::text AS visitor_email,
    COUNT(*)::bigint AS page_view_count,
    COALESCE(AVG(pv.duration_seconds), 0)::float8 AS avg_duration_seconds,
    MAX(pv.created_at)::timestamptz AS last_seen_at
FROM page_views pv
LEFT JOIN visitor_emails ve ON ve.visitor_id = pv.visitor_id
WHERE pv.link_id IN (SELECT l.id FROM links l WHERE l.document_id = $1 AND l.workspace_id = $2 AND l.status != 'deleted')
  AND pv.workspace_id = $2
GROUP BY pv.visitor_id, ve.visitor_email
ORDER BY last_seen_at DESC
LIMIT $3;

-- name: GetLastAccessLogByLink :one
SELECT id, tenant_id, workspace_id, link_id, visitor_id, visitor_email, event_type, ip, user_agent, created_at
FROM access_logs
WHERE link_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetLastLinkOpenByVisitor :one
SELECT created_at
FROM access_logs
WHERE link_id = $1
  AND visitor_id = $2
  AND event_type = 'link_opened'
ORDER BY created_at DESC
LIMIT 1;

-- name: GetLastPageViewByVisitorPage :one
SELECT created_at
FROM page_views
WHERE link_id = $1
  AND visitor_id = $2
  AND page_number = $3
ORDER BY created_at DESC
LIMIT 1;

-- name: GetDocumentsByIDs :many
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE id = ANY($1::uuid[]) AND workspace_id = $2 AND deleted_at IS NULL;

-- name: GetLinkAccessMetricsBatch :many
SELECT
    link_id,
    COUNT(*) FILTER (WHERE event_type = 'link_opened')::bigint AS opens,
    COUNT(DISTINCT visitor_id) FILTER (WHERE event_type = 'link_opened')::bigint AS unique_visitors,
    COUNT(*) FILTER (WHERE event_type = 'download_attempted')::bigint AS downloads
FROM access_logs
WHERE link_id = ANY($1::uuid[])
GROUP BY link_id;

-- name: GetLinkPageViewMetricsBatch :many
SELECT
    link_id,
    COALESCE(AVG(duration_seconds), 0)::float8 AS avg_duration_seconds,
    COUNT(*) FILTER (WHERE duration_seconds >= 3)::bigint AS key_page_views,
    COUNT(*)::bigint AS total_page_views
FROM page_views
WHERE link_id = ANY($1::uuid[])
GROUP BY link_id;

-- name: GetLinkBounceCountsBatch :many
SELECT a.link_id, COUNT(*)::bigint AS bounce_count
FROM access_logs a
WHERE a.link_id = ANY($1::uuid[])
  AND a.event_type = 'link_opened'
  AND a.visitor_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM page_views p
      WHERE p.link_id = a.link_id AND p.visitor_id = a.visitor_id
  )
GROUP BY a.link_id;

-- name: GetLinkKeyPageViewMetricsBatch :many
-- Batch version of GetLinkKeyPageViewMetrics for O(1) dashboard heat scoring.
SELECT
    pv.link_id,
    COUNT(*) FILTER (WHERE duration_seconds >= 3) AS engaged_key_page_views,
    COUNT(*) AS total_key_page_views
FROM page_views pv
JOIN links l ON l.id = pv.link_id
JOIN pages p ON p.document_id = l.document_id AND p.page_number = pv.page_number
WHERE pv.link_id = ANY(sqlc.arg(link_ids)::uuid[])
  AND p.title IS NOT NULL AND p.title <> ''
  AND lower(p.title) LIKE ANY (sqlc.arg(patterns)::text[])
GROUP BY pv.link_id;

-- name: ListLinkHeatScoresByWorkspace :many
-- Raw pre-aggregated metrics used by the dashboard heat score computation.
SELECT
    link_id,
    workspace_id,
    created_at,
    opens,
    unique_visitors,
    downloads,
    avg_duration_seconds,
    total_page_views,
    engaged_page_views,
    bounce_count,
    last_access_at
FROM link_heat_scores
WHERE workspace_id = $1;

-- name: GetLastAccessLogsByLinks :many
SELECT DISTINCT ON (link_id) id, tenant_id, workspace_id, link_id, visitor_id, visitor_email, event_type, ip, user_agent, created_at
FROM access_logs
WHERE link_id = ANY($1::uuid[])
ORDER BY link_id, created_at DESC;

-- name: ListAccessLogsByWorkspace :many
SELECT id, tenant_id, workspace_id, link_id, visitor_id, visitor_email, event_type, ip, user_agent, created_at
FROM access_logs
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ListPageViewsByWorkspace :many
SELECT id, tenant_id, workspace_id, link_id, visitor_id, page_number, duration_seconds, scroll_depth, created_at
FROM page_views
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: GetPageAnalyticsByDocument :many
SELECT
    p.page_number,
    COUNT(pv.id) AS view_count,
    COALESCE(AVG(pv.duration_seconds), 0)::float8 AS avg_duration_seconds,
    COALESCE(MAX(pv.created_at), p.created_at) AS last_viewed_at
FROM pages p
LEFT JOIN links l ON l.document_id = p.document_id AND l.status != 'deleted'
LEFT JOIN page_views pv ON pv.link_id = l.id AND pv.page_number = p.page_number
WHERE p.document_id = $1 AND p.workspace_id = $2
GROUP BY p.page_number, p.created_at
ORDER BY p.page_number;

-- name: GetPageTitlesByDocument :many
SELECT
    p.page_number,
    COALESCE(NULLIF(TRIM(p.title), ''), LEFT(c.text, 80), '')::text AS title
FROM pages p
LEFT JOIN LATERAL (
    SELECT text FROM chunks WHERE page_id = p.id ORDER BY id LIMIT 1
) c ON true
WHERE p.document_id = $1 AND p.workspace_id = $2
ORDER BY p.page_number;

-- name: GetPageExitCountsByDocument :many
SELECT page_number, COUNT(*) AS exit_count
FROM (
    SELECT DISTINCT ON (link_id, visitor_id) link_id, visitor_id, page_number
    FROM page_views
    WHERE link_id IN (
        SELECT id FROM links WHERE document_id = $1 AND status != 'deleted'
    )
    ORDER BY link_id, visitor_id, created_at DESC
) last_views
GROUP BY page_number;

-- name: CreateDealRoom :one
INSERT INTO deal_rooms (
    tenant_id, workspace_id, slug, name, description, template_type, settings,
    requires_nda, requires_approval, status, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetDealRoomByID :one
SELECT *
FROM deal_rooms
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL
LIMIT 1;

-- name: GetDealRoomBySlug :one
SELECT *
FROM deal_rooms
WHERE slug = $1 AND status = 'active' AND deleted_at IS NULL
LIMIT 1;

-- name: ListDealRoomsByWorkspace :many
SELECT *
FROM deal_rooms
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: AddRoomMember :one
INSERT INTO room_members (tenant_id, workspace_id, room_id, email, user_id, role, nda_status, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, tenant_id, workspace_id, room_id, email, user_id, role, nda_status, nda_signed_at, status, created_at, updated_at;

-- name: GetRoomMemberByEmail :one
SELECT id, tenant_id, workspace_id, room_id, email, user_id, role, nda_status, nda_signed_at, status, created_at, updated_at
FROM room_members
WHERE room_id = $1 AND email = $2
LIMIT 1;

-- name: UpdateRoomMemberStatus :exec
UPDATE room_members
SET status = $1, updated_at = now()
WHERE room_id = $2 AND email = $3;

-- name: UpdateRoomMemberNDA :exec
UPDATE room_members
SET nda_status = 'signed', nda_signed_at = now(), updated_at = now()
WHERE room_id = $1 AND email = $2;

-- name: ListRoomMembers :many
SELECT id, tenant_id, workspace_id, room_id, email, user_id, role, nda_status, nda_signed_at, status, created_at, updated_at
FROM room_members
WHERE room_id = $1
ORDER BY created_at DESC;

-- name: CreateAccessRequest :one
INSERT INTO room_access_requests (tenant_id, workspace_id, room_id, email, reason, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, tenant_id, workspace_id, room_id, email, reason, status, reviewed_by, reviewed_at, created_at, updated_at;

-- name: GetAccessRequestByID :one
SELECT id, tenant_id, workspace_id, room_id, email, reason, status, reviewed_by, reviewed_at, created_at, updated_at
FROM room_access_requests
WHERE id = $1 AND room_id = $2
LIMIT 1;

-- name: UpdateAccessRequestStatus :exec
UPDATE room_access_requests
SET status = $1, reviewed_by = $2, reviewed_at = now(), updated_at = now()
WHERE id = $3;

-- name: ListAccessRequestsByRoom :many
SELECT id, tenant_id, workspace_id, room_id, email, reason, status, reviewed_by, reviewed_at, created_at, updated_at
FROM room_access_requests
WHERE room_id = $1
ORDER BY created_at DESC;

-- name: UpdateDealRoomSettings :exec
UPDATE deal_rooms
SET settings = $1::jsonb, updated_at = now()
WHERE id = $2 AND workspace_id = $3;

-- name: DeleteDealRoomDocument :exec
DELETE FROM deal_room_documents
WHERE document_id = $1 AND room_id = $2;


-- name: UpdateDealRoomDocumentFolder :exec
UPDATE deal_room_documents
SET folder_path = $1
WHERE id = $2 AND room_id = $3;

-- name: UpdateDealRoomDocumentSortOrder :exec
UPDATE deal_room_documents
SET sort_order = $1
WHERE id = $2 AND room_id = $3;

-- name: CountDocumentsInFolder :one
SELECT COUNT(*) AS count
FROM deal_room_documents
WHERE room_id = $1
  AND (folder_path = $2 OR folder_path LIKE $2 || '/%');

-- name: UpdateDealRoomDocumentsFolderPath :exec
UPDATE deal_room_documents
SET folder_path = $1
WHERE room_id = $2 AND folder_path = $3;

-- name: UpdateRoomFolderPermissionsFolderPath :exec
UPDATE room_member_folder_permissions
SET folder_path = $1, updated_at = now()
WHERE room_id = $2 AND folder_path = $3;

-- name: DeleteRoomFolderPermissions :exec
DELETE FROM room_member_folder_permissions
WHERE room_id = $1 AND folder_path = $2;

-- name: DeleteRoomFolderPermissionsPrefix :exec
DELETE FROM room_member_folder_permissions
WHERE room_id = $1 AND (folder_path = $2 OR folder_path LIKE $2 || '/%');

-- name: DeleteRoomMember :exec
DELETE FROM room_members
WHERE id = $1 AND room_id = $2;

-- name: GetRoomMemberByID :one
SELECT id, tenant_id, workspace_id, room_id, email, user_id, role, nda_status, nda_signed_at, status, created_at, updated_at
FROM room_members
WHERE id = $1 AND room_id = $2
LIMIT 1;

-- name: GetRoomMemberByUserID :one
SELECT id, tenant_id, workspace_id, room_id, email, user_id, role, nda_status, nda_signed_at, status, created_at, updated_at
FROM room_members
WHERE room_id = $1 AND user_id = $2
LIMIT 1;

-- name: ListRoomMembersWithUser :many
SELECT
    rm.id,
    rm.tenant_id,
    rm.workspace_id,
    rm.room_id,
    rm.email,
    rm.user_id,
    rm.role,
    rm.nda_status,
    rm.nda_signed_at,
    rm.status,
    rm.created_at,
    rm.updated_at,
    COALESCE(u.email, '')::text AS user_name
FROM room_members rm
LEFT JOIN users u ON u.id = rm.user_id
WHERE rm.room_id = $1
ORDER BY rm.created_at DESC;

-- name: GetDealRoomFolderPaths :one
SELECT COALESCE(settings->'folders', '[]'::jsonb)::text AS folders
FROM deal_rooms
WHERE id = $1 AND workspace_id = $2;

-- name: ListDealRoomDocumentsWithMeta :many
SELECT
    drd.id,
    drd.tenant_id,
    drd.workspace_id,
    drd.room_id,
    drd.document_id,
    drd.folder_path,
    drd.sort_order,
    drd.created_at,
    COALESCE(d.title, ''::text) AS document_title,
    d.page_count,
    COALESCE(d.file_size, 0::bigint) AS file_size,
    d.source_type,
    d.status
FROM deal_room_documents drd
JOIN documents d ON d.id = drd.document_id
WHERE drd.room_id = $1 AND d.deleted_at IS NULL
ORDER BY drd.folder_path, drd.sort_order;

-- name: HasDealRoomDocument :one
SELECT EXISTS(
    SELECT 1 FROM deal_room_documents drd
    JOIN documents d ON d.id = drd.document_id
    WHERE drd.room_id = $1 AND drd.document_id = $2 AND d.deleted_at IS NULL
) AS exists;

-- name: GetDealRoomDocumentFolderPath :one
SELECT drd.folder_path
FROM deal_room_documents drd
JOIN documents d ON d.id = drd.document_id
WHERE drd.room_id = $1 AND drd.document_id = $2 AND d.deleted_at IS NULL;

-- name: DeleteLinkDocumentsByDealRoomDocument :exec
DELETE FROM link_documents ld
WHERE ld.document_id = $1
  AND ld.link_id IN (SELECT id FROM links WHERE deal_room_id = $2);

-- name: AddDealRoomDocument :one
INSERT INTO deal_room_documents (tenant_id, workspace_id, room_id, document_id, folder_path, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, tenant_id, workspace_id, room_id, document_id, folder_path, sort_order, created_at;

-- name: ListDealRoomDocuments :many
SELECT id, tenant_id, workspace_id, room_id, document_id, folder_path, sort_order, created_at
FROM deal_room_documents
WHERE room_id = $1
ORDER BY folder_path, sort_order;

-- name: GetDealRoomAggregatesByWorkspace :many
SELECT
    dr.id AS room_id,
    COUNT(DISTINCT drd.id) AS document_count,
    COUNT(DISTINCT rm.id) AS member_count,
    COUNT(DISTINCT rar.id) FILTER (WHERE rar.status = 'pending') AS pending_count,
    (COUNT(DISTINCT al.visitor_id) FILTER (WHERE al.visitor_id IS NOT NULL) +
    COUNT(DISTINCT al.visitor_email) FILTER (WHERE al.visitor_email IS NOT NULL AND al.visitor_id IS NULL))::bigint AS visitor_count,
    COUNT(DISTINCT q.id) FILTER (WHERE q.status = 'pending') AS pending_question_count,
    MAX(al.created_at)::timestamptz AS last_accessed_at,
    COALESCE(
        LEAST(100,
            (COUNT(DISTINCT al.visitor_id) FILTER (WHERE al.visitor_id IS NOT NULL) +
             COUNT(DISTINCT al.visitor_email) FILTER (WHERE al.visitor_email IS NOT NULL AND al.visitor_id IS NULL)) * 5
            + COUNT(DISTINCT al.id) * 2
        ),
        0
    )::int AS heat_score
FROM deal_rooms dr
LEFT JOIN deal_room_documents drd ON drd.room_id = dr.id
LEFT JOIN room_members rm ON rm.room_id = dr.id
LEFT JOIN room_access_requests rar ON rar.room_id = dr.id
LEFT JOIN links l ON l.deal_room_id = dr.id AND l.status NOT IN ('deleted', 'disabled')
LEFT JOIN access_logs al ON al.link_id = l.id
LEFT JOIN link_visitor_questions q ON q.link_id = l.id
WHERE dr.workspace_id = $1 AND dr.deleted_at IS NULL
GROUP BY dr.id;

-- name: SetFolderPermission :one
INSERT INTO room_member_folder_permissions (tenant_id, workspace_id, room_id, email, folder_path, permission)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (room_id, email, folder_path) DO UPDATE SET permission = EXCLUDED.permission, updated_at = now()
RETURNING id, tenant_id, workspace_id, room_id, email, folder_path, permission, created_at, updated_at;

-- name: GetFolderPermission :one
SELECT id, tenant_id, workspace_id, room_id, email, folder_path, permission, created_at, updated_at
FROM room_member_folder_permissions
WHERE room_id = $1 AND email = $2 AND folder_path = $3
LIMIT 1;

-- name: GetFolderPermissionsByRoomAndEmail :many
SELECT id, tenant_id, workspace_id, room_id, email, folder_path, permission, created_at, updated_at
FROM room_member_folder_permissions
WHERE room_id = $1 AND email = $2;

-- name: CreateNDAAgreement :exec
INSERT INTO room_nda_agreements (room_id, email, ip, user_agent)
VALUES ($1, $2, $3, $4)
ON CONFLICT (room_id, email) DO NOTHING;

-- name: HasNDAAgreement :one
SELECT EXISTS (
    SELECT 1 FROM room_nda_agreements
    WHERE room_id = $1 AND email = $2
) AS has_agreement;
-- name: CreateTenantDomain :one
INSERT INTO tenant_domains (tenant_id, domain, domain_type, is_primary)
VALUES ($1, $2, $3, $4)
RETURNING id, tenant_id, domain, domain_type, is_primary, ssl_status, ssl_expires_at, verified_at, created_at, updated_at;

-- name: GetTenantDomainByDomain :one
SELECT id, tenant_id, domain, domain_type, is_primary, ssl_status, ssl_expires_at, verified_at, created_at, updated_at
FROM tenant_domains
WHERE domain = $1 LIMIT 1;

-- name: ListTenantDomainsByTenant :many
SELECT id, tenant_id, domain, domain_type, is_primary, ssl_status, ssl_expires_at, verified_at, created_at, updated_at
FROM tenant_domains
WHERE tenant_id = $1
ORDER BY created_at DESC;

-- name: UpdateTenantDomainSSL :exec
UPDATE tenant_domains
SET ssl_status = $1, ssl_expires_at = $2, verified_at = $3, updated_at = now()
WHERE id = $4 AND tenant_id = $5;

-- name: DeleteTenantDomain :exec
DELETE FROM tenant_domains
WHERE id = $1 AND tenant_id = $2;

-- name: ListTenantDomainsExpiringBefore :many
SELECT id, tenant_id, domain, domain_type, is_primary, ssl_status, ssl_expires_at, verified_at, created_at, updated_at
FROM tenant_domains
WHERE ssl_status = 'issued' AND ssl_expires_at < $1
ORDER BY ssl_expires_at ASC;

-- name: GetTenantBySlug :one
SELECT id, name, created_at
FROM tenants
WHERE slug = $1 LIMIT 1;

-- name: GetWorkspaceByTenantAndSlug :one
SELECT * FROM workspaces WHERE tenant_id = $1 AND slug = $2 LIMIT 1;

-- name: ListWorkspacesByUserAndTenant :many
SELECT w.id, w.tenant_id, w.name, w.slug, w.brand_color, w.force_email_verification, w.watermark_downloads, w.two_factor_enabled, w.created_at, m.role
FROM workspaces w
JOIN workspace_members m ON m.workspace_id = w.id
WHERE m.user_id = $1 AND w.tenant_id = $2
ORDER BY w.created_at DESC;

-- name: CreateSuggestion :one
INSERT INTO suggestions (tenant_id, workspace_id, contact_id, link_id, document_id, type, subtype, reason, action, metadata, context, rule_id)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.arg(subtype), $7, $8, sqlc.arg(metadata)::jsonb, sqlc.arg(context)::jsonb, sqlc.arg(rule_id))
RETURNING *;

-- name: ListSuggestionsByLink :many
SELECT *
FROM suggestions
WHERE link_id = $1 AND workspace_id = $2 AND dismissed = false
ORDER BY created_at DESC;

-- name: CountRecentSuggestionsByLinkTypeSubtype :one
SELECT COUNT(*) AS count
FROM suggestions
WHERE link_id = $1 AND workspace_id = $2 AND type = $3 AND subtype = $4 AND dismissed = false AND created_at > now() - interval '24 hours';

-- name: CountRecentQuestionSuggestionsBySession :one
SELECT COUNT(*) AS count
FROM suggestions
WHERE workspace_id = $1
  AND subtype = 'question'
  AND dismissed = false
  AND created_at > now() - interval '24 hours'
  AND metadata @> sqlc.arg(session_metadata)::jsonb;

-- name: DismissSuggestion :exec
UPDATE suggestions
SET dismissed = true, updated_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: InsertSuggestionOutbox :execrows
INSERT INTO suggestion_outbox (tenant_id, workspace_id, link_id, lang)
VALUES ($1, $2, $3, $4)
ON CONFLICT (link_id, workspace_id) WHERE processed_at IS NULL DO NOTHING;

-- name: ListPendingSuggestionOutbox :many
SELECT id, tenant_id, workspace_id, link_id, lang, created_at, processed_at, attempts, last_error
FROM suggestion_outbox
WHERE processed_at IS NULL AND attempts < $1
ORDER BY created_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: MarkSuggestionOutboxProcessed :exec
UPDATE suggestion_outbox
SET processed_at = now()
WHERE id = $1;

-- name: IncrementSuggestionOutboxAttempts :exec
UPDATE suggestion_outbox
SET attempts = attempts + 1, last_error = $2
WHERE id = $1;

-- name: GetSuggestionByID :one
SELECT id, tenant_id, workspace_id, contact_id, link_id, document_id, type, reason, action, dismissed, created_at, updated_at
FROM suggestions
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: GetNotificationSettings :one
SELECT workspace_id, email_enabled, slack_webhook_url, slack_connected, hubspot_connected, salesforce_connected, updated_at
FROM notification_settings
WHERE workspace_id = $1;

-- name: UpsertNotificationSettings :one
INSERT INTO notification_settings (
    workspace_id, email_enabled, slack_webhook_url, slack_connected, hubspot_connected, salesforce_connected
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (workspace_id)
DO UPDATE SET
    email_enabled = EXCLUDED.email_enabled,
    slack_webhook_url = EXCLUDED.slack_webhook_url,
    slack_connected = EXCLUDED.slack_connected,
    hubspot_connected = EXCLUDED.hubspot_connected,
    salesforce_connected = EXCLUDED.salesforce_connected,
    updated_at = now()
RETURNING workspace_id, email_enabled, slack_webhook_url, slack_connected, hubspot_connected, salesforce_connected, updated_at;

-- name: CreateNotification :one
INSERT INTO notifications (workspace_id, user_id, channel, subject, body, recipient_email, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: AcquirePendingNotifications :many
SELECT *
FROM notifications
WHERE status IN ('pending', 'failed')
  AND (next_attempt_at IS NULL OR next_attempt_at <= now())
  AND attempts < $2
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkNotificationSent :exec
UPDATE notifications
SET status = 'sent',
    sent_at = now(),
    provider_message_id = $2,
    attempts = attempts + 1,
    updated_at = now()
WHERE id = $1;

-- name: MarkNotificationFailed :exec
UPDATE notifications
SET attempts = attempts + 1,
    last_error = $2,
    status = CASE WHEN attempts + 1 >= $3 THEN 'dead' ELSE 'pending' END,
    next_attempt_at = CASE WHEN attempts + 1 >= $3 THEN NULL ELSE now() + ($4 * interval '1 second') END,
    updated_at = now()
WHERE id = $1;

-- name: UpdateNotificationBody :exec
UPDATE notifications
SET body = $2, updated_at = now()
WHERE id = $1;

-- name: CreateOAuthState :exec
INSERT INTO oauth_states (state, workspace_id, provider, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetOAuthState :one
SELECT state, workspace_id, provider, expires_at
FROM oauth_states
WHERE state = $1 AND provider = $2
LIMIT 1;

-- name: DeleteOAuthState :exec
DELETE FROM oauth_states WHERE state = $1;

-- name: UpsertIntegrationToken :exec
INSERT INTO integration_tokens (workspace_id, provider, access_token, refresh_token, expires_at, scope, external_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (workspace_id, provider) DO UPDATE SET
    access_token = EXCLUDED.access_token,
    refresh_token = EXCLUDED.refresh_token,
    expires_at = EXCLUDED.expires_at,
    scope = EXCLUDED.scope,
    external_id = EXCLUDED.external_id,
    updated_at = now();

-- name: GetIntegrationToken :one
SELECT workspace_id, provider, access_token, refresh_token, expires_at, scope, external_id, created_at, updated_at
FROM integration_tokens
WHERE workspace_id = $1 AND provider = $2 LIMIT 1;

-- name: DeleteIntegrationToken :exec
DELETE FROM integration_tokens WHERE workspace_id = $1 AND provider = $2;

-- name: CreateSyncLog :one
INSERT INTO integration_sync_logs (workspace_id, provider, direction, record_type, external_id, status, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, workspace_id, provider, direction, record_type, external_id, status, payload, error_message, created_at;

-- name: ListSyncLogsByWorkspace :many
SELECT id, workspace_id, provider, direction, record_type, external_id, status, payload, error_message, created_at
FROM integration_sync_logs
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT 50;

-- name: CreateSyncLogWithError :one
INSERT INTO integration_sync_logs (workspace_id, provider, direction, record_type, external_id, status, payload, error_message)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, workspace_id, provider, direction, record_type, external_id, status, payload, error_message, created_at;

-- name: CreateIntegrationMapping :one
INSERT INTO integration_mappings (workspace_id, provider, local_record_type, local_id, external_id, external_url, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (workspace_id, provider, local_record_type, local_id) DO UPDATE SET
    external_id = EXCLUDED.external_id,
    external_url = EXCLUDED.external_url,
    metadata = EXCLUDED.metadata,
    updated_at = now()
RETURNING id, workspace_id, provider, local_record_type, local_id, external_id, external_url, metadata, created_at, updated_at;

-- name: GetIntegrationMapping :one
SELECT id, workspace_id, provider, local_record_type, local_id, external_id, external_url, metadata, created_at, updated_at
FROM integration_mappings
WHERE workspace_id = $1 AND provider = $2 AND local_record_type = $3 AND local_id = $4
LIMIT 1;

-- name: CreateHubSpotSyncJob :one
INSERT INTO hubspot_sync_jobs (workspace_id, record_type, record_id, direction, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, workspace_id, status, record_type, record_id, direction, attempts, error_message, payload, created_at, updated_at;

-- name: ListPendingHubSpotSyncJobs :many
SELECT id, workspace_id, status, record_type, record_id, direction, attempts, error_message, payload, created_at, updated_at
FROM hubspot_sync_jobs
WHERE status = 'pending' AND attempts < 3
ORDER BY created_at ASC
LIMIT $1;

-- name: MarkHubSpotSyncJobProcessing :exec
UPDATE hubspot_sync_jobs
SET status = 'processing', attempts = attempts + 1, updated_at = now()
WHERE id = $1 AND status = 'pending';

-- name: MarkHubSpotSyncJobCompleted :exec
UPDATE hubspot_sync_jobs
SET status = 'completed', attempts = attempts + 1, updated_at = now()
WHERE id = $1;

-- name: MarkHubSpotSyncJobFailed :exec
UPDATE hubspot_sync_jobs
SET attempts = attempts + 1,
    error_message = $2,
    status = CASE WHEN attempts + 1 >= 3 THEN 'failed' ELSE 'pending' END,
    updated_at = now()
WHERE id = $1;

-- name: GetWorkspaceByIDAndTenant :one
SELECT * FROM workspaces WHERE id = $1 AND tenant_id = $2 LIMIT 1;

-- name: CreateInvitation :one
INSERT INTO workspace_invitations (workspace_id, email, role, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING token, workspace_id, email, role, expires_at, used_at, created_at;

-- name: GetInvitationByToken :one
SELECT token, workspace_id, email, role, expires_at, used_at, created_at
FROM workspace_invitations
WHERE token = $1 LIMIT 1;

-- name: MarkInvitationUsed :exec
UPDATE workspace_invitations
SET used_at = now()
WHERE token = $1;
-- name: DeletePagesByDocument :exec
DELETE FROM pages WHERE document_id = $1;

-- name: DeleteChunksByDocument :exec
DELETE FROM chunks WHERE chunks.document_id = $1 OR chunks.page_id IN (SELECT id FROM pages WHERE pages.document_id = $1);

-- name: CreateChunkBox :exec
INSERT INTO chunk_boxes (chunk_id, document_id, page_number, coordinate_space, x, y, w, h, source, confidence)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: CreateChunkWithBBox :one
INSERT INTO chunks (tenant_id, workspace_id, page_id, document_id, chunk_index, chunk_type, text, normalized_text, bbox, embedding, search_vector)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, to_tsvector('english', $7))
RETURNING id, tenant_id, workspace_id, page_id, document_id, chunk_index, chunk_type, text, normalized_text, bbox, embedding, search_vector;

-- name: CreateChunkWithBBoxNoEmbed :one
INSERT INTO chunks (tenant_id, workspace_id, page_id, document_id, chunk_index, chunk_type, text, normalized_text, bbox, search_vector)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, to_tsvector('english', $7))
RETURNING id, tenant_id, workspace_id, page_id, document_id, chunk_index, chunk_type, text, normalized_text, bbox, search_vector;

-- name: SearchChunksByTrigram :many
SELECT
    c.id,
    c.text,
    c.bbox,
    c.normalized_text,
    p.page_number,
    p.document_id,
    similarity(c.normalized_text, sqlc.arg(query)) AS rank
FROM chunks c
JOIN pages p ON p.id = c.page_id
WHERE c.workspace_id = $1
  AND c.normalized_text IS NOT NULL
  AND c.normalized_text <> ''
  AND similarity(c.normalized_text, sqlc.arg(query)) > 0.1
ORDER BY rank DESC
LIMIT $2;

-- name: SearchChunksByVectorInDocuments :many
SELECT
    c.id,
    c.text,
    c.bbox,
    p.page_number,
    p.document_id,
    (c.embedding <=> sqlc.arg(embedding)::vector)::float8 AS distance
FROM chunks c
JOIN pages p ON p.id = c.page_id
WHERE c.workspace_id = $1
  AND p.document_id = ANY(sqlc.arg(document_ids)::uuid[])
  AND c.embedding IS NOT NULL
ORDER BY c.embedding <=> sqlc.arg(embedding)::vector
LIMIT $2;

-- name: SearchChunksByTextInDocuments :many
SELECT
    c.id,
    c.text,
    c.bbox,
    p.page_number,
    p.document_id,
    ts_rank(c.search_vector, plainto_tsquery('english', sqlc.arg(query))) AS rank
FROM chunks c
JOIN pages p ON p.id = c.page_id
WHERE c.workspace_id = $1
  AND p.document_id = ANY(sqlc.arg(document_ids)::uuid[])
  AND c.search_vector @@ plainto_tsquery('english', sqlc.arg(query))
ORDER BY rank DESC
LIMIT $2;

-- name: ListChunksByDocumentIDs :many
SELECT
    c.id,
    c.text,
    c.chunk_index,
    p.page_number,
    p.document_id
FROM chunks c
JOIN pages p ON p.id = c.page_id
WHERE p.document_id = ANY(sqlc.arg(document_ids)::uuid[])
ORDER BY p.document_id, p.page_number, c.chunk_index;

-- name: SearchChunksByTrigramInDocuments :many
SELECT
    c.id,
    c.text,
    c.bbox,
    c.normalized_text,
    p.page_number,
    p.document_id,
    similarity(c.normalized_text, sqlc.arg(query)) AS rank
FROM chunks c
JOIN pages p ON p.id = c.page_id
WHERE c.workspace_id = $1
  AND p.document_id = ANY(sqlc.arg(document_ids)::uuid[])
  AND c.normalized_text IS NOT NULL
  AND c.normalized_text <> ''
  AND similarity(c.normalized_text, sqlc.arg(query)) > 0.1
ORDER BY rank DESC
LIMIT $2;

-- name: SearchHybridWithBBox :many
SELECT
    c.id,
    c.text,
    c.bbox,
    p.page_number,
    p.document_id,
    cb.x AS box_x,
    cb.y AS box_y,
    cb.w AS box_w,
    cb.h AS box_h,
    'hybrid' AS match_type
FROM chunks c
JOIN pages p ON p.id = c.page_id
LEFT JOIN LATERAL (
    SELECT x, y, w, h FROM chunk_boxes WHERE chunk_id = c.id ORDER BY id LIMIT 1
) cb ON true
WHERE c.workspace_id = $1
  AND c.id = ANY(sqlc.arg(chunk_ids)::uuid[])
ORDER BY p.page_number
LIMIT $2;

-- name: GetDocumentByIDAndTenant :one
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE id = $1 AND workspace_id = $2 AND tenant_id = $3 AND deleted_at IS NULL
LIMIT 1;

-- name: ListDocumentsByCategory :many
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE workspace_id = $1 AND category = $2 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateDocumentCategory :exec
UPDATE documents
SET category = $1, updated_at = now()
WHERE id = $2 AND workspace_id = $3;

-- name: CreateSignal :one
INSERT INTO signals (
    tenant_id, workspace_id, suggestion_id, type, subtype, title, description, explanation, suggestion,
    document_id, contact_id, link_id, priority, metadata, context
) VALUES ($1, $2, $3, $4, sqlc.arg(subtype), $5, $6, $7, $8, $9, $10, $11, $12, sqlc.arg(metadata)::jsonb, sqlc.arg(context)::jsonb)
ON CONFLICT (workspace_id, suggestion_id) WHERE suggestion_id IS NOT NULL DO UPDATE SET
    updated_at = now()
RETURNING *;

-- name: GetSignalBySuggestion :one
SELECT *
FROM signals
WHERE suggestion_id = $1 AND workspace_id = $2 LIMIT 1;

-- name: GetSignalByID :one
SELECT *
FROM signals
WHERE id = $1 AND workspace_id = $2 LIMIT 1;

-- name: ListSignalsByWorkspace :many
SELECT *
FROM signals
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: CreateActionItem :one
INSERT INTO action_items (
    tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (signal_id) DO UPDATE SET
    updated_at = now()
RETURNING *;

-- name: CreateOperationalActionItem :one
INSERT INTO action_items (
    tenant_id, workspace_id, source_type, source_id, title, impact, due_at, status, action_type
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (workspace_id, source_type, source_id) DO UPDATE SET
    updated_at = now()
RETURNING *;

-- name: ListActionItemsByWorkspace :many
-- Returns pending action items plus recently completed/snoozed/ignored items
-- so the "completed" UI list does not grow indefinitely. Done items are kept
-- for 1 day; snoozed/ignored items are kept for 30 days.
SELECT *
FROM action_items
WHERE workspace_id = $1
  AND (
      status = 'pending'
      OR (status = 'done' AND updated_at > now() - interval '1 day')
      OR (status IN ('snoozed', 'ignored') AND updated_at > now() - interval '30 days')
  )
ORDER BY created_at DESC;

-- name: GetActionItemByID :one
SELECT *
FROM action_items
WHERE id = $1 AND workspace_id = $2 LIMIT 1;

-- name: GetActionItemBySource :one
SELECT *
FROM action_items
WHERE workspace_id = $1 AND source_type = $2 AND source_id = $3 LIMIT 1;

-- name: ListPendingActionItemsBySourceType :many
SELECT *
FROM action_items
WHERE workspace_id = $1 AND source_type = $2 AND status = 'pending'
ORDER BY created_at DESC;

-- name: UpdateActionItemStatus :one
UPDATE action_items
SET status = $1, updated_at = now()
WHERE id = $2 AND workspace_id = $3
RETURNING *;

-- name: ListPendingLinkAccessRequestsByWorkspace :many
SELECT r.id, r.email, r.link_id, l.name AS link_name
FROM link_access_requests r
JOIN links l ON l.id = r.link_id
WHERE r.workspace_id = $1 AND r.status = 'pending'
ORDER BY r.created_at DESC;

-- name: ListPendingRoomAccessRequestsByWorkspace :many
SELECT r.id, r.email, r.room_id, dr.name AS room_name
FROM room_access_requests r
JOIN deal_rooms dr ON dr.id = r.room_id
WHERE r.workspace_id = $1 AND r.status = 'pending'
ORDER BY r.created_at DESC;

-- name: ListPendingRoomNDAsByWorkspace :many
SELECT m.id, m.email, m.room_id, dr.name AS room_name
FROM room_members m
JOIN deal_rooms dr ON dr.id = m.room_id
WHERE m.workspace_id = $1 AND m.nda_status = 'pending'
ORDER BY m.created_at DESC;

-- name: ListPendingLinkQuestionsByWorkspace :many
SELECT q.id, q.visitor_email, q.question, q.link_id, l.name AS link_name
FROM link_visitor_questions q
JOIN links l ON l.id = q.link_id
WHERE q.workspace_id = $1 AND q.status = 'pending'
ORDER BY q.created_at DESC;

-- name: ListPendingUploadedFilesByWorkspace :many
SELECT f.id, f.original_filename, f.link_id, l.name AS link_name
FROM link_uploaded_files f
JOIN links l ON l.id = f.link_id
WHERE f.workspace_id = $1 AND f.status = 'pending_review'
ORDER BY f.created_at DESC;

-- name: ListExpiringLinksByWorkspace :many
SELECT l.id, l.name
FROM links l
WHERE l.workspace_id = $1
  AND l.status = 'active'
  AND l.expires_at IS NOT NULL
  AND l.expires_at > now()
  AND l.expires_at <= now() + interval '7 days'
ORDER BY l.expires_at ASC;

-- name: ListExpiringRoomsByWorkspace :many
SELECT dr.id, dr.name
FROM deal_rooms dr
WHERE dr.workspace_id = $1
  AND dr.status = 'active'
  AND dr.deleted_at IS NULL
  AND dr.expires_at IS NOT NULL
  AND dr.expires_at > now()
  AND dr.expires_at <= now() + interval '7 days'
ORDER BY dr.expires_at ASC;

-- name: CountWeeklyVisitorsByWorkspace :one
SELECT COUNT(DISTINCT COALESCE(visitor_id, visitor_email)) AS visitor_count
FROM access_logs
WHERE workspace_id = $1
  AND created_at >= now() - interval '7 days';

-- name: CountPendingQuestionsByWorkspace :one
SELECT COUNT(*) AS pending_count
FROM link_visitor_questions
WHERE workspace_id = $1 AND status = 'pending';

-- name: ListRecentActivitiesByWorkspace :many
SELECT
    id,
    event_type,
    actor,
    object_type,
    object_name,
    object_id,
    created_at
FROM (
    SELECT
        al.id::text AS id,
        CASE al.event_type
            WHEN 'link_opened' THEN 'visit'
            ELSE 'download'
        END AS event_type,
        COALESCE(NULLIF(al.visitor_email, ''), al.visitor_id, 'Unknown') AS actor,
        CASE WHEN l.deal_room_id IS NOT NULL THEN 'room' ELSE 'document' END AS object_type,
        COALESCE(dr.name, d.title, 'Shared link') AS object_name,
        COALESCE(dr.id, d.id, l.id)::text AS object_id,
        al.created_at
    FROM access_logs al
    JOIN links l ON l.id = al.link_id
    LEFT JOIN deal_rooms dr ON dr.id = l.deal_room_id
    LEFT JOIN documents d ON d.id = l.document_id
    WHERE al.workspace_id = $1

    UNION ALL

    SELECT
        q.id::text AS id,
        'question' AS event_type,
        COALESCE(NULLIF(q.visitor_email, ''), q.visitor_id, 'Unknown') AS actor,
        CASE WHEN l.deal_room_id IS NOT NULL THEN 'room' ELSE 'document' END AS object_type,
        COALESCE(dr.name, d.title, 'Shared link') AS object_name,
        COALESCE(dr.id, d.id, l.id)::text AS object_id,
        q.created_at
    FROM link_visitor_questions q
    JOIN links l ON l.id = q.link_id
    LEFT JOIN deal_rooms dr ON dr.id = l.deal_room_id
    LEFT JOIN documents d ON d.id = l.document_id
    WHERE q.workspace_id = $1

    UNION ALL

    SELECT
        d.id::text AS id,
        'upload' AS event_type,
        COALESCE(NULLIF(u.email, ''), 'System') AS actor,
        'document' AS object_type,
        d.title AS object_name,
        d.id::text AS object_id,
        d.created_at
    FROM documents d
    LEFT JOIN users u ON u.id = d.created_by
    WHERE d.workspace_id = $1 AND d.deleted_at IS NULL
) combined
ORDER BY created_at DESC
LIMIT $2;

-- name: ListSuggestionsByWorkspace :many
SELECT *
FROM suggestions
WHERE workspace_id = $1 AND dismissed = false
ORDER BY created_at DESC;

-- name: ListUnsyncedSuggestionsByWorkspace :many
SELECT *
FROM suggestions
WHERE workspace_id = $1
  AND (synced_at IS NULL OR updated_at > synced_at)
ORDER BY created_at DESC;

-- name: ListSignalsBySuggestionIDs :many
SELECT *
FROM signals
WHERE suggestion_id = ANY($1::uuid[]);

-- name: MarkSuggestionsSynced :exec
UPDATE suggestions
SET synced_at = now()
WHERE id = ANY($1::uuid[]);

-- name: CreateSignalRuleRun :one
INSERT INTO signal_rule_run (
    tenant_id,
    workspace_id,
    link_id,
    run_started_at,
    duration_ms,
    input_snapshot,
    matched_rule_ids,
    generated_suggestion_ids,
    bucket_skipped_rule_ids,
    shadow_matched_rule_ids,
    error
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpsertLinkFeature :one
INSERT INTO link_features (
    tenant_id,
    workspace_id,
    link_id,
    window_start,
    opens,
    unique_visitors,
    revisits,
    avg_duration_seconds,
    total_page_views,
    key_page_views,
    downloads,
    bounces,
    distinct_ips_1h,
    distinct_emails_24h,
    unknown_emails_24h,
    downloads_24h
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
ON CONFLICT (link_id, window_start) DO UPDATE SET
    opens = EXCLUDED.opens,
    unique_visitors = EXCLUDED.unique_visitors,
    revisits = EXCLUDED.revisits,
    avg_duration_seconds = EXCLUDED.avg_duration_seconds,
    total_page_views = EXCLUDED.total_page_views,
    key_page_views = EXCLUDED.key_page_views,
    downloads = EXCLUDED.downloads,
    bounces = EXCLUDED.bounces,
    distinct_ips_1h = EXCLUDED.distinct_ips_1h,
    distinct_emails_24h = EXCLUDED.distinct_emails_24h,
    unknown_emails_24h = EXCLUDED.unknown_emails_24h,
    downloads_24h = EXCLUDED.downloads_24h,
    updated_at = now()
RETURNING *;

-- name: GetLinkFeature :one
SELECT *
FROM link_features
WHERE link_id = $1
ORDER BY window_start DESC
LIMIT 1;

-- name: ListStaleLinkFeatures :many
SELECT *
FROM link_features
WHERE updated_at < $1
ORDER BY updated_at ASC
LIMIT $2;

-- name: ListRecentlyActiveLinkIDs :many
SELECT DISTINCT link_id
FROM access_logs
WHERE created_at > now() - interval '1 hour'
ORDER BY link_id
LIMIT $1;

-- name: ListActionItemsBySignal :many
SELECT id, tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type, created_at, updated_at
FROM action_items
WHERE signal_id = $1
ORDER BY created_at DESC;

-- name: ListContactsByWorkspace :many
SELECT id, workspace_id, email, name, created_at
FROM contacts
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: GetContactByEmailAndWorkspace :one
SELECT id, workspace_id, email, name, created_at
FROM contacts
WHERE email = $1 AND workspace_id = $2
LIMIT 1;

-- name: GetContactAggregatesByWorkspace :many
SELECT
    c.id AS contact_id,
    LOWER(COALESCE(c.email, al.visitor_email)) AS email,
    COUNT(DISTINCT al.id) FILTER (WHERE al.event_type = 'link_opened') AS opens,
    COUNT(DISTINCT al.link_id) AS unique_links,
    COUNT(DISTINCT al.visitor_id) AS unique_visitors,
    COALESCE(SUM(pv.duration_seconds), 0)::bigint AS total_duration_seconds,
    COUNT(pv.id)::bigint AS total_page_views,
    COUNT(DISTINCT al.id) FILTER (WHERE al.event_type = 'download_attempted') AS downloads,
    MAX(al.created_at)::timestamptz AS last_seen_at
FROM access_logs al
LEFT JOIN contacts c ON c.email = al.visitor_email AND c.workspace_id = al.workspace_id
LEFT JOIN page_views pv ON pv.workspace_id = al.workspace_id AND pv.visitor_id = al.visitor_id
WHERE al.workspace_id = $1 AND al.visitor_email IS NOT NULL AND al.visitor_email <> ''
GROUP BY c.id, LOWER(COALESCE(c.email, al.visitor_email))
ORDER BY opens DESC
LIMIT $2;

-- name: GetContactByID :one
SELECT id, workspace_id, email, name, created_at
FROM contacts
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: UpsertContactByEmail :one
INSERT INTO contacts (workspace_id, email, name)
VALUES ($1, $2, NULLIF($3, ''))
ON CONFLICT (workspace_id, email) DO UPDATE SET
    name = COALESCE(EXCLUDED.name, contacts.name)
RETURNING id, workspace_id, email, name, created_at;

-- name: CreateDeal :one
INSERT INTO deals (workspace_id, contact_id, name, stage, amount, currency, status, close_date)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, workspace_id, contact_id, name, stage, amount, currency, status, close_date, created_at, updated_at;

-- name: ListDealsByWorkspace :many
SELECT id, workspace_id, contact_id, name, stage, amount, currency, status, close_date, created_at, updated_at
FROM deals
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: GetDealByID :one
SELECT id, workspace_id, contact_id, name, stage, amount, currency, status, close_date, created_at, updated_at
FROM deals
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: FindUnsyncedContactEmails :many
SELECT DISTINCT al.visitor_email AS email
FROM access_logs al
WHERE al.workspace_id = $1
  AND al.visitor_email IS NOT NULL
  AND al.visitor_email <> ''
  AND NOT EXISTS (
      SELECT 1 FROM contacts c
      WHERE c.workspace_id = al.workspace_id AND c.email = al.visitor_email
  );

-- name: GetContactAggregateByEmail :one
SELECT
    COUNT(DISTINCT al.id) FILTER (WHERE al.event_type = 'link_opened') AS opens,
    COUNT(DISTINCT al.link_id) AS unique_links,
    COUNT(DISTINCT al.visitor_id) AS unique_visitors,
    COALESCE(SUM(pv.duration_seconds), 0)::bigint AS total_duration_seconds,
    COUNT(pv.id)::bigint AS total_page_views,
    COUNT(DISTINCT al.id) FILTER (WHERE al.event_type = 'download_attempted') AS downloads,
    MAX(al.created_at)::timestamptz AS last_seen_at
FROM access_logs al
LEFT JOIN page_views pv ON pv.workspace_id = al.workspace_id AND pv.visitor_id = al.visitor_id
WHERE al.workspace_id = $1 AND al.visitor_email ILIKE $2;

-- name: ListContactActivitiesByEmail :many
WITH visitor_ids AS (
    SELECT DISTINCT al.visitor_id
    FROM access_logs al
    WHERE al.workspace_id = $1
      AND al.visitor_email ILIKE $2
      AND al.visitor_id IS NOT NULL
      AND al.visitor_id <> ''
)
SELECT
    e.id,
    e.link_id,
    e.event_type,
    COALESCE(e.page_number, 0)::int AS page_number,
    COALESCE(e.duration_seconds, 0)::int AS duration_seconds,
    e.created_at,
    l.document_id,
    COALESCE(d.title, '')::text AS document_title
FROM (
    SELECT
        id,
        link_id,
        event_type,
        NULL::int AS page_number,
        0 AS duration_seconds,
        created_at,
        visitor_id
    FROM access_logs al2
    WHERE al2.workspace_id = $1 AND al2.visitor_email ILIKE $2
    UNION ALL
    SELECT
        id,
        link_id,
        'page_viewed'::text AS event_type,
        page_number,
        duration_seconds,
        created_at,
        visitor_id
    FROM page_views pv2
    WHERE pv2.workspace_id = $1 AND pv2.visitor_id IN (SELECT visitor_id FROM visitor_ids)
) e
JOIN links l ON l.id = e.link_id
LEFT JOIN documents d ON d.id = l.document_id
ORDER BY e.created_at DESC
LIMIT $3;

-- name: ListContactViewedDocumentIDs :many
WITH visitor_ids AS (
    SELECT DISTINCT al.visitor_id
    FROM access_logs al
    WHERE al.workspace_id = $1
      AND al.visitor_email ILIKE $2
      AND al.visitor_id IS NOT NULL
      AND al.visitor_id <> ''
)
SELECT DISTINCT l.document_id::text AS document_id
FROM (
    SELECT link_id FROM access_logs al2
    WHERE al2.workspace_id = $1 AND al2.visitor_email ILIKE $2
    UNION
    SELECT link_id FROM page_views pv2
    WHERE pv2.workspace_id = $1 AND pv2.visitor_id IN (SELECT visitor_id FROM visitor_ids)
) e
JOIN links l ON l.id = e.link_id
WHERE l.document_id IS NOT NULL;

-- name: CreateContact :one
INSERT INTO contacts (workspace_id, email, name)
VALUES (sqlc.arg(workspace_id), sqlc.arg(email), NULLIF(sqlc.arg(name), ''))
RETURNING id, workspace_id, email, name, created_at;

-- name: CreateLinkContact :exec
INSERT INTO link_contacts (link_id, contact_id, access_code)
VALUES ($1, $2, $3);

-- name: DeleteLinkContactsByLink :exec
DELETE FROM link_contacts
WHERE link_id = $1;

-- name: ListLinkContactsByLinkID :many
SELECT lc.contact_id
FROM link_contacts lc
WHERE lc.link_id = $1;

-- name: GetLinkContactsByPublicToken :many
SELECT lc.id, lc.link_id, lc.contact_id, lc.access_code, lc.code_sent_at, lc.used_at, lc.created_at,
       c.email AS contact_email, c.name AS contact_name
FROM link_contacts lc
JOIN links l ON l.id = lc.link_id
JOIN contacts c ON c.id = lc.contact_id
WHERE l.public_token = $1;

-- name: GetLinkContactByEmail :one
SELECT lc.id, lc.link_id, lc.contact_id, lc.access_code, lc.code_sent_at, lc.used_at, lc.created_at,
       c.email AS contact_email, c.name AS contact_name
FROM link_contacts lc
JOIN links l ON l.id = lc.link_id
JOIN contacts c ON c.id = lc.contact_id
WHERE l.public_token = $1 AND c.email = $2
LIMIT 1;

-- name: GetLinkContactByCode :one
SELECT lc.id, lc.link_id, lc.contact_id, lc.access_code, lc.code_sent_at, lc.used_at, lc.created_at,
       c.email AS contact_email, c.name AS contact_name
FROM link_contacts lc
JOIN links l ON l.id = lc.link_id
JOIN contacts c ON c.id = lc.contact_id
WHERE l.public_token = $1 AND lc.access_code = $2
LIMIT 1;

-- name: UpdateLinkContactAccessCode :exec
UPDATE link_contacts
SET access_code = $2, code_sent_at = now(), used_at = NULL
WHERE id = $1;

-- name: CreateLinkDocument :exec
INSERT INTO link_documents (link_id, document_id, sort_order)
VALUES ($1, $2, $3);

-- name: ListLinkDocumentsByLink :many
SELECT ld.id, ld.link_id, ld.document_id, ld.sort_order, ld.created_at,
       COALESCE(d.title, ''::text) AS title,
       COALESCE(d.source_type, ''::text) AS source_type,
       COALESCE(d.page_count, 0)::int AS page_count,
       d.status,
       COALESCE(d.file_size, 0)::bigint AS file_size
FROM link_documents ld
JOIN documents d ON d.id = ld.document_id AND d.deleted_at IS NULL
WHERE ld.link_id = $1
ORDER BY ld.sort_order ASC, ld.created_at ASC;

-- name: ListLinkDocumentsByPublicToken :many
SELECT ld.id, ld.link_id, ld.document_id, ld.sort_order, ld.created_at,
       COALESCE(d.title, ''::text) AS title,
       COALESCE(d.source_type, ''::text) AS source_type,
       COALESCE(d.page_count, 0)::int AS page_count,
       d.status,
       COALESCE(d.file_size, 0)::bigint AS file_size
FROM link_documents ld
JOIN links l ON l.id = ld.link_id
JOIN documents d ON d.id = ld.document_id AND d.deleted_at IS NULL
WHERE l.public_token = $1
ORDER BY ld.sort_order ASC, ld.created_at ASC;

-- name: DeleteLinkDocumentsByLink :exec
DELETE FROM link_documents
WHERE link_id = $1;

-- name: HasLinkDocument :one
SELECT EXISTS(
  SELECT 1 FROM link_documents
  WHERE link_id = $1 AND document_id = $2
) AS exists;

-- name: GetDocumentByIDForLink :one
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE id = $1 AND workspace_id = $2 LIMIT 1;

-- name: CreateSecurityEvent :exec
INSERT INTO security_events (tenant_id, workspace_id, link_id, event_type, visitor_id, email, ip, user_agent, reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: CountSecurityEventsByIPAndWindow :one
SELECT COUNT(*) AS count
FROM security_events
WHERE ip = $1
  AND event_type = $2
  AND created_at > now() - ($3)::interval;

-- name: ListSecurityEventsByLink :many
SELECT id, link_id, event_type, visitor_id, email, ip, user_agent, reason, created_at
FROM security_events
WHERE link_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListRecentSecurityEventsByLink :many
SELECT id, link_id, event_type, visitor_id, email, ip, user_agent, reason, created_at
FROM security_events
WHERE link_id = $1
  AND created_at > now() - interval '24 hours'
ORDER BY created_at DESC;

-- name: CreateEmailLog :one
INSERT INTO email_logs (recipient, email_type, provider, status, subject, workspace_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateEmailLogStatus :exec
UPDATE email_logs
SET status = $2, provider_message_id = $3, error_message = $4, updated_at = NOW()
WHERE id = $1;

-- name: UpdateEmailLogStatusByProviderMessageID :exec
UPDATE email_logs
SET status = $2, updated_at = NOW()
WHERE provider_message_id = $1;

-- name: GetEmailLogByID :one
SELECT * FROM email_logs WHERE id = $1 LIMIT 1;

-- name: GetEmailLogByProviderMessageID :one
SELECT * FROM email_logs WHERE provider_message_id = $1 LIMIT 1;

-- name: CreateEmailEvent :exec
INSERT INTO email_events (email_log_id, event_type, user_agent, ip_address, link_url)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING;

-- name: CountEmailEventsByLogID :many
SELECT event_type, COUNT(*) AS count
FROM email_events
WHERE email_log_id = $1
GROUP BY event_type;

-- name: ListLinksByDealRoom :many
SELECT *
FROM links
WHERE workspace_id = $1 AND deal_room_id = $2 AND status NOT IN ('deleted', 'disabled')
ORDER BY created_at DESC;

-- name: CreateLinkAccessRule :exec
INSERT INTO link_access_rules (
    tenant_id, workspace_id, link_id, rule_type, value, action, sort_order
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (link_id, rule_type, value, action) DO NOTHING;

-- name: DeleteLinkAccessRulesByLink :exec
DELETE FROM link_access_rules
WHERE link_id = $1;

-- name: ListLinkAccessRulesByLink :many
SELECT id, tenant_id, workspace_id, link_id, rule_type, value, action, sort_order, created_at, updated_at
FROM link_access_rules
WHERE link_id = $1
ORDER BY action DESC, sort_order ASC, created_at ASC;

-- name: InsertLinkAccessRuleRevision :exec
INSERT INTO link_access_rule_revisions (
    tenant_id, workspace_id, link_id, changed_by, rules_snapshot
) VALUES ($1, $2, $3, $4, $5);

-- name: CreateLinkInvitation :one
INSERT INTO link_invitations (
    tenant_id, workspace_id, link_id, email, token, token_hash, status, expires_at, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, tenant_id, workspace_id, link_id, email, token, token_hash, status, expires_at, used_at, created_by, created_at, updated_at;

-- name: GetLinkInvitationByToken :one
SELECT id, tenant_id, workspace_id, link_id, email, token, token_hash, status, expires_at, used_at, created_by, created_at, updated_at
FROM link_invitations
WHERE token_hash = $1 OR (token_hash IS NULL AND token = $1)
LIMIT 1;

-- name: UpdateLinkInvitationTokenHash :exec
UPDATE link_invitations
SET token_hash = $1,
    updated_at = now()
WHERE id = $2;

-- name: GetLinkInvitationByLinkAndEmail :one
SELECT id, tenant_id, workspace_id, link_id, email, token, status, expires_at, used_at, created_by, created_at, updated_at
FROM link_invitations
WHERE link_id = $1 AND email = $2
LIMIT 1;

-- name: UpdateLinkInvitationStatus :one
UPDATE link_invitations
SET status = $1, used_at = $2, updated_at = now()
WHERE id = $3
RETURNING id, tenant_id, workspace_id, link_id, email, token, status, expires_at, used_at, created_by, created_at, updated_at;

-- name: ListLinkInvitationsByLink :many
SELECT id, tenant_id, workspace_id, link_id, email, token, status, expires_at, used_at, created_by, created_at, updated_at
FROM link_invitations
WHERE link_id = $1
ORDER BY created_at DESC;

-- name: TouchLinkUpdatedAt :exec
UPDATE links
SET updated_at = now()
WHERE id = $1;

-- name: GetLinkInvitationByID :one
SELECT id, tenant_id, workspace_id, link_id, email, token, status, expires_at, used_at, created_by, created_at, updated_at
FROM link_invitations
WHERE id = $1
LIMIT 1;

-- name: DeleteLinkAccessRuleByLinkAndValue :exec
DELETE FROM link_access_rules
WHERE link_id = $1 AND rule_type = $2 AND value = $3 AND action = $4;

-- name: ResetLinkInvitation :one
UPDATE link_invitations
SET token = $1,
    token_hash = $2,
    status = 'pending',
    expires_at = $3,
    used_at = NULL,
    updated_at = now()
WHERE id = $4
RETURNING id, tenant_id, workspace_id, link_id, email, token, token_hash, status, expires_at, used_at, created_by, created_at, updated_at;

-- name: CreateLinkAccessRequest :one
INSERT INTO link_access_requests (
    tenant_id, workspace_id, link_id, email, reason, status
) VALUES ($1, $2, $3, $4, $5, 'pending')
RETURNING *;

-- name: GetLinkAccessRequestByID :one
SELECT *
FROM link_access_requests
WHERE id = $1
LIMIT 1;

-- name: GetLinkAccessRequestByLinkAndEmail :one
SELECT *
FROM link_access_requests
WHERE link_id = $1 AND email = $2
LIMIT 1;

-- name: ListLinkAccessRequestsByLink :many
SELECT *
FROM link_access_requests
WHERE link_id = $1
ORDER BY created_at DESC;

-- name: CountPendingLinkAccessRequestsByLinkAndEmail :one
SELECT COUNT(*)
FROM link_access_requests
WHERE link_id = $1 AND email = $2 AND status = 'pending';

-- name: UpdateLinkAccessRequestStatus :one
UPDATE link_access_requests
SET status = $1,
    reviewed_by = $2,
    reviewed_at = now(),
    updated_at = now()
WHERE id = $3
RETURNING *;

-- name: CreateVisitorQuestion :one
INSERT INTO link_visitor_questions (
    tenant_id, workspace_id, link_id, visitor_id, visitor_email, question
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListVisitorQuestionsByLink :many
SELECT * FROM link_visitor_questions
WHERE link_id = $1
ORDER BY created_at DESC;

-- name: ListVisitorQuestionsByVisitor :many
SELECT * FROM link_visitor_questions
WHERE link_id = $1 AND visitor_id = $2
ORDER BY created_at DESC;

-- name: AnswerVisitorQuestion :one
UPDATE link_visitor_questions
SET answer = $1, answered_by = $2, status = 'answered', updated_at = now()
WHERE id = $3 AND workspace_id = $4
RETURNING *;

-- name: GetVisitorQuestionByID :one
SELECT *
FROM link_visitor_questions
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: UpdateVisitorQuestionAnswer :exec
UPDATE link_visitor_questions
SET answer = $1, answered_by = $2, status = 'answered', updated_at = now()
WHERE id = $3;

-- name: CreateFileRequest :one
INSERT INTO link_file_requests (
    tenant_id, workspace_id, link_id, visitor_id, visitor_email, message
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListFileRequestsByLink :many
SELECT * FROM link_file_requests
WHERE link_id = $1
ORDER BY created_at DESC;

-- name: ListFileRequestsByVisitor :many
SELECT * FROM link_file_requests
WHERE link_id = $1 AND visitor_id = $2
ORDER BY created_at DESC;

-- name: CountPendingFileRequests :one
SELECT COUNT(*) AS count
FROM link_file_requests
WHERE link_id = $1 AND visitor_id = $2 AND status = 'pending';

-- name: UpdateFileRequestStatus :exec
UPDATE link_file_requests
SET status = $1, updated_at = now()
WHERE id = $2;

-- name: CountPendingFileRequestsByVisitor :one
SELECT COUNT(*)::int
FROM link_file_requests
WHERE link_id = $1 AND visitor_id = $2 AND status = 'pending';

-- name: GetFileRequestByID :one
SELECT * FROM link_file_requests
WHERE id = $1
LIMIT 1;

-- name: ListNotificationRulesByWorkspace :many
SELECT * FROM notification_rules
WHERE workspace_id = $1
ORDER BY rule_type;

-- name: UpsertNotificationRule :one
INSERT INTO notification_rules (tenant_id, workspace_id, rule_type, channels, enabled, unsubscribable, merge_window_minutes)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (workspace_id, rule_type) DO UPDATE SET
    channels = EXCLUDED.channels,
    enabled = EXCLUDED.enabled,
    merge_window_minutes = EXCLUDED.merge_window_minutes,
    updated_at = now()
RETURNING *;

-- name: DeleteNotificationRule :exec
DELETE FROM notification_rules
WHERE workspace_id = $1 AND rule_type = $2;

-- name: FindMergeableNotification :one
SELECT * FROM notifications
WHERE workspace_id = $1
  AND channel = $2
  AND status = 'pending'
  AND subject ILIKE $3
  AND created_at > now() - ($4 || ' minutes')::interval
  AND metadata ->> 'link_id' = $5::text
ORDER BY created_at DESC
LIMIT 1;

-- name: ListLinksExpiringWithin :many
SELECT * FROM links
WHERE status = 'active'
  AND expires_at IS NOT NULL
  AND expires_at > now()
  AND expires_at <= now() + ($1 || ' hours')::interval
  AND (last_reminder_sent_at IS NULL OR last_reminder_sent_at < now() - interval '23 hours')
ORDER BY expires_at ASC;

-- name: UpdateLinkLastReminderSent :exec
UPDATE links
SET last_reminder_sent_at = now(), updated_at = now()
WHERE id = $1;

-- name: GetVisitorFirstAccess :one
SELECT MIN(created_at)::timestamptz AS first_accessed_at
FROM access_logs
WHERE link_id = $1 AND visitor_id = $2 AND event_type = 'link_opened';

-- name: CountVisitorAccesses :one
SELECT COUNT(*)::int
FROM access_logs
WHERE link_id = $1 AND visitor_id = $2 AND event_type = 'link_opened';

-- name: UpsertLinkIndexFile :one
INSERT INTO link_index_files (tenant_id, workspace_id, link_id, status, content_html)
VALUES ($1, $2, $3, 'generating', NULL)
ON CONFLICT (link_id) DO UPDATE SET
    status = 'generating',
    content_html = NULL,
    error_message = NULL,
    updated_at = now()
RETURNING *;

-- name: GetLinkIndexFileByLink :one
SELECT * FROM link_index_files
WHERE link_id = $1;

-- name: UpdateLinkIndexFileReady :exec
UPDATE link_index_files
SET status = 'ready', content_html = $1, generated_at = now(), updated_at = now()
WHERE link_id = $2;

-- name: UpdateLinkIndexFileFailed :exec
UPDATE link_index_files
SET status = 'failed', error_message = $1, updated_at = now()
WHERE link_id = $2;

-- name: CreateUploadedFile :one
INSERT INTO link_uploaded_files (tenant_id, workspace_id, link_id, original_filename, storage_key, file_size, mime_type, uploader_email, uploader_visitor_id, uploader_ip, uploader_user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListUploadedFilesByLink :many
SELECT * FROM link_uploaded_files
WHERE link_id = $1
ORDER BY created_at DESC;

-- name: UpdateUploadedFileStatus :exec
UPDATE link_uploaded_files
SET status = $1, reviewed_by = $2, reviewed_at = now()
WHERE id = $3;

-- name: GetUploadedFileByID :one
SELECT * FROM link_uploaded_files
WHERE id = $1;

-- name: DeleteAccessLogsBefore :execrows
DELETE FROM access_logs
WHERE created_at < $1;

-- name: DeletePageViewsBefore :execrows
DELETE FROM page_views
WHERE created_at < $1;

-- name: DeleteSecurityEventsBefore :execrows
DELETE FROM security_events
WHERE created_at < $1;

-- name: UpdateQuestionIntentTag :exec
UPDATE link_visitor_questions
SET intent_tag = $1
WHERE id = $2;

-- name: UpsertCrmSyncState :exec
INSERT INTO crm_sync_state (workspace_id, event_min, event_max, contact_email, link_id, event_types, summary, pushed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (workspace_id, link_id, contact_email, event_min) DO UPDATE SET
    event_max = $3, event_types = $6, summary = $7, pushed_at = now();

-- name: GetLastCrmSyncTime :one
SELECT COALESCE(MAX(event_max), '1970-01-01'::timestamptz)::timestamptz AS last_sync
FROM crm_sync_state
WHERE workspace_id = $1;

-- name: ListWorkspacesWithCrmEnabled :many
SELECT id, crm_config, webhook_secret FROM workspaces
WHERE crm_config->>'syncEnabled' = 'true';

-- name: GetUnsyncedCrmEvents :many
SELECT
    al.link_id,
    al.visitor_email AS contact_email,
    al.event_type,
    al.created_at AS event_time,
    l.name AS link_name,
    CASE al.event_type
        WHEN 'link_opened' THEN 'Opened link: ' || l.name
        WHEN 'file_downloaded' THEN 'Downloaded file from: ' || l.name
        ELSE al.event_type || ' on ' || l.name
    END AS event_summary
FROM access_logs al
JOIN links l ON l.id = al.link_id
WHERE al.workspace_id = $1
  AND al.created_at > $2
  AND al.visitor_email != ''
ORDER BY al.visitor_email, al.link_id, al.created_at;

-- name: ListDormantLinks :many
WITH link_activity AS (
    SELECT
        l.id, l.workspace_id, l.name, l.created_by,
        MAX(al.created_at) AS last_active_at,
        COUNT(*) FILTER (WHERE al.created_at > NOW() - INTERVAL '30 days')::bigint AS recent_events,
        MAX(daily.cnt)::bigint AS peak_daily_events,
        bool_or(al.event_type = 'forward_visited') AS was_forwarded,
        bool_or(al.event_type = 'file_downloaded') AS had_downloads
    FROM links l
    JOIN access_logs al ON al.link_id = l.id
    JOIN (
        SELECT link_id, DATE(created_at) AS day, COUNT(*)::bigint AS cnt
        FROM access_logs WHERE created_at > NOW() - INTERVAL '30 days'
        GROUP BY link_id, DATE(created_at)
    ) daily ON daily.link_id = l.id
    WHERE l.status = 'active'
      AND l.workspace_id = $1
    GROUP BY l.id, l.workspace_id, l.name, l.created_by
    HAVING MAX(al.created_at) < NOW() - INTERVAL '7 days'
       AND MAX(al.created_at) > NOW() - INTERVAL '30 days'
)
SELECT id, workspace_id, name, created_by, last_active_at, recent_events,
       peak_daily_events, was_forwarded, had_downloads
FROM link_activity
ORDER BY (peak_daily_events * (1.0 + EXTRACT(DAY FROM NOW() - last_active_at) / 7.0)) DESC
LIMIT 20;

-- name: CreateSuggestionFeedback :one
INSERT INTO suggestion_feedback (tenant_id, workspace_id, suggestion_id, feedback_type)
VALUES ($1, $2, $3, $4)
ON CONFLICT (suggestion_id, feedback_type) DO NOTHING
RETURNING *;

-- name: GetRulePerformanceSummary :many
-- Per-rule calibration metrics for a workspace.
SELECT
    s.rule_id,
    COUNT(*) FILTER (WHERE s.id IS NOT NULL) AS generated_count,
    COUNT(DISTINCT f_dismissed.suggestion_id) AS dismissed_count,
    COUNT(DISTINCT f_acted.suggestion_id) AS acted_count,
    COUNT(DISTINCT f_spam.suggestion_id) AS spam_count
FROM suggestions s
LEFT JOIN suggestion_feedback f_dismissed
    ON f_dismissed.suggestion_id = s.id AND f_dismissed.feedback_type = 'dismissed'
LEFT JOIN suggestion_feedback f_acted
    ON f_acted.suggestion_id = s.id AND f_acted.feedback_type = 'acted'
LEFT JOIN suggestion_feedback f_spam
    ON f_spam.suggestion_id = s.id AND f_spam.feedback_type = 'spam'
WHERE s.workspace_id = $1
  AND s.rule_id IS NOT NULL
  AND s.rule_id <> ''
GROUP BY s.rule_id
ORDER BY generated_count DESC;

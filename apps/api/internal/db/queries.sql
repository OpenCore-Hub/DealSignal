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
    id, tenant_id, workspace_id, created_by, title, source_type, status, storage_key, file_size
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, page_count, created_at, updated_at, deleted_at;

-- name: GetDocumentByID :one
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL
LIMIT 1;

-- name: ListDocumentsByWorkspace :many
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: ListRecentDocumentsByWorkspace :many
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2;

-- name: UpdateDocumentStatus :exec
UPDATE documents
SET status = $1, page_count = $2, updated_at = now()
WHERE id = $3;

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
INSERT INTO pages (tenant_id, workspace_id, document_id, page_number, image_object_key, width, height)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, tenant_id, workspace_id, document_id, page_number, image_object_key, width, height, created_at;

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
INSERT INTO assistant_sessions (workspace_id, user_id, title)
VALUES ($1, $2, $3)
RETURNING id, workspace_id, user_id, link_id, document_id, title, created_at, updated_at;

-- name: GetAssistantSession :one
SELECT id, workspace_id, user_id, link_id, document_id, title, created_at, updated_at
FROM assistant_sessions
WHERE id = $1 AND workspace_id = $2 AND user_id = $3
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
    tenant_id, workspace_id, document_id, public_token, name, permission_type,
    allowed_emails, allowed_domains, password_hash, expires_at, max_access_count,
    download_enabled, watermark_enabled, status, created_by,
    require_email, require_password, require_nda
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
RETURNING id, tenant_id, workspace_id, document_id, public_token, name, permission_type,
          allowed_emails, allowed_domains, password_hash, expires_at, max_access_count,
          access_count, download_enabled, watermark_enabled, status, created_by, created_at,
          updated_at, require_email, require_password, require_nda;

-- name: GetLinkByIDAndWorkspace :one
SELECT id, tenant_id, workspace_id, document_id, public_token, name, permission_type,
       allowed_emails, allowed_domains, password_hash, expires_at, max_access_count,
       access_count, download_enabled, watermark_enabled, status, created_by, created_at,
       updated_at, require_email, require_password, require_nda
FROM links
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: GetLinkByPublicToken :one
SELECT id, tenant_id, workspace_id, document_id, public_token, name, permission_type,
       allowed_emails, allowed_domains, password_hash, expires_at, max_access_count,
       access_count, download_enabled, watermark_enabled, status, created_by, created_at,
       updated_at, require_email, require_password, require_nda
FROM links
WHERE public_token = $1
LIMIT 1;

-- name: IncrementLinkAccessCount :exec
UPDATE links
SET access_count = access_count + 1, updated_at = now()
WHERE id = $1;

-- name: ListLinksByWorkspace :many
SELECT id, tenant_id, workspace_id, document_id, public_token, name, permission_type,
       allowed_emails, allowed_domains, password_hash, expires_at, max_access_count,
       access_count, download_enabled, watermark_enabled, status, created_by, created_at,
       updated_at, require_email, require_password, require_nda
FROM links
WHERE workspace_id = $1 AND status != 'deleted'
ORDER BY created_at DESC;

-- name: ListRecentLinksByWorkspace :many
SELECT id, tenant_id, workspace_id, document_id, public_token, name, permission_type,
       allowed_emails, allowed_domains, password_hash, expires_at, max_access_count,
       access_count, download_enabled, watermark_enabled, status, created_by, created_at,
       updated_at, require_email, require_password, require_nda
FROM links
WHERE workspace_id = $1 AND status != 'deleted'
ORDER BY created_at DESC
LIMIT $2;

-- name: ListLinksByDocument :many
SELECT id, tenant_id, workspace_id, document_id, public_token, name, permission_type,
       allowed_emails, allowed_domains, password_hash, expires_at, max_access_count,
       access_count, download_enabled, watermark_enabled, status, created_by, created_at,
       updated_at, require_email, require_password, require_nda
FROM links
WHERE workspace_id = $1 AND document_id = $2 AND status != 'deleted'
ORDER BY created_at DESC;

-- name: UpdateLinkStatus :one
UPDATE links
SET status = $1, updated_at = now()
WHERE id = $2 AND workspace_id = $3
RETURNING id, tenant_id, workspace_id, document_id, public_token, name, permission_type,
          allowed_emails, allowed_domains, password_hash, expires_at, max_access_count,
          access_count, download_enabled, watermark_enabled, status, created_by, created_at,
       updated_at, require_email, require_password, require_nda;

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
    SET access_count = access_count + 1, updated_at = now()
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
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetLinkAccessMetrics :one
SELECT
    COUNT(*) FILTER (WHERE event_type = 'link_opened') AS opens,
    COUNT(DISTINCT visitor_id) FILTER (WHERE event_type = 'link_opened') AS unique_visitors,
    COUNT(*) FILTER (WHERE event_type = 'download_attempted') AS downloads
FROM access_logs
WHERE link_id = $1;

-- name: GetLinkPageViewMetrics :one
SELECT
    COALESCE(AVG(duration_seconds), 0)::float8 AS avg_duration_seconds,
    COUNT(*) FILTER (WHERE duration_seconds >= 3) AS key_page_views,
    COUNT(*) AS total_page_views
FROM page_views
WHERE link_id = $1;

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
        NULL::inet AS ip,
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
ORDER BY e.created_at DESC;

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

-- name: ListAccessLogsByWorkspace :many
SELECT id, tenant_id, workspace_id, link_id, visitor_id, visitor_email, event_type, ip, user_agent, created_at
FROM access_logs
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: ListPageViewsByWorkspace :many
SELECT id, tenant_id, workspace_id, link_id, visitor_id, page_number, duration_seconds, scroll_depth, created_at
FROM page_views
WHERE workspace_id = $1
ORDER BY created_at DESC;

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
    COALESCE(LEFT(c.text, 80), '')::text AS title
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
RETURNING id, tenant_id, workspace_id, slug, name, description, template_type, settings,
          requires_nda, requires_approval, status, created_by, created_at, updated_at, deleted_at;

-- name: GetDealRoomByID :one
SELECT id, tenant_id, workspace_id, slug, name, description, template_type, settings,
       requires_nda, requires_approval, status, created_by, created_at, updated_at, deleted_at
FROM deal_rooms
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL
LIMIT 1;

-- name: GetDealRoomBySlug :one
SELECT id, tenant_id, workspace_id, slug, name, description, template_type, settings,
       requires_nda, requires_approval, status, created_by, created_at, updated_at, deleted_at
FROM deal_rooms
WHERE slug = $1 AND status = 'active' AND deleted_at IS NULL
LIMIT 1;

-- name: ListDealRoomsByWorkspace :many
SELECT id, tenant_id, workspace_id, slug, name, description, template_type, settings,
       requires_nda, requires_approval, status, created_by, created_at, updated_at, deleted_at
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

-- name: AddDealRoomDocument :one
INSERT INTO deal_room_documents (tenant_id, workspace_id, room_id, document_id, folder_path, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, tenant_id, workspace_id, room_id, document_id, folder_path, sort_order, created_at;

-- name: ListDealRoomDocuments :many
SELECT id, tenant_id, workspace_id, room_id, document_id, folder_path, sort_order, created_at
FROM deal_room_documents
WHERE room_id = $1
ORDER BY folder_path, sort_order;

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
INSERT INTO suggestions (tenant_id, workspace_id, contact_id, link_id, document_id, type, reason, action)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, tenant_id, workspace_id, contact_id, link_id, document_id, type, reason, action, dismissed, created_at, updated_at;

-- name: ListSuggestionsByLink :many
SELECT id, tenant_id, workspace_id, contact_id, link_id, document_id, type, reason, action, dismissed, created_at, updated_at
FROM suggestions
WHERE link_id = $1 AND workspace_id = $2 AND dismissed = false
ORDER BY created_at DESC;

-- name: CountRecentSuggestionsByLinkAndType :one
SELECT COUNT(*) AS count
FROM suggestions
WHERE link_id = $1 AND workspace_id = $2 AND type = $3 AND dismissed = false AND created_at > now() - interval '24 hours';

-- name: DismissSuggestion :exec
UPDATE suggestions
SET dismissed = true, updated_at = now()
WHERE id = $1 AND workspace_id = $2;

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
INSERT INTO notifications (workspace_id, user_id, channel, subject, body)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, workspace_id, user_id, channel, subject, body, status, attempts, last_error, created_at, updated_at;

-- name: ListPendingNotifications :many
SELECT id, workspace_id, user_id, channel, subject, body, status, attempts, last_error, created_at, updated_at
FROM notifications
WHERE status = 'pending' AND attempts < 3
ORDER BY created_at ASC
LIMIT 100;

-- name: MarkNotificationSent :exec
UPDATE notifications
SET status = 'sent', attempts = attempts + 1, updated_at = now()
WHERE id = $1;

-- name: MarkNotificationFailed :exec
UPDATE notifications
SET attempts = attempts + 1,
    last_error = $2,
    status = CASE WHEN attempts + 1 >= 3 THEN 'failed' ELSE 'pending' END,
    updated_at = now()
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
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE id = $1 AND workspace_id = $2 AND tenant_id = $3 AND deleted_at IS NULL
LIMIT 1;

-- name: CreateSignal :one
INSERT INTO signals (
    tenant_id, workspace_id, suggestion_id, type, title, description, explanation, suggestion,
    document_id, contact_id, link_id, priority
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, tenant_id, workspace_id, suggestion_id, type, title, description, explanation, suggestion,
          document_id, contact_id, link_id, priority, created_at, updated_at;

-- name: GetSignalBySuggestion :one
SELECT id, tenant_id, workspace_id, suggestion_id, type, title, description, explanation, suggestion,
       document_id, contact_id, link_id, priority, created_at, updated_at
FROM signals
WHERE suggestion_id = $1 AND workspace_id = $2 LIMIT 1;

-- name: ListSignalsByWorkspace :many
SELECT id, tenant_id, workspace_id, suggestion_id, type, title, description, explanation, suggestion,
       document_id, contact_id, link_id, priority, created_at, updated_at
FROM signals
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: CreateActionItem :one
INSERT INTO action_items (
    tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type, created_at, updated_at;

-- name: ListActionItemsByWorkspace :many
SELECT id, tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type, created_at, updated_at
FROM action_items
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: GetActionItemByID :one
SELECT id, tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type, created_at, updated_at
FROM action_items
WHERE id = $1 AND workspace_id = $2 LIMIT 1;

-- name: UpdateActionItemStatus :one
UPDATE action_items
SET status = $1, updated_at = now()
WHERE id = $2 AND workspace_id = $3
RETURNING id, tenant_id, workspace_id, signal_id, title, impact, due_at, status, action_type, created_at, updated_at;

-- name: ListSuggestionsByWorkspace :many
SELECT id, tenant_id, workspace_id, contact_id, link_id, document_id, type, reason, action, dismissed, created_at, updated_at
FROM suggestions
WHERE workspace_id = $1 AND dismissed = false
ORDER BY created_at DESC;

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

-- name: GetContactAggregatesByWorkspace :many
SELECT
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
GROUP BY LOWER(COALESCE(c.email, al.visitor_email))
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

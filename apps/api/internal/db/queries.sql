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

-- name: GetDocumentByTitleInWorkspace :one
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE workspace_id = $1 AND title = $2 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: ReplaceDocumentFile :one
UPDATE documents
SET storage_key = $1,
    file_size = $2,
    source_type = $3,
    status = 'uploaded',
    page_count = NULL,
    category = $6,
    updated_at = now()
WHERE id = $4 AND workspace_id = $5 AND deleted_at IS NULL
RETURNING id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at;

-- name: ResetIngestionJobByDocument :exec
UPDATE ingestion_jobs
SET status = 'queued', attempts = 0, error_message = NULL, updated_at = now()
WHERE document_id = $1;

-- name: ListDocumentIDsInDealRoomsByWorkspace :many
SELECT DISTINCT drd.document_id
FROM deal_room_documents drd
JOIN documents d ON d.id = drd.document_id AND d.deleted_at IS NULL
WHERE drd.workspace_id = $1;

-- name: CountDealRoomMembershipsByDocument :one
SELECT COUNT(*)::bigint AS count
FROM deal_room_documents
WHERE document_id = $1;

-- name: ListDocumentsByWorkspace :many
SELECT id, tenant_id, workspace_id, created_by, COALESCE(title, ''::text) as title, source_type, status, storage_key, COALESCE(file_size, 0::bigint) as file_size, category, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: ListRecentlyAccessedDocumentsByWorkspace :many
-- Aggregate last-access per document first to avoid docs × links × access_logs explosion.
WITH last_access AS (
    SELECT
        l.document_id,
        MAX(al.created_at) AS last_accessed_at
    FROM access_logs al
    JOIN links l ON l.id = al.link_id
        AND l.status = 'active'
        AND l.document_id IS NOT NULL
    WHERE al.workspace_id = $1
    GROUP BY l.document_id
)
SELECT
    d.id, d.tenant_id, d.workspace_id, d.created_by, COALESCE(d.title, ''::text) as title, d.source_type, d.status, d.storage_key, COALESCE(d.file_size, 0::bigint) as file_size, d.category, d.page_count, d.created_at, d.updated_at, d.deleted_at,
    la.last_accessed_at::timestamptz AS last_accessed_at
FROM documents d
JOIN last_access la ON la.document_id = d.id
WHERE d.workspace_id = $1 AND d.deleted_at IS NULL AND d.status != 'archived'
ORDER BY la.last_accessed_at DESC, d.created_at DESC;

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

-- name: ListSharedDocumentsByWorkspace :many
SELECT d.id, d.tenant_id, d.workspace_id, d.created_by, COALESCE(d.title, ''::text) as title, d.source_type, d.status, d.storage_key, COALESCE(d.file_size, 0::bigint) as file_size, d.category, d.page_count, d.created_at, d.updated_at, d.deleted_at
FROM documents d
WHERE d.workspace_id = $1 AND d.deleted_at IS NULL AND d.status != 'archived'
  AND EXISTS (SELECT 1 FROM links l WHERE l.document_id = d.id AND l.status = 'active')
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

-- name: SoftDeleteLinksByDocument :execrows
UPDATE links
SET status = 'deleted', updated_at = now()
WHERE workspace_id = $1 AND document_id = $2 AND status <> 'deleted';

-- name: SoftDeleteOrphanScopedLinksForDocument :execrows
-- Soft-delete multi-doc links whose only remaining scoped member is this document.
UPDATE links l
SET status = 'deleted', updated_at = now()
WHERE l.workspace_id = $1
  AND l.status <> 'deleted'
  AND EXISTS (
      SELECT 1 FROM link_documents ld WHERE ld.link_id = l.id AND ld.document_id = $2
  )
  AND NOT EXISTS (
      SELECT 1 FROM link_documents ld2
      WHERE ld2.link_id = l.id AND ld2.document_id <> $2
  );

-- name: DeleteDealRoomDocumentsByDocument :exec
DELETE FROM deal_room_documents
WHERE workspace_id = $1 AND document_id = $2;

-- name: DeleteLinkDocumentsByDocument :exec
DELETE FROM link_documents
WHERE document_id = $1;

-- name: GetDocumentDeleteImpact :one
SELECT
    (
        (SELECT COUNT(*)::bigint
         FROM links l1
         WHERE l1.workspace_id = sqlc.arg(workspace_id)
           AND l1.document_id = sqlc.arg(document_id)
           AND l1.status NOT IN ('deleted', 'disabled'))
        +
        (SELECT COUNT(*)::bigint
         FROM link_documents ld
         JOIN links l2 ON l2.id = ld.link_id
         WHERE ld.document_id = sqlc.arg(document_id)
           AND l2.workspace_id = sqlc.arg(workspace_id)
           AND l2.status NOT IN ('deleted', 'disabled')
           AND (l2.document_id IS NULL OR l2.document_id <> sqlc.arg(document_id)))
    )::bigint AS active_link_count,
    (SELECT COUNT(*)::bigint
     FROM deal_room_documents drd
     WHERE drd.workspace_id = sqlc.arg(workspace_id)
       AND drd.document_id = sqlc.arg(document_id))::bigint AS deal_room_count;

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

-- name: GetPageTitleByDocumentAndNumber :one
-- Title used for heat / key-page matching (empty when missing).
SELECT COALESCE(NULLIF(TRIM(title), ''), '')::text AS title
FROM pages
WHERE document_id = $1 AND page_number = $2
LIMIT 1;

-- name: CreateLink :one
INSERT INTO links (
    tenant_id, workspace_id, document_id, deal_room_id, public_token, name, permission_type, expires_at, max_access_count,
    download_enabled, watermark_enabled, status, created_by,
    require_email, require_nda, require_email_verification,
    require_password, password_hash,
    qa_enabled, file_requests_enabled, index_file_enabled, screenshot_protection_enabled,
    link_type, target_folder_path,
    custom_domain, tags, notify_on_access,
    has_document_scope, folder_scope_paths, folder_scope_mode, nda_document_id, nda_template_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32)
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
-- All non-deleted workspace links (document + deal-room). Used by analytics/billing.
SELECT *
FROM links
WHERE workspace_id = $1 AND status NOT IN ('deleted', 'disabled')
ORDER BY created_at DESC;

-- name: ListDocumentLinksByWorkspace :many
-- Document Library share list: document links only (never deal-room shares).
SELECT *
FROM links
WHERE workspace_id = $1
  AND deal_room_id IS NULL
  AND document_id IS NOT NULL
  AND status NOT IN ('deleted', 'disabled')
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
WHERE workspace_id = $1
  AND document_id = $2
  AND deal_room_id IS NULL
  AND status NOT IN ('deleted', 'disabled')
ORDER BY created_at DESC;

-- name: UpdateLinkStatus :one
UPDATE links
SET status = $1, updated_at = now()
WHERE id = $2 AND workspace_id = $3
RETURNING *;

-- name: UpdateLinkDownloadEnabled :one
UPDATE links
SET download_enabled = $1, updated_at = now()
WHERE id = $2 AND workspace_id = $3
  AND status NOT IN ('deleted', 'disabled')
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
    nda_document_id = $12,
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
    folder_scope_mode = $27,
    updated_at = now()
WHERE id = $28 AND workspace_id = $29
RETURNING *;



-- name: UpdateLinkAskPolicy :one
UPDATE links SET
    ask_mode = $3,
    ask_ai_enabled = $4,
    ask_ai_monthly_quota = $5,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;



-- name: SetLinkNDABinding :exec
UPDATE links
SET nda_template_id = $1,
    nda_document_id = $2,
    updated_at = now()
WHERE id = $3 AND workspace_id = $4;

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

-- name: ExistsLinkNameInDealRoom :one
-- Case-insensitive uniqueness among non-deleted links in a deal room.
-- exclude_id may be NULL when creating a new link.
SELECT EXISTS (
  SELECT 1
  FROM links
  WHERE deal_room_id = sqlc.arg(deal_room_id)
    AND status <> 'deleted'
    AND name IS NOT NULL
    AND btrim(name) <> ''
    AND lower(btrim(name)) = lower(btrim(sqlc.arg(name)))
    AND (sqlc.narg(exclude_id)::uuid IS NULL OR id <> sqlc.narg(exclude_id))
) AS exists;

-- name: ExistsLinkNameInWorkspace :one
-- Case-insensitive uniqueness among non-deleted document links in a workspace.
SELECT EXISTS (
  SELECT 1
  FROM links
  WHERE workspace_id = sqlc.arg(workspace_id)
    AND deal_room_id IS NULL
    AND status <> 'deleted'
    AND name IS NOT NULL
    AND btrim(name) <> ''
    AND lower(btrim(name)) = lower(btrim(sqlc.arg(name)))
    AND (sqlc.narg(exclude_id)::uuid IS NULL OR id <> sqlc.narg(exclude_id))
) AS exists;

-- name: UpdateLinkFolderScopePaths :exec
UPDATE links
SET folder_scope_paths = $1, updated_at = now()
WHERE id = $2 AND workspace_id = $3;

-- name: UpdateLinkFolderScopeMode :exec
UPDATE links
SET folder_scope_mode = $1, has_document_scope = $2, updated_at = now()
WHERE id = $3 AND workspace_id = $4;

-- name: CreateLinkNDAAgreement :one
INSERT INTO link_nda_agreements (
    tenant_id, workspace_id, link_id, visitor_id, email, ip, user_agent,
    nda_template_id, content_sha256, signer_name, certificate_id, signed_file_key, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetLinkNDAAgreementByLinkVisitorTemplate :one
SELECT *
FROM link_nda_agreements
WHERE link_id = $1
  AND visitor_id = $2
  AND nda_template_id = $3
  AND status = 'signed'
LIMIT 1;

-- name: GetLinkNDAAgreementByCertificate :one
SELECT *
FROM link_nda_agreements
WHERE certificate_id = $1
LIMIT 1;

-- name: GetLinkNDAAgreementByID :one
SELECT *
FROM link_nda_agreements
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: UpdateLinkNDAAgreementSignedFile :one
UPDATE link_nda_agreements
SET signed_file_key = $1
WHERE id = $2
RETURNING *;

-- name: ListLinkNDAAgreementsByTemplate :many
SELECT *
FROM link_nda_agreements
WHERE workspace_id = $1 AND nda_template_id = $2
ORDER BY signed_at DESC;

-- name: ListLinkNDAAgreementsByLink :many
SELECT *
FROM link_nda_agreements
WHERE workspace_id = $1 AND link_id = $2
ORDER BY signed_at DESC;

-- name: CreateNDATemplate :one
INSERT INTO nda_templates (
    tenant_id, workspace_id, name, source_document_id, content_sha256,
    require_signer_name, status, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetNDATemplateByID :one
SELECT *
FROM nda_templates
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: GetNDATemplateBySourceDocument :one
SELECT *
FROM nda_templates
WHERE workspace_id = $1 AND source_document_id = $2
LIMIT 1;

-- name: ListNDATemplatesByWorkspace :many
SELECT *
FROM nda_templates
WHERE workspace_id = $1 AND status = $2
ORDER BY updated_at DESC;

-- name: ListAllNDATemplatesByWorkspace :many
SELECT *
FROM nda_templates
WHERE workspace_id = $1
ORDER BY updated_at DESC;

-- name: UpdateNDATemplate :one
UPDATE nda_templates
SET name = $1,
    require_signer_name = $2,
    updated_at = now()
WHERE id = $3 AND workspace_id = $4 AND status = 'active'
RETURNING *;

-- name: ArchiveNDATemplate :one
UPDATE nda_templates
SET status = 'archived', updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: CountNDATemplateResponses :one
SELECT COUNT(*)::bigint
FROM link_nda_agreements
WHERE nda_template_id = $1;

-- name: CountNDATemplateLinks :one
SELECT COUNT(*)::bigint
FROM links
WHERE nda_template_id = $1 AND status NOT IN ('deleted');

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
INSERT INTO page_views (
    tenant_id, workspace_id, link_id, visitor_id, page_number, duration_seconds, scroll_depth, reading_session_id, document_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7::numeric, sqlc.narg(reading_session_id), sqlc.narg(document_id));

-- name: GetOpenReadingSession :one
SELECT *
FROM reading_sessions
WHERE link_id = $1
  AND visitor_id = $2
  AND ended_at IS NULL
LIMIT 1;

-- name: CloseReadingSession :exec
UPDATE reading_sessions
SET ended_at = last_activity_at
WHERE id = $1
  AND ended_at IS NULL;

-- name: CreateReadingSession :one
INSERT INTO reading_sessions (
    tenant_id,
    workspace_id,
    link_id,
    document_id,
    visitor_id,
    started_at,
    last_activity_at,
    max_page,
    distinct_page_count,
    total_duration_seconds
) VALUES (
    $1, $2, $3, $4, $5, now(), now(), $6, 0, 0
)
RETURNING *;

-- name: UpsertReadingSessionPage :exec
INSERT INTO reading_session_pages (session_id, page_number, first_seen_at, duration_seconds)
VALUES ($1, $2, now(), $3)
ON CONFLICT (session_id, page_number) DO UPDATE
SET duration_seconds = reading_session_pages.duration_seconds + EXCLUDED.duration_seconds;

-- name: RefreshReadingSessionStats :one
UPDATE reading_sessions rs
SET
    last_activity_at = now(),
    max_page = GREATEST(rs.max_page, sqlc.arg(page_number)),
    total_duration_seconds = rs.total_duration_seconds + sqlc.arg(duration_seconds),
    distinct_page_count = (
        SELECT COUNT(*)::int FROM reading_session_pages p WHERE p.session_id = rs.id
    )
WHERE rs.id = sqlc.arg(id)
RETURNING *;

-- name: GetLinkAccessMetrics :one
SELECT
    COUNT(*) FILTER (WHERE event_type = 'link_opened') AS opens,
    COUNT(DISTINCT visitor_id) FILTER (WHERE event_type = 'link_opened') AS unique_visitors,
    COUNT(*) FILTER (WHERE event_type = 'forward_signal') AS forward_signals,
    COUNT(*) FILTER (WHERE event_type = 'download_attempted') AS downloads
FROM access_logs
WHERE link_id = $1;

-- name: GetLinkAccessMetrics24h :one
-- Rolling 24-hour access metrics used by signal rules.
SELECT
    COUNT(*) FILTER (WHERE event_type = 'link_opened') AS opens,
    COUNT(DISTINCT visitor_id) FILTER (WHERE event_type = 'link_opened') AS unique_visitors,
    COUNT(*) FILTER (WHERE event_type = 'forward_signal') AS forward_signals,
    COUNT(*) FILTER (WHERE event_type = 'download_attempted') AS downloads
FROM access_logs
WHERE link_id = $1
  AND created_at > now() - interval '24 hours';

-- name: GetLinkAccessMetrics24hBatch :many
-- Rolling 24-hour access metrics for Deal Radar Leak Watch confidence.
SELECT
    link_id,
    COUNT(*) FILTER (WHERE event_type = 'link_opened')::bigint AS opens,
    COUNT(DISTINCT visitor_id) FILTER (WHERE event_type = 'link_opened')::bigint AS unique_visitors,
    COUNT(*) FILTER (WHERE event_type = 'forward_signal')::bigint AS forward_signals,
    COUNT(*) FILTER (WHERE event_type = 'download_attempted')::bigint AS downloads
FROM access_logs
WHERE link_id = ANY($1::uuid[])
  AND created_at > now() - interval '24 hours'
GROUP BY link_id;

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
JOIN pages p ON p.document_id = COALESCE(pv.document_id, l.document_id) AND p.page_number = pv.page_number
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
JOIN pages p ON p.document_id = COALESCE(pv.document_id, l.document_id) AND p.page_number = pv.page_number
WHERE pv.link_id = $1
  AND p.title IS NOT NULL AND p.title <> ''
  AND pv.created_at > now() - interval '24 hours'
  AND lower(p.title) LIKE ANY (sqlc.arg(patterns)::text[]);

-- name: CountVisitorEngagedKeyPageViews :one
-- Engaged (≥3s) key-page views for one visitor on a link (heat keyword patterns).
SELECT COUNT(*)::bigint AS count
FROM page_views pv
JOIN links l ON l.id = pv.link_id
JOIN pages p ON p.document_id = COALESCE(pv.document_id, l.document_id) AND p.page_number = pv.page_number
WHERE pv.link_id = $1
  AND pv.visitor_id = $2
  AND pv.duration_seconds >= 3
  AND p.title IS NOT NULL AND p.title <> ''
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
JOIN pages p ON p.document_id = COALESCE(pv.document_id, l.document_id) AND p.page_number = pv.page_number
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
ORDER BY e.created_at DESC, e.id ASC
LIMIT $2 OFFSET $3;

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
ORDER BY last_access_at DESC, visitor_id ASC
LIMIT $2 OFFSET $3;

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

-- name: ListHighExitPagesByLink :many
-- Per-visitor last page = exit. Rank by exit_rate for bounce deep links.
WITH views AS (
    SELECT pv.page_number, COUNT(*)::bigint AS view_count
    FROM page_views pv
    WHERE pv.link_id = sqlc.arg(link_id)
    GROUP BY pv.page_number
),
exits AS (
    SELECT last_views.page_number, COUNT(*)::bigint AS exit_count
    FROM (
        SELECT DISTINCT ON (pv.visitor_id) pv.visitor_id, pv.page_number
        FROM page_views pv
        WHERE pv.link_id = sqlc.arg(link_id)
        ORDER BY pv.visitor_id, pv.created_at DESC
    ) last_views
    GROUP BY last_views.page_number
)
SELECT
    v.page_number,
    v.view_count,
    COALESCE(e.exit_count, 0)::bigint AS exit_count,
    CASE
        WHEN v.view_count > 0
            THEN LEAST(1.0, COALESCE(e.exit_count, 0)::float8 / v.view_count::float8)
        ELSE 0::float8
    END AS exit_rate
FROM views v
LEFT JOIN exits e ON e.page_number = v.page_number
WHERE v.view_count >= 2
ORDER BY exit_rate DESC, exit_count DESC, view_count DESC
LIMIT 5;

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

-- name: GetVisitorSummariesByDocumentInRange :many
WITH visitor_emails AS (
    SELECT al.visitor_id, MAX(al.visitor_email) AS visitor_email
    FROM access_logs al
    WHERE al.link_id IN (
        SELECT l.id FROM links l
        WHERE l.document_id = sqlc.arg(document_id)
          AND l.workspace_id = sqlc.arg(workspace_id)
          AND l.status != 'deleted'
    )
      AND al.workspace_id = sqlc.arg(workspace_id)
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
WHERE pv.link_id IN (
    SELECT l.id FROM links l
    WHERE l.document_id = sqlc.arg(document_id)
      AND l.workspace_id = sqlc.arg(workspace_id)
      AND l.status != 'deleted'
)
  AND pv.workspace_id = sqlc.arg(workspace_id)
  AND pv.created_at >= sqlc.arg(range_start)
  AND pv.created_at < sqlc.arg(range_end)
GROUP BY pv.visitor_id, ve.visitor_email
ORDER BY last_seen_at DESC
LIMIT sqlc.arg(page_limit);

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
  AND document_id IS NOT DISTINCT FROM sqlc.narg(document_id)::uuid
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
    COUNT(*) FILTER (WHERE event_type = 'forward_signal')::bigint AS forward_signals,
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
-- Anti-join via visitor sets; count open rows (same metric as the old
-- correlated NOT EXISTS) for visitors who never recorded a page view.
WITH opens AS (
    SELECT a.link_id, a.visitor_id
    FROM access_logs a
    WHERE a.link_id = ANY($1::uuid[])
      AND a.event_type = 'link_opened'
      AND a.visitor_id IS NOT NULL
      AND a.visitor_id <> ''
),
viewed AS (
    SELECT DISTINCT p.link_id, p.visitor_id
    FROM page_views p
    WHERE p.link_id = ANY($1::uuid[])
      AND p.visitor_id IS NOT NULL
      AND p.visitor_id <> ''
)
SELECT o.link_id, COUNT(*)::bigint AS bounce_count
FROM opens o
LEFT JOIN viewed v ON v.link_id = o.link_id AND v.visitor_id = o.visitor_id
WHERE v.visitor_id IS NULL
GROUP BY o.link_id;

-- name: GetLinkKeyPageViewMetricsBatch :many
-- Batch version of GetLinkKeyPageViewMetrics for O(1) dashboard heat scoring.
SELECT
    pv.link_id,
    COUNT(*) FILTER (WHERE duration_seconds >= 3) AS engaged_key_page_views,
    COUNT(*) AS total_key_page_views
FROM page_views pv
JOIN links l ON l.id = pv.link_id
JOIN pages p ON p.document_id = COALESCE(pv.document_id, l.document_id) AND p.page_number = pv.page_number
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
    forward_signals,
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

-- name: GetPageAnalyticsByDocumentInRange :many
SELECT
    p.page_number,
    COUNT(pv.id) AS view_count,
    COALESCE(AVG(pv.duration_seconds), 0)::float8 AS avg_duration_seconds,
    COALESCE(MAX(pv.created_at), p.created_at) AS last_viewed_at
FROM pages p
LEFT JOIN links l ON l.document_id = p.document_id AND l.status != 'deleted'
LEFT JOIN page_views pv
  ON pv.link_id = l.id
 AND pv.page_number = p.page_number
 AND pv.created_at >= sqlc.arg(range_start)
 AND pv.created_at < sqlc.arg(range_end)
WHERE p.document_id = sqlc.arg(document_id) AND p.workspace_id = sqlc.arg(workspace_id)
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
        SELECT id FROM links WHERE links.document_id = $1 AND status != 'deleted'
    )
    ORDER BY link_id, visitor_id, created_at DESC
) last_views
GROUP BY page_number;

-- name: GetPageExitCountsByDocumentInRange :many
SELECT page_number, COUNT(*) AS exit_count
FROM (
    SELECT DISTINCT ON (pv.link_id, pv.visitor_id) pv.link_id, pv.visitor_id, pv.page_number
    FROM page_views pv
    WHERE pv.link_id IN (
        SELECT id FROM links
        WHERE links.document_id = sqlc.arg(document_id) AND status != 'deleted'
    )
      AND pv.created_at >= sqlc.arg(range_start)
      AND pv.created_at < sqlc.arg(range_end)
    ORDER BY pv.link_id, pv.visitor_id, pv.created_at DESC
) last_views
GROUP BY page_number;

-- name: GetDocumentVisitorReach :many
-- Legacy per-visitor reach (kept for tests / fallback). Prefer GetDocumentReadingSessionReach.
SELECT
    pv.visitor_id,
    MAX(pv.page_number)::int AS max_page,
    COUNT(DISTINCT pv.page_number)::bigint AS distinct_pages,
    COALESCE(SUM(pv.duration_seconds), 0)::bigint AS total_duration_seconds
FROM page_views pv
WHERE pv.link_id IN (
    SELECT l.id FROM links l
    WHERE l.document_id = sqlc.arg(document_id)
      AND l.workspace_id = sqlc.arg(workspace_id)
      AND l.status != 'deleted'
)
  AND pv.workspace_id = sqlc.arg(workspace_id)
  AND pv.visitor_id IS NOT NULL
  AND pv.visitor_id <> ''
GROUP BY pv.visitor_id
ORDER BY MAX(pv.created_at) DESC;

-- name: GetDocumentReadingSessionReach :many
-- Formal idle-gap reading sessions for document funnel.
SELECT
    rs.id,
    rs.max_page,
    rs.distinct_page_count::bigint AS distinct_pages,
    rs.total_duration_seconds::bigint AS total_duration_seconds
FROM reading_sessions rs
WHERE rs.document_id = sqlc.arg(document_id)
  AND rs.workspace_id = sqlc.arg(workspace_id)
  AND EXISTS (
      SELECT 1 FROM links l
      WHERE l.id = rs.link_id
        AND l.document_id = sqlc.arg(document_id)
        AND l.workspace_id = sqlc.arg(workspace_id)
        AND l.status != 'deleted'
  )
ORDER BY rs.last_activity_at DESC;

-- name: GetDocumentReadingSessionReachInRange :many
SELECT
    rs.id,
    rs.max_page,
    rs.distinct_page_count::bigint AS distinct_pages,
    rs.total_duration_seconds::bigint AS total_duration_seconds
FROM reading_sessions rs
WHERE rs.document_id = sqlc.arg(document_id)
  AND rs.workspace_id = sqlc.arg(workspace_id)
  AND rs.last_activity_at >= sqlc.arg(range_start)
  AND rs.last_activity_at < sqlc.arg(range_end)
  AND EXISTS (
      SELECT 1 FROM links l
      WHERE l.id = rs.link_id
        AND l.document_id = sqlc.arg(document_id)
        AND l.workspace_id = sqlc.arg(workspace_id)
        AND l.status != 'deleted'
  )
ORDER BY rs.last_activity_at DESC;

-- name: ListDocumentReadingSessions :many
-- Insights session timeline: who / when / deepest page.
WITH visitor_emails AS (
    SELECT al.visitor_id, MAX(al.visitor_email) AS visitor_email
    FROM access_logs al
    WHERE al.workspace_id = sqlc.arg(workspace_id)
      AND al.link_id IN (
          SELECT l.id FROM links l
          WHERE l.document_id = sqlc.arg(document_id)
            AND l.workspace_id = sqlc.arg(workspace_id)
            AND l.status != 'deleted'
      )
      AND al.visitor_email IS NOT NULL
      AND al.visitor_email <> ''
    GROUP BY al.visitor_id
)
SELECT
    rs.id,
    rs.link_id,
    rs.visitor_id,
    COALESCE(ve.visitor_email, '')::text AS visitor_email,
    rs.started_at,
    rs.last_activity_at,
    rs.ended_at,
    rs.max_page,
    rs.distinct_page_count,
    rs.total_duration_seconds
FROM reading_sessions rs
LEFT JOIN visitor_emails ve ON ve.visitor_id = rs.visitor_id
WHERE rs.document_id = sqlc.arg(document_id)
  AND rs.workspace_id = sqlc.arg(workspace_id)
  AND EXISTS (
      SELECT 1 FROM links l
      WHERE l.id = rs.link_id
        AND l.document_id = sqlc.arg(document_id)
        AND l.workspace_id = sqlc.arg(workspace_id)
        AND l.status != 'deleted'
  )
ORDER BY rs.last_activity_at DESC
LIMIT sqlc.arg(page_limit);

-- name: ListDocumentReadingSessionsInRange :many
WITH visitor_emails AS (
    SELECT al.visitor_id, MAX(al.visitor_email) AS visitor_email
    FROM access_logs al
    WHERE al.workspace_id = sqlc.arg(workspace_id)
      AND al.link_id IN (
          SELECT l.id FROM links l
          WHERE l.document_id = sqlc.arg(document_id)
            AND l.workspace_id = sqlc.arg(workspace_id)
            AND l.status != 'deleted'
      )
      AND al.visitor_email IS NOT NULL
      AND al.visitor_email <> ''
    GROUP BY al.visitor_id
)
SELECT
    rs.id,
    rs.link_id,
    rs.visitor_id,
    COALESCE(ve.visitor_email, '')::text AS visitor_email,
    rs.started_at,
    rs.last_activity_at,
    rs.ended_at,
    rs.max_page,
    rs.distinct_page_count,
    rs.total_duration_seconds
FROM reading_sessions rs
LEFT JOIN visitor_emails ve ON ve.visitor_id = rs.visitor_id
WHERE rs.document_id = sqlc.arg(document_id)
  AND rs.workspace_id = sqlc.arg(workspace_id)
  AND rs.last_activity_at >= sqlc.arg(range_start)
  AND rs.last_activity_at < sqlc.arg(range_end)
  AND EXISTS (
      SELECT 1 FROM links l
      WHERE l.id = rs.link_id
        AND l.document_id = sqlc.arg(document_id)
        AND l.workspace_id = sqlc.arg(workspace_id)
        AND l.status != 'deleted'
  )
ORDER BY rs.last_activity_at DESC
LIMIT sqlc.arg(page_limit);

-- name: ListReadingSessionPagesBySessionIDs :many
SELECT session_id, page_number, duration_seconds
FROM reading_session_pages
WHERE session_id = ANY(sqlc.arg(session_ids)::uuid[])
ORDER BY session_id, page_number ASC;

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

-- name: CountDealRoomsByWorkspace :one
SELECT COUNT(*)::bigint AS count
FROM deal_rooms
WHERE workspace_id = $1
  AND deleted_at IS NULL
  AND (
    sqlc.arg(query)::text = ''
    OR name ILIKE '%' || sqlc.arg(query) || '%'
    OR COALESCE(description, '') ILIKE '%' || sqlc.arg(query) || '%'
  );

-- name: ListDealRoomsByWorkspacePage :many
SELECT *
FROM deal_rooms
WHERE workspace_id = $1
  AND deleted_at IS NULL
  AND (
    sqlc.arg(query)::text = ''
    OR name ILIKE '%' || sqlc.arg(query) || '%'
    OR COALESCE(description, '') ILIKE '%' || sqlc.arg(query) || '%'
  )
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetDealRoomAggregatesForRooms :many
-- Page-scoped aggregates: same CTE shape as workspace aggregates, filtered to room IDs.
WITH rooms AS (
    SELECT id
    FROM deal_rooms
    WHERE deal_rooms.workspace_id = $1
      AND deal_rooms.deleted_at IS NULL
      AND deal_rooms.id = ANY(sqlc.arg(room_ids)::uuid[])
),
doc_counts AS (
    SELECT room_id, COUNT(*)::bigint AS document_count
    FROM deal_room_documents
    WHERE room_id IN (SELECT id FROM rooms)
    GROUP BY room_id
),
member_counts AS (
    SELECT room_id, COUNT(*)::bigint AS member_count
    FROM room_members
    WHERE room_id IN (SELECT id FROM rooms)
    GROUP BY room_id
),
pending_counts AS (
    SELECT room_id, COUNT(*)::bigint AS pending_count
    FROM room_access_requests
    WHERE room_id IN (SELECT id FROM rooms) AND status = 'pending'
    GROUP BY room_id
),
room_links AS (
    SELECT id AS link_id, deal_room_id AS room_id
    FROM links
    WHERE deal_room_id IN (SELECT id FROM rooms)
      AND status NOT IN ('deleted', 'disabled')
),
visitor_stats AS (
    SELECT
        rl.room_id,
        (COUNT(DISTINCT al.visitor_id) FILTER (WHERE al.visitor_id IS NOT NULL) +
         COUNT(DISTINCT al.visitor_email) FILTER (WHERE al.visitor_email IS NOT NULL AND al.visitor_id IS NULL))::bigint AS visitor_count,
        COUNT(DISTINCT al.id)::bigint AS access_event_count,
        MAX(al.created_at)::timestamptz AS last_accessed_at
    FROM room_links rl
    JOIN access_logs al ON al.link_id = rl.link_id
    GROUP BY rl.room_id
),
question_counts AS (
    SELECT
        rl.room_id,
        COUNT(DISTINCT t.id) FILTER (
            WHERE t.lane IN ('host', 'hybrid')
              AND t.status IN ('host_pending', 'host_escalated')
              AND (t.formal_status IS NULL OR t.formal_status NOT IN ('pending_review', 'scheduled'))
        )::bigint AS pending_question_count
    FROM room_links rl
    JOIN link_ask_turns t ON t.link_id = rl.link_id
    GROUP BY rl.room_id
)
SELECT
    r.id AS room_id,
    COALESCE(d.document_count, 0)::bigint AS document_count,
    COALESCE(m.member_count, 0)::bigint AS member_count,
    COALESCE(p.pending_count, 0)::bigint AS pending_count,
    COALESCE(v.visitor_count, 0)::bigint AS visitor_count,
    COALESCE(q.pending_question_count, 0)::bigint AS pending_question_count,
    v.last_accessed_at,
    COALESCE(
        LEAST(100, COALESCE(v.visitor_count, 0) * 5 + COALESCE(v.access_event_count, 0) * 2),
        0
    )::int AS heat_score
FROM rooms r
LEFT JOIN doc_counts d ON d.room_id = r.id
LEFT JOIN member_counts m ON m.room_id = r.id
LEFT JOIN pending_counts p ON p.room_id = r.id
LEFT JOIN visitor_stats v ON v.room_id = r.id
LEFT JOIN question_counts q ON q.room_id = r.id;

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
SET nda_status = 'signed',
    nda_signed_at = now(),
    status = 'active',
    updated_at = now()
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

-- name: GetPendingAccessRequestByRoomAndEmail :one
SELECT id, tenant_id, workspace_id, room_id, email, reason, status, reviewed_by, reviewed_at, created_at, updated_at
FROM room_access_requests
WHERE room_id = $1 AND email = $2 AND status = 'pending'
LIMIT 1;

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
    drd.locked,
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

-- name: DocumentInAnyDealRoom :one
SELECT EXISTS(
    SELECT 1 FROM deal_room_documents drd
    JOIN documents d ON d.id = drd.document_id
    WHERE drd.document_id = $1 AND d.deleted_at IS NULL
) AS exists;

-- name: CountDocumentChunks :one
SELECT COUNT(*)::bigint
FROM chunks
WHERE document_id = $1;

-- name: CountDocumentPages :one
SELECT COUNT(*)::bigint
FROM pages
WHERE document_id = $1;

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
RETURNING id, tenant_id, workspace_id, room_id, document_id, folder_path, sort_order, created_at, locked;

-- name: ListDealRoomDocuments :many
SELECT id, tenant_id, workspace_id, room_id, document_id, folder_path, sort_order, created_at, locked
FROM deal_room_documents
WHERE room_id = $1
ORDER BY folder_path, sort_order;

-- name: GetDealRoomDocument :one
SELECT id, tenant_id, workspace_id, room_id, document_id, folder_path, sort_order, created_at, locked
FROM deal_room_documents
WHERE id = $1 AND room_id = $2;

-- name: GetDealRoomDocumentByDocumentID :one
SELECT id, tenant_id, workspace_id, room_id, document_id, folder_path, sort_order, created_at, locked
FROM deal_room_documents
WHERE room_id = $1 AND document_id = $2;

-- name: SetDealRoomDocumentsLocked :exec
UPDATE deal_room_documents
SET locked = sqlc.arg(locked)
WHERE room_id = sqlc.arg(room_id)
  AND document_id = ANY(sqlc.arg(document_ids)::uuid[]);


-- name: GetDealRoomAggregatesByWorkspace :many
-- Aggregate per metric in independent CTEs to avoid LEFT JOIN row explosion
-- (docs × members × requests × access_logs) which tanks list latency.
WITH rooms AS (
    SELECT id
    FROM deal_rooms
    WHERE deal_rooms.workspace_id = $1 AND deal_rooms.deleted_at IS NULL
),
doc_counts AS (
    SELECT room_id, COUNT(*)::bigint AS document_count
    FROM deal_room_documents
    WHERE room_id IN (SELECT id FROM rooms)
    GROUP BY room_id
),
member_counts AS (
    SELECT room_id, COUNT(*)::bigint AS member_count
    FROM room_members
    WHERE room_id IN (SELECT id FROM rooms)
    GROUP BY room_id
),
pending_counts AS (
    SELECT room_id, COUNT(*)::bigint AS pending_count
    FROM room_access_requests
    WHERE room_id IN (SELECT id FROM rooms) AND status = 'pending'
    GROUP BY room_id
),
room_links AS (
    SELECT id AS link_id, deal_room_id AS room_id
    FROM links
    WHERE deal_room_id IN (SELECT id FROM rooms)
      AND status NOT IN ('deleted', 'disabled')
),
visitor_stats AS (
    SELECT
        rl.room_id,
        (COUNT(DISTINCT al.visitor_id) FILTER (WHERE al.visitor_id IS NOT NULL) +
         COUNT(DISTINCT al.visitor_email) FILTER (WHERE al.visitor_email IS NOT NULL AND al.visitor_id IS NULL))::bigint AS visitor_count,
        COUNT(DISTINCT al.id)::bigint AS access_event_count,
        MAX(al.created_at)::timestamptz AS last_accessed_at
    FROM room_links rl
    JOIN access_logs al ON al.link_id = rl.link_id
    GROUP BY rl.room_id
),
question_counts AS (
    SELECT
        rl.room_id,
        COUNT(DISTINCT t.id) FILTER (
            WHERE t.lane IN ('host', 'hybrid')
              AND t.status IN ('host_pending', 'host_escalated')
              AND (t.formal_status IS NULL OR t.formal_status NOT IN ('pending_review', 'scheduled'))
        )::bigint AS pending_question_count
    FROM room_links rl
    JOIN link_ask_turns t ON t.link_id = rl.link_id
    GROUP BY rl.room_id
)
SELECT
    r.id AS room_id,
    COALESCE(d.document_count, 0)::bigint AS document_count,
    COALESCE(m.member_count, 0)::bigint AS member_count,
    COALESCE(p.pending_count, 0)::bigint AS pending_count,
    COALESCE(v.visitor_count, 0)::bigint AS visitor_count,
    COALESCE(q.pending_question_count, 0)::bigint AS pending_question_count,
    v.last_accessed_at,
    COALESCE(
        LEAST(100, COALESCE(v.visitor_count, 0) * 5 + COALESCE(v.access_event_count, 0) * 2),
        0
    )::int AS heat_score
FROM rooms r
LEFT JOIN doc_counts d ON d.room_id = r.id
LEFT JOIN member_counts m ON m.room_id = r.id
LEFT JOIN pending_counts p ON p.room_id = r.id
LEFT JOIN visitor_stats v ON v.room_id = r.id
LEFT JOIN question_counts q ON q.room_id = r.id;

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
WHERE link_id = $1
  AND workspace_id = $2
  AND dismissed = false
  AND (snoozed_until IS NULL OR snoozed_until <= now())
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

-- name: CountActiveSuggestionsByMetadata :one
SELECT COUNT(*) AS count
FROM suggestions
WHERE workspace_id = $1
  AND dismissed = false
  AND metadata @> sqlc.arg(match_metadata)::jsonb;

-- name: DismissSuggestion :exec
UPDATE suggestions
SET dismissed = true, snoozed_until = NULL, updated_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: DismissSuggestionsByMetadata :exec
UPDATE suggestions
SET dismissed = true, snoozed_until = NULL, updated_at = now()
WHERE workspace_id = $1
  AND dismissed = false
  AND metadata @> sqlc.arg(match_metadata)::jsonb;

-- name: SnoozeSuggestion :one
UPDATE suggestions
SET snoozed_until = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND dismissed = false
RETURNING *;

-- name: SnoozeActionItemsBySignal :exec
UPDATE action_items
SET status = 'snoozed',
    snoozed_until = sqlc.arg(snoozed_until),
    updated_at = now()
WHERE signal_id = sqlc.arg(signal_id)
  AND workspace_id = sqlc.arg(workspace_id)
  AND status = 'pending';

-- name: SnoozeActionItemBySource :exec
UPDATE action_items
SET status = 'snoozed',
    snoozed_until = sqlc.arg(snoozed_until),
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND source_type = sqlc.arg(source_type)
  AND source_id = sqlc.arg(source_id)
  AND status = 'pending';

-- name: ReactivateExpiredSnoozedActions :exec
-- Wake timed snoozes; also heal legacy rows snoozed without snoozed_until (default 24h).
UPDATE action_items
SET status = 'pending', snoozed_until = NULL, updated_at = now()
WHERE workspace_id = $1
  AND status = 'snoozed'
  AND (
      (snoozed_until IS NOT NULL AND snoozed_until <= now())
      OR (snoozed_until IS NULL AND updated_at <= now() - interval '24 hours')
  );

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

-- name: GetWorkspaceOutboundWebhook :one
SELECT workspace_id, tenant_id, url, secret, enabled, event_types, created_at, updated_at
FROM workspace_outbound_webhooks
WHERE workspace_id = $1;

-- name: UpsertWorkspaceOutboundWebhook :one
INSERT INTO workspace_outbound_webhooks (
    workspace_id, tenant_id, url, secret, enabled, event_types
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (workspace_id)
DO UPDATE SET
    url = EXCLUDED.url,
    secret = EXCLUDED.secret,
    enabled = EXCLUDED.enabled,
    event_types = EXCLUDED.event_types,
    updated_at = now()
RETURNING workspace_id, tenant_id, url, secret, enabled, event_types, created_at, updated_at;

-- name: DeleteWorkspaceOutboundWebhook :exec
DELETE FROM workspace_outbound_webhooks
WHERE workspace_id = $1;

-- name: GetWorkspaceKeyPageSettings :one
SELECT workspace_id, tenant_id, default_circle, extra_keywords, created_at, updated_at
FROM workspace_key_page_settings
WHERE workspace_id = $1;

-- name: UpsertWorkspaceKeyPageSettings :one
INSERT INTO workspace_key_page_settings (
    workspace_id, tenant_id, default_circle, extra_keywords
) VALUES ($1, $2, $3, $4)
ON CONFLICT (workspace_id)
DO UPDATE SET
    default_circle = EXCLUDED.default_circle,
    extra_keywords = EXCLUDED.extra_keywords,
    updated_at = now()
RETURNING workspace_id, tenant_id, default_circle, extra_keywords, created_at, updated_at;

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

-- name: DeleteSheetPageRangesByDocument :exec
DELETE FROM document_sheet_page_ranges WHERE document_id = $1;

-- name: UpsertDocumentSheetPageRange :exec
INSERT INTO document_sheet_page_ranges (document_id, sheet_name, page_start, page_end)
VALUES ($1, $2, $3, $4)
ON CONFLICT (document_id, sheet_name) DO UPDATE SET
    page_start = EXCLUDED.page_start,
    page_end = EXCLUDED.page_end;

-- name: ListSheetPageRangesByDocument :many
SELECT sheet_name, page_start, page_end
FROM document_sheet_page_ranges
WHERE document_id = $1
ORDER BY page_start, sheet_name;

-- name: ListSheetPageRangesByDocuments :many
SELECT document_id, sheet_name, page_start, page_end
FROM document_sheet_page_ranges
WHERE document_id = ANY($1::uuid[])
ORDER BY document_id, page_start, sheet_name;

-- name: DeleteChunksByDocument :exec
DELETE FROM chunks WHERE chunks.document_id = $1 OR chunks.page_id IN (SELECT id FROM pages WHERE pages.document_id = $1);

-- name: CreateChunkBox :exec
INSERT INTO chunk_boxes (chunk_id, document_id, page_number, coordinate_space, x, y, w, h, source, confidence)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: CreateChunkWithBBox :one
-- Preview/ingest path: text + bbox only (no retrieval index columns).
INSERT INTO chunks (tenant_id, workspace_id, page_id, document_id, chunk_index, chunk_type, text, bbox)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, tenant_id, workspace_id, page_id, document_id, chunk_index, chunk_type, text, bbox;

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

-- name: SearchTableRowsByDocuments :many
-- Knowledge Q&A table lane (ceiling Phase I2). pattern is already ILIKE-escaped by Go.
SELECT
    c.id,
    c.document_id,
    c.chunk_index,
    c.text,
    c.bbox
FROM chunks c
WHERE c.workspace_id = sqlc.arg(workspace_id)
  AND c.chunk_type = 'table_row'
  AND c.document_id = ANY(sqlc.arg(document_ids)::uuid[])
  AND c.text ILIKE '%' || sqlc.arg(pattern) || '%' ESCAPE '\'
ORDER BY c.document_id, c.chunk_index
LIMIT sqlc.arg(row_limit);

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
-- Upsert keeps source_* as the resolve key; target_id is the navigation parent
-- (e.g. deal_room id). Re-open done items when the underlying event is still pending.
INSERT INTO action_items (
    tenant_id, workspace_id, source_type, source_id, target_id, title, impact, due_at, status, action_type
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (workspace_id, source_type, source_id) DO UPDATE SET
    target_id = EXCLUDED.target_id,
    title = EXCLUDED.title,
    impact = EXCLUDED.impact,
    due_at = EXCLUDED.due_at,
    action_type = EXCLUDED.action_type,
    -- Re-open resolved items when the event is still pending; respect snooze/ignore.
    status = CASE
        WHEN action_items.status IN ('snoozed', 'ignored') THEN action_items.status
        ELSE 'pending'
    END,
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

-- name: ListActionItemsByWorkspaceForUser :many
-- Same retention window as ListActionItemsByWorkspace, but share-link access
-- todos (document or deal-room) are only visible to the link creator
-- (source_id = link id).
SELECT a.*
FROM action_items a
WHERE a.workspace_id = $1
  AND (
      a.status = 'pending'
      OR (a.status = 'done' AND a.updated_at > now() - interval '1 day')
      OR (a.status IN ('snoozed', 'ignored') AND a.updated_at > now() - interval '30 days')
  )
  AND (
      (
          a.source_type IS DISTINCT FROM 'link_access_request'
          AND a.source_type IS DISTINCT FROM 'deal_room_link_access_request'
      )
      OR EXISTS (
          SELECT 1
          FROM links l
          WHERE l.workspace_id = a.workspace_id
            AND l.id = NULLIF(a.source_id, '')::uuid
            AND l.created_by = $2
      )
  )
ORDER BY a.created_at DESC;

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
SET status = sqlc.arg(status),
    snoozed_until = CASE
        WHEN sqlc.arg(status) = 'snoozed' THEN COALESCE(sqlc.narg(snoozed_until), now() + interval '24 hours')
        ELSE NULL
    END,
    outcome = CASE
        WHEN sqlc.arg(status) = 'done' THEN COALESCE(NULLIF(sqlc.narg(outcome)::text, ''), 'acted')
        ELSE NULL
    END,
    updated_at = now()
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id)
RETURNING *;

-- name: CountRecentActionOutcomesByWorkspace :many
-- Closed-loop learning for Deal Radar: 30d done outcomes by signal subtype / action type.
SELECT
    COALESCE(NULLIF(s.subtype, ''), a.action_type) AS kind,
    a.outcome,
    COUNT(*)::bigint AS count
FROM action_items a
LEFT JOIN signals s
    ON s.id = a.signal_id
   AND s.workspace_id = a.workspace_id
WHERE a.workspace_id = $1
  AND a.status = 'done'
  AND a.outcome IS NOT NULL
  AND a.updated_at > now() - interval '30 days'
GROUP BY 1, 2;

-- name: ListPendingDocumentLinkAccessRequestsByWorkspace :many
-- Dashboard sync: document-library share applications only.
SELECT r.id, r.email, r.link_id, l.name AS link_name
FROM link_access_requests r
JOIN links l ON l.id = r.link_id
WHERE r.workspace_id = $1
  AND r.status = 'pending'
  AND l.deal_room_id IS NULL
  AND l.document_id IS NOT NULL
ORDER BY r.created_at DESC;

-- name: ListPendingDealRoomLinkAccessRequestsByWorkspace :many
-- Dashboard sync: deal-room share applications only (never document library).
SELECT r.id, r.email, r.link_id, l.deal_room_id, l.name AS link_name, dr.name AS room_name
FROM link_access_requests r
JOIN links l ON l.id = r.link_id
JOIN deal_rooms dr ON dr.id = l.deal_room_id
WHERE r.workspace_id = $1
  AND r.status = 'pending'
  AND l.deal_room_id IS NOT NULL
ORDER BY r.created_at DESC;

-- name: ListPendingDocumentLinkAccessRequestsDetailedByWorkspace :many
-- Document Library share inbox: document links only (never deal-room shares).
-- Creator-scoped: only link.created_by may see applicant emails.
SELECT
    r.id,
    r.link_id,
    r.email,
    r.reason,
    r.signer_name,
    r.status,
    r.created_at,
    r.updated_at,
    l.name AS link_name,
    l.public_token,
    l.custom_domain,
    COALESCE(
        (
            SELECT d.title
            FROM documents d
            WHERE d.id = l.document_id
              AND d.deleted_at IS NULL
        ),
        (
            SELECT d.title
            FROM link_documents ld
            JOIN documents d ON d.id = ld.document_id AND d.deleted_at IS NULL
            WHERE ld.link_id = l.id
            ORDER BY ld.sort_order ASC, ld.created_at ASC
            LIMIT 1
        ),
        COALESCE(l.name, '')
    )::text AS document_title
FROM link_access_requests r
JOIN links l ON l.id = r.link_id
WHERE r.workspace_id = $1
  AND l.workspace_id = $1
  AND l.created_by = $2
  AND r.status = 'pending'
  AND l.status NOT IN ('deleted', 'disabled')
  AND l.deal_room_id IS NULL
  AND l.document_id IS NOT NULL
ORDER BY r.created_at DESC;

-- name: ListPendingDealRoomLinkAccessRequestsDetailedByWorkspace :many
-- Deal-room share inbox: pending requests for links in one room only.
-- Creator-scoped: only link.created_by may see applicant emails.
SELECT
    r.id,
    r.link_id,
    r.email,
    r.reason,
    r.signer_name,
    r.status,
    r.created_at,
    r.updated_at,
    l.name AS link_name,
    l.public_token,
    l.custom_domain,
    COALESCE(
        (
            SELECT d.title
            FROM documents d
            WHERE d.id = l.document_id
              AND d.deleted_at IS NULL
        ),
        (
            SELECT d.title
            FROM link_documents ld
            JOIN documents d ON d.id = ld.document_id AND d.deleted_at IS NULL
            WHERE ld.link_id = l.id
            ORDER BY ld.sort_order ASC, ld.created_at ASC
            LIMIT 1
        ),
        COALESCE(l.name, '')
    )::text AS document_title
FROM link_access_requests r
JOIN links l ON l.id = r.link_id
WHERE r.workspace_id = $1
  AND l.workspace_id = $1
  AND l.created_by = $2
  AND r.status = 'pending'
  AND l.status NOT IN ('deleted', 'disabled')
  AND l.deal_room_id = $3
ORDER BY r.created_at DESC;

-- name: GetLinkAccessRequestByIDAndWorkspace :one
SELECT *
FROM link_access_requests
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

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

-- name: ListPendingAskTurnsByWorkspace :many
SELECT t.id, s.visitor_email, t.question, t.link_id, l.name AS link_name, l.deal_room_id
FROM link_ask_turns t
JOIN link_ask_sessions s ON s.id = t.session_id
JOIN links l ON l.id = t.link_id
WHERE t.workspace_id = $1
  AND t.lane IN ('host', 'hybrid')
  AND t.status IN ('host_pending', 'host_escalated')
  AND (t.formal_status IS NULL OR t.formal_status NOT IN ('pending_review', 'scheduled'))
ORDER BY t.created_at DESC;

-- name: ListPendingFormalAskTurnsByWorkspace :many
SELECT t.id, s.visitor_email, t.question, t.link_id, l.name AS link_name, l.deal_room_id
FROM link_ask_turns t
JOIN link_ask_sessions s ON s.id = t.session_id
JOIN links l ON l.id = t.link_id
WHERE t.workspace_id = $1
  AND t.lane IN ('host', 'hybrid')
  AND t.status IN ('host_pending', 'host_escalated')
  AND t.formal_status IN ('pending_review', 'scheduled')
ORDER BY t.created_at DESC;

-- name: ListPendingUploadedFilesByWorkspace :many
SELECT f.id, f.original_filename, f.link_id, l.name AS link_name
FROM link_uploaded_files f
JOIN links l ON l.id = f.link_id
WHERE f.workspace_id = $1 AND f.status = 'pending_review'
ORDER BY f.created_at DESC;

-- name: ListAskQARecordsByLink :many
SELECT t.question, t.host_answer, s.visitor_email, t.created_at
FROM link_ask_turns t
JOIN link_ask_sessions s ON s.id = t.session_id
WHERE t.link_id = $1
  AND t.host_answer IS NOT NULL
  AND btrim(t.host_answer) <> ''
ORDER BY t.created_at DESC;

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

-- name: GetWorkspaceDailyLinkOpens :many
-- UTC calendar-day series of link opens for Insights overview trend.
SELECT
    ((al.created_at AT TIME ZONE 'UTC')::date)::text AS day,
    COUNT(*)::bigint AS opens,
    COUNT(
      DISTINCT COALESCE(
        NULLIF(al.visitor_id, ''),
        LOWER(NULLIF(al.visitor_email, ''))
      )
    )::bigint AS unique_visitors
FROM access_logs al
WHERE al.workspace_id = sqlc.arg(workspace_id)
  AND al.event_type = 'link_opened'
  AND al.created_at >= (timezone('utc', now()) - (sqlc.arg(day_count)::int * interval '1 day'))
GROUP BY 1
ORDER BY 1;

-- name: GetWorkspaceDailyLinkOpensInRange :many
-- Dense-capable daily opens for an arbitrary UTC half-open window [start, end).
SELECT
    ((al.created_at AT TIME ZONE 'UTC')::date)::text AS day,
    COUNT(*)::bigint AS opens,
    COUNT(
      DISTINCT COALESCE(
        NULLIF(al.visitor_id, ''),
        LOWER(NULLIF(al.visitor_email, ''))
      )
    )::bigint AS unique_visitors
FROM access_logs al
WHERE al.workspace_id = sqlc.arg(workspace_id)
  AND al.event_type = 'link_opened'
  AND al.created_at >= sqlc.arg(range_start)
  AND al.created_at < sqlc.arg(range_end)
GROUP BY 1
ORDER BY 1;

-- name: CountWorkspaceLinkOpenVisitorsInRange :one
-- Distinct visitors with ≥1 link_opened in [range_start, range_end) (UTC).
SELECT COUNT(
  DISTINCT COALESCE(
    NULLIF(al.visitor_id, ''),
    LOWER(NULLIF(al.visitor_email, ''))
  )
)::bigint AS unique_visitors
FROM access_logs al
WHERE al.workspace_id = sqlc.arg(workspace_id)
  AND al.event_type = 'link_opened'
  AND al.created_at >= sqlc.arg(range_start)
  AND al.created_at < sqlc.arg(range_end);

-- name: CountWorkspaceLinkOpensInRange :one
SELECT COUNT(*)::bigint AS opens
FROM access_logs al
WHERE al.workspace_id = sqlc.arg(workspace_id)
  AND al.event_type = 'link_opened'
  AND al.created_at >= sqlc.arg(range_start)
  AND al.created_at < sqlc.arg(range_end);

-- name: ListEnabledDailyDigestRules :many
SELECT id, tenant_id, workspace_id, rule_type, channels, enabled, unsubscribable, merge_window_minutes, created_at, updated_at
FROM notification_rules
WHERE rule_type = 'daily_digest'
  AND enabled = true
ORDER BY workspace_id;

-- name: CountDigestNotificationsForDay :one
SELECT COUNT(*)::bigint
FROM notifications n
WHERE n.workspace_id = sqlc.arg(workspace_id)
  AND n.channel = sqlc.arg(channel)
  AND n.metadata->>'rule_type' = 'daily_digest'
  AND n.metadata->>'digest_day' = sqlc.arg(digest_day)::text
  AND n.status IN ('pending', 'processing', 'sent', 'failed');

-- name: ListWorkspaceOwnerAdminIDs :many
SELECT user_id
FROM workspace_members
WHERE workspace_id = $1
  AND role IN ('owner', 'admin')
ORDER BY joined_at ASC;

-- name: GetWorkspacePageViewEngagementInRange :one
-- Page-view engagement for Insights overview KPI strip.
SELECT
    COUNT(*)::bigint AS page_view_count,
    COALESCE(AVG(pv.duration_seconds), 0)::float8 AS avg_duration_seconds,
    COALESCE(
      PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY pv.duration_seconds),
      0
    )::float8 AS median_duration_seconds
FROM page_views pv
WHERE pv.workspace_id = sqlc.arg(workspace_id)
  AND pv.created_at >= sqlc.arg(range_start)
  AND pv.created_at < sqlc.arg(range_end);

-- name: GetWorkspaceReadingSessionStatsInRange :one
-- Workspace reading-session completion for Insights command-center KPI.
-- Measurable = sessions whose document has a known page_count > 0.
-- Completed = max_page >= document.page_count (same rule as document funnel).
SELECT
    COUNT(*)::bigint AS session_count,
    COUNT(*) FILTER (
        WHERE d.page_count IS NOT NULL AND d.page_count > 0
    )::bigint AS measurable_sessions,
    COUNT(*) FILTER (
        WHERE d.page_count IS NOT NULL
          AND d.page_count > 0
          AND rs.max_page >= d.page_count
    )::bigint AS completed_sessions
FROM reading_sessions rs
JOIN documents d ON d.id = rs.document_id AND d.workspace_id = rs.workspace_id
WHERE rs.workspace_id = sqlc.arg(workspace_id)
  AND rs.document_id IS NOT NULL
  AND rs.last_activity_at >= sqlc.arg(range_start)
  AND rs.last_activity_at < sqlc.arg(range_end);

-- name: CountPendingQuestionsByWorkspace :one
SELECT COUNT(*) AS pending_count
FROM link_ask_turns
WHERE workspace_id = $1
  AND lane IN ('host', 'hybrid')
  AND status IN ('host_pending', 'host_escalated')
  AND (formal_status IS NULL OR formal_status NOT IN ('pending_review', 'scheduled'));

-- name: ListRecentActivitiesByWorkspace :many
-- Bound each leg before UNION so access_logs/questions/uploads cannot unbounded-scan.
SELECT
    id,
    event_type,
    actor,
    object_type,
    object_name,
    object_id,
    created_at
FROM (
    (
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
          AND al.created_at >= now() - interval '30 days'
        ORDER BY al.created_at DESC
        LIMIT $2
    )

    UNION ALL

    (
        SELECT
            t.id::text AS id,
            'question' AS event_type,
            COALESCE(NULLIF(s.visitor_email, ''), t.visitor_id, 'Unknown') AS actor,
            CASE WHEN l.deal_room_id IS NOT NULL THEN 'room' ELSE 'document' END AS object_type,
            COALESCE(dr.name, d.title, 'Shared link') AS object_name,
            COALESCE(dr.id, d.id, l.id)::text AS object_id,
            t.created_at
        FROM link_ask_turns t
        JOIN link_ask_sessions s ON s.id = t.session_id
        JOIN links l ON l.id = t.link_id
        LEFT JOIN deal_rooms dr ON dr.id = l.deal_room_id
        LEFT JOIN documents d ON d.id = l.document_id
        WHERE t.workspace_id = $1
          AND t.lane IN ('host', 'hybrid')
          AND t.created_at >= now() - interval '30 days'
        ORDER BY t.created_at DESC
        LIMIT $2
    )

    UNION ALL

    (
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
          AND d.created_at >= now() - interval '30 days'
        ORDER BY d.created_at DESC
        LIMIT $2
    )
) combined
ORDER BY created_at DESC
LIMIT $2;

-- name: ListSuggestionsByWorkspace :many
SELECT *
FROM suggestions
WHERE workspace_id = $1
  AND dismissed = false
  AND (snoozed_until IS NULL OR snoozed_until <= now())
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
    forward_signals,
    distinct_ips_1h,
    distinct_emails_24h,
    unknown_emails_24h,
    downloads_24h
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
ON CONFLICT (link_id, window_start) DO UPDATE SET
    opens = EXCLUDED.opens,
    unique_visitors = EXCLUDED.unique_visitors,
    revisits = EXCLUDED.revisits,
    avg_duration_seconds = EXCLUDED.avg_duration_seconds,
    total_page_views = EXCLUDED.total_page_views,
    key_page_views = EXCLUDED.key_page_views,
    downloads = EXCLUDED.downloads,
    bounces = EXCLUDED.bounces,
    forward_signals = EXCLUDED.forward_signals,
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
-- Split log stats and page-view stats to avoid access_logs ⨯ page_views row explosion.
-- key_page_views uses the same title LIKE patterns as link heat (heat.KeyPagePatterns).
-- forward_signals counts persisted access_logs.event_type='forward_signal' markers.
-- bounces matches GetLinkBounceCount: link_opened with visitor_id and no page_views on that link.
WITH email_logs AS (
    SELECT
        LOWER(al.visitor_email) AS email,
        al.id,
        al.link_id,
        al.visitor_id,
        al.event_type,
        al.created_at
    FROM access_logs al
    WHERE al.workspace_id = $1
      AND al.visitor_email IS NOT NULL
      AND al.visitor_email <> ''
),
log_stats AS (
    SELECT
        email,
        COUNT(DISTINCT id) FILTER (WHERE event_type = 'link_opened')::bigint AS opens,
        COUNT(DISTINCT link_id)::bigint AS unique_links,
        COUNT(DISTINCT visitor_id) FILTER (WHERE visitor_id IS NOT NULL AND visitor_id <> '')::bigint AS unique_visitors,
        COUNT(DISTINCT id) FILTER (WHERE event_type = 'download_attempted')::bigint AS downloads,
        COUNT(DISTINCT id) FILTER (WHERE event_type = 'forward_signal')::bigint AS forward_signals,
        MAX(created_at)::timestamptz AS last_seen_at
    FROM email_logs
    GROUP BY email
),
bounce_stats AS (
    SELECT
        el.email,
        COUNT(DISTINCT el.id)::bigint AS bounces
    FROM email_logs el
    WHERE el.event_type = 'link_opened'
      AND el.visitor_id IS NOT NULL
      AND el.visitor_id <> ''
      AND NOT EXISTS (
          SELECT 1
          FROM page_views p
          WHERE p.link_id = el.link_id
            AND p.visitor_id = el.visitor_id
      )
    GROUP BY el.email
),
visitor_emails AS (
    SELECT DISTINCT email, visitor_id
    FROM email_logs
    WHERE visitor_id IS NOT NULL AND visitor_id <> ''
),
pv_stats AS (
    SELECT
        ve.email,
        COALESCE(SUM(pv.duration_seconds), 0)::bigint AS total_duration_seconds,
        COUNT(pv.id)::bigint AS total_page_views
    FROM visitor_emails ve
    JOIN page_views pv ON pv.workspace_id = $1 AND pv.visitor_id = ve.visitor_id
    GROUP BY ve.email
),
key_page_stats AS (
    SELECT
        ve.email,
        COUNT(pv.id)::bigint AS key_page_views
    FROM visitor_emails ve
    JOIN page_views pv ON pv.workspace_id = $1 AND pv.visitor_id = ve.visitor_id
    JOIN links l ON l.id = pv.link_id AND l.workspace_id = $1 AND l.status != 'deleted'
    JOIN pages p ON p.document_id = COALESCE(pv.document_id, l.document_id) AND p.page_number = pv.page_number
    WHERE p.title IS NOT NULL AND p.title <> ''
      AND lower(p.title) LIKE ANY (sqlc.arg(patterns)::text[])
    GROUP BY ve.email
)
SELECT
    c.id AS contact_id,
    ls.email,
    ls.opens,
    ls.unique_links,
    ls.unique_visitors,
    COALESCE(ps.total_duration_seconds, 0)::bigint AS total_duration_seconds,
    COALESCE(ps.total_page_views, 0)::bigint AS total_page_views,
    COALESCE(kps.key_page_views, 0)::bigint AS key_page_views,
    ls.forward_signals,
    ls.downloads,
    COALESCE(bs.bounces, 0)::bigint AS bounces,
    ls.last_seen_at
FROM log_stats ls
LEFT JOIN contacts c ON c.workspace_id = $1 AND LOWER(c.email) = ls.email
LEFT JOIN pv_stats ps ON ps.email = ls.email
LEFT JOIN key_page_stats kps ON kps.email = ls.email
LEFT JOIN bounce_stats bs ON bs.email = ls.email
ORDER BY ls.opens DESC
LIMIT $2;

-- name: GetContactByID :one
SELECT id, workspace_id, email, name, created_at
FROM contacts
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: UpsertContactByEmail :one
INSERT INTO contacts (workspace_id, email, name)
VALUES (sqlc.arg(workspace_id), sqlc.arg(email), NULLIF(sqlc.arg(name), ''))
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
      WHERE c.workspace_id = al.workspace_id AND LOWER(c.email) = LOWER(al.visitor_email)
  );

-- name: GetContactAggregateByEmail :one
WITH email_logs AS (
    SELECT
        al.id,
        al.link_id,
        al.visitor_id,
        al.event_type,
        al.created_at
    FROM access_logs al
    WHERE al.workspace_id = $1 AND LOWER(al.visitor_email) = LOWER(sqlc.arg(visitor_email)::text)
),
log_stats AS (
    SELECT
        COUNT(DISTINCT id) FILTER (WHERE event_type = 'link_opened')::bigint AS opens,
        COUNT(DISTINCT link_id)::bigint AS unique_links,
        COUNT(DISTINCT visitor_id) FILTER (WHERE visitor_id IS NOT NULL AND visitor_id <> '')::bigint AS unique_visitors,
        COUNT(DISTINCT id) FILTER (WHERE event_type = 'download_attempted')::bigint AS downloads,
        COUNT(DISTINCT id) FILTER (WHERE event_type = 'forward_signal')::bigint AS forward_signals,
        MAX(created_at)::timestamptz AS last_seen_at
    FROM email_logs
),
bounce_stats AS (
    SELECT COUNT(DISTINCT el.id)::bigint AS bounces
    FROM email_logs el
    WHERE el.event_type = 'link_opened'
      AND el.visitor_id IS NOT NULL
      AND el.visitor_id <> ''
      AND NOT EXISTS (
          SELECT 1
          FROM page_views p
          WHERE p.link_id = el.link_id
            AND p.visitor_id = el.visitor_id
      )
),
visitor_ids AS (
    SELECT DISTINCT visitor_id
    FROM email_logs
    WHERE visitor_id IS NOT NULL AND visitor_id <> ''
),
pv_stats AS (
    SELECT
        COALESCE(SUM(pv.duration_seconds), 0)::bigint AS total_duration_seconds,
        COUNT(pv.id)::bigint AS total_page_views
    FROM page_views pv
    WHERE pv.workspace_id = $1
      AND pv.visitor_id IN (SELECT visitor_id FROM visitor_ids)
),
key_page_stats AS (
    SELECT COUNT(pv.id)::bigint AS key_page_views
    FROM page_views pv
    JOIN links l ON l.id = pv.link_id AND l.workspace_id = $1 AND l.status != 'deleted'
    JOIN pages p ON p.document_id = COALESCE(pv.document_id, l.document_id) AND p.page_number = pv.page_number
    WHERE pv.workspace_id = $1
      AND pv.visitor_id IN (SELECT visitor_id FROM visitor_ids)
      AND p.title IS NOT NULL AND p.title <> ''
      AND lower(p.title) LIKE ANY (sqlc.arg(patterns)::text[])
)
SELECT
    COALESCE((SELECT opens FROM log_stats), 0)::bigint AS opens,
    COALESCE((SELECT unique_links FROM log_stats), 0)::bigint AS unique_links,
    COALESCE((SELECT unique_visitors FROM log_stats), 0)::bigint AS unique_visitors,
    COALESCE((SELECT total_duration_seconds FROM pv_stats), 0)::bigint AS total_duration_seconds,
    COALESCE((SELECT total_page_views FROM pv_stats), 0)::bigint AS total_page_views,
    COALESCE((SELECT key_page_views FROM key_page_stats), 0)::bigint AS key_page_views,
    COALESCE((SELECT forward_signals FROM log_stats), 0)::bigint AS forward_signals,
    COALESCE((SELECT downloads FROM log_stats), 0)::bigint AS downloads,
    COALESCE((SELECT bounces FROM bounce_stats), 0)::bigint AS bounces,
    (SELECT last_seen_at FROM log_stats) AS last_seen_at;

-- name: ListContactActivitiesByEmail :many
WITH visitor_ids AS (
    SELECT DISTINCT al.visitor_id
    FROM access_logs al
    WHERE al.workspace_id = $1
      AND LOWER(al.visitor_email) = LOWER(sqlc.arg(visitor_email)::text)
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
    WHERE al2.workspace_id = $1 AND LOWER(al2.visitor_email) = LOWER(sqlc.arg(visitor_email)::text)
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
LIMIT sqlc.arg(row_limit);

-- name: ListContactViewedDocumentIDs :many
WITH visitor_ids AS (
    SELECT DISTINCT al.visitor_id
    FROM access_logs al
    WHERE al.workspace_id = $1
      AND LOWER(al.visitor_email) = LOWER(sqlc.arg(visitor_email)::text)
      AND al.visitor_id IS NOT NULL
      AND al.visitor_id <> ''
)
SELECT DISTINCT l.document_id::text AS document_id
FROM (
    SELECT link_id FROM access_logs al2
    WHERE al2.workspace_id = $1 AND LOWER(al2.visitor_email) = LOWER(sqlc.arg(visitor_email)::text)
    UNION
    SELECT link_id FROM page_views pv2
    WHERE pv2.workspace_id = $1 AND pv2.visitor_id IN (SELECT visitor_id FROM visitor_ids)
) e
JOIN links l ON l.id = e.link_id
WHERE l.document_id IS NOT NULL;

-- name: ListContactViewedDocuments :many
-- Viewed documents with titles for contact detail "Documents" tab.
WITH visitor_ids AS (
    SELECT DISTINCT al.visitor_id
    FROM access_logs al
    WHERE al.workspace_id = $1
      AND LOWER(al.visitor_email) = LOWER(sqlc.arg(visitor_email)::text)
      AND al.visitor_id IS NOT NULL
      AND al.visitor_id <> ''
)
SELECT DISTINCT
    l.document_id::text AS document_id,
    COALESCE(d.title, '')::text AS title
FROM (
    SELECT link_id FROM access_logs al2
    WHERE al2.workspace_id = $1 AND LOWER(al2.visitor_email) = LOWER(sqlc.arg(visitor_email)::text)
    UNION
    SELECT link_id FROM page_views pv2
    WHERE pv2.workspace_id = $1 AND pv2.visitor_id IN (SELECT visitor_id FROM visitor_ids)
) e
JOIN links l ON l.id = e.link_id
LEFT JOIN documents d ON d.id = l.document_id
WHERE l.document_id IS NOT NULL
ORDER BY title ASC;

-- name: ListContactViewedDocumentIDsByWorkspace :many
-- One-shot batch of viewed documents for all visitor emails in a workspace.
SELECT DISTINCT
    LOWER(al.visitor_email) AS email,
    l.document_id::text AS document_id
FROM access_logs al
JOIN links l ON l.id = al.link_id AND l.document_id IS NOT NULL
WHERE al.workspace_id = $1
  AND al.visitor_email IS NOT NULL
  AND al.visitor_email <> ''
UNION
SELECT DISTINCT
    LOWER(al.visitor_email) AS email,
    l.document_id::text AS document_id
FROM access_logs al
JOIN page_views pv ON pv.workspace_id = al.workspace_id
    AND pv.visitor_id = al.visitor_id
JOIN links l ON l.id = pv.link_id AND l.document_id IS NOT NULL
WHERE al.workspace_id = $1
  AND al.visitor_email IS NOT NULL
  AND al.visitor_email <> ''
  AND al.visitor_id IS NOT NULL
  AND al.visitor_id <> '';

-- name: CreateContact :one
INSERT INTO contacts (workspace_id, email, name)
VALUES (sqlc.arg(workspace_id), sqlc.arg(email), NULLIF(sqlc.arg(name), ''))
RETURNING id, workspace_id, email, name, created_at;

-- name: CreateLinkContact :exec
INSERT INTO link_contacts (link_id, contact_id, access_code, code_send_status)
VALUES ($1, $2, $3, 'pending');

-- name: CreateLinkContactWithDelivery :exec
-- Preserves delivery metadata when recreating link_contacts (e.g. document link update).
INSERT INTO link_contacts (
    link_id, contact_id, access_code,
    code_send_status, code_send_error, code_sent_at, used_at
) VALUES (
    $1, $2, $3,
    sqlc.arg(code_send_status),
    NULLIF(sqlc.arg(code_send_error), ''),
    sqlc.narg(code_sent_at),
    sqlc.narg(used_at)
);

-- name: DeleteLinkContactsByLink :exec
DELETE FROM link_contacts
WHERE link_id = $1;

-- name: ListLinkContactsByLinkID :many
SELECT lc.contact_id
FROM link_contacts lc
WHERE lc.link_id = $1;

-- name: GetLinkContactsByPublicToken :many
SELECT lc.id, lc.link_id, lc.contact_id, lc.access_code, lc.code_sent_at, lc.used_at, lc.created_at,
       lc.code_send_status, lc.code_send_error,
       c.email AS contact_email, c.name AS contact_name
FROM link_contacts lc
JOIN links l ON l.id = lc.link_id
JOIN contacts c ON c.id = lc.contact_id
WHERE l.public_token = $1;

-- name: GetLinkContactByEmail :one
SELECT lc.id, lc.link_id, lc.contact_id, lc.access_code, lc.code_sent_at, lc.used_at, lc.created_at,
       lc.code_send_status, lc.code_send_error,
       c.email AS contact_email, c.name AS contact_name
FROM link_contacts lc
JOIN links l ON l.id = lc.link_id
JOIN contacts c ON c.id = lc.contact_id
WHERE l.public_token = $1 AND c.email = $2
LIMIT 1;

-- name: GetLinkContactByCode :one
SELECT lc.id, lc.link_id, lc.contact_id, lc.access_code, lc.code_sent_at, lc.used_at, lc.created_at,
       lc.code_send_status, lc.code_send_error,
       c.email AS contact_email, c.name AS contact_name
FROM link_contacts lc
JOIN links l ON l.id = lc.link_id
JOIN contacts c ON c.id = lc.contact_id
WHERE l.public_token = $1 AND lc.access_code = $2
LIMIT 1;

-- name: UpdateLinkContactAccessCode :exec
UPDATE link_contacts
SET access_code = $2,
    code_sent_at = now(),
    used_at = NULL,
    code_send_status = 'pending',
    code_send_error = NULL
WHERE id = $1;

-- name: UpdateLinkContactSendStatusByEmail :exec
UPDATE link_contacts lc
SET code_send_status = sqlc.arg(status),
    code_send_error = NULLIF(sqlc.arg(error_message), '')
FROM links l, contacts c
WHERE lc.link_id = l.id
  AND lc.contact_id = c.id
  AND l.public_token = sqlc.arg(public_token)
  AND lower(c.email) = lower(sqlc.arg(email));

-- name: ListLinkAccessCodeContactsByLink :many
SELECT
    c.email::text AS contact_email,
    COALESCE(c.name, '')::text AS contact_name,
    lc.code_sent_at,
    lc.code_send_status,
    COALESCE(lc.code_send_error, '')::text AS code_send_error,
    lc.used_at,
    lc.created_at
FROM link_contacts lc
JOIN contacts c ON c.id = lc.contact_id
WHERE lc.link_id = $1
ORDER BY lc.code_sent_at DESC NULLS LAST, c.email ASC
LIMIT $2 OFFSET $3;

-- name: CountLinkAccessCodeFailedByLink :one
SELECT COUNT(*)::bigint AS count
FROM link_contacts
WHERE link_id = $1 AND code_send_status = 'failed';

-- name: CountLinkAccessCodeRemediableByLink :one
SELECT COUNT(*)::bigint AS count
FROM link_contacts
WHERE link_id = $1
  AND (
    code_send_status = 'failed'
    OR (
      code_send_status = 'pending'
      AND created_at <= now() - interval '2 minutes'
    )
  );

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

-- name: CountCaptureAttemptsByLink24h :one
SELECT COUNT(*)::bigint AS count
FROM security_events
WHERE link_id = $1
  AND event_type = 'capture_attempt'
  AND created_at > now() - interval '24 hours';

-- name: CountCaptureAttempts24hBatch :many
SELECT link_id, COUNT(*)::bigint AS count
FROM security_events
WHERE link_id = ANY($1::uuid[])
  AND event_type = 'capture_attempt'
  AND created_at > now() - interval '24 hours'
GROUP BY link_id;

-- name: ListAskHighRiskSecurityEventsByLink :many
-- Owner-visible Visitor Ask high-risk events (US#32): block (blocked_email /
-- blocked_domain / not_in_allow_list), scope_violation, rate_limit_exceeded.
SELECT id, link_id, event_type, visitor_id, email, ip, user_agent, reason, created_at
FROM security_events
WHERE link_id = sqlc.arg(link_id)
  AND event_type = ANY (ARRAY[
    'rate_limit_exceeded',
    'scope_violation',
    'blocked_email',
    'blocked_domain',
    'not_in_allow_list',
    'ask_ai_rate_limited',
    'ask_escalated',
    'ask_formal_submitted'
  ]::text[])
  AND (sqlc.narg(event_type)::text IS NULL OR event_type = sqlc.narg(event_type))
  AND (sqlc.narg(created_after)::timestamptz IS NULL OR created_at >= sqlc.narg(created_after))
  AND (sqlc.narg(created_before)::timestamptz IS NULL OR created_at < sqlc.narg(created_before))
ORDER BY created_at DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListAskHighRiskSecurityEventsByRoom :many
SELECT
    se.id,
    se.link_id,
    se.event_type,
    se.visitor_id,
    se.email,
    se.ip,
    se.user_agent,
    se.reason,
    se.created_at
FROM security_events se
INNER JOIN links l ON l.id = se.link_id
WHERE l.deal_room_id = sqlc.arg(deal_room_id)
  AND l.workspace_id = sqlc.arg(workspace_id)
  AND l.status NOT IN ('deleted', 'disabled')
  AND (sqlc.narg(link_id)::uuid IS NULL OR se.link_id = sqlc.narg(link_id))
  AND se.event_type = ANY (ARRAY[
    'rate_limit_exceeded',
    'scope_violation',
    'blocked_email',
    'blocked_domain',
    'not_in_allow_list',
    'ask_ai_rate_limited',
    'ask_escalated',
    'ask_formal_submitted'
  ]::text[])
  AND (sqlc.narg(event_type)::text IS NULL OR se.event_type = sqlc.narg(event_type))
  AND (sqlc.narg(created_after)::timestamptz IS NULL OR se.created_at >= sqlc.narg(created_after))
  AND (sqlc.narg(created_before)::timestamptz IS NULL OR se.created_at < sqlc.narg(created_before))
ORDER BY se.created_at DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- Insights access-audit: permission / gate failures across the workspace.
-- Folder grain: document placement path, else link target_folder_path (deal-room shares).
-- name: CountWorkspaceAccessAuditByType :many
SELECT se.event_type, COUNT(*)::bigint AS count
FROM security_events se
LEFT JOIN links l ON l.id = se.link_id
LEFT JOIN deal_room_documents drd
  ON drd.document_id = l.document_id
 AND drd.room_id = l.deal_room_id
WHERE se.workspace_id = sqlc.arg(workspace_id)
  AND se.created_at >= sqlc.arg(range_start)
  AND se.created_at < sqlc.arg(range_end)
  AND se.event_type = ANY (ARRAY[
    'blocked_email',
    'blocked_domain',
    'not_in_allow_list',
    'no_allow_match',
    'invalid_password',
    'scope_violation',
    'security_gate_failed',
    'session_security_config_changed',
    'expired_link_accessed',
    'revoked_link_accessed',
    'max_access_reached',
    'invite_token_failed',
    'invite_token_expired',
    'invite_token_revoked',
    'rate_limit_exceeded'
  ]::text[])
  AND (sqlc.narg(event_type)::text IS NULL OR se.event_type = sqlc.narg(event_type))
  AND (sqlc.narg(deal_room_id)::uuid IS NULL OR l.deal_room_id = sqlc.narg(deal_room_id))
  AND (sqlc.narg(member_id)::uuid IS NULL OR l.created_by = sqlc.narg(member_id))
  AND (
    sqlc.narg(folder_path)::text IS NULL
    OR COALESCE(NULLIF(BTRIM(drd.folder_path), ''), NULLIF(BTRIM(l.target_folder_path), ''), '') = sqlc.narg(folder_path)
  )
GROUP BY se.event_type
ORDER BY count DESC, se.event_type ASC;

-- name: CountWorkspaceAccessAuditByDealRoom :many
SELECT
    l.deal_room_id,
    COALESCE(dr.name, '')::text AS deal_room_name,
    COUNT(*)::bigint AS count
FROM security_events se
LEFT JOIN links l ON l.id = se.link_id
LEFT JOIN deal_rooms dr ON dr.id = l.deal_room_id AND dr.workspace_id = se.workspace_id
LEFT JOIN deal_room_documents drd
  ON drd.document_id = l.document_id
 AND drd.room_id = l.deal_room_id
WHERE se.workspace_id = sqlc.arg(workspace_id)
  AND se.created_at >= sqlc.arg(range_start)
  AND se.created_at < sqlc.arg(range_end)
  AND se.event_type = ANY (ARRAY[
    'blocked_email',
    'blocked_domain',
    'not_in_allow_list',
    'no_allow_match',
    'invalid_password',
    'scope_violation',
    'security_gate_failed',
    'session_security_config_changed',
    'expired_link_accessed',
    'revoked_link_accessed',
    'max_access_reached',
    'invite_token_failed',
    'invite_token_expired',
    'invite_token_revoked',
    'rate_limit_exceeded'
  ]::text[])
  AND (sqlc.narg(event_type)::text IS NULL OR se.event_type = sqlc.narg(event_type))
  AND (sqlc.narg(member_id)::uuid IS NULL OR l.created_by = sqlc.narg(member_id))
  AND (
    sqlc.narg(folder_path)::text IS NULL
    OR COALESCE(NULLIF(BTRIM(drd.folder_path), ''), NULLIF(BTRIM(l.target_folder_path), ''), '') = sqlc.narg(folder_path)
  )
GROUP BY l.deal_room_id, dr.name
ORDER BY count DESC
LIMIT 20;

-- name: CountWorkspaceAccessAuditByMember :many
SELECT
    l.created_by AS member_id,
    COALESCE(u.email, '')::text AS member_email,
    COUNT(*)::bigint AS count
FROM security_events se
LEFT JOIN links l ON l.id = se.link_id
LEFT JOIN users u ON u.id = l.created_by
LEFT JOIN deal_room_documents drd
  ON drd.document_id = l.document_id
 AND drd.room_id = l.deal_room_id
WHERE se.workspace_id = sqlc.arg(workspace_id)
  AND se.created_at >= sqlc.arg(range_start)
  AND se.created_at < sqlc.arg(range_end)
  AND se.event_type = ANY (ARRAY[
    'blocked_email',
    'blocked_domain',
    'not_in_allow_list',
    'no_allow_match',
    'invalid_password',
    'scope_violation',
    'security_gate_failed',
    'session_security_config_changed',
    'expired_link_accessed',
    'revoked_link_accessed',
    'max_access_reached',
    'invite_token_failed',
    'invite_token_expired',
    'invite_token_revoked',
    'rate_limit_exceeded'
  ]::text[])
  AND (sqlc.narg(event_type)::text IS NULL OR se.event_type = sqlc.narg(event_type))
  AND (sqlc.narg(deal_room_id)::uuid IS NULL OR l.deal_room_id = sqlc.narg(deal_room_id))
  AND (
    sqlc.narg(folder_path)::text IS NULL
    OR COALESCE(NULLIF(BTRIM(drd.folder_path), ''), NULLIF(BTRIM(l.target_folder_path), ''), '') = sqlc.narg(folder_path)
  )
GROUP BY l.created_by, u.email
ORDER BY count DESC
LIMIT 20;

-- name: CountWorkspaceAccessAuditByFolder :many
SELECT
    COALESCE(NULLIF(BTRIM(drd.folder_path), ''), NULLIF(BTRIM(l.target_folder_path), ''), '')::text AS folder_path,
    l.deal_room_id,
    COALESCE(dr.name, '')::text AS deal_room_name,
    COUNT(*)::bigint AS count
FROM security_events se
LEFT JOIN links l ON l.id = se.link_id
LEFT JOIN deal_rooms dr ON dr.id = l.deal_room_id AND dr.workspace_id = se.workspace_id
LEFT JOIN deal_room_documents drd
  ON drd.document_id = l.document_id
 AND drd.room_id = l.deal_room_id
WHERE se.workspace_id = sqlc.arg(workspace_id)
  AND se.created_at >= sqlc.arg(range_start)
  AND se.created_at < sqlc.arg(range_end)
  AND se.event_type = ANY (ARRAY[
    'blocked_email',
    'blocked_domain',
    'not_in_allow_list',
    'no_allow_match',
    'invalid_password',
    'scope_violation',
    'security_gate_failed',
    'session_security_config_changed',
    'expired_link_accessed',
    'revoked_link_accessed',
    'max_access_reached',
    'invite_token_failed',
    'invite_token_expired',
    'invite_token_revoked',
    'rate_limit_exceeded'
  ]::text[])
  AND (sqlc.narg(event_type)::text IS NULL OR se.event_type = sqlc.narg(event_type))
  AND (sqlc.narg(deal_room_id)::uuid IS NULL OR l.deal_room_id = sqlc.narg(deal_room_id))
  AND (sqlc.narg(member_id)::uuid IS NULL OR l.created_by = sqlc.narg(member_id))
GROUP BY
    COALESCE(NULLIF(BTRIM(drd.folder_path), ''), NULLIF(BTRIM(l.target_folder_path), ''), ''),
    l.deal_room_id,
    dr.name
ORDER BY count DESC
LIMIT 20;

-- name: ListWorkspaceAccessAuditEvents :many
SELECT
    se.id,
    se.link_id,
    se.event_type,
    se.visitor_id,
    se.email,
    se.reason,
    se.created_at,
    COALESCE(
        NULLIF(d.title, ''),
        NULLIF(dr.name, ''),
        NULLIF(l.name, ''),
        ''
    )::text AS document_title,
    l.deal_room_id,
    COALESCE(dr.name, '')::text AS deal_room_name,
    l.created_by AS member_id,
    COALESCE(u.email, '')::text AS member_email,
    COALESCE(NULLIF(BTRIM(drd.folder_path), ''), NULLIF(BTRIM(l.target_folder_path), ''), '')::text AS folder_path
FROM security_events se
LEFT JOIN links l ON l.id = se.link_id
LEFT JOIN documents d ON d.id = l.document_id
LEFT JOIN deal_rooms dr ON dr.id = l.deal_room_id AND dr.workspace_id = se.workspace_id
LEFT JOIN users u ON u.id = l.created_by
LEFT JOIN deal_room_documents drd
  ON drd.document_id = l.document_id
 AND drd.room_id = l.deal_room_id
WHERE se.workspace_id = sqlc.arg(workspace_id)
  AND se.created_at >= sqlc.arg(range_start)
  AND se.created_at < sqlc.arg(range_end)
  AND se.event_type = ANY (ARRAY[
    'blocked_email',
    'blocked_domain',
    'not_in_allow_list',
    'no_allow_match',
    'invalid_password',
    'scope_violation',
    'security_gate_failed',
    'session_security_config_changed',
    'expired_link_accessed',
    'revoked_link_accessed',
    'max_access_reached',
    'invite_token_failed',
    'invite_token_expired',
    'invite_token_revoked',
    'rate_limit_exceeded'
  ]::text[])
  AND (sqlc.narg(event_type)::text IS NULL OR se.event_type = sqlc.narg(event_type))
  AND (sqlc.narg(deal_room_id)::uuid IS NULL OR l.deal_room_id = sqlc.narg(deal_room_id))
  AND (sqlc.narg(member_id)::uuid IS NULL OR l.created_by = sqlc.narg(member_id))
  AND (
    sqlc.narg(folder_path)::text IS NULL
    OR COALESCE(NULLIF(BTRIM(drd.folder_path), ''), NULLIF(BTRIM(l.target_folder_path), ''), '') = sqlc.narg(folder_path)
  )
ORDER BY se.created_at DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- Insights key-page compliance: views whose page title matches heat key-page keywords.
-- name: GetWorkspaceKeyPageComplianceSummary :one
SELECT
    COUNT(*)::bigint AS total_views,
    COUNT(*) FILTER (WHERE pv.duration_seconds >= 3)::bigint AS engaged_views,
    COUNT(DISTINCT pv.visitor_id) FILTER (
        WHERE pv.visitor_id IS NOT NULL AND btrim(pv.visitor_id) <> ''
    )::bigint AS unique_visitors,
    COUNT(DISTINCT (COALESCE(pv.document_id, l.document_id), pv.page_number))::bigint AS distinct_pages
FROM page_views pv
JOIN links l ON l.id = pv.link_id
JOIN pages p ON p.document_id = COALESCE(pv.document_id, l.document_id) AND p.page_number = pv.page_number
WHERE pv.workspace_id = sqlc.arg(workspace_id)
  AND pv.created_at >= sqlc.arg(range_start)
  AND pv.created_at < sqlc.arg(range_end)
  AND p.title IS NOT NULL AND btrim(p.title) <> ''
  AND lower(p.title) LIKE ANY (sqlc.arg(patterns)::text[]);

-- name: ListWorkspaceKeyPageComplianceByPage :many
SELECT
    COALESCE(pv.document_id, l.document_id) AS document_id,
    COALESCE(d.title, '')::text AS document_title,
    pv.page_number,
    COALESCE(NULLIF(TRIM(p.title), ''), 'Page ' || pv.page_number)::text AS page_title,
    COUNT(*)::bigint AS views,
    COUNT(DISTINCT pv.visitor_id) FILTER (
        WHERE pv.visitor_id IS NOT NULL AND btrim(pv.visitor_id) <> ''
    )::bigint AS unique_visitors,
    COALESCE(AVG(pv.duration_seconds), 0)::float8 AS avg_duration_seconds,
    MAX(pv.created_at)::timestamptz AS last_viewed_at
FROM page_views pv
JOIN links l ON l.id = pv.link_id
JOIN documents d ON d.id = COALESCE(pv.document_id, l.document_id)
JOIN pages p ON p.document_id = COALESCE(pv.document_id, l.document_id) AND p.page_number = pv.page_number
WHERE pv.workspace_id = sqlc.arg(workspace_id)
  AND pv.created_at >= sqlc.arg(range_start)
  AND pv.created_at < sqlc.arg(range_end)
  AND p.title IS NOT NULL AND btrim(p.title) <> ''
  AND lower(p.title) LIKE ANY (sqlc.arg(patterns)::text[])
GROUP BY COALESCE(pv.document_id, l.document_id), d.title, pv.page_number, p.title
ORDER BY views DESC, last_viewed_at DESC NULLS LAST
LIMIT 50;

-- name: ListWorkspaceKeyPageComplianceEvents :many
WITH visitor_emails AS (
    SELECT al.visitor_id, MAX(al.visitor_email) AS visitor_email
    FROM access_logs al
    WHERE al.workspace_id = sqlc.arg(workspace_id)
      AND al.visitor_id IS NOT NULL
      AND al.visitor_email IS NOT NULL
      AND al.visitor_email <> ''
    GROUP BY al.visitor_id
)
SELECT
    pv.id,
    pv.link_id,
    COALESCE(pv.visitor_id, '')::text AS visitor_id,
    COALESCE(ve.visitor_email, '')::text AS visitor_email,
    COALESCE(pv.document_id, l.document_id) AS document_id,
    COALESCE(d.title, '')::text AS document_title,
    pv.page_number,
    COALESCE(NULLIF(TRIM(p.title), ''), 'Page ' || pv.page_number)::text AS page_title,
    pv.duration_seconds,
    pv.created_at,
    l.deal_room_id,
    COALESCE(dr.name, '')::text AS deal_room_name
FROM page_views pv
JOIN links l ON l.id = pv.link_id
LEFT JOIN documents d ON d.id = COALESCE(pv.document_id, l.document_id)
JOIN pages p ON p.document_id = COALESCE(pv.document_id, l.document_id) AND p.page_number = pv.page_number
LEFT JOIN deal_rooms dr ON dr.id = l.deal_room_id AND dr.workspace_id = pv.workspace_id
LEFT JOIN visitor_emails ve ON ve.visitor_id = pv.visitor_id
WHERE pv.workspace_id = sqlc.arg(workspace_id)
  AND pv.created_at >= sqlc.arg(range_start)
  AND pv.created_at < sqlc.arg(range_end)
  AND p.title IS NOT NULL AND btrim(p.title) <> ''
  AND lower(p.title) LIKE ANY (sqlc.arg(patterns)::text[])
ORDER BY pv.created_at DESC, pv.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

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
-- Deal-room share list: room-scoped links only (never document-library shares).
SELECT *
FROM links
WHERE workspace_id = $1 AND deal_room_id = $2 AND status NOT IN ('deleted', 'disabled')
ORDER BY created_at DESC;

-- name: GetDealRoomAnalytics :one
WITH room_links AS (
    SELECT l.id, l.status
    FROM links l
    WHERE l.workspace_id = sqlc.arg(workspace_id)
      AND l.deal_room_id = sqlc.arg(deal_room_id)
      AND l.status NOT IN ('deleted', 'disabled')
),
link_access AS (
    SELECT al.visitor_id, al.created_at
    FROM access_logs al
    WHERE al.link_id IN (SELECT id FROM room_links)
      AND al.event_type = 'link_opened'
),
daily_views AS (
    SELECT DATE(la.created_at)::text AS day, COUNT(*)::bigint AS views
    FROM link_access la
    WHERE la.created_at >= now() - interval '30 days'
    GROUP BY DATE(la.created_at)
    ORDER BY day
)
SELECT
    COALESCE((SELECT COUNT(*) FROM link_access), 0)::bigint AS total_views,
    COALESCE(
        (SELECT COUNT(DISTINCT la.visitor_id)
         FROM link_access la
         WHERE la.visitor_id IS NOT NULL AND la.visitor_id <> ''),
        0
    )::bigint AS unique_visitors,
    COALESCE(
        (SELECT COUNT(*) FROM room_links rl WHERE rl.status = 'active'),
        0
    )::bigint AS active_link_count,
    COALESCE(
        (SELECT COUNT(*)::bigint
         FROM deal_room_documents drd
         WHERE drd.room_id = sqlc.arg(deal_room_id)),
        0
    )::bigint AS document_count,
    COALESCE(
        (SELECT jsonb_agg(jsonb_build_object('day', dv.day, 'views', dv.views) ORDER BY dv.day)
         FROM daily_views dv),
        '[]'::jsonb
    )::jsonb AS views_over_time;

-- name: ListRecentVisitorsByDealRoom :many
SELECT
    COALESCE(al.visitor_id, '')::text AS visitor_id,
    COALESCE(MAX(al.visitor_email), '')::text AS visitor_email,
    MIN(al.created_at)::timestamptz AS first_access_at,
    MAX(al.created_at)::timestamptz AS last_access_at,
    COUNT(*) FILTER (WHERE al.event_type = 'link_opened')::bigint AS total_views
FROM access_logs al
WHERE al.link_id IN (
        SELECT l.id
        FROM links l
        WHERE l.workspace_id = sqlc.arg(workspace_id)
          AND l.deal_room_id = sqlc.arg(deal_room_id)
          AND l.status NOT IN ('deleted', 'disabled')
    )
  AND al.visitor_id IS NOT NULL
  AND al.visitor_id <> ''
GROUP BY al.visitor_id
ORDER BY last_access_at DESC, al.visitor_id ASC
LIMIT sqlc.arg(page_limit);

-- name: CountLinksByDealRoomFiltered :one
SELECT count(*)::bigint
FROM links
WHERE workspace_id = sqlc.arg(workspace_id)
  AND deal_room_id = sqlc.arg(deal_room_id)
  AND status NOT IN ('deleted', 'disabled')
  AND (
    sqlc.arg(query)::text = ''
    OR coalesce(name, '') ILIKE '%' || sqlc.arg(query) || '%' ESCAPE '\'
    OR public_token ILIKE '%' || sqlc.arg(query) || '%' ESCAPE '\'
  );

-- name: ListLinksByDealRoomPage :many
SELECT *
FROM links
WHERE workspace_id = sqlc.arg(workspace_id)
  AND deal_room_id = sqlc.arg(deal_room_id)
  AND status NOT IN ('deleted', 'disabled')
  AND (
    sqlc.arg(query)::text = ''
    OR coalesce(name, '') ILIKE '%' || sqlc.arg(query) || '%' ESCAPE '\'
    OR public_token ILIKE '%' || sqlc.arg(query) || '%' ESCAPE '\'
  )
ORDER BY
  CASE WHEN sqlc.arg(sort_asc)::boolean THEN created_at END ASC NULLS LAST,
  CASE WHEN NOT sqlc.arg(sort_asc)::boolean THEN created_at END DESC NULLS LAST,
  id ASC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

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

-- name: ConsumeLinkInvitation :one
-- Atomically mark a pending invitation as used. Concurrent Access calls race
-- here; only one RETURNING row wins and may proceed.
UPDATE link_invitations
SET status = 'used',
    used_at = now(),
    updated_at = now()
WHERE id = $1
  AND status = 'pending'
  AND (expires_at IS NULL OR expires_at > now())
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
    tenant_id, workspace_id, link_id, email, reason, signer_name, status
) VALUES ($1, $2, $3, $4, $5, $6, 'pending')
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
WHERE id = $3 AND status = 'pending'
RETURNING *;

-- name: ReopenLinkAccessRequest :one
UPDATE link_access_requests
SET status = 'pending',
    reason = $2,
    signer_name = $3,
    reviewed_by = NULL,
    reviewed_at = NULL,
    updated_at = now()
WHERE id = $1 AND status <> 'pending'
RETURNING *;

-- name: RejectApprovedLinkAccessRequestByEmail :one
UPDATE link_access_requests
SET status = 'rejected',
    reviewed_by = $3,
    reviewed_at = now(),
    updated_at = now()
WHERE link_id = $1
  AND email = $2
  AND status = 'approved'
RETURNING *;

-- name: GetLinkAskSessionByLinkVisitor :one
SELECT *
FROM link_ask_sessions
WHERE link_id = $1 AND visitor_id = $2
LIMIT 1;

-- name: GetLinkAskSessionByID :one
SELECT *
FROM link_ask_sessions
WHERE id = $1
LIMIT 1;

-- name: CreateLinkAskSession :one
INSERT INTO link_ask_sessions (
    tenant_id, workspace_id, link_id, visitor_id, visitor_email
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: TouchLinkAskSession :one
UPDATE link_ask_sessions
SET updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetLinkAskSessionVisitorEmailIfEmpty :one
UPDATE link_ask_sessions
SET visitor_email = $2,
    updated_at = now()
WHERE id = $1
  AND (visitor_email IS NULL OR visitor_email = '')
RETURNING *;

-- name: CreateLinkAskTurn :one
INSERT INTO link_ask_turns (
    session_id,
    tenant_id,
    workspace_id,
    link_id,
    visitor_id,
    question,
    lane,
    status,
    route_reason,
    formal_status,
    formal_anonymize
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListLinkAskTurnsByVisitor :many
SELECT *
FROM link_ask_turns
WHERE link_id = $1 AND visitor_id = $2
ORDER BY created_at ASC;

-- name: MarkLinkAskTurnHostAnsweredByID :execrows
UPDATE link_ask_turns
SET status = 'host_answered',
    host_answer = $1,
    answered_by = $2,
    updated_at = now()
WHERE id = $3
  AND workspace_id = $4
  AND link_id = $5
  AND status IN ('host_pending', 'host_escalated');

-- name: GetLinkAskTurnByID :one
SELECT *
FROM link_ask_turns
WHERE id = $1
  AND workspace_id = $2
  AND link_id = $3
LIMIT 1;

-- name: GetLinkAskTurnByVisitor :one
SELECT *
FROM link_ask_turns
WHERE id = $1
  AND link_id = $2
  AND visitor_id = $3
  AND workspace_id = $4
LIMIT 1;

-- name: CountLinkAskAITurnsThisMonth :one
SELECT COUNT(*)::int AS count
FROM link_ask_turns
WHERE link_id = $1
  AND lane = 'ai'
  AND created_at >= date_trunc('month', now() AT TIME ZONE 'UTC');

-- name: GetLinkAskTurnSummary :one
SELECT
  COUNT(*)::bigint AS total_turns,
  COUNT(*) FILTER (WHERE lane = 'ai' AND status = 'ai_answered')::bigint AS ai_answered,
  COUNT(*) FILTER (WHERE lane = 'ai' AND status = 'ai_refused')::bigint AS ai_refused,
  COUNT(*) FILTER (WHERE lane IN ('host', 'hybrid') AND status IN ('host_pending', 'host_escalated'))::bigint AS host_pending,
  COUNT(*) FILTER (WHERE lane IN ('host', 'hybrid') AND status = 'host_answered')::bigint AS host_answered,
  COUNT(*) FILTER (WHERE route_reason = 'user_escalate')::bigint AS user_escalated,
  COUNT(*) FILTER (WHERE route_reason = 'low_confidence')::bigint AS auto_escalated
FROM link_ask_turns
WHERE link_id = $1;

-- name: EscalateLinkAskTurnToHost :execrows
UPDATE link_ask_turns
SET lane = 'hybrid',
    status = 'host_escalated',
    route_reason = $1,
    updated_at = now()
WHERE id = $2
  AND link_id = $3
  AND workspace_id = $4
  AND visitor_id = $5
  AND status IN ('ai_refused', 'ai_answered');

-- name: UpdateLinkAskTurnAIResult :execrows
UPDATE link_ask_turns
SET status = $1,
    ai_payload = $2,
    updated_at = now()
WHERE id = $3
  AND link_id = $4
  AND workspace_id = $5
  AND visitor_id = $6
  AND status IN ('routing', 'ai_streaming');

-- name: GetOwnerAskTurnByID :one
SELECT
    t.id,
    t.session_id,
    t.tenant_id,
    t.workspace_id,
    t.link_id,
    t.visitor_id,
    t.question,
    t.lane,
    t.status,
    t.ai_payload,
    t.host_answer,
    t.answered_by,
    t.route_reason,
    t.pinned_faq_at,
    t.pinned_faq_by,
    t.pinned_faq_sort,
    t.formal_status,
    t.formal_publish_at,
    t.formal_published_at,
    t.formal_anonymize,
    t.created_at,
    t.updated_at,
    COALESCE(s.visitor_email::text, '')::text AS visitor_email
FROM link_ask_turns t
LEFT JOIN link_ask_sessions s ON s.id = t.session_id
WHERE t.id = $1
  AND t.workspace_id = $2
  AND t.link_id = $3
LIMIT 1;

-- name: ListLinkAskTurnsByLink :many
SELECT
    t.id,
    t.session_id,
    t.tenant_id,
    t.workspace_id,
    t.link_id,
    t.visitor_id,
    t.question,
    t.lane,
    t.status,
    t.ai_payload,
    t.host_answer,
    t.answered_by,
    t.route_reason,
    t.pinned_faq_at,
    t.pinned_faq_by,
    t.pinned_faq_sort,
    t.formal_status,
    t.formal_publish_at,
    t.formal_published_at,
    t.formal_anonymize,
    t.created_at,
    t.updated_at,
    COALESCE(s.visitor_email::text, '')::text AS visitor_email
FROM link_ask_turns t
LEFT JOIN link_ask_sessions s ON s.id = t.session_id
WHERE t.link_id = $1
  AND t.workspace_id = $2
ORDER BY t.created_at DESC;

-- name: ListRoomAskTurns :many
SELECT
    t.id,
    t.session_id,
    t.tenant_id,
    t.workspace_id,
    t.link_id,
    t.visitor_id,
    t.question,
    t.lane,
    t.status,
    t.ai_payload,
    t.host_answer,
    t.answered_by,
    t.route_reason,
    t.pinned_faq_at,
    t.pinned_faq_by,
    t.pinned_faq_sort,
    t.formal_status,
    t.formal_publish_at,
    t.formal_published_at,
    t.formal_anonymize,
    t.created_at,
    t.updated_at,
    COALESCE(s.visitor_email::text, '')::text AS visitor_email
FROM link_ask_turns t
INNER JOIN links l ON l.id = t.link_id AND l.deal_room_id = $1
LEFT JOIN link_ask_sessions s ON s.id = t.session_id
WHERE t.workspace_id = $2
ORDER BY t.created_at DESC
LIMIT $3;

-- name: PinLinkAskTurnFAQ :execrows
UPDATE link_ask_turns
SET pinned_faq_at = now(),
    pinned_faq_by = $1,
    pinned_faq_sort = $5,
    updated_at = now()
WHERE id = $2
  AND workspace_id = $3
  AND link_id = $4
  AND pinned_faq_at IS NULL;

-- name: MaxLinkPinnedFAQSort :one
SELECT COALESCE(MAX(pinned_faq_sort), -1)::int AS max_sort
FROM link_ask_turns
WHERE link_id = $1
  AND workspace_id = $2
  AND pinned_faq_at IS NOT NULL;

-- name: SetLinkAskTurnFAQSort :exec
UPDATE link_ask_turns
SET pinned_faq_sort = $1,
    updated_at = now()
WHERE id = $2
  AND workspace_id = $3
  AND link_id = $4
  AND pinned_faq_at IS NOT NULL;

-- name: UnpinLinkAskTurnFAQ :execrows
UPDATE link_ask_turns
SET pinned_faq_at = NULL,
    pinned_faq_by = NULL,
    pinned_faq_sort = NULL,
    updated_at = now()
WHERE id = $1
  AND workspace_id = $2
  AND link_id = $3
  AND pinned_faq_at IS NOT NULL;

-- name: ListLinkPinnedAskFAQs :many
SELECT *
FROM link_ask_turns
WHERE link_id = $1
  AND workspace_id = $2
  AND pinned_faq_at IS NOT NULL
ORDER BY pinned_faq_sort ASC NULLS LAST, pinned_faq_at DESC
LIMIT $3;

-- name: ListRoomPublicAskFAQs :many
SELECT
    t.id,
    t.session_id,
    t.tenant_id,
    t.workspace_id,
    t.link_id,
    t.visitor_id,
    t.question,
    t.lane,
    t.status,
    t.ai_payload,
    t.host_answer,
    t.answered_by,
    t.route_reason,
    t.pinned_faq_at,
    t.pinned_faq_by,
    t.pinned_faq_sort,
    t.created_at,
    t.updated_at,
    l.name AS link_name
FROM link_ask_turns t
INNER JOIN links l ON l.id = t.link_id AND l.deal_room_id = $1
WHERE t.workspace_id = $2
  AND t.pinned_faq_at IS NOT NULL
ORDER BY t.pinned_faq_sort ASC NULLS LAST, t.pinned_faq_at DESC
LIMIT $3;

-- name: ListLinkPinnedAskTurnsByLink :many
SELECT
    t.id,
    t.session_id,
    t.tenant_id,
    t.workspace_id,
    t.link_id,
    t.visitor_id,
    t.question,
    t.lane,
    t.status,
    t.ai_payload,
    t.host_answer,
    t.answered_by,
    t.route_reason,
    t.pinned_faq_at,
    t.pinned_faq_by,
    t.pinned_faq_sort,
    t.formal_status,
    t.formal_publish_at,
    t.formal_published_at,
    t.formal_anonymize,
    t.created_at,
    t.updated_at,
    COALESCE(s.visitor_email::text, '')::text AS visitor_email
FROM link_ask_turns t
LEFT JOIN link_ask_sessions s ON s.id = t.session_id
WHERE t.link_id = $1
  AND t.workspace_id = $2
  AND t.pinned_faq_at IS NOT NULL
ORDER BY t.pinned_faq_sort ASC NULLS LAST, t.pinned_faq_at DESC
LIMIT $3;

-- name: ListRoomPinnedAskTurns :many
SELECT
    t.id,
    t.session_id,
    t.tenant_id,
    t.workspace_id,
    t.link_id,
    t.visitor_id,
    t.question,
    t.lane,
    t.status,
    t.ai_payload,
    t.host_answer,
    t.answered_by,
    t.route_reason,
    t.pinned_faq_at,
    t.pinned_faq_by,
    t.pinned_faq_sort,
    t.formal_status,
    t.formal_publish_at,
    t.formal_published_at,
    t.formal_anonymize,
    t.created_at,
    t.updated_at,
    COALESCE(s.visitor_email::text, '')::text AS visitor_email
FROM link_ask_turns t
INNER JOIN links l ON l.id = t.link_id AND l.deal_room_id = $1
LEFT JOIN link_ask_sessions s ON s.id = t.session_id
WHERE t.workspace_id = $2
  AND t.pinned_faq_at IS NOT NULL
ORDER BY t.pinned_faq_sort ASC NULLS LAST, t.pinned_faq_at DESC
LIMIT $3;

-- name: PublishDueFormalAskTurns :many
UPDATE link_ask_turns
SET formal_status = 'published',
    formal_published_at = now(),
    status = 'host_answered',
    updated_at = now()
WHERE link_id = $1
  AND workspace_id = $2
  AND formal_status = 'scheduled'
  AND formal_publish_at IS NOT NULL
  AND formal_publish_at <= now()
RETURNING id, link_id, host_answer, answered_by;

-- name: PublishDueFormalAskTurnsByRoom :many
UPDATE link_ask_turns t
SET formal_status = 'published',
    formal_published_at = now(),
    status = 'host_answered',
    updated_at = now()
FROM links l
WHERE l.id = t.link_id
  AND l.deal_room_id = $1
  AND t.workspace_id = $2
  AND t.formal_status = 'scheduled'
  AND t.formal_publish_at IS NOT NULL
  AND t.formal_publish_at <= now()
RETURNING t.id, t.link_id, t.host_answer, t.answered_by;

-- name: PublishDueFormalAskTurnsGlobal :many
-- Background worker batch: claim due scheduled formal turns without blocking readers.
UPDATE link_ask_turns
SET formal_status = 'published',
    formal_published_at = now(),
    status = 'host_answered',
    updated_at = now()
WHERE id IN (
    SELECT id
    FROM link_ask_turns
    WHERE formal_status = 'scheduled'
      AND formal_publish_at IS NOT NULL
      AND formal_publish_at <= now()
    ORDER BY formal_publish_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, workspace_id, link_id, host_answer, answered_by;

-- name: ScheduleFormalAskTurn :execrows
UPDATE link_ask_turns
SET host_answer = $1,
    answered_by = $2,
    formal_status = $3,
    formal_publish_at = $4,
    formal_published_at = $5,
    formal_anonymize = $6,
    status = $7,
    updated_at = now()
WHERE id = $8
  AND workspace_id = $9
  AND link_id = $10
  AND formal_status IN ('pending_review', 'scheduled');

-- name: ListLinkPublishedFormalAsk :many
SELECT
    t.id,
    t.session_id,
    t.tenant_id,
    t.workspace_id,
    t.link_id,
    t.visitor_id,
    t.question,
    t.lane,
    t.status,
    t.ai_payload,
    t.host_answer,
    t.answered_by,
    t.route_reason,
    t.pinned_faq_at,
    t.pinned_faq_by,
    t.pinned_faq_sort,
    t.formal_status,
    t.formal_publish_at,
    t.formal_published_at,
    t.formal_anonymize,
    t.created_at,
    t.updated_at,
    COALESCE(s.visitor_email::text, '')::text AS visitor_email
FROM link_ask_turns t
LEFT JOIN link_ask_sessions s ON s.id = t.session_id
WHERE t.link_id = $1
  AND t.workspace_id = $2
  AND t.formal_status = 'published'
ORDER BY t.formal_published_at DESC NULLS LAST, t.created_at DESC
LIMIT $3;

-- name: ListRoomPublishedFormalAsk :many
SELECT
    t.id,
    t.session_id,
    t.tenant_id,
    t.workspace_id,
    t.link_id,
    t.visitor_id,
    t.question,
    t.lane,
    t.status,
    t.ai_payload,
    t.host_answer,
    t.answered_by,
    t.route_reason,
    t.pinned_faq_at,
    t.pinned_faq_by,
    t.pinned_faq_sort,
    t.formal_status,
    t.formal_publish_at,
    t.formal_published_at,
    t.formal_anonymize,
    t.created_at,
    t.updated_at,
    l.name AS link_name,
    COALESCE(s.visitor_email::text, '')::text AS visitor_email
FROM link_ask_turns t
INNER JOIN links l ON l.id = t.link_id AND l.deal_room_id = $1
LEFT JOIN link_ask_sessions s ON s.id = t.session_id
WHERE t.workspace_id = $2
  AND t.formal_status = 'published'
ORDER BY t.formal_published_at DESC NULLS LAST, t.created_at DESC
LIMIT $3;


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

-- name: GetVisitorLastAccess :one
SELECT MAX(created_at)::timestamptz AS last_accessed_at
FROM access_logs
WHERE link_id = $1 AND visitor_id = $2 AND event_type = 'link_opened';

-- name: CountOtherLinkVisitors :one
-- Distinct visitors on a link excluding one visitor (call before recording
-- that visitor's open to detect forward/virality).
SELECT COUNT(DISTINCT visitor_id)::bigint
FROM access_logs
WHERE link_id = $1
  AND event_type = 'link_opened'
  AND visitor_id IS NOT NULL
  AND visitor_id <> ''
  AND visitor_id <> $2;

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
        (COUNT(DISTINCT al.visitor_id) FILTER (
            WHERE al.event_type = 'link_opened'
              AND al.visitor_id IS NOT NULL
              AND al.visitor_id <> ''
        ) > 1) AS was_forwarded,
        bool_or(al.event_type = 'download_attempted') AS had_downloads
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

-- name: ListDealRoomsByIDs :many
SELECT *
FROM deal_rooms
WHERE workspace_id = $1
  AND deleted_at IS NULL
  AND id = ANY(sqlc.arg(ids)::uuid[]);

-- ---------------------------------------------------------------------------
-- External docling-rag knowledge base mapping
-- ---------------------------------------------------------------------------

-- name: GetWorkspaceRagTenant :one
SELECT *
FROM workspace_rag_tenants
WHERE workspace_id = $1;

-- name: UpsertWorkspaceRagTenant :one
INSERT INTO workspace_rag_tenants (workspace_id, external_tenant_slug, tenant_api_key)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id) DO UPDATE
SET external_tenant_slug = EXCLUDED.external_tenant_slug,
    tenant_api_key = EXCLUDED.tenant_api_key,
    updated_at = now()
RETURNING *;

-- name: GetDealRoomRagCorpus :one
SELECT *
FROM deal_room_rag_corpora
WHERE room_id = $1;

-- name: UpsertDealRoomRagCorpus :one
INSERT INTO deal_room_rag_corpora (
    room_id, workspace_id, external_tenant_slug, external_kb_slug, status, error_message
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (room_id) DO UPDATE
SET external_tenant_slug = EXCLUDED.external_tenant_slug,
    external_kb_slug = EXCLUDED.external_kb_slug,
    status = EXCLUDED.status,
    error_message = EXCLUDED.error_message,
    updated_at = now()
RETURNING *;

-- name: UpdateDealRoomRagCorpusStatus :one
UPDATE deal_room_rag_corpora
SET status = $2,
    error_message = $3,
    last_synced_at = CASE WHEN $2 = 'ready' OR $2 = 'degraded' THEN now() ELSE last_synced_at END,
    updated_at = now()
WHERE room_id = $1
RETURNING *;

-- name: UpsertDealRoomRagDocument :one
INSERT INTO deal_room_rag_documents (
    room_id, document_id, workspace_id, external_name, status, last_error
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (room_id, document_id) DO UPDATE
SET external_name = EXCLUDED.external_name,
    status = EXCLUDED.status,
    last_error = EXCLUDED.last_error,
    updated_at = now()
RETURNING *;

-- name: UpdateDealRoomRagDocumentSync :one
UPDATE deal_room_rag_documents
SET status = $3,
    external_document_id = COALESCE(sqlc.narg(external_document_id), external_document_id),
    chunk_count = COALESCE(sqlc.narg(chunk_count), chunk_count),
    last_error = $4,
    updated_at = now()
WHERE room_id = $1 AND document_id = $2
RETURNING *;

-- name: ListDealRoomRagDocuments :many
SELECT *
FROM deal_room_rag_documents
WHERE room_id = $1
ORDER BY updated_at DESC;

-- name: GetDealRoomRagDocument :one
SELECT *
FROM deal_room_rag_documents
WHERE room_id = $1 AND document_id = $2;

-- name: MarkMissingRagDocumentsDeleted :exec
UPDATE deal_room_rag_documents
SET status = 'deleted',
    updated_at = now()
WHERE room_id = $1
  AND status <> 'deleted'
  AND NOT (document_id = ANY(sqlc.arg(active_document_ids)::uuid[]));

-- name: EnqueueKnowledgeSyncJob :one
INSERT INTO knowledge_sync_jobs (workspace_id, room_id, document_id, job_type)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPendingKnowledgeSyncJobs :many
SELECT *
FROM knowledge_sync_jobs
WHERE status = 'pending'
ORDER BY
  CASE job_type
    WHEN 'delete_doc' THEN 0
    WHEN 'sync_room' THEN 1
    ELSE 2
  END,
  created_at ASC
LIMIT $1;

-- name: CancelPendingKnowledgeIngestJobs :exec
UPDATE knowledge_sync_jobs
SET status = 'done',
    last_error = 'superseded by delete',
    updated_at = now()
WHERE room_id = $1
  AND document_id = $2
  AND job_type = 'ingest_doc'
  AND status = 'pending';

-- name: ClaimKnowledgeSyncJob :one
UPDATE knowledge_sync_jobs
SET status = 'running',
    attempts = attempts + 1,
    updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: FinishKnowledgeSyncJob :exec
UPDATE knowledge_sync_jobs
SET status = $2,
    last_error = $3,
    updated_at = now()
WHERE id = $1;

-- name: GetLatestKnowledgeSyncJobForRoom :one
SELECT *
FROM knowledge_sync_jobs
WHERE room_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateKnowledgeQASession :one
INSERT INTO knowledge_qa_sessions (
    workspace_id, room_id, created_by, title, status
) VALUES (
    $1, $2, $3, $4, 'active'
)
RETURNING *;

-- name: GetKnowledgeQASession :one
SELECT *
FROM knowledge_qa_sessions
WHERE id = $1 AND room_id = $2;

-- name: GetActiveKnowledgeQASessionForRoom :one
SELECT *
FROM knowledge_qa_sessions
WHERE room_id = $1 AND status = 'active'
ORDER BY COALESCE(last_turn_at, created_at) DESC
LIMIT 1;

-- name: ListKnowledgeQASessionSummariesForRoom :many
-- Keyset page: pass has_cursor=false for the first page.
SELECT
    s.id,
    s.workspace_id,
    s.room_id,
    s.created_by,
    s.title,
    s.status,
    s.created_at,
    s.updated_at,
    s.last_turn_at,
    (
        SELECT COUNT(*)::int
        FROM knowledge_qa_turns t
        WHERE t.session_id = s.id
    ) AS turn_count,
    COALESCE(
        (
            SELECT t.question
            FROM knowledge_qa_turns t
            WHERE t.session_id = s.id
            ORDER BY t.sequence ASC
            LIMIT 1
        ),
        ''
    )::text AS question_preview
FROM knowledge_qa_sessions s
WHERE s.room_id = sqlc.arg(room_id)
  AND (
    NOT sqlc.arg(has_cursor)::bool
    OR COALESCE(s.last_turn_at, s.created_at) < sqlc.arg(cursor_at)
    OR (
        COALESCE(s.last_turn_at, s.created_at) = sqlc.arg(cursor_at)
        AND s.id < sqlc.arg(cursor_id)
    )
  )
ORDER BY COALESCE(s.last_turn_at, s.created_at) DESC, s.id DESC
LIMIT sqlc.arg(page_limit);

-- name: CloseKnowledgeQASession :one
UPDATE knowledge_qa_sessions
SET status = 'closed',
    updated_at = now()
WHERE id = $1 AND room_id = $2 AND status = 'active'
RETURNING *;

-- name: CloseActiveKnowledgeQASessionsForRoom :exec
UPDATE knowledge_qa_sessions
SET status = 'closed',
    updated_at = now()
WHERE room_id = $1 AND status = 'active';

-- name: LockKnowledgeQASession :one
SELECT *
FROM knowledge_qa_sessions
WHERE id = $1
FOR UPDATE;

-- name: TouchKnowledgeQASessionAfterTurn :exec
UPDATE knowledge_qa_sessions
SET last_turn_at = sqlc.arg(last_turn_at),
    title = COALESCE(NULLIF(title, ''), sqlc.arg(title_fallback)),
    state = sqlc.arg(state)::jsonb,
    updated_at = now()
WHERE id = sqlc.arg(id);

-- name: NextKnowledgeQATurnSequence :one
SELECT COALESCE(MAX(sequence), 0)::int AS max_sequence
FROM knowledge_qa_turns
WHERE session_id = $1;

-- name: CreateKnowledgeQATurn :one
INSERT INTO knowledge_qa_turns (
    session_id,
    room_id,
    workspace_id,
    sequence,
    question,
    answer,
    refused,
    result_status,
    corpus_status_snapshot,
    hits,
    mode,
    top_k,
    error_summary,
    created_by,
    client_request_id,
    retrieve_query,
    rewrite_applied,
    rewrite_basis,
    bound_answer,
    corpus_fingerprint,
    duration_ms
) VALUES (
    sqlc.arg(session_id),
    sqlc.arg(room_id),
    sqlc.arg(workspace_id),
    sqlc.arg(sequence),
    sqlc.arg(question),
    sqlc.narg(answer),
    sqlc.arg(refused),
    sqlc.arg(result_status),
    sqlc.narg(corpus_status_snapshot)::jsonb,
    sqlc.arg(hits)::jsonb,
    sqlc.narg(mode),
    sqlc.narg(top_k),
    sqlc.narg(error_summary),
    sqlc.arg(created_by),
    sqlc.narg(client_request_id),
    sqlc.narg(retrieve_query),
    sqlc.arg(rewrite_applied),
    sqlc.narg(rewrite_basis),
    sqlc.narg(bound_answer)::jsonb,
    sqlc.narg(corpus_fingerprint),
    sqlc.arg(duration_ms)
)
RETURNING *;

-- name: GetKnowledgeQATurnByClientRequest :one
SELECT *
FROM knowledge_qa_turns
WHERE room_id = sqlc.arg(room_id)
  AND created_by = sqlc.arg(created_by)
  AND client_request_id = sqlc.arg(client_request_id);

-- name: ListKnowledgeQATurnsForSession :many
SELECT *
FROM knowledge_qa_turns
WHERE session_id = $1
ORDER BY sequence ASC;

-- name: CountKnowledgeQATurnsForSession :one
SELECT COUNT(*)::int AS count
FROM knowledge_qa_turns
WHERE session_id = $1;

-- name: GetKnowledgeQATurnForRoom :one
SELECT *
FROM knowledge_qa_turns
WHERE id = $1 AND room_id = $2;

-- name: UpsertKnowledgeQAFeedback :one
INSERT INTO knowledge_qa_feedback (
    turn_id, user_id, kind, note
) VALUES (
    sqlc.arg(turn_id),
    sqlc.arg(user_id),
    sqlc.arg(kind),
    sqlc.narg(note)
)
ON CONFLICT (turn_id, user_id) DO UPDATE SET
    kind = EXCLUDED.kind,
    note = EXCLUDED.note,
    updated_at = now()
RETURNING *;

-- name: ListKnowledgeQAFeedbackForSessionUser :many
SELECT f.id, f.turn_id, f.user_id, f.kind, f.note, f.created_at, f.updated_at
FROM knowledge_qa_feedback f
INNER JOIN knowledge_qa_turns t ON t.id = f.turn_id
WHERE t.session_id = sqlc.arg(session_id)
  AND f.user_id = sqlc.arg(user_id);

-- name: DeleteExpiredKnowledgeQASessions :execrows
-- Turns + feedback cascade via FK. Cutoff is exclusive (activity strictly older).
-- Prefer archive-then-delete (ListExpired… + DeleteKnowledgeQASession) when object store is configured.
DELETE FROM knowledge_qa_sessions
WHERE COALESCE(last_turn_at, updated_at) < sqlc.arg(cutoff);

-- name: ListExpiredKnowledgeQASessionsForArchive :many
SELECT *
FROM knowledge_qa_sessions
WHERE COALESCE(last_turn_at, updated_at) < sqlc.arg(cutoff)
ORDER BY COALESCE(last_turn_at, updated_at) ASC
LIMIT sqlc.arg(page_limit);

-- name: DeleteKnowledgeQASession :execrows
DELETE FROM knowledge_qa_sessions
WHERE id = $1;

-- name: CountKnowledgeQATurnsForWorkspaceSince :one
SELECT COUNT(*)::bigint AS count
FROM knowledge_qa_turns
WHERE workspace_id = sqlc.arg(workspace_id)
  AND created_at >= sqlc.arg(since);

-- name: CountKnowledgeQATurnsForWorkspaceByStatusSince :many
SELECT result_status, COUNT(*)::bigint AS count
FROM knowledge_qa_turns
WHERE workspace_id = sqlc.arg(workspace_id)
  AND created_at >= sqlc.arg(since)
GROUP BY result_status
ORDER BY result_status ASC;

-- name: AvgKnowledgeQATurnDurationMsForWorkspaceSince :one
SELECT
    COALESCE(AVG(duration_ms), 0)::float8 AS avg_ms,
    COUNT(*)::bigint AS n
FROM knowledge_qa_turns
WHERE workspace_id = sqlc.arg(workspace_id)
  AND created_at >= sqlc.arg(since);

-- name: P95KnowledgeQATurnDurationMsForWorkspaceSince :one
SELECT
    COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::float8 AS p95_ms,
    COUNT(*)::bigint AS n
FROM knowledge_qa_turns
WHERE workspace_id = sqlc.arg(workspace_id)
  AND created_at >= sqlc.arg(since);

-- name: SumKnowledgeQACostUnitsForWorkspaceSince :one
SELECT COALESCE(SUM(
    CASE
        WHEN bound_answer ? 'costUnits'
             AND jsonb_typeof(bound_answer->'costUnits') = 'number'
            THEN (bound_answer->>'costUnits')::bigint
        ELSE 0
    END
), 0)::bigint AS cost_units
FROM knowledge_qa_turns
WHERE workspace_id = sqlc.arg(workspace_id)
  AND created_at >= sqlc.arg(since);

-- name: CountKnowledgeQARefusalsByKindForWorkspaceSince :many
SELECT
    COALESCE(bound_answer->'refusal'->>'kind', '')::text AS kind,
    COUNT(*)::bigint AS count
FROM knowledge_qa_turns
WHERE workspace_id = sqlc.arg(workspace_id)
  AND created_at >= sqlc.arg(since)
  AND bound_answer ? 'refusal'
  AND COALESCE(bound_answer->'refusal'->>'kind', '') <> ''
GROUP BY 1
ORDER BY 1 ASC;

-- name: CountKnowledgeQAJudgmentsByKindForWorkspaceSince :many
SELECT
    COALESCE(bound_answer->'judgment'->>'kind', '')::text AS kind,
    COUNT(*)::bigint AS count
FROM knowledge_qa_turns
WHERE workspace_id = sqlc.arg(workspace_id)
  AND created_at >= sqlc.arg(since)
  AND bound_answer ? 'judgment'
  AND COALESCE(bound_answer->'judgment'->>'kind', '') <> ''
GROUP BY 1
ORDER BY 1 ASC;

-- name: CreateKnowledgeQASessionArchive :one
INSERT INTO knowledge_qa_session_archives (
    workspace_id,
    room_id,
    session_id,
    title,
    storage_key,
    turn_count,
    corpus_fingerprint,
    status,
    created_by
) VALUES (
    sqlc.arg(workspace_id),
    sqlc.arg(room_id),
    sqlc.arg(session_id),
    sqlc.narg(title),
    sqlc.arg(storage_key),
    sqlc.arg(turn_count),
    sqlc.narg(corpus_fingerprint),
    sqlc.arg(status),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: ListKnowledgeQASessionArchivesForRoom :many
SELECT *
FROM knowledge_qa_session_archives
WHERE room_id = $1
ORDER BY archived_at DESC
LIMIT sqlc.arg(page_limit);

-- name: GetKnowledgeQASessionArchive :one
SELECT *
FROM knowledge_qa_session_archives
WHERE id = $1 AND room_id = $2;

-- name: MarkKnowledgeQASessionArchiveRestored :one
UPDATE knowledge_qa_session_archives
SET status = 'restored_readonly'
WHERE id = $1 AND room_id = $2
RETURNING *;

-- name: CountKnowledgeQASessionArchivesForWorkspace :one
SELECT COUNT(*)::bigint AS count
FROM knowledge_qa_session_archives
WHERE workspace_id = $1;

-- name: CountKnowledgeQASessionArchivesForRoom :one
SELECT COUNT(*)::bigint AS count
FROM knowledge_qa_session_archives
WHERE room_id = $1;

-- name: GetKnowledgeQARoomMission :one
SELECT room_id, workspace_id, pack_id, updated_at, updated_by
FROM knowledge_qa_room_missions
WHERE room_id = $1;

-- name: UpsertKnowledgeQARoomMission :one
INSERT INTO knowledge_qa_room_missions (
    room_id, workspace_id, pack_id, updated_by, updated_at
) VALUES (
    sqlc.arg(room_id),
    sqlc.arg(workspace_id),
    sqlc.arg(pack_id),
    sqlc.narg(updated_by),
    now()
)
ON CONFLICT (room_id) DO UPDATE SET
    pack_id = EXCLUDED.pack_id,
    workspace_id = EXCLUDED.workspace_id,
    updated_by = EXCLUDED.updated_by,
    updated_at = now()
RETURNING *;

-- name: UpsertKnowledgeQAEvalCandidate :one
INSERT INTO knowledge_qa_eval_candidates (
    room_id, workspace_id, turn_id, feedback_kind, question, answer, note,
    snapshot, corpus_fingerprint, created_by, review_status, expect, reviewed_at, reviewed_by
) VALUES (
    sqlc.arg(room_id),
    sqlc.arg(workspace_id),
    sqlc.arg(turn_id),
    sqlc.arg(feedback_kind),
    sqlc.arg(question),
    sqlc.narg(answer),
    sqlc.narg(note),
    sqlc.narg(snapshot),
    sqlc.narg(corpus_fingerprint),
    sqlc.arg(created_by),
    'pending',
    NULL,
    NULL,
    NULL
)
ON CONFLICT (turn_id, feedback_kind) DO UPDATE SET
    question = EXCLUDED.question,
    answer = EXCLUDED.answer,
    note = EXCLUDED.note,
    snapshot = EXCLUDED.snapshot,
    corpus_fingerprint = EXCLUDED.corpus_fingerprint,
    created_by = EXCLUDED.created_by,
    review_status = 'pending',
    expect = NULL,
    reviewed_at = NULL,
    reviewed_by = NULL,
    created_at = now()
RETURNING id, room_id, workspace_id, turn_id, feedback_kind, question, answer, note,
    snapshot, corpus_fingerprint, review_status, expect, reviewed_at, reviewed_by, created_at, created_by;

-- name: ListKnowledgeQAEvalCandidatesForRoom :many
SELECT id, room_id, workspace_id, turn_id, feedback_kind, question, answer, note,
    snapshot, corpus_fingerprint, review_status, expect, reviewed_at, reviewed_by, created_at, created_by
FROM knowledge_qa_eval_candidates
WHERE room_id = sqlc.arg(room_id)
  AND (sqlc.narg(feedback_kind)::text IS NULL OR feedback_kind = sqlc.narg(feedback_kind))
  AND (sqlc.narg(review_status)::text IS NULL OR review_status = sqlc.narg(review_status))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_n);

-- name: GetKnowledgeQAEvalCandidateForRoom :one
SELECT id, room_id, workspace_id, turn_id, feedback_kind, question, answer, note,
    snapshot, corpus_fingerprint, review_status, expect, reviewed_at, reviewed_by, created_at, created_by
FROM knowledge_qa_eval_candidates
WHERE id = sqlc.arg(id) AND room_id = sqlc.arg(room_id);

-- name: ReviewKnowledgeQAEvalCandidate :one
UPDATE knowledge_qa_eval_candidates
SET review_status = sqlc.arg(review_status),
    expect = sqlc.narg(expect),
    reviewed_at = now(),
    reviewed_by = sqlc.arg(reviewed_by)
WHERE id = sqlc.arg(id) AND room_id = sqlc.arg(room_id)
RETURNING id, room_id, workspace_id, turn_id, feedback_kind, question, answer, note,
    snapshot, corpus_fingerprint, review_status, expect, reviewed_at, reviewed_by, created_at, created_by;

-- name: CountKnowledgeQAEvalCandidatesByStatusForWorkspace :many
SELECT review_status, COUNT(*)::bigint AS count
FROM knowledge_qa_eval_candidates
WHERE workspace_id = sqlc.arg(workspace_id)
GROUP BY review_status;

-- name: ListAcceptedKnowledgeQAEvalCandidatesForRoom :many
SELECT id, room_id, workspace_id, turn_id, feedback_kind, question, answer, note,
    snapshot, corpus_fingerprint, review_status, expect, reviewed_at, reviewed_by, created_at, created_by
FROM knowledge_qa_eval_candidates
WHERE room_id = sqlc.arg(room_id)
  AND review_status = 'accepted'
ORDER BY reviewed_at DESC NULLS LAST, created_at DESC
LIMIT sqlc.arg(limit_n);


-- name: GetDealRoomAccessPolicy :one
SELECT *
FROM deal_room_access_policies
WHERE deal_room_id = $1 AND workspace_id = $2
LIMIT 1;

-- name: UpsertDealRoomAccessPolicy :one
INSERT INTO deal_room_access_policies (
    deal_room_id, tenant_id, workspace_id,
    require_email, require_email_verification, require_password, password_hash,
    require_nda, nda_template_id, nda_document_id,
    watermark_enabled, download_enabled, screenshot_protection_enabled,
    file_requests_enabled, index_file_enabled, qa_enabled,
    allowed_emails, blocked_emails, configured, updated_by
) VALUES (
    $1, $2, $3,
    $4, $5, $6, $7,
    $8, $9, $10,
    $11, $12, $13,
    $14, $15, $16,
    $17, $18, $19, $20
)
ON CONFLICT (deal_room_id) DO UPDATE SET
    require_email = EXCLUDED.require_email,
    require_email_verification = EXCLUDED.require_email_verification,
    require_password = EXCLUDED.require_password,
    password_hash = EXCLUDED.password_hash,
    require_nda = EXCLUDED.require_nda,
    nda_template_id = EXCLUDED.nda_template_id,
    nda_document_id = EXCLUDED.nda_document_id,
    watermark_enabled = EXCLUDED.watermark_enabled,
    download_enabled = EXCLUDED.download_enabled,
    screenshot_protection_enabled = EXCLUDED.screenshot_protection_enabled,
    file_requests_enabled = EXCLUDED.file_requests_enabled,
    index_file_enabled = EXCLUDED.index_file_enabled,
    qa_enabled = EXCLUDED.qa_enabled,
    allowed_emails = EXCLUDED.allowed_emails,
    blocked_emails = EXCLUDED.blocked_emails,
    configured = EXCLUDED.configured,
    updated_by = EXCLUDED.updated_by,
    updated_at = now()
RETURNING *;

-- name: AppendDealRoomAccessPolicyAllowEmail :one
UPDATE deal_room_access_policies
SET allowed_emails = (
        SELECT ARRAY(
            SELECT DISTINCT lower(trim(e))
            FROM unnest(allowed_emails || ARRAY[sqlc.arg(email)::text]) AS e
            WHERE trim(e) <> ''
        )
    ),
    updated_at = now()
WHERE deal_room_id = sqlc.arg(deal_room_id)
  AND workspace_id = sqlc.arg(workspace_id)
  AND configured = true
RETURNING *;

-- name: ListActiveDealRoomLinkIDs :many
SELECT id
FROM links
WHERE deal_room_id = $1
  AND workspace_id = $2
  AND status NOT IN ('deleted', 'disabled')
ORDER BY created_at ASC;

-- name: DeleteDealRoomLinkBlocksForEmails :exec
DELETE FROM link_access_rules lar
USING links l
WHERE lar.link_id = l.id
  AND l.deal_room_id = sqlc.arg(deal_room_id)
  AND l.workspace_id = sqlc.arg(workspace_id)
  AND l.status NOT IN ('deleted', 'disabled')
  AND lar.action = 'block'
  AND lar.rule_type = 'email'
  AND lar.value = ANY(sqlc.arg(emails)::text[]);

-- name: ApplyDealRoomLinkAccessFromPolicy :exec
UPDATE links
SET require_email = sqlc.arg(require_email),
    require_email_verification = sqlc.arg(require_email_verification),
    require_password = sqlc.arg(require_password),
    password_hash = sqlc.narg(password_hash),
    require_nda = sqlc.arg(require_nda),
    nda_template_id = sqlc.narg(nda_template_id),
    nda_document_id = sqlc.narg(nda_document_id),
    watermark_enabled = sqlc.arg(watermark_enabled),
    download_enabled = sqlc.arg(download_enabled),
    screenshot_protection_enabled = sqlc.arg(screenshot_protection_enabled),
    file_requests_enabled = sqlc.arg(file_requests_enabled),
    index_file_enabled = sqlc.arg(index_file_enabled),
    qa_enabled = sqlc.arg(qa_enabled),
    security_version = security_version + 1,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND workspace_id = sqlc.arg(workspace_id)
  AND deal_room_id = sqlc.arg(deal_room_id)
  AND status NOT IN ('deleted', 'disabled');

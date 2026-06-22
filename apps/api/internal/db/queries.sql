-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING id, email, password_hash, created_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, created_at
FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT id, email, password_hash, created_at
FROM users
WHERE id = $1 LIMIT 1;

-- name: CreateTenant :one
INSERT INTO tenants (name)
VALUES ($1)
RETURNING id, name, created_at;

-- name: CreateWorkspace :one
INSERT INTO workspaces (tenant_id, name, slug, brand_color)
VALUES ($1, $2, $3, $4) RETURNING id, tenant_id, name, slug, brand_color, created_at;

-- name: GetWorkspaceByID :one
SELECT w.id, w.tenant_id, w.name, w.slug, w.brand_color, w.created_at
FROM workspaces w
WHERE w.id = $1 LIMIT 1;

-- name: GetWorkspaceBySlug :one
SELECT w.id, w.tenant_id, w.name, w.slug, w.brand_color, w.created_at
FROM workspaces w
WHERE w.slug = $1 LIMIT 1;

-- name: ListWorkspacesByUser :many
SELECT w.id, w.tenant_id, w.name, w.slug, w.brand_color, w.created_at, m.role
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
SELECT workspace_id, user_id, role, joined_at
FROM workspace_members
WHERE workspace_id = $1
ORDER BY joined_at DESC;

-- name: CreateDocument :one
INSERT INTO documents (
    id, tenant_id, workspace_id, created_by, title, source_type, status, storage_key
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, tenant_id, workspace_id, created_by, title, source_type, status, storage_key, page_count, created_at, updated_at, deleted_at;

-- name: GetDocumentByID :one
SELECT id, tenant_id, workspace_id, created_by, title, source_type, status, storage_key, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL
LIMIT 1;

-- name: ListDocumentsByWorkspace :many
SELECT id, tenant_id, workspace_id, created_by, title, source_type, status, storage_key, page_count, created_at, updated_at, deleted_at
FROM documents
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateDocumentStatus :exec
UPDATE documents
SET status = $1, page_count = $2, updated_at = now()
WHERE id = $3;

-- name: CreateIngestionJob :one
INSERT INTO ingestion_jobs (tenant_id, workspace_id, document_id, status)
VALUES ($1, $2, $3, $4)
RETURNING id, tenant_id, workspace_id, document_id, status, attempts, error_message, created_at, updated_at;

-- name: GetIngestionJobByDocument :one
SELECT id, tenant_id, workspace_id, document_id, status, attempts, error_message, created_at, updated_at
FROM ingestion_jobs
WHERE document_id = $1
LIMIT 1;

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

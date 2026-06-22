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
RETURNING id, workspace_id, user_id, title, created_at, updated_at;

-- name: GetAssistantSession :one
SELECT id, workspace_id, user_id, title, created_at, updated_at
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

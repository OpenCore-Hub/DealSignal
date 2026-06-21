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
VALUES ($1, $2, $3, $4)
RETURNING id, tenant_id, name, slug, brand_color, created_at;

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

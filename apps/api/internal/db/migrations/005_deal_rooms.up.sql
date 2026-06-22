CREATE TABLE IF NOT EXISTS deal_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    template_type TEXT,
    settings JSONB NOT NULL DEFAULT '{}',
    requires_nda BOOLEAN NOT NULL DEFAULT false,
    requires_approval BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived','deleted')),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (workspace_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_deal_rooms_workspace ON deal_rooms(workspace_id);
CREATE INDEX IF NOT EXISTS idx_deal_rooms_status ON deal_rooms(status);
CREATE INDEX IF NOT EXISTS idx_deal_rooms_slug ON deal_rooms(slug);

CREATE TABLE IF NOT EXISTS room_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    role TEXT NOT NULL DEFAULT 'viewer' CHECK (role IN ('owner','admin','contributor','viewer')),
    nda_status TEXT NOT NULL DEFAULT 'not_required' CHECK (nda_status IN ('not_required','pending','signed')),
    nda_signed_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','revoked')),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (room_id, email)
);

CREATE INDEX IF NOT EXISTS idx_room_members_room ON room_members(room_id);
CREATE INDEX IF NOT EXISTS idx_room_members_email ON room_members(email);
CREATE INDEX IF NOT EXISTS idx_room_members_user ON room_members(user_id);

CREATE TABLE IF NOT EXISTS room_access_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    reason TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','cancelled','revoked')),
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_room_access_requests_room ON room_access_requests(room_id);
CREATE INDEX IF NOT EXISTS idx_room_access_requests_email ON room_access_requests(email);
CREATE INDEX IF NOT EXISTS idx_room_access_requests_status ON room_access_requests(status);

CREATE TABLE IF NOT EXISTS deal_room_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    folder_path TEXT NOT NULL DEFAULT '/',
    sort_order INT NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (room_id, document_id)
);

CREATE INDEX IF NOT EXISTS idx_deal_room_documents_room ON deal_room_documents(room_id);
CREATE INDEX IF NOT EXISTS idx_deal_room_documents_folder ON deal_room_documents(room_id, folder_path);

CREATE TABLE IF NOT EXISTS room_member_folder_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    folder_path TEXT NOT NULL,
    permission TEXT NOT NULL CHECK (permission IN ('view','download','none')),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (room_id, email, folder_path)
);

CREATE INDEX IF NOT EXISTS idx_room_folder_permissions_room_email ON room_member_folder_permissions(room_id, email);

CREATE TABLE IF NOT EXISTS room_nda_agreements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    ip INET,
    user_agent TEXT,
    agreed_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (room_id, email)
);

CREATE INDEX IF NOT EXISTS idx_room_nda_agreements_room ON room_nda_agreements(room_id);

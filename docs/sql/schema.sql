-- DealSignal initial PostgreSQL schema
-- Target: PostgreSQL 15+

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE workspace_mode AS ENUM ('founder', 'investment_firm', 'sales', 'mixed');
CREATE TYPE membership_role AS ENUM ('owner', 'admin', 'member', 'viewer');
CREATE TYPE contact_segment AS ENUM ('investor', 'lp', 'buyer', 'customer', 'partner', 'other');
CREATE TYPE document_status AS ENUM ('draft', 'processing', 'ready', 'archived', 'failed');
CREATE TYPE document_processing_status AS ENUM ('uploaded', 'processing', 'ready', 'failed');
CREATE TYPE library_item_status AS ENUM ('draft', 'in_review', 'approved', 'archived');
CREATE TYPE link_access_mode AS ENUM ('public', 'email_verification', 'allowlist', 'password', 'approval_required', 'nda_required');
CREATE TYPE download_policy AS ENUM ('allowed', 'disabled', 'watermarked');
CREATE TYPE friction_level AS ENUM ('low', 'medium', 'high');
CREATE TYPE smart_link_status AS ENUM ('active', 'expired', 'revoked', 'archived');
CREATE TYPE recipient_status AS ENUM ('invited', 'verified', 'approved', 'blocked', 'revoked');
CREATE TYPE access_scope_type AS ENUM ('smart_link', 'deal_room');
CREATE TYPE access_grant_status AS ENUM ('pending', 'approved', 'denied', 'revoked');
CREATE TYPE room_type AS ENUM ('seed_fundraising', 'series_a_fundraising', 'lp_update', 'ma_diligence', 'enterprise_sales', 'partner_enablement', 'custom');
CREATE TYPE room_status AS ENUM ('draft', 'active', 'archived');
CREATE TYPE room_member_role AS ENUM ('viewer', 'questioner', 'collaborator');
CREATE TYPE principal_type AS ENUM ('contact', 'account', 'domain', 'role');
CREATE TYPE question_status AS ENUM ('open', 'answered', 'closed');
CREATE TYPE download_status AS ENUM ('allowed', 'blocked', 'failed');
CREATE TYPE score_type AS ENUM ('investor_intent', 'lp_engagement', 'buyer_engagement', 'deal_intent', 'room_engagement');
CREATE TYPE score_label AS ENUM ('cold', 'warm', 'hot');
CREATE TYPE recommendation_status AS ENUM ('open', 'dismissed', 'completed');
CREATE TYPE notification_channel AS ENUM ('email', 'slack', 'crm', 'in_app');
CREATE TYPE notification_status AS ENUM ('queued', 'sent', 'failed', 'cancelled');
CREATE TYPE integration_provider AS ENUM ('slack', 'hubspot', 'salesforce', 'gmail', 'outlook', 'google_drive', 'dropbox');
CREATE TYPE integration_status AS ENUM ('connected', 'disconnected', 'error');
CREATE TYPE crm_object_type AS ENUM ('contact', 'account', 'smart_link', 'deal_room', 'document');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email CITEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug CITEXT NOT NULL UNIQUE,
    mode workspace_mode NOT NULL DEFAULT 'mixed',
    default_security_preset JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workspace_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role membership_role NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, user_id)
);

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    domain CITEXT,
    segment contact_segment NOT NULL DEFAULT 'other',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email CITEXT NOT NULL,
    name TEXT,
    title TEXT,
    segment contact_segment NOT NULL DEFAULT 'other',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, email)
);

CREATE TABLE account_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    relationship_label TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, contact_id)
);

CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    owner_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    description TEXT,
    status document_status NOT NULL DEFAULT 'draft',
    current_version_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE document_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    original_filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    file_size_bytes BIGINT NOT NULL CHECK (file_size_bytes >= 0),
    checksum_sha256 TEXT,
    storage_bucket TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    processing_status document_processing_status NOT NULL DEFAULT 'uploaded',
    page_count INTEGER CHECK (page_count IS NULL OR page_count >= 0),
    processing_error TEXT,
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (document_id, version_number),
    UNIQUE (storage_bucket, storage_key)
);

ALTER TABLE documents
    ADD CONSTRAINT documents_current_version_id_fkey
    FOREIGN KEY (current_version_id) REFERENCES document_versions(id) ON DELETE SET NULL;

CREATE TABLE document_pages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    document_version_id UUID NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
    page_number INTEGER NOT NULL CHECK (page_number > 0),
    thumbnail_storage_key TEXT,
    text_excerpt TEXT,
    width_px INTEGER CHECK (width_px IS NULL OR width_px > 0),
    height_px INTEGER CHECK (height_px IS NULL OR height_px > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (document_version_id, page_number)
);

CREATE TABLE library_collections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE library_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    collection_id UUID REFERENCES library_collections(id) ON DELETE SET NULL,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    status library_item_status NOT NULL DEFAULT 'draft',
    approved_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, document_id)
);

CREATE TABLE smart_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    document_version_id UUID REFERENCES document_versions(id) ON DELETE SET NULL,
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    access_mode link_access_mode NOT NULL DEFAULT 'email_verification',
    download_policy download_policy NOT NULL DEFAULT 'allowed',
    watermark_enabled BOOLEAN NOT NULL DEFAULT false,
    watermark_template TEXT,
    password_hash TEXT,
    allowed_email_domains CITEXT[] NOT NULL DEFAULT '{}',
    recipient_friction_level friction_level NOT NULL DEFAULT 'medium',
    status smart_link_status NOT NULL DEFAULT 'active',
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoked_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((access_mode <> 'password') OR password_hash IS NOT NULL)
);

CREATE TABLE smart_link_recipients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    smart_link_id UUID NOT NULL REFERENCES smart_links(id) ON DELETE CASCADE,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    email CITEXT NOT NULL,
    status recipient_status NOT NULL DEFAULT 'invited',
    verified_at TIMESTAMPTZ,
    first_opened_at TIMESTAMPTZ,
    last_opened_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (smart_link_id, email)
);

CREATE TABLE access_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    scope_type access_scope_type NOT NULL,
    scope_id UUID NOT NULL,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    email CITEXT NOT NULL,
    status access_grant_status NOT NULL DEFAULT 'pending',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    resolved_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT,
    UNIQUE (scope_type, scope_id, email)
);

CREATE TABLE deal_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    description TEXT,
    room_type room_type NOT NULL DEFAULT 'custom',
    status room_status NOT NULL DEFAULT 'draft',
    default_access_mode link_access_mode NOT NULL DEFAULT 'email_verification',
    download_policy download_policy NOT NULL DEFAULT 'watermarked',
    watermark_enabled BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE deal_room_folders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    deal_room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    parent_folder_id UUID REFERENCES deal_room_folders(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (deal_room_id, parent_folder_id, name)
);

CREATE TABLE deal_room_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    deal_room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    folder_id UUID NOT NULL REFERENCES deal_room_folders(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    document_version_id UUID REFERENCES document_versions(id) ON DELETE SET NULL,
    display_name TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (folder_id, document_id)
);

CREATE TABLE deal_room_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    deal_room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    email CITEXT NOT NULL,
    role room_member_role NOT NULL DEFAULT 'viewer',
    status recipient_status NOT NULL DEFAULT 'invited',
    invited_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    invited_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    verified_at TIMESTAMPTZ,
    last_opened_at TIMESTAMPTZ,
    UNIQUE (deal_room_id, email)
);

CREATE TABLE deal_room_access_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    deal_room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    folder_id UUID REFERENCES deal_room_folders(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    principal_type principal_type NOT NULL,
    principal_value TEXT NOT NULL,
    can_view BOOLEAN NOT NULL DEFAULT true,
    can_download BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (folder_id IS NOT NULL OR document_id IS NOT NULL)
);

CREATE TABLE deal_room_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    deal_room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    asked_by_contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    assigned_to_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    related_document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    question_text TEXT NOT NULL,
    answer_text TEXT,
    status question_status NOT NULL DEFAULT 'open',
    answered_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    answered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE view_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    smart_link_id UUID REFERENCES smart_links(id) ON DELETE SET NULL,
    deal_room_id UUID REFERENCES deal_rooms(id) ON DELETE SET NULL,
    deal_room_file_id UUID REFERENCES deal_room_files(id) ON DELETE SET NULL,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    recipient_email CITEXT,
    ip_address INET,
    user_agent TEXT,
    referrer TEXT,
    country_code TEXT,
    region TEXT,
    city TEXT,
    device_type TEXT,
    browser_name TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ,
    CHECK (smart_link_id IS NOT NULL OR deal_room_id IS NOT NULL)
);

CREATE TABLE activity_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    view_session_id UUID REFERENCES view_sessions(id) ON DELETE SET NULL,
    smart_link_id UUID REFERENCES smart_links(id) ON DELETE SET NULL,
    deal_room_id UUID REFERENCES deal_rooms(id) ON DELETE SET NULL,
    document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    actor_email CITEXT,
    event_type TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE page_view_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    view_session_id UUID NOT NULL REFERENCES view_sessions(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    document_version_id UUID NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
    page_number INTEGER NOT NULL CHECK (page_number > 0),
    visible_started_at TIMESTAMPTZ NOT NULL,
    visible_ended_at TIMESTAMPTZ,
    duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE download_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    view_session_id UUID REFERENCES view_sessions(id) ON DELETE SET NULL,
    smart_link_id UUID REFERENCES smart_links(id) ON DELETE SET NULL,
    deal_room_id UUID REFERENCES deal_rooms(id) ON DELETE SET NULL,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    actor_email CITEXT,
    download_status download_status NOT NULL,
    watermarked BOOLEAN NOT NULL DEFAULT false,
    blocked_reason TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE intent_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    score_type score_type NOT NULL,
    score INTEGER NOT NULL CHECK (score >= 0 AND score <= 100),
    label score_label NOT NULL,
    explanation TEXT NOT NULL,
    factors JSONB NOT NULL DEFAULT '{}'::jsonb,
    contact_id UUID REFERENCES contacts(id) ON DELETE CASCADE,
    account_id UUID REFERENCES accounts(id) ON DELETE CASCADE,
    smart_link_id UUID REFERENCES smart_links(id) ON DELETE CASCADE,
    deal_room_id UUID REFERENCES deal_rooms(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        contact_id IS NOT NULL
        OR account_id IS NOT NULL
        OR smart_link_id IS NOT NULL
        OR deal_room_id IS NOT NULL
        OR document_id IS NOT NULL
    )
);

CREATE TABLE recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_for_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    contact_id UUID REFERENCES contacts(id) ON DELETE CASCADE,
    account_id UUID REFERENCES accounts(id) ON DELETE CASCADE,
    smart_link_id UUID REFERENCES smart_links(id) ON DELETE CASCADE,
    deal_room_id UUID REFERENCES deal_rooms(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    recommended_action TEXT NOT NULL,
    status recommendation_status NOT NULL DEFAULT 'open',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel notification_channel NOT NULL,
    event_type TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, user_id, channel, event_type)
);

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    channel notification_channel NOT NULL,
    status notification_status NOT NULL DEFAULT 'queued',
    subject TEXT,
    body TEXT NOT NULL,
    destination TEXT,
    related_event_id UUID REFERENCES activity_events(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    queued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    failure_reason TEXT
);

CREATE TABLE integrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider integration_provider NOT NULL,
    status integration_status NOT NULL DEFAULT 'connected',
    external_account_id TEXT,
    encrypted_credentials BYTEA,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    connected_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider, external_account_id)
);

CREATE TABLE crm_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    integration_id UUID NOT NULL REFERENCES integrations(id) ON DELETE CASCADE,
    object_type crm_object_type NOT NULL,
    local_object_id UUID NOT NULL,
    external_object_id TEXT NOT NULL,
    external_object_url TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (integration_id, object_type, local_object_id),
    UNIQUE (integration_id, object_type, external_object_id)
);

CREATE INDEX idx_workspace_memberships_workspace_id ON workspace_memberships(workspace_id);
CREATE INDEX idx_workspace_memberships_user_id ON workspace_memberships(user_id);

CREATE INDEX idx_accounts_workspace_id ON accounts(workspace_id);
CREATE INDEX idx_accounts_workspace_domain ON accounts(workspace_id, domain);

CREATE INDEX idx_contacts_workspace_id ON contacts(workspace_id);
CREATE INDEX idx_contacts_workspace_segment ON contacts(workspace_id, segment);

CREATE INDEX idx_documents_workspace_status ON documents(workspace_id, status);
CREATE INDEX idx_documents_workspace_created_at ON documents(workspace_id, created_at DESC);
CREATE INDEX idx_document_versions_document_id ON document_versions(document_id);
CREATE INDEX idx_document_pages_version_page ON document_pages(document_version_id, page_number);

CREATE INDEX idx_library_items_workspace_status ON library_items(workspace_id, status);
CREATE INDEX idx_library_items_collection_id ON library_items(collection_id);

CREATE INDEX idx_smart_links_workspace_status ON smart_links(workspace_id, status, created_at DESC);
CREATE INDEX idx_smart_links_slug ON smart_links(slug);
CREATE INDEX idx_smart_link_recipients_link_email ON smart_link_recipients(smart_link_id, email);
CREATE INDEX idx_smart_link_recipients_contact_id ON smart_link_recipients(contact_id);

CREATE INDEX idx_access_grants_scope ON access_grants(scope_type, scope_id, status);
CREATE INDEX idx_access_grants_workspace_email ON access_grants(workspace_id, email);

CREATE INDEX idx_deal_rooms_workspace_status ON deal_rooms(workspace_id, status, updated_at DESC);
CREATE INDEX idx_deal_room_folders_room_id ON deal_room_folders(deal_room_id);
CREATE INDEX idx_deal_room_files_room_id ON deal_room_files(deal_room_id);
CREATE INDEX idx_deal_room_members_room_email ON deal_room_members(deal_room_id, email);
CREATE INDEX idx_deal_room_questions_room_status ON deal_room_questions(deal_room_id, status);

CREATE INDEX idx_view_sessions_workspace_started_at ON view_sessions(workspace_id, started_at DESC);
CREATE INDEX idx_view_sessions_smart_link_started_at ON view_sessions(smart_link_id, started_at DESC);
CREATE INDEX idx_view_sessions_deal_room_started_at ON view_sessions(deal_room_id, started_at DESC);
CREATE INDEX idx_view_sessions_contact_started_at ON view_sessions(contact_id, started_at DESC);

CREATE INDEX idx_activity_events_workspace_occurred_at ON activity_events(workspace_id, occurred_at DESC);
CREATE INDEX idx_activity_events_type_occurred_at ON activity_events(workspace_id, event_type, occurred_at DESC);
CREATE INDEX idx_activity_events_smart_link_id ON activity_events(smart_link_id);
CREATE INDEX idx_activity_events_deal_room_id ON activity_events(deal_room_id);
CREATE INDEX idx_activity_events_contact_id ON activity_events(contact_id);

CREATE INDEX idx_page_view_events_session_id ON page_view_events(view_session_id);
CREATE INDEX idx_page_view_events_document_page ON page_view_events(document_version_id, page_number);

CREATE INDEX idx_download_events_workspace_occurred_at ON download_events(workspace_id, occurred_at DESC);
CREATE INDEX idx_download_events_smart_link_id ON download_events(smart_link_id);
CREATE INDEX idx_download_events_deal_room_id ON download_events(deal_room_id);

CREATE INDEX idx_intent_scores_workspace_type_calculated_at ON intent_scores(workspace_id, score_type, calculated_at DESC);
CREATE INDEX idx_intent_scores_contact_type_calculated_at ON intent_scores(contact_id, score_type, calculated_at DESC);
CREATE INDEX idx_intent_scores_smart_link_type_calculated_at ON intent_scores(smart_link_id, score_type, calculated_at DESC);
CREATE INDEX idx_intent_scores_deal_room_type_calculated_at ON intent_scores(deal_room_id, score_type, calculated_at DESC);

CREATE INDEX idx_recommendations_workspace_status ON recommendations(workspace_id, status, created_at DESC);
CREATE INDEX idx_notifications_workspace_status ON notifications(workspace_id, status, queued_at DESC);
CREATE INDEX idx_integrations_workspace_provider ON integrations(workspace_id, provider);
CREATE INDEX idx_crm_mappings_local_object ON crm_mappings(object_type, local_object_id);

COMMIT;

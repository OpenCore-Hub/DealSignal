-- Migration: tables for Visitor Q&A (SHORT-008) and File Requests (SHORT-009).
--
-- link_visitor_questions  - visitors ask questions; owner answers them.
-- link_file_requests      - visitors request missing files; owner fulfills.

CREATE TABLE link_visitor_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    visitor_id TEXT NOT NULL,
    visitor_email TEXT,
    question TEXT NOT NULL,
    answer TEXT,
    answered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'answered')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_link_visitor_questions_link_id ON link_visitor_questions(link_id);
CREATE INDEX idx_link_visitor_questions_visitor ON link_visitor_questions(visitor_id);

CREATE TABLE link_file_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    visitor_id TEXT,
    visitor_email TEXT,
    message TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'fulfilled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_link_file_requests_link_id ON link_file_requests(link_id);
CREATE INDEX idx_link_file_requests_status ON link_file_requests(status);

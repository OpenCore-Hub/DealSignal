ALTER TABLE tenants ADD COLUMN IF NOT EXISTS slug CITEXT;
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenants_slug ON tenants(slug) WHERE slug IS NOT NULL;

CREATE TABLE IF NOT EXISTS tenant_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    domain_type TEXT NOT NULL DEFAULT 'SUBDOMAIN',
    is_primary BOOLEAN NOT NULL DEFAULT false,
    ssl_status TEXT NOT NULL DEFAULT 'pending',
    ssl_expires_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_tenant_domains_type CHECK (domain_type IN ('SUBDOMAIN', 'CUSTOM', 'PUBLIC_LINK')),
    CONSTRAINT chk_tenant_domains_ssl CHECK (ssl_status IN ('pending', 'issued', 'expired', 'error'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_domains_domain ON tenant_domains(domain);
CREATE INDEX IF NOT EXISTS idx_tenant_domains_tenant_id ON tenant_domains(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_domains_type ON tenant_domains(domain_type);

UPDATE links SET nda_template_id = NULL WHERE nda_template_id IS NOT NULL;

DROP INDEX IF EXISTS idx_link_nda_agreements_link_visitor_template;
DROP INDEX IF EXISTS idx_link_nda_agreements_template;
DROP INDEX IF EXISTS idx_link_nda_agreements_certificate;
DROP INDEX IF EXISTS idx_links_nda_template_id;

ALTER TABLE link_nda_agreements
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS signed_file_key,
    DROP COLUMN IF EXISTS certificate_id,
    DROP COLUMN IF EXISTS signer_name,
    DROP COLUMN IF EXISTS content_sha256,
    DROP COLUMN IF EXISTS nda_template_id;

ALTER TABLE links DROP COLUMN IF EXISTS nda_template_id;

DROP INDEX IF EXISTS idx_nda_templates_workspace;
DROP TABLE IF EXISTS nda_templates;

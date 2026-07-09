-- Revert link sharing fields migration.

ALTER TABLE links
    DROP COLUMN IF EXISTS custom_domain,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS notify_on_access;

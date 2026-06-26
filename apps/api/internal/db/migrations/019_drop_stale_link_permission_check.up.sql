-- Drop the stale permission_type check constraint left behind by migration 004.
-- Migration 010 added chk_links_permission_type with 'nda', but the original
-- links_permission_type_check (without 'nda') was never removed, so any link
-- with permission_type = 'nda' would fail to insert.
ALTER TABLE links
    DROP CONSTRAINT IF EXISTS links_permission_type_check;

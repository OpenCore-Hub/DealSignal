-- Migration: remove screenshot protection flag.
ALTER TABLE links
    DROP COLUMN IF EXISTS screenshot_protection_enabled;

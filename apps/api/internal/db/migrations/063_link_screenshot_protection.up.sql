-- Migration: real backend column for screenshot protection switch.
ALTER TABLE links
    ADD COLUMN IF NOT EXISTS screenshot_protection_enabled BOOLEAN NOT NULL DEFAULT false;

-- Persist marker-only forward_signal counts on link_features so suggestion
-- heat can use the cached snapshot without a live overlay query.
ALTER TABLE link_features
    ADD COLUMN IF NOT EXISTS forward_signals INT NOT NULL DEFAULT 0;

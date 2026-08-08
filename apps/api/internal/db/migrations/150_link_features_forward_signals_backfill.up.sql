-- Backfill cached forward_signals so FeatureStore snapshots are truthful
-- immediately after 149 (DEFAULT 0) without waiting for the feature worker.
UPDATE link_features lf
SET forward_signals = COALESCE((
    SELECT COUNT(*)::int
    FROM access_logs al
    WHERE al.link_id = lf.link_id
      AND al.event_type = 'forward_signal'
), 0);

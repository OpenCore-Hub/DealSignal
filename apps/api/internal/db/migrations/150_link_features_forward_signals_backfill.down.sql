-- Lossy reverse: clear cached forward_signals (worker / 149 path will recompute).
UPDATE link_features
SET forward_signals = 0;

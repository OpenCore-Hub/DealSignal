-- Clean up duplicate signals/action_items and enforce uniqueness to prevent future duplicates.
-- A duplicate signal is defined as the same suggestion_id within the same workspace.
-- A duplicate action_item is defined as multiple actions for the same signal_id.

-- 1. Remove duplicate action_items, keeping the newest row for each signal_id.
DELETE FROM action_items
WHERE id IN (
    SELECT id
    FROM (
        SELECT id,
               row_number() OVER (
                   PARTITION BY signal_id ORDER BY created_at DESC, id DESC
               ) AS rn
        FROM action_items
    ) ranked
    WHERE ranked.rn > 1
);

-- 2. Remove duplicate signals, keeping the newest row for each (workspace_id, suggestion_id).
DELETE FROM signals
WHERE id IN (
    SELECT id
    FROM (
        SELECT id,
               row_number() OVER (
                   PARTITION BY workspace_id, suggestion_id ORDER BY created_at DESC, id DESC
               ) AS rn
        FROM signals
        WHERE suggestion_id IS NOT NULL
    ) ranked
    WHERE ranked.rn > 1
);

-- 3. Enforce one signal per suggestion within a workspace.
CREATE UNIQUE INDEX IF NOT EXISTS idx_signals_unique_suggestion
    ON signals (workspace_id, suggestion_id)
    WHERE suggestion_id IS NOT NULL;

-- 4. Enforce one action per signal.
CREATE UNIQUE INDEX IF NOT EXISTS idx_action_items_unique_signal
    ON action_items (signal_id);

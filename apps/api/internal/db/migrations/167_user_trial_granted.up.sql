-- Once-only trial grant: a user who has ever owned a workspace cannot mint
-- another 14-day trial after leaving. Backfill existing owners so remint
-- after leave is closed for current tenants.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS trial_granted_at TIMESTAMPTZ;

UPDATE users u
SET trial_granted_at = now()
WHERE u.trial_granted_at IS NULL
  AND EXISTS (
    SELECT 1
    FROM workspace_members wm
    WHERE wm.user_id = u.id AND wm.role = 'owner'
  );

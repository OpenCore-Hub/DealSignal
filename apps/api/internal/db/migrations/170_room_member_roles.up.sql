-- Unify room roles with the workspace plane: member/guest replace contributor/viewer.
ALTER TABLE room_members DROP CONSTRAINT IF EXISTS room_members_role_check;

UPDATE room_members SET role = 'member' WHERE role = 'contributor';
UPDATE room_members SET role = 'guest' WHERE role = 'viewer';

ALTER TABLE room_members
    ADD CONSTRAINT room_members_role_check
    CHECK (role IN ('owner', 'admin', 'member', 'guest'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_room_members_room_user
    ON room_members (room_id, user_id)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_room_members_unbound_email
    ON room_members (workspace_id, lower(email))
    WHERE user_id IS NULL;

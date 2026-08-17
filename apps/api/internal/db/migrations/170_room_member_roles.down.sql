DROP INDEX IF EXISTS idx_room_members_unbound_email;
DROP INDEX IF EXISTS idx_room_members_room_user;

ALTER TABLE room_members DROP CONSTRAINT IF EXISTS room_members_role_check;

UPDATE room_members SET role = 'contributor' WHERE role = 'member';
UPDATE room_members SET role = 'viewer' WHERE role = 'guest';

ALTER TABLE room_members
    ADD CONSTRAINT room_members_role_check
    CHECK (role IN ('owner', 'admin', 'contributor', 'viewer'));

-- Diligence-gate false positives: room operators (owner/admin) and empty-email
-- members were marked nda_status=pending on NDA-enabled room create/invite.
-- Radar sync then emitted "Unlock investor diligence access" with no visitor.
--
-- 1) Operators / empty email never owe a room NDA.
-- 2) Close pending room_nda actions; SyncWorkspace recreates member-keyed
--    items (source_id=member, target_id=room) only for real external parties.

UPDATE room_members
SET nda_status = 'not_required',
    updated_at = now()
WHERE nda_status = 'pending'
  AND (
      role IN ('owner', 'admin')
      OR email IS NULL
      OR BTRIM(email) = ''
  );

UPDATE action_items
SET status = 'done',
    updated_at = now()
WHERE status = 'pending'
  AND source_type = 'room_nda';

-- Add a unique constraint so contacts can be upserted by (workspace_id, email).
DELETE FROM contacts a
USING contacts b
WHERE a.id < b.id
  AND a.workspace_id = b.workspace_id
  AND a.email IS NOT NULL
  AND b.email IS NOT NULL
  AND a.email = b.email;

ALTER TABLE contacts
ADD CONSTRAINT contacts_workspace_email_unique UNIQUE (workspace_id, email);

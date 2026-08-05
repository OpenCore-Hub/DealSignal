-- Backfill allow rules for document share links that already have link_contacts
-- but no matching email allow rule. Aligns CheckPublicEmail / NDA Continue with
-- deal-room allowlist semantics without touching deal-room links.
INSERT INTO link_access_rules (
    tenant_id,
    workspace_id,
    link_id,
    rule_type,
    value,
    action,
    sort_order
)
SELECT
    l.tenant_id,
    l.workspace_id,
    l.id,
    'email',
    lower(trim(c.email)),
    'allow',
    0
FROM links l
JOIN link_contacts lc ON lc.link_id = l.id
JOIN contacts c ON c.id = lc.contact_id
WHERE l.deal_room_id IS NULL
  AND l.require_email_verification = true
  AND l.status <> 'deleted'
  AND c.email IS NOT NULL
  AND trim(c.email) <> ''
ON CONFLICT (link_id, rule_type, value, action) DO NOTHING;

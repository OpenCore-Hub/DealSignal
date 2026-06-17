# [DS-054] SCIM

## Description
Support SCIM user and group provisioning.

## Source
Original #35

## Version
v0.6.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] SCIM can create/update/deactivate users
- [ ] Workspace memberships sync from SCIM
- [ ] SCIM tokens are scoped and revocable
- [ ] Errors are auditable

## Validation
- [ ] SCIM deactivate removes workspace access

## Dependencies
DS-053

## Type
backend

## Priority
low

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-054-scim
- Version: v0.6.0
- Priority: low

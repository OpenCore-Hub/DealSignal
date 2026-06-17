# [DS-015] Link detail and management

## Description
Build link detail and management UI for status, security, copy, revoke, and activity summaries.

## Source
Original #8

## Version
v0.2.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Page header shows link name/document/status/security mode
- [ ] User can copy and revoke link
- [ ] Intent score placeholder or real score is visible
- [ ] Recent activity summary is visible
- [ ] Revoking immediately blocks viewer access

## Validation
- [ ] Click Revoke and verify status revoked
- [ ] Link detail loads without browser errors

## Dependencies
DS-008, DS-014

## Type
frontend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-015-link-detail-and-management
- Version: v0.2.0
- Priority: high

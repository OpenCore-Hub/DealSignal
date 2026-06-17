# [DS-051] Advanced audit export

## Description
Export richer audit and activity data for compliance and enterprise review.

## Source
Original #33

## Version
v0.6.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Exports support date/user/action filters
- [ ] Exports include access/download/security events
- [ ] Exports can include room-level events
- [ ] Export files are access controlled

## Validation
- [ ] Download filtered audit export and validate rows

## Dependencies
DS-030

## Type
backend

## Priority
low

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-051-advanced-audit-export
- Version: v0.6.0
- Priority: low

# [DS-026] Deal room viewer

## Description
Build the external recipient viewer for deal rooms and room files.

## Source
New

## Version
v0.3.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Recipient can open authorized room
- [ ] Only authorized folders/files are visible
- [ ] Opening files creates view sessions and page events
- [ ] Blocked files show clear explanation

## Validation
- [ ] Room viewer shows only permitted files
- [ ] Opening room file writes events

## Dependencies
DS-024, DS-012

## Type
fullstack

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-026-deal-room-viewer
- Version: v0.3.0
- Priority: high

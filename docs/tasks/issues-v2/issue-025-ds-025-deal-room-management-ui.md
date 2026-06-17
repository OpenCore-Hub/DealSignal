# [DS-025] Deal room management UI

## Description
Build sender-side UI for creating and managing deal rooms, folders, files, and members.

## Source
Original #19

## Version
v0.3.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] User can create room
- [ ] User can add folders/files
- [ ] User can invite members
- [ ] User can preview room structure
- [ ] Desktop UX handles realistic room size

## Validation
- [ ] Create room from browser
- [ ] Invite member and verify room member row

## Dependencies
DS-024

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-025-deal-room-management-ui
- Version: v0.3.0
- Priority: medium

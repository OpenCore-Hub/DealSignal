# [DS-024] Deal room backend

## Description
Implement lightweight deal rooms for multi-document transaction spaces.

## Source
Original #18

## Version
v0.3.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Rooms can be created/listed/updated
- [ ] Folders can be created
- [ ] Documents can be mounted into folders
- [ ] Room members can be invited
- [ ] Workspace isolation is enforced

## Validation
- [ ] Create room/folder/file/member rows
- [ ] Access rules are enforced by API

## Dependencies
DS-004, DS-002

## Type
backend

## Priority
medium

## Risk Class
build_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-024-deal-room-backend
- Version: v0.3.0
- Priority: medium

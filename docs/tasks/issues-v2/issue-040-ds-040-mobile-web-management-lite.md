# [DS-040] Mobile web management lite

## Description
Build lightweight mobile management for activity, hot signals, links, rooms, access requests, and notification settings.

## Source
Original #42

## Version
v0.4.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Bottom nav includes Activity/Hot/Links/Rooms/Me
- [ ] Hot signals are readable on mobile
- [ ] Link/room summaries support key actions
- [ ] Access requests can be approved/denied

## Validation
- [ ] Open mobile viewport and see hot signals
- [ ] Approve access request from mobile

## Dependencies
DS-019, DS-021

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-040-mobile-web-management-lite
- Version: v0.4.0
- Priority: medium

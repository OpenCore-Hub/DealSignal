# [DS-055] Data retention policies

## Description
Allow admins to configure retention for activity, analytics, downloads, and document artifacts.

## Source
Original #36

## Version
v0.6.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Workspace can define retention windows
- [ ] Cleanup job enforces retention
- [ ] Archived/deleted records behave predictably
- [ ] Policy changes are audited

## Validation
- [ ] Run cleanup with test retention and verify old events handled

## Dependencies
DS-013, DS-016

## Type
backend

## Priority
low

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-055-data-retention-policies
- Version: v0.6.0
- Priority: low

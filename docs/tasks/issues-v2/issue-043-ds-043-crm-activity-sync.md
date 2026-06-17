# [DS-043] CRM activity sync

## Description
Sync selected activity events to CRM objects.

## Source
Original #24

## Version
v0.5.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Open/download/hot-score events can sync
- [ ] Sync respects workspace settings
- [ ] Sync errors are logged
- [ ] Duplicate activity is not sent repeatedly

## Validation
- [ ] Simulated page event creates CRM sync payload

## Dependencies
DS-042, DS-014

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-043-crm-activity-sync
- Version: v0.5.0
- Priority: medium

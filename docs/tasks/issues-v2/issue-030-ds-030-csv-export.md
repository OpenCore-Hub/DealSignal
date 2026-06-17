# [DS-030] CSV export

## Description
Export link/document/room analytics for sender reporting.

## Source
Original #20

## Version
v0.3.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Exports support smart link analytics
- [ ] Exports include page-level activity
- [ ] Exports support room analytics where available
- [ ] CSV uses safe headers and no-store responses

## Validation
- [ ] Download CSV and validate expected columns

## Dependencies
DS-017

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-030-csv-export
- Version: v0.3.0
- Priority: medium

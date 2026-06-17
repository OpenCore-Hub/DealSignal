# [DS-046] Content library UI

## Description
Build UI for browsing, filtering, approving, and using content library assets.

## Source
Original #27

## Version
v0.5.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Collections and items are visible
- [ ] Filters by status/type work
- [ ] Approved assets are distinguishable
- [ ] Users can create smart links from library documents

## Validation
- [ ] Open content library and create link from approved document

## Dependencies
DS-045

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-046-content-library-ui
- Version: v0.5.0
- Priority: medium

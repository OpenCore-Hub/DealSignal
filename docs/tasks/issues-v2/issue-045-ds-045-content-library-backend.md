# [DS-045] Content library backend

## Description
Implement managed sales/fundraising content collections with approval status.

## Source
Original #26

## Version
v0.5.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Users can create collections
- [ ] Documents can be added as library items
- [ ] Items have draft/in_review/approved/archived status
- [ ] Approved version can be tracked

## Validation
- [ ] Create approved content item and query library

## Dependencies
DS-004

## Type
backend

## Priority
medium

## Risk Class
build_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-045-content-library-backend
- Version: v0.5.0
- Priority: medium

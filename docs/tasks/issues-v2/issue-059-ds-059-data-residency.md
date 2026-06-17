# [DS-059] Data residency

## Description
Support region-aware data placement for enterprise customers that require residency controls.

## Source
Original #38

## Version
v0.7.0+

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Region metadata is represented
- [ ] Storage and DB strategy is documented
- [ ] Workspace can be assigned region
- [ ] Cross-region risks are documented

## Validation
- [ ] Create workspace with region setting and verify metadata

## Dependencies
DS-001

## Type
infra

## Priority
low

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-059-data-residency
- Version: v0.7.0+
- Priority: low

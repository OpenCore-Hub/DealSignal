# [DS-047] Content performance v1

## Description
Measure which content drives opens, depth, hot scores, and drop-offs.

## Source
New

## Version
v0.5.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Content performance aggregates views/depth/scores
- [ ] Top converting documents are visible
- [ ] Drop-off pages are identified
- [ ] Results can feed Insights

## Validation
- [ ] Seed activities and verify content performance rankings

## Dependencies
DS-045, DS-038

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-047-content-performance-v1
- Version: v0.5.0
- Priority: medium

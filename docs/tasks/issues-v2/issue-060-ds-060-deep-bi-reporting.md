# [DS-060] Deep BI reporting

## Description
Add deeper BI reporting after sufficient event volume and customer demand exist.

## Source
Original #39

## Version
v0.7.0+

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Reports support funnels/cohorts/content performance
- [ ] Reports can be filtered by segment/account/user/date
- [ ] Exports are supported
- [ ] Heavy queries are optimized or pre-aggregated

## Validation
- [ ] Generate report over seeded dataset

## Dependencies
DS-017, DS-018

## Type
backend

## Priority
low

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-060-deep-bi-reporting
- Version: v0.7.0+
- Priority: low

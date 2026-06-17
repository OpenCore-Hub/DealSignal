# [DS-048] Custom domain

## Description
Support custom domains for branded viewer and portal experiences.

## Source
Original #32

## Version
v0.5.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Workspace can add domain
- [ ] DNS verification shows required record
- [ ] Verified domain serves viewer/portal routes
- [ ] Remove/retry flows are supported

## Validation
- [ ] Verify mocked DNS and open viewer URL on custom host

## Dependencies
DS-012

## Type
infra

## Priority
low

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-048-custom-domain
- Version: v0.5.0
- Priority: low

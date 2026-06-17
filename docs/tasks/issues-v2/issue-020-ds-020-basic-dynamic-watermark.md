# [DS-020] Basic dynamic watermark

## Description
Add a basic dynamic watermark overlay for sensitive document viewing.

## Source
Original #16

## Version
v0.2.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Watermark can include recipient email
- [ ] Watermark can include timestamp/link name
- [ ] Watermark displays when enabled
- [ ] Download policy respects watermark settings

## Validation
- [ ] Screenshot viewer and verify watermark
- [ ] Disabled watermark does not render

## Dependencies
DS-012

## Type
backend

## Priority
medium

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-020-basic-dynamic-watermark
- Version: v0.2.0
- Priority: medium

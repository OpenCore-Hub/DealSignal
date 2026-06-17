# [DS-034] Advanced watermark templates

## Description
Support configurable watermark templates beyond the basic overlay.

## Source
Original #21

## Version
v0.4.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Workspace can define watermark templates
- [ ] Templates support recipient/link/time fields
- [ ] Templates can be applied per link/room
- [ ] Preview shows rendered watermark

## Validation
- [ ] Apply template and verify viewer watermark

## Dependencies
DS-020

## Type
backend

## Priority
medium

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-034-advanced-watermark-templates
- Version: v0.4.0
- Priority: medium

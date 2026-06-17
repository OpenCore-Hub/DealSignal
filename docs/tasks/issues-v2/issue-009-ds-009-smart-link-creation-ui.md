# [DS-009] Smart link creation UI

## Description
Build the sender UI for creating smart links with security presets and recipient-friction messaging.

## Source
Original #7

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] User can select Fast Share / Balanced / High Security presets
- [ ] Security controls show recipient friction impact
- [ ] User can configure email verification, download, watermark, NDA, expiration
- [ ] Created link is copyable

## Validation
- [ ] Create link in browser and copy URL
- [ ] High Security preset shows high friction

## Dependencies
DS-008

## Type
frontend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-009-smart-link-creation-ui
- Version: v0.1.0
- Priority: high

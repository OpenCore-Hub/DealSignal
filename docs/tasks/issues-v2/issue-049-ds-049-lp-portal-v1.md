# [DS-049] LP portal v1

## Description
Build a branded LP portal experience for investment-firm use cases.

## Source
Original #31 + #46

## Version
v0.5.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Portal homepage shows brand/latest reports/unread content
- [ ] LP permissions filter visible rooms/files
- [ ] Folder navigation and search work
- [ ] Desktop and mobile layouts are usable

## Validation
- [ ] Different LP accounts see different authorized content

## Dependencies
DS-024, DS-035

## Type
fullstack

## Priority
low

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-049-lp-portal-v1
- Version: v0.5.0
- Priority: low

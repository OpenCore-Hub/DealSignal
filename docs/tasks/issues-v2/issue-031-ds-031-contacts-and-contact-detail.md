# [DS-031] Contacts and contact detail

## Description
Build contacts list and detail pages so DealSignal moves beyond anonymous email analytics.

## Source
Original #43

## Version
v0.3.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Contacts list shows name/email/account/segment/score
- [ ] Contact detail shows timeline and viewed documents/rooms
- [ ] Filters by segment/account/score
- [ ] CRM mapping placeholder is visible

## Validation
- [ ] Open contacts page and see list
- [ ] Open contact detail and see timeline

## Dependencies
DS-002, DS-017

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-031-contacts-and-contact-detail
- Version: v0.3.0
- Priority: medium

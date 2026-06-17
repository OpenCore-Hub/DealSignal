# [DS-017] Recipient activity timeline

## Description
Show sender a recipient-level chronological timeline of opens, page reads, downloads, and access issues.

## Source
Original #13

## Version
v0.2.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Timeline groups events by recipient email/contact
- [ ] Events show readable labels and timestamps
- [ ] Page-level behavior is summarized
- [ ] Timeline can be filtered by link/document

## Validation
- [ ] Generate viewer events and see them in timeline
- [ ] Timeline distinguishes open/page/download/denied

## Dependencies
DS-014, DS-016

## Type
fullstack

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-017-recipient-activity-timeline
- Version: v0.2.0
- Priority: high

# [DS-021] Email alert system

## Description
Send sender-side email alerts for first opens, hot scores, and access requests.

## Source
Original #17

## Version
v0.2.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] First-open alert can be sent
- [ ] Hot-score alert can be sent
- [ ] Preferences prevent unwanted alerts
- [ ] Failures are stored for retry/visibility

## Validation
- [ ] Trigger first open and verify email queued/sent
- [ ] Disable preference and verify no email

## Dependencies
DS-018

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-021-email-alert-system
- Version: v0.2.0
- Priority: medium

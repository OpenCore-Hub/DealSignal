# [DS-016] Download and access-denied events

## Description
Record allowed downloads, blocked downloads, and access denied attempts as first-class commercial signals.

## Source
Original #12

## Version
v0.2.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Allowed downloads create download_events
- [ ] Blocked downloads create download_events with blockedReason
- [ ] Access denied events create activity_events
- [ ] Sender can distinguish security risk from normal engagement

## Validation
- [ ] Blocked download creates row
- [ ] Denied access appears in activity timeline

## Dependencies
DS-010, DS-014

## Type
backend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-016-download-and-access-denied-events
- Version: v0.2.0
- Priority: high

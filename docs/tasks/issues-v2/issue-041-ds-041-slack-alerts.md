# [DS-041] Slack alerts

## Description
Send hot signal and activity alerts to Slack.

## Source
Original #22

## Version
v0.5.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Workspace can connect Slack
- [ ] Hot score and first-open alerts can be sent
- [ ] Messages link back to DealSignal
- [ ] Failures are recorded

## Validation
- [ ] Trigger hot score and verify Slack notification in test/mocked adapter

## Dependencies
DS-021

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-041-slack-alerts
- Version: v0.5.0
- Priority: medium

# [DS-050] Notification rules

## Description
Give users configurable notification rules for email, Slack, CRM, and in-app channels.

## Source
New

## Version
v0.5.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Rules support first-open/hot-score/access-request/download-blocked
- [ ] Rules can be enabled/disabled per user/channel
- [ ] Notification preferences are enforced
- [ ] Queued notifications include related event context

## Validation
- [ ] Disable hot-score Slack rule and verify no Slack notification

## Dependencies
DS-021, DS-041

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-050-notification-rules
- Version: v0.5.0
- Priority: medium

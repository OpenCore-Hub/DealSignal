# [DS-019] Hot signals dashboard

## Description
Build the main dashboard around hot signals, recent activity, risks, and recommended follow-ups.

## Source
Original #15

## Version
v0.2.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Dashboard shows hot/warm/cold recipients
- [ ] Shows recommended follow-ups
- [ ] Shows recent activity
- [ ] Shows risk/security events
- [ ] Supports founder/sales/investor-firm copy variants

## Validation
- [ ] Open dashboard and see hot signal cards
- [ ] New hot score appears after simulated activity

## Dependencies
DS-017, DS-018

## Type
frontend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-019-hot-signals-dashboard
- Version: v0.2.0
- Priority: high

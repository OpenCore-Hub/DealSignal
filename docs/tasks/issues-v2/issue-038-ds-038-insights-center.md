# [DS-038] Insights center

## Description
Build an insights surface for intent analytics, content performance, page performance, team performance, and risk/audit views.

## Source
Original #44

## Version
v0.4.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Intent Analytics cards are visible
- [ ] Content/page performance views show top and drop-off pages
- [ ] Risk and audit views show security events
- [ ] Date range and segment filters work

## Validation
- [ ] Open Insights and filter by date range

## Dependencies
DS-017, DS-018

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-038-insights-center
- Version: v0.4.0
- Priority: medium

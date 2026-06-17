# [DS-023] Demo workspace and seed data

## Description
Create deterministic demo workspaces for founder, investment-firm, and sales storylines.

## Source
New

## Version
v0.2.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Seed includes documents, links, recipients, events, scores, recommendations
- [ ] Demo data can be reset
- [ ] Demo supports screenshots and sales walkthroughs

## Validation
- [ ] Run seed and open dashboard with hot/warm/cold examples

## Dependencies
DS-019

## Type
infra

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-023-demo-workspace-and-seed-data
- Version: v0.2.0
- Priority: medium

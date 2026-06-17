# [DS-039] Insight definitions v1

## Description
Implement first-class definitions for recurring commercial insights.

## Source
New

## Version
v0.4.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Detect stalled recipient
- [ ] Detect returning hot contact
- [ ] Detect key-page spike
- [ ] Detect unexpected geography or blocked access risk
- [ ] Each insight has explanation and severity

## Validation
- [ ] Seed events and verify expected insights are produced

## Dependencies
DS-038

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-039-insight-definitions-v1
- Version: v0.4.0
- Priority: medium

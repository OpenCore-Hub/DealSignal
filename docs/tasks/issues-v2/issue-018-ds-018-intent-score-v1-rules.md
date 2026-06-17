# [DS-018] Intent score v1 rules

## Description
Calculate explainable hot/warm/cold scores using deterministic v1 rules before introducing AI scoring.

## Source
Original #14 + New

## Version
v0.2.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Scores are 0-100
- [ ] Labels are cold/warm/hot
- [ ] Factors JSON records input signals
- [ ] Explanation text is human-readable
- [ ] Scores update after relevant activity

## Validation
- [ ] Simulated activity changes score cold→warm→hot
- [ ] Explanation references actual factors

## Dependencies
DS-013, DS-016, DS-017

## Type
backend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-018-intent-score-v1-rules
- Version: v0.2.0
- Priority: high

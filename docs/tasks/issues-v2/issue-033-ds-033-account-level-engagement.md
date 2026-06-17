# [DS-033] Account-level engagement

## Description
Aggregate engagement across contacts into account-level scores and timelines.

## Source
New

## Version
v0.4.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Account detail shows contacts and account timeline
- [ ] Account score uses contact/link/room activity
- [ ] Account-level recommendations can be generated
- [ ] Domain matching associates contacts to accounts

## Validation
- [ ] Multiple contacts from same account roll up into account score

## Dependencies
DS-031, DS-018

## Type
fullstack

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-033-account-level-engagement
- Version: v0.4.0
- Priority: medium

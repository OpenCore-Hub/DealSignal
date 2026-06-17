# [DS-029] Room engagement score

## Description
Calculate room-level engagement scores for contacts/accounts based on room visits and file/page activity.

## Source
New

## Version
v0.3.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Room score uses file opens, depth, repeats, downloads, and questions
- [ ] Score has explanation and factors
- [ ] Room scores appear in dashboard/room detail

## Validation
- [ ] Simulated room activity changes room score
- [ ] Explanation references room behavior

## Dependencies
DS-018, DS-026

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-029-room-engagement-score
- Version: v0.3.0
- Priority: medium

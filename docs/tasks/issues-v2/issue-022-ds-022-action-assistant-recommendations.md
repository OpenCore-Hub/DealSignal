# [DS-022] Action assistant recommendations

## Description
Generate concrete next-best-action recommendations from recipient behavior and score explanations.

## Source
Original #28

## Version
v0.2.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Recommendations are created for hot or high-change activity
- [ ] Each recommendation has title/body/action/status
- [ ] Recommendation links to contact/link/room context
- [ ] Sender can dismiss or complete recommendations

## Validation
- [ ] Simulated hot activity creates recommendation
- [ ] Dismissed recommendation no longer appears open

## Dependencies
DS-018

## Type
backend

## Priority
high

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-022-action-assistant-recommendations
- Version: v0.2.0
- Priority: high

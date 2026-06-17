# [DS-036] AI follow-up draft

## Description
Generate follow-up email drafts based on activity and recommendations.

## Source
Original #30

## Version
v0.4.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Draft includes subject/body/CTA
- [ ] Draft uses recipient timeline and score factors
- [ ] Sender can copy draft
- [ ] No email is auto-sent in v1

## Validation
- [ ] Generate draft for hot recipient and verify it references actual behavior

## Dependencies
DS-022

## Type
backend

## Priority
low

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-036-ai-follow-up-draft
- Version: v0.4.0
- Priority: low

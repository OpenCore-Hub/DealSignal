# [DS-064] AI risk and opportunity summary

## Description
Summarize account/contact/room activity into AI-generated risks, opportunities, and recommended next moves.

## Source
New

## Version
v0.7.0+

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Summary uses actual events/scores only
- [ ] Risks and opportunities cite evidence
- [ ] Summary can be regenerated
- [ ] Output can create recommendations

## Validation
- [ ] Generate summary for seeded hot account and verify evidence references

## Dependencies
DS-018, DS-039, DS-062

## Type
backend

## Priority
medium

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-064-ai-risk-and-opportunity-summary
- Version: v0.7.0+
- Priority: medium

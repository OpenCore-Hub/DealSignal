# [DS-037] Follow-up draft prompt contract

## Description
Define prompt inputs, outputs, tone variants, safety constraints, and evaluation fixtures for AI follow-up drafts.

## Source
New

## Version
v0.4.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Prompt supports founder/sales/investor-firm tone
- [ ] Output schema is stable
- [ ] Draft avoids fabricating activity
- [ ] Fixtures cover hot/warm/cold examples

## Validation
- [ ] Run prompt fixtures and validate schema

## Dependencies
DS-036

## Type
backend

## Priority
medium

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-037-follow-up-draft-prompt-contract
- Version: v0.4.0
- Priority: medium

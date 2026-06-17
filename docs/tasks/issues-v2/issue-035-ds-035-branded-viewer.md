# [DS-035] Branded viewer

## Description
Allow sender-controlled branding in recipient viewer.

## Source
Original #29

## Version
v0.4.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Viewer can display workspace logo/theme
- [ ] Branding can be scoped by workspace or link
- [ ] Fallback branding is clean
- [ ] Branding does not bypass security messages

## Validation
- [ ] Viewer shows custom logo/theme after settings update

## Dependencies
DS-012

## Type
frontend

## Priority
low

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-035-branded-viewer
- Version: v0.4.0
- Priority: low

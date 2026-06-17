# [DS-032] Settings center

## Description
Create a coherent settings center for workspace, members, branding, security defaults, integrations, billing placeholders, and data/privacy.

## Source
Original #45

## Version
v0.3.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Settings has workspace/members/branding/security/integrations/billing/data sections
- [ ] Members can be invited and roles changed
- [ ] Security defaults can be viewed/edited
- [ ] Branding changes preview in viewer

## Validation
- [ ] Open settings and switch subpages
- [ ] Brand setting updates viewer preview

## Dependencies
DS-002

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-032-settings-center
- Version: v0.3.0
- Priority: medium

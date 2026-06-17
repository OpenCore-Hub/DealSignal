# [DS-053] SSO

## Description
Add SAML/OIDC SSO for enterprise workspaces.

## Source
Original #34

## Version
v0.6.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Workspace can configure SSO provider
- [ ] Users can sign in via SSO
- [ ] Domain restrictions can route to SSO
- [ ] Fallback/admin recovery path exists

## Validation
- [ ] Mock SAML/OIDC login creates authenticated session

## Dependencies
DS-002

## Type
backend

## Priority
low

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-053-sso
- Version: v0.6.0
- Priority: low

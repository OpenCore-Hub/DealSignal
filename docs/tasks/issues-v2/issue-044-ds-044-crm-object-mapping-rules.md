# [DS-044] CRM object mapping rules

## Description
Map local contacts/accounts/links/rooms/documents to external CRM objects.

## Source
New

## Version
v0.5.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Contacts match by email
- [ ] Accounts match by domain/name
- [ ] Mappings are stored in crm_mappings
- [ ] Ambiguous matches require user action

## Validation
- [ ] Known contact maps to external CRM contact
- [ ] Ambiguous domain match is not auto-linked

## Dependencies
DS-042, DS-031, DS-033

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-044-crm-object-mapping-rules
- Version: v0.5.0
- Priority: medium

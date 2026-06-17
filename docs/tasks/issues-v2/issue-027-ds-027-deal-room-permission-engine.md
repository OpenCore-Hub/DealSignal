# [DS-027] Deal room permission engine

## Description
Resolve effective room permissions from member, contact, account, domain, folder, and document rules.

## Source
New

## Version
v0.3.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Supports contact/account/domain/role principals
- [ ] Supports folder/document scopes
- [ ] Resolves canView/canDownload
- [ ] Permission checks are shared by API and viewer

## Validation
- [ ] Domain rule grants room access
- [ ] Document-specific deny/allow behaves correctly

## Dependencies
DS-024

## Type
backend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-027-deal-room-permission-engine
- Version: v0.3.0
- Priority: high

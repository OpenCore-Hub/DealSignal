# [DS-008] Smart link backend

## Description
Implement smart link creation, secure slug generation, access modes, expiration, revoke, download policy, and watermark settings.

## Source
Original #6

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Create one or more unique smart links per document
- [ ] Supports public/email_verification/allowlist/password/approval_required/nda_required modes
- [ ] Password mode requires password_hash
- [ ] Active/expired/revoked states are enforced

## Validation
- [ ] Create smart link and verify DB row
- [ ] Expired link resolves as expired
- [ ] Revoked link blocks viewer access

## Dependencies
DS-004

## Type
backend

## Priority
high

## Risk Class
build_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-008-smart-link-backend
- Version: v0.1.0
- Priority: high

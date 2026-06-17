# [DS-056] Admin security policies

## Description
Give admins default controls for secure sharing and viewer behavior.

## Source
New

## Version
v0.6.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Default access mode can be configured
- [ ] Default download policy can be configured
- [ ] Default watermark policy can be configured
- [ ] Allowed domain/session expiry defaults can be configured

## Validation
- [ ] New smart link inherits workspace security defaults

## Dependencies
DS-032, DS-052

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-056-admin-security-policies
- Version: v0.6.0
- Priority: medium

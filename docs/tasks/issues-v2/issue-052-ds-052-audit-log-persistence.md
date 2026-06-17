# [DS-052] Audit log persistence

## Description
Persist immutable audit logs separately from user-facing activity events.

## Source
New

## Version
v0.6.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] audit_logs records actor/action/target/before/after/ip/userAgent
- [ ] Critical admin/security actions write audit logs
- [ ] Audit logs cannot be edited through app APIs
- [ ] Audit reads are admin-only

## Validation
- [ ] Change security setting and verify audit log row

## Dependencies
DS-014

## Type
backend

## Priority
high

## Risk Class
build_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-052-audit-log-persistence
- Version: v0.6.0
- Priority: high

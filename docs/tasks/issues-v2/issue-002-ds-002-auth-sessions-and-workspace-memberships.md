# [DS-002] Auth, sessions, and workspace memberships

## Description
Implement users, login/session handling, workspace memberships, and role-based API guards.

## Source
Original #2

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [x] Users can register and log in
- [x] Users can create or join workspaces
- [x] Workspace members have owner/admin/member/viewer roles
- [x] Every protected API is workspace-scoped

## Validation
- [x] Cross-workspace resource access returns 403
- [x] A workspace owner membership exists after signup

## Dependencies
DS-001

## Type
backend

## Priority
high

## Risk Class
build_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-002-auth-sessions-and-workspace-memberships
- Version: v0.1.0
- Priority: high

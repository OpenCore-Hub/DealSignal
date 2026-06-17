# [DS-011] Viewer session token security

## Description
Issue server-bound viewer session tokens so event ingestion cannot be forged with only a session id.

## Source
New

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Session start returns sessionId and opaque sessionToken
- [ ] Only hashed sessionToken is stored
- [ ] Page/download/heartbeat events require sessionToken
- [ ] Events are rejected if token/session/scope mismatch

## Validation
- [ ] Forged event with sessionId but wrong token returns 401
- [ ] Valid session token accepts page event

## Dependencies
DS-010

## Type
backend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-011-viewer-session-token-security
- Version: v0.1.0
- Priority: high

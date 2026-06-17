# [DS-010] Viewer access gate

## Description
Implement the public viewer gate that resolves smart links and enforces access before content loads.

## Source
Original #9

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Valid public link opens viewer
- [ ] Expired/revoked link shows clear block reason
- [ ] Email verification collects and verifies recipient email
- [ ] Password and allowlist modes block unauthorized viewers
- [ ] No document bytes are returned before access passes

## Validation
- [ ] Revoked link shows blocked page without content
- [ ] Public link opens without account

## Dependencies
DS-008

## Type
fullstack

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-010-viewer-access-gate
- Version: v0.1.0
- Priority: high

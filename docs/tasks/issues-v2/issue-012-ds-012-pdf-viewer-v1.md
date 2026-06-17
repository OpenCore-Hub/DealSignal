# [DS-012] PDF viewer v1

## Description
Build a readable desktop/mobile PDF viewer with basic navigation and hooks for event tracking.

## Source
Original #10

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Viewer supports desktop and mobile layout
- [ ] Shows page number and previous/next navigation
- [ ] Supports download only when policy allows
- [ ] Emits page visibility hooks
- [ ] First readable page loads quickly for normal PDFs

## Validation
- [ ] Open link on desktop and mobile viewport
- [ ] Blocked download is not exposed

## Dependencies
DS-006, DS-010, DS-011

## Type
frontend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-012-pdf-viewer-v1
- Version: v0.1.0
- Priority: high

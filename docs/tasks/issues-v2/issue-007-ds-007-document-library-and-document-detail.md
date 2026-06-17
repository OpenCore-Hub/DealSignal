# [DS-007] Document library and document detail

## Description
Build the sender-side document library and basic document detail surface.

## Source
Original #5

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Documents list shows name/type/status/link count/open count/update time
- [ ] Users can filter by status/type/owner
- [ ] Document detail shows overview, pages, links, versions, settings placeholders
- [ ] Loading/empty/error states are handled

## Validation
- [ ] Open Documents page and see uploaded documents
- [ ] Click document and load detail page

## Dependencies
DS-004, DS-006

## Type
frontend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-007-document-library-and-document-detail
- Version: v0.1.0
- Priority: high

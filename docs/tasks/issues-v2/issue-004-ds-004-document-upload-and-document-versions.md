# [DS-004] Document upload and document_versions

## Description
Implement document upload API and version records backed by private object storage.

## Source
Original #3

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Users can upload supported document files to a workspace
- [ ] documents and document_versions rows are created transactionally
- [ ] Version numbers increment per document
- [ ] Upload failure rolls back metadata and best-effort deletes storage object

## Validation
- [ ] Upload PDF creates document + version
- [ ] Second version increments version_number

## Dependencies
DS-001, DS-002, DS-003

## Type
backend

## Priority
high

## Risk Class
build_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-004-document-upload-and-document-versions
- Version: v0.1.0
- Priority: high

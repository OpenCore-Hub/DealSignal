# [DS-006] PDF page extraction and document_pages

## Description
Extract page-level metadata, thumbnails or placeholders, and text excerpts from uploaded PDFs.

## Source
Original #4

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] PDF processing writes one document_pages row per page
- [ ] page_count is persisted on document_versions
- [ ] Each page stores text_excerpt where extractable
- [ ] Scanned/empty pages fail gracefully or are marked low-text

## Validation
- [ ] 10-page PDF produces 10 document_pages rows
- [ ] Ready version exposes page_count

## Dependencies
DS-005

## Type
backend

## Priority
high

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-006-pdf-page-extraction-and-document-pages
- Version: v0.1.0
- Priority: high

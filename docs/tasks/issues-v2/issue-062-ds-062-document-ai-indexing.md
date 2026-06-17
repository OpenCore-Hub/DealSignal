# [DS-062] Document AI indexing

## Description
Add embeddings and document chunks for later AI Q&A and opportunity/risk summaries.

## Source
New

## Version
v0.7.0+

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] document_chunks schema exists
- [ ] Worker embeds chunks with controlled concurrency
- [ ] Embedding model/dimensions are recorded
- [ ] Vector search integration test passes

## Validation
- [ ] Process document and retrieve expected chunk by semantic query

## Dependencies
DS-005, DS-006

## Type
backend

## Priority
medium

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-062-document-ai-indexing
- Version: v0.7.0+
- Priority: medium

# [DS-005] Document processing worker

## Description
Create a real background worker for document processing instead of only enqueueing jobs.

## Source
New

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Upload enqueues a process_document_version job
- [ ] Worker consumes jobs with retry/backoff
- [ ] processing_status transitions uploaded → processing → ready/failed
- [ ] processing_error is stored on failure

## Validation
- [ ] Seed a job and verify worker marks version ready
- [ ] Force parser failure and verify failed status + retry behavior

## Dependencies
DS-004

## Type
backend

## Priority
high

## Risk Class
build_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-005-document-processing-worker
- Version: v0.1.0
- Priority: high

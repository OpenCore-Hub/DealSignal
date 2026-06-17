# [DS-003] Private object storage provider

## Description
Implement a private S3/R2-compatible storage abstraction for sensitive deal materials.

## Source
New

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [x] Files are stored by bucket/key, not public URL
- [x] Provider supports put/getStream/delete/signed access or proxy access
- [x] Checksum is recorded for uploaded files
- [x] Storage errors surface actionable messages

## Validation
- [x] Upload and retrieve a test object through provider
- [x] Verify no public object URL is stored in app data

## Dependencies
DS-001

## Type
infra

## Priority
high

## Risk Class
build_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-003-private-object-storage-provider
- Version: v0.1.0
- Priority: high

# [DS-001] Project scaffold and schema baseline

## Description
Establish the monorepo, database schema, migration flow, and baseline checks required for all later work.

## Source
Original #1

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] PostgreSQL schema can be created from migrations
- [ ] pnpm build/typecheck/lint baseline passes
- [ ] README documents local database and migration commands

## Validation
- [ ] Run database migration in a clean environment
- [ ] Run pnpm -r typecheck

## Dependencies
None

## Type
infra

## Priority
high

## Risk Class
build_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-001-project-scaffold-and-schema-baseline
- Version: v0.1.0
- Priority: high

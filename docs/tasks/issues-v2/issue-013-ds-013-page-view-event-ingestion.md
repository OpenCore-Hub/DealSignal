# [DS-013] Page view event ingestion

## Description
Persist page visibility events with reliable duration, idempotency, and workspace scoping.

## Source
Original #11

## Version
v0.1.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] page_view_events stores visibleStartedAt/visibleEndedAt/durationMs
- [ ] Events are tied to view_sessions, documents, and versions
- [ ] Duration is server-sanity-capped
- [ ] Duplicate bursts do not inflate duration excessively

## Validation
- [ ] Browse pages and verify durationMs values
- [ ] Invalid page number or session token is rejected

## Dependencies
DS-011, DS-012

## Type
backend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-013-page-view-event-ingestion
- Version: v0.1.0
- Priority: high

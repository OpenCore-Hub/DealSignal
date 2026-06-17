# [DS-014] Activity event taxonomy

## Description
Define and implement the canonical activity event taxonomy that powers timelines, scoring, alerts, and integrations.

## Source
New

## Version
v0.2.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Event types are documented and typed
- [ ] Core events write to activity_events
- [ ] Metadata shape is stable per event type
- [ ] Events are workspace-scoped and ordered by occurredAt

## Validation
- [ ] Create link/open/page/download/access-denied events and verify activity feed rows

## Dependencies
DS-013

## Type
backend

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-014-activity-event-taxonomy
- Version: v0.2.0
- Priority: high

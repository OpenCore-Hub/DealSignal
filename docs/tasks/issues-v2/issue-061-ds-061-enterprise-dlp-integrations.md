# [DS-061] Enterprise DLP integrations

## Description
Integrate with enterprise DLP systems where customer demand justifies the complexity.

## Source
Original #41

## Version
v0.7.0+

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] DLP provider configuration exists
- [ ] Document/share events can be evaluated
- [ ] Blocked actions record reason
- [ ] Failures fail safe where appropriate

## Validation
- [ ] Mock DLP block prevents configured action

## Dependencies
DS-055

## Type
backend

## Priority
low

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-061-enterprise-dlp-integrations
- Version: v0.7.0+
- Priority: low

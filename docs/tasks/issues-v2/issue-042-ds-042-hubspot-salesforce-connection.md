# [DS-042] HubSpot / Salesforce connection

## Description
Connect CRM providers for later object mapping and activity sync.

## Source
Original #23

## Version
v0.5.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] User can connect HubSpot/Salesforce
- [ ] Credentials are encrypted
- [ ] Connection status is visible
- [ ] Disconnect revokes or disables sync

## Validation
- [ ] Connect mocked CRM provider and verify integration row

## Dependencies
DS-002

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-042-hubspot-salesforce-connection
- Version: v0.5.0
- Priority: medium

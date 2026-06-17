# [DS-028] Deal room templates

## Description
Provide starter room templates for fundraising, LP updates, M&A diligence, and enterprise sales.

## Source
Original #25

## Version
v0.3.0

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] User can create room from template
- [ ] Template creates folders and checklist placeholders
- [ ] Templates are segment-aware
- [ ] User can edit generated structure

## Validation
- [ ] Create fundraising room from template
- [ ] Template folders appear correctly

## Dependencies
DS-024

## Type
fullstack

## Priority
medium

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-028-deal-room-templates
- Version: v0.3.0
- Priority: medium

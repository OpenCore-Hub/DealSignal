# [DS-058] Advanced workflow automation

## Description
Automate multi-step follow-up and routing workflows after core recommendations prove value.

## Source
Original #37

## Version
v0.7.0+

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Users can define trigger/action workflows
- [ ] Workflows use activity and score events
- [ ] Executions are logged
- [ ] Failures can retry or be inspected

## Validation
- [ ] Hot score triggers configured workflow execution

## Dependencies
DS-022

## Type
backend

## Priority
low

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-058-advanced-workflow-automation
- Version: v0.7.0+
- Priority: low

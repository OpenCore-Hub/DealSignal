# [DS-063] Deal room Q&A

## Description
Allow authorized recipients and senders to ask questions across deal-room documents with citations.

## Source
New

## Version
v0.7.0+

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
- [ ] Q&A only searches authorized files
- [ ] Answers include document/page citations
- [ ] Unready files are reported not blocking whole room
- [ ] Conversation history is scoped correctly

## Validation
- [ ] Ask room question and verify answer citations from authorized files only

## Dependencies
DS-024, DS-027, DS-062

## Type
fullstack

## Priority
medium

## Risk Class
unknown

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-063-deal-room-q-a
- Version: v0.7.0+
- Priority: medium

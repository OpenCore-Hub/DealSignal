# [DS-065] i18n foundation for English and Chinese

## Description
Establish DealSignal's bilingual internationalization foundation so every future user-facing implementation supports English and Chinese by default.

## Source
New; product-wide implementation quality gate

## Version
v0.1.0

## Hard Constraints
- Every new user-facing UI string must use a translation key.
- English and Chinese translation files must have identical key structure.
- API errors intended for users must expose stable machine-readable error codes that can be localized by the web app.

## i18n Requirements
- [ ] Web app has an i18n provider and `useI18n()` hook.
- [ ] English locale file exists.
- [ ] Chinese locale file exists.
- [ ] Translation keys are type-safe in application code.
- [ ] Missing locale keys fail `pnpm i18n:check`.
- [ ] Hardcoded visible JSX strings fail `pnpm i18n:check`.
- [ ] Existing app shell uses translation keys instead of hardcoded visible text.

## Acceptance Criteria
- [ ] `apps/web/src/i18n` provides locale resolution, provider, hook, and typed translation keys.
- [ ] `apps/web/src/i18n/locales/en.json` and `zh-CN.json` exist and pass key parity checks.
- [ ] `pnpm i18n:check` runs translation parity and hardcoded-string checks.
- [ ] The root app renders English and Chinese using the same translation keys.
- [ ] Future UI/fullstack issues can reference this issue as the bilingual implementation gate.

## Validation
- [ ] Run `pnpm i18n:check`.
- [ ] Run `pnpm --filter @dealsignal/web typecheck`.
- [ ] Run `pnpm --filter @dealsignal/web build`.
- [ ] Run `pnpm -r lint`.

## Dependencies
DS-001

## Type
fullstack

## Priority
high

## Risk Class
test_failure

## PRD Reference
docs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md

## Loop-it Notes
- Branch hint: feat/ds-065-i18n-foundation
- Version: v0.1.0
- Priority: high

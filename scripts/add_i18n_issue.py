#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Append DS-065 i18n foundation to v2 issue manifest and local docs."""

from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / 'docs' / 'tasks' / 'issue-manifest-v2.json'
ISSUES_DIR = ROOT / 'docs' / 'tasks' / 'issues-v2'
ROADMAP = ROOT / 'docs' / 'roadmap-dealsignal-v2.md'

data = json.loads(MANIFEST.read_text(encoding='utf-8'))
if any(issue['local_id'] == 'DS-065' for issue in data['issues']):
    print('DS-065 already exists')
    raise SystemExit(0)

seq = max(issue['seq'] for issue in data['issues']) + 1
local_path = 'docs/tasks/issues-v2/issue-065-ds-065-i18n-foundation-english-and-chinese.md'
issue = {
    'seq': seq,
    'local_id': 'DS-065',
    'title': 'i18n foundation for English and Chinese',
    'version': 'v0.1.0',
    'type': 'fullstack',
    'priority': 'high',
    'risk_class': 'test_failure',
    'dependencies': ['DS-001'],
    'source': 'New; product-wide implementation quality gate',
    'local_path': local_path,
    'github_number': None,
    'github_url': None,
    'status': 'in_progress',
}

body = '''# [DS-065] i18n foundation for English and Chinese

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
'''

( ROOT / local_path ).write_text(body, encoding='utf-8')
data['issues'].append(issue)
data['issue_count'] = len(data['issues'])
MANIFEST.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding='utf-8')

roadmap = ROADMAP.read_text(encoding='utf-8')
needle = '| 13 | DS-013 | Page view event ingestion | backend | high | test_failure | DS-011, DS-012 |'
replacement = needle + '\n| 65 | DS-065 | i18n foundation for English and Chinese | fullstack | high | test_failure | DS-001 |'
if needle in roadmap and 'DS-065' not in roadmap:
    roadmap = roadmap.replace(needle, replacement)
ROADMAP.write_text(roadmap, encoding='utf-8')
print(local_path)

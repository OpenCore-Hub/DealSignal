#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Refresh GitHub issue tracking docs and mark active implementation issue."""

from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / 'docs' / 'tasks' / 'issue-manifest-v2.json'
MAP = ROOT / 'docs' / 'tasks' / 'github-issues-v2.md'

data = json.loads(MANIFEST.read_text(encoding='utf-8'))
for issue in data['issues']:
  if issue['local_id'] == 'DS-002':
    issue['status'] = 'in_progress'

MANIFEST.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding='utf-8')

lines = [
  '# DealSignal GitHub Issue Map v2',
  '',
  '> GitHub tracking map generated from `docs/tasks/issue-manifest-v2.json`.',
  '',
  '| Seq | ID | GitHub | Version | Priority | Type | Status | Title |',
  '|---:|---|---|---|---|---|---|---|',
]

for issue in data['issues']:
  if issue.get('github_number'):
    gh = '[#{}]({})'.format(issue['github_number'], issue['github_url'])
  else:
    gh = 'Not created'
  lines.append(
    '| {seq} | {local_id} | {gh} | {version} | {priority} | {type} | {status} | {title} |'.format(
      seq=issue['seq'],
      local_id=issue['local_id'],
      gh=gh,
      version=issue['version'],
      priority=issue['priority'],
      type=issue['type'],
      status=issue['status'],
      title=issue['title'],
    )
  )

MAP.write_text('\n'.join(lines) + '\n', encoding='utf-8')
print(MAP.relative_to(ROOT))

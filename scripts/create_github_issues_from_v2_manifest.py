#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Create GitHub issues from docs/tasks/issue-manifest-v2.json.

Idempotency: skips manifest entries that already have github_number.
"""

from __future__ import annotations

import json
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / "docs" / "tasks" / "issue-manifest-v2.json"

LABELS = {
    "type:backend": "5319e7",
    "type:frontend": "0e8a16",
    "type:fullstack": "1d76db",
    "type:infra": "fbca04",
    "type:docs": "0075ca",
    "priority:high": "d93f0b",
    "priority:medium": "fbca04",
    "priority:low": "0e8a16",
    "version:v0.1.0": "b60205",
    "version:v0.2.0": "b60205",
    "version:v0.3.0": "b60205",
    "version:v0.4.0": "b60205",
    "version:v0.5.0": "b60205",
    "version:v0.6.0": "b60205",
    "version:v0.7.0+": "b60205",
    "risk:build_failure": "d93f0b",
    "risk:test_failure": "fbca04",
    "risk:unknown": "cfd3d7",
}


def run(args: list[str], *, input_text: str | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(args, cwd=ROOT, input=input_text, text=True, capture_output=True, check=False)


def ensure_labels() -> None:
    existing_proc = run(["gh", "label", "list", "--limit", "500", "--json", "name"])
    existing = set()
    if existing_proc.returncode == 0:
        existing = {item["name"] for item in json.loads(existing_proc.stdout or "[]")}
    for name, color in LABELS.items():
        if name in existing:
            continue
        desc = name.replace(":", " ")
        proc = run(["gh", "label", "create", name, "--color", color, "--description", desc])
        if proc.returncode != 0 and "already exists" not in proc.stderr.lower():
            print(f"WARN: failed to create label {name}: {proc.stderr.strip()}")


def main() -> None:
    data = json.loads(MANIFEST.read_text(encoding="utf-8"))
    ensure_labels()
    created = []
    for issue in data["issues"]:
        if issue.get("github_number"):
            print(f"skip {issue['local_id']} already created as #{issue['github_number']}")
            continue
        body_path = ROOT / issue["local_path"]
        body = body_path.read_text(encoding="utf-8")
        title = f"[{issue['local_id']}] {issue['title']}"
        labels = [
            f"type:{issue['type']}",
            f"priority:{issue['priority']}",
            f"version:{issue['version']}",
            f"risk:{issue['risk_class']}",
        ]
        args = ["gh", "issue", "create", "--title", title, "--body", body]
        for label in labels:
            args.extend(["--label", label])
        proc = run(args)
        if proc.returncode != 0:
            print(f"ERROR creating {issue['local_id']}: {proc.stderr.strip()}")
            continue
        url = proc.stdout.strip().splitlines()[-1]
        number = url.rstrip("/").split("/")[-1]
        issue["github_number"] = int(number)
        issue["github_url"] = url
        issue["status"] = "open"
        created.append((issue["local_id"], number, url))
        MANIFEST.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"created {issue['local_id']} -> #{number} {url}")
    print(f"created_count={len(created)}")


if __name__ == "__main__":
    main()

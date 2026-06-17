#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Append Issues 21-46 to Section 10 of both PRDs."""

from pathlib import Path
import re

ISSUES_DIR = Path(__file__).parent / "issues"
PRD_EN = Path(__file__).parent.parent / "tasks" / "prd-dealsignal-v1.md"
PRD_ZH = Path(__file__).parent.parent / "tasks" / "prd-dealsignal-v1-zh.md"


def parse_issue(issue_id: int):
    files = list(ISSUES_DIR.glob(f"issue-{issue_id:03d}-*.md"))
    if not files:
        return None
    path = files[0]
    content = path.read_text(encoding="utf-8")

    title = content.split("\n")[0].lstrip("# ").strip()
    source = re.search(r"## Source\s+(.+?)(?=\n##)", content, re.DOTALL)
    source = source.group(1).strip() if source else ""
    desc = re.search(r"## Description\s+(.+?)(?=\n##)", content, re.DOTALL)
    desc = desc.group(1).strip() if desc else ""
    hard = re.search(r"## Hard Constraints\s+(.+?)(?=\n##)", content, re.DOTALL)
    hard = hard.group(1).strip() if hard else ""
    ac_match = re.search(r"## Acceptance Criteria\s+(.+?)(?=\n##)", content, re.DOTALL)
    ac = ac_match.group(1).strip() if ac_match else ""
    val_match = re.search(r"## Validation\s+(.+?)(?=\n##)", content, re.DOTALL)
    val = val_match.group(1).strip() if val_match else ""
    deps = re.search(r"## Dependencies\s+(.+?)(?=\n##)", content, re.DOTALL)
    deps = deps.group(1).strip() if deps else "None"
    itype = re.search(r"## Type\s+(.+?)(?=\n##)", content, re.DOTALL)
    itype = itype.group(1).strip() if itype else ""
    priority = re.search(r"## Priority\s+(.+?)(?=\n##)", content, re.DOTALL)
    priority = priority.group(1).strip() if priority else ""
    risk = re.search(r"## Risk Class\s+(.+?)(?=\n##)", content, re.DOTALL)
    risk = risk.group(1).strip() if risk else ""
    return {
        "id": issue_id,
        "title": title,
        "source": source,
        "description": desc,
        "hard": hard,
        "ac": ac,
        "validation": val,
        "deps": deps,
        "type": itype,
        "priority": priority,
        "risk": risk,
    }


def format_issue_en(issue):
    deps = issue["deps"]
    deps_line = deps if deps == "None" else ", ".join(
        f"Issue {d.strip().split()[-1]}" if d.strip().startswith("Issue") or d.strip().startswith("#") else d
        for d in deps.replace("#", "Issue ").split(",")
    )

    lines = [
        f"### Issue {issue['id']}: {issue['title']}",
        f"- Source: {issue['source']}",
        f"- Type: {issue['type']}",
        f"- Priority: {issue['priority']}",
        f"- Dependencies: {deps_line}",
        f"- Why this slice: {issue['description']}",
        "- Acceptance Criteria:",
    ]
    for line in issue["ac"].split("\n"):
        if line.strip():
            lines.append(f"  {line.strip()}")
    lines.append("- Validation:")
    for line in issue["validation"].split("\n"):
        if line.strip():
            lines.append(f"  {line.strip()}")
    lines.append("- Loop-it notes:")
    lines.append(f"  - Branch hint: feat/issue-{issue['id']}-{issue['title'].lower().replace(' ', '-').replace('/', '-').replace('（', '').replace('）', '').replace('、', '-')[:40]}")
    lines.append(f"  - Risk class: {issue['risk']}")
    lines.append("")
    return "\n".join(lines)


def format_issue_zh(issue):
    deps = issue["deps"]
    deps_line = deps if deps == "None" else ", ".join(
        f"Issue {d.strip().split()[-1]}" if d.strip().startswith("Issue") or d.strip().startswith("#") else d
        for d in deps.replace("#", "Issue ").split(",")
    )

    lines = [
        f"### Issue {issue['id']}：{issue['title']}",
        f"- Source: {issue['source']}",
        f"- Type: {issue['type']}",
        f"- Priority: {issue['priority']}",
        f"- Dependencies: {deps_line}",
        f"- Why this slice: {issue['description']}",
        "- Acceptance Criteria:",
    ]
    for line in issue["ac"].split("\n"):
        if line.strip():
            lines.append(f"  {line.strip()}")
    lines.append("- Validation:")
    for line in issue["validation"].split("\n"):
        if line.strip():
            lines.append(f"  {line.strip()}")
    lines.append("- Loop-it notes:")
    lines.append(f"  - Branch hint: feat/issue-{issue['id']}-{issue['title'].lower().replace(' ', '-').replace('/', '-').replace('（', '').replace('）', '').replace('、', '-')[:40]}")
    lines.append(f"  - Risk class: {issue['risk']}")
    lines.append("")
    return "\n".join(lines)


def insert_before_section_11(prd_path, new_blocks, section_11_title):
    content = prd_path.read_text(encoding="utf-8")
    marker = f"\n## {section_11_title}\n"
    if marker not in content:
        print(f"Marker not found in {prd_path}")
        return
    idx = content.index(marker)
    new_content = content[:idx] + "\n" + new_blocks + content[idx:]
    prd_path.write_text(new_content, encoding="utf-8")
    print(f"Updated {prd_path}")


def main():
    issues = [parse_issue(i) for i in range(21, 47)]
    issues = [i for i in issues if i]
    print(f"Parsed {len(issues)} issues (21-46)")

    blocks_en = "\n".join(format_issue_en(i) for i in issues)
    blocks_zh = "\n".join(format_issue_zh(i) for i in issues)

    insert_before_section_11(PRD_EN, blocks_en, "11. Downstream Handoff")
    insert_before_section_11(PRD_ZH, blocks_zh, "11. 下游交接")


if __name__ == "__main__":
    main()

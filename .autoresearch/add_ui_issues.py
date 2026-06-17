#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Add missing UI prototype issues to complete P0/P1/P2/P3 coverage."""

from pathlib import Path

ISSUES_DIR = Path(__file__).parent / "issues"
ISSUES_DIR.mkdir(parents=True, exist_ok=True)

new_issues = [
    {
        "id": 42,
        "title": "移动端轻量管理后台（Mobile Web Management Lite）",
        "type": "frontend",
        "priority": "medium",
        "dependencies": "Issue 15, Issue 17",
        "source": "UI/page-prototypes.md Section 11",
        "risk_class": "test_failure",
        "description": "实现发送方在移动设备上的轻量管理界面，包括底部导航、Activity Feed、Hot Signals、Link/Room Summary、Access Requests 和通知设置。复杂的数据室搭建和文档上传仍保留在桌面端。",
        "acceptance_criteria": [
            "底部导航包含 Activity / Hot / Links / Rooms / Me",
            "Activity Feed 展示 first open、repeat open、hot score、forward、access request 等事件",
            "Hot Signals 卡片展示收件人、评分、解释、建议动作",
            "Link Summary 支持复制链接、发送跟进、撤销、打开桌面分析",
            "Room Summary 支持批准访问、查看活跃收件人、打开桌面房间",
            "Access Requests 支持一键批准/拒绝/批准域名",
            "在 iOS Safari 和 Chrome Android 上验证可用"
        ],
        "validation": [
            "在移动端浏览器打开管理后台，Hot Signals 列表正常显示",
            "点击 Approve 后 access_grant 状态更新为 approved"
        ],
        "hard_constraints": [],
        "prd_reference": "UI/page-prototypes.md Section 11",
        "spec_reference": "SPEC file: `tasks/spec-dealsignal-v1.md`; relevant sections: Section 2.1 System Context, 4.1 Analytics; API endpoints: GET /analytics/dashboard, GET /analytics/links/:id, POST /rooms/:id/members; data model: recommendations, access_grants, intent_scores"
    },
    {
        "id": 43,
        "title": "联系人管理（Contacts + Contact Detail）",
        "type": "frontend",
        "priority": "medium",
        "dependencies": "Issue 2, Issue 13",
        "source": "UI/page-prototypes.md Section 15",
        "risk_class": "test_failure",
        "description": "实现 Contacts 列表与 Contact Detail 页面，展示投资人/LP/客户/合伙人的互动历史、数据室访问记录、总体热度评分和推荐下一步动作。支持公司与账户级视图。",
        "acceptance_criteria": [
            "Contacts 列表展示姓名、邮箱、组织、细分标签、总体评分",
            "支持按 segment、组织、评分筛选",
            "Contact Detail 展示个人资料、看过的文档、访问过的数据室、时间线",
            "展示 Overall engagement score 和 Recommended next action",
            "Company/Account Detail 展示关联联系人、账户级评分、相关链接和房间",
            "支持与 CRM 映射状态联动（P1）"
        ],
        "validation": [
            "在浏览器中打开 Contacts 页面可见联系人列表",
            "点击联系人进入 Detail 后时间线与评分加载正常"
        ],
        "hard_constraints": [],
        "prd_reference": "UI/page-prototypes.md Section 15",
        "spec_reference": "SPEC file: `tasks/spec-dealsignal-v1.md`; relevant sections: Section 4.1 Analytics, 7.2 External Interfaces; API endpoints: GET /contacts, GET /contacts/:id, GET /analytics/contacts/:id; data model: contacts, accounts, account_contacts, intent_scores, activity_events"
    },
    {
        "id": 44,
        "title": "洞察分析中心（Insights）",
        "type": "frontend",
        "priority": "medium",
        "dependencies": "Issue 13, Issue 14",
        "source": "UI/page-prototypes.md Section 16",
        "risk_class": "test_failure",
        "description": "实现 Insights 页面，包含 Intent Analytics、Content Performance、Page Performance、Team Performance 和 Risk & Audit 视图，帮助用户优化内容并识别机会与风险。",
        "acceptance_criteria": [
            "Intent Analytics 展示 Hot / Warm / Cold 收件人、停滞收件人、活跃度上升的账户",
            "Content Performance 展示 Top converting documents、drop-off pages、最高/最低互动页面",
            "Page Performance 展示每页平均停留时间、重读率、跳出率",
            "Team Performance 展示成员活跃度、发送链接数、产生的高意图信号",
            "Risk and Audit 展示被阻止访问、异常地区、下载事件、撤销/过期链接",
            "支持按时间范围和 segment 筛选"
        ],
        "validation": [
            "打开 Insights 页面可见 Intent Analytics 卡片",
            "筛选时间范围后图表与表格数据更新"
        ],
        "hard_constraints": [],
        "prd_reference": "UI/page-prototypes.md Section 16",
        "spec_reference": "SPEC file: `tasks/spec-dealsignal-v1.md`; relevant sections: Section 4.1 Analytics, 8.3 Database Considerations; API endpoints: GET /analytics/insights/*; data model: activity_events, page_view_events, intent_scores, download_events"
    },
    {
        "id": 45,
        "title": "设置中心（Settings）",
        "type": "frontend",
        "priority": "medium",
        "dependencies": "Issue 2",
        "source": "UI/page-prototypes.md Section 18",
        "risk_class": "test_failure",
        "description": "实现 Settings 页面，支持工作区配置、成员管理、角色权限、品牌设置、安全默认值、集成连接、账单和数据隐私设置。",
        "acceptance_criteria": [
            "Workspace 设置：名称、slug、模式（founder/investment_firm/sales/mixed）",
            "Members 设置：邀请成员、分配角色 owner/admin/member/viewer、移除成员",
            "Branding 设置：上传 logo、设置主色、预览品牌化阅读器",
            "Security defaults：默认访问模式、下载策略、水印策略",
            "Integrations：连接/断开 Slack、HubSpot、Salesforce",
            "Billing：展示当前计划与使用配额（可占位）",
            "Data and privacy：数据保留、删除请求入口"
        ],
        "validation": [
            "在浏览器中打开 Settings 可切换各子页面",
            "修改品牌设置后 viewer 顶部栏显示自定义 logo"
        ],
        "hard_constraints": [],
        "prd_reference": "UI/page-prototypes.md Section 18",
        "spec_reference": "SPEC file: `tasks/spec-dealsignal-v1.md`; relevant sections: Section 7.1 Authentication & Authorization, 7.2 External Interfaces; API endpoints: GET/PUT /workspaces/:id, GET/POST /workspaces/:id/memberships, GET/POST /integrations; data model: workspaces, workspace_memberships, integrations, notification_preferences"
    },
    {
        "id": 46,
        "title": "品牌化 LP 门户 UI（LP Portal）",
        "type": "frontend",
        "priority": "low",
        "dependencies": "Issue 18, Issue 29",
        "source": "P2: LP Portal, UI/page-prototypes.md Section 10.3 Mobile Room Viewer",
        "risk_class": "unknown",
        "description": "为投资机构实现品牌化 LP 门户界面，LP 登录后可见 fund deck、季度报告、税务文件等聚合材料，支持按 LP 权限展示不同内容。",
        "acceptance_criteria": [
            "门户首页展示工作区品牌、最新报告、未读内容",
            "按 LP 账户/联系人权限过滤可见房间和文件",
            "支持文件夹导航和文件搜索",
            "展示通知和新内容上线提醒",
            "响应式布局支持桌面和移动端"
        ],
        "validation": [
            "LP 登录门户后可见被授权的报告列表",
            "不同 LP 账户看到的内容按权限区分"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P2, UI/page-prototypes.md Section 10.3",
        "spec_reference": "SPEC file: `tasks/spec-dealsignal-v1.md`; relevant sections: Section 4.1 Rooms; API endpoints: GET /rooms/:id, GET /rooms/:id/files; data model: deal_rooms, deal_room_members, deal_room_access_rules"
    }
]


def slugify(text):
    return text.lower().replace(" ", "-").replace("/", "-").replace("（", "").replace("）", "").replace("、", "-").replace("+", "-")[:60]


def build_body(issue):
    deps = issue["dependencies"]
    if deps == "None":
        deps_line = "无"
    else:
        deps_line = ", ".join(f"#{d.strip().split()[-1]}" if d.strip().startswith("Issue") else d for d in deps.split(","))

    lines = [
        f"# {issue['title']}",
        "",
        "## Description",
        issue["description"],
        "",
        "## Source",
        issue["source"],
        "",
        "## Hard Constraints",
    ]
    if issue["hard_constraints"]:
        for hc in issue["hard_constraints"]:
            lines.append(f"- {hc}")
    else:
        lines.append("- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。")
    lines.extend([
        "",
        "## Acceptance Criteria",
    ])
    for ac in issue["acceptance_criteria"]:
        lines.append(f"- [ ] {ac}")
    lines.extend([
        "",
        "## Validation",
    ])
    for v in issue["validation"]:
        lines.append(f"- [ ] {v}")
    lines.extend([
        "",
        "## Dependencies",
        deps_line,
        "",
        "## Type",
        issue["type"],
        "",
        "## Priority",
        issue["priority"],
        "",
        "## Risk Class",
        issue["risk_class"],
        "",
        "## PRD Reference",
        issue["prd_reference"],
        "",
        "## SPEC Reference",
        issue["spec_reference"],
        "",
    ])
    return "\n".join(lines)


def main():
    for issue in new_issues:
        slug = slugify(issue["title"])
        filename = f"issue-{issue['id']:03d}-{slug}.md"
        filepath = ISSUES_DIR / filename
        filepath.write_text(build_body(issue), encoding="utf-8")
        print(f"Created {filepath}")


if __name__ == "__main__":
    main()

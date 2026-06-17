#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Generate local Issue markdown files from DealSignal PRD.md + UI prototypes."""

import os
from pathlib import Path

ISSUES_DIR = Path(__file__).parent / ".autoresearch" / "issues"
ISSUES_DIR.mkdir(parents=True, exist_ok=True)

COMMON_HC = [
    "所有多租户表必须按 workspace_id 隔离",
    "访问控制必须在文档内容加载前执行",
    "分析事件表为 append-only，不得更新历史记录",
    "收件人只有在发送方策略明确要求时才需要创建账户"
]

issues = [
    {
        "id": 1,
        "title": "项目脚手架与数据库 Schema 落地",
        "type": "infra",
        "priority": "high",
        "dependencies": "None",
        "source": "database-model.md Section 4, sql/schema.sql",
        "risk_class": "build_failure",
        "description": "搭建 DealSignal 项目基础结构，落地 PostgreSQL 数据库 schema、迁移机制、基础配置，使后续后端功能可以基于统一的数据模型开发。",
        "acceptance_criteria": [
            "PostgreSQL schema 通过初始化脚本可正常创建所有 P0 表与索引",
            "迁移工具配置完成并可在新环境一键应用",
            "项目目录结构清晰区分 backend、frontend、shared 等模块",
            "README 包含本地启动数据库与运行迁移的命令",
            "TypeScript / lint / build 基础检查通过"
        ],
        "validation": [
            "运行迁移命令后数据库包含 users、workspaces、documents、document_versions、smart_links 等核心表",
            "CI 或本地脚本执行 lint 与 build 不报错"
        ],
        "hard_constraints": ["所有多租户表必须按 workspace_id 隔离"],
        "prd_reference": "PRD.md Section 7, database-model.md, sql/schema.sql"
    },
    {
        "id": 2,
        "title": "认证与工作区模型",
        "type": "backend",
        "priority": "high",
        "dependencies": "Issue 1",
        "source": "database-model.md Section 2.1, PRD.md Section 4",
        "risk_class": "build_failure",
        "description": "实现用户认证、工作区创建、成员关系与角色权限，为后续多租户功能提供身份与权限基础。",
        "acceptance_criteria": [
            "用户可通过邮箱注册并登录",
            "用户可创建多个工作区并在其间切换",
            "工作区成员角色支持 owner / admin / member / viewer",
            "所有 API 默认按 workspace_id 过滤",
            "单元测试覆盖认证与角色校验"
        ],
        "validation": [
            "调用非当前工作区的资源返回 403",
            "数据库中存在 owner 角色的 workspace_memberships 记录"
        ],
        "hard_constraints": ["所有多租户表必须按 workspace_id 隔离"],
        "prd_reference": "PRD.md Section 4, database-model.md Section 2.1"
    },
    {
        "id": 3,
        "title": "文档上传与版本管理后端",
        "type": "backend",
        "priority": "high",
        "dependencies": "Issue 1, Issue 2",
        "source": "US-001, FR-1, FR-27",
        "risk_class": "build_failure",
        "description": "实现文档上传 API、对象存储写入、版本记录与基本元数据管理，支持 PDF / PPT / DOC / XLS / 图片 / 视频等文件类型。",
        "acceptance_criteria": [
            "用户可向工作区上传支持的文件类型",
            "文件内容写入对象存储，数据库存储 bucket/key/大小/checksum",
            "同一文档支持多版本（document_versions）",
            "上传失败返回具体错误信息",
            "API 返回上传进度或状态"
        ],
        "validation": [
            "上传 PDF 后 documents 与 document_versions 表生成对应记录",
            "重复上传同一 document 产生递增的 version_number"
        ],
        "hard_constraints": ["所有多租户表必须按 workspace_id 隔离"],
        "prd_reference": "PRD.md Section 7.1, US-001, FR-1, FR-27"
    },
    {
        "id": 4,
        "title": "文档处理流水线（PDF/PPT 转页、缩略图、文本提取）",
        "type": "backend",
        "priority": "high",
        "dependencies": "Issue 3",
        "source": "US-001, FR-12, database-model.md Section 2.3",
        "risk_class": "unknown",
        "description": "异步处理上传的文档，生成页级记录、缩略图、文本摘录，为页面级分析提供数据。",
        "acceptance_criteria": [
            "PDF 上传后自动解析为 document_pages 记录",
            "每页生成缩略图 storage key 与 text_excerpt",
            "处理状态机支持 uploaded / processing / ready / failed",
            "处理失败时记录 processing_error 并允许重试",
            "PPT 与 DOC 至少解析为单页/结构化记录（MVP 可降级）"
        ],
        "validation": [
            "上传 10 页 PDF 后 document_pages 出现 10 条记录",
            "文档 ready 后可查询到 page_count"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 7.1, database-model.md Section 2.3"
    },
    {
        "id": 5,
        "title": "文档库列表与详情页面",
        "type": "frontend",
        "priority": "high",
        "dependencies": "Issue 3, Issue 4",
        "source": "US-001, UI/page-prototypes.md Section 6-7",
        "risk_class": "test_failure",
        "description": "实现桌面端文档列表与文档详情页，支持查看文档状态、版本、链接数、打开次数、Top pages 等指标。",
        "acceptance_criteria": [
            "文档列表展示名称、类型、状态、链接数、打开数、更新时间",
            "支持按状态 / 类型 / 所有者筛选",
            "文档详情页包含 Overview / Pages / Links / Versions / Settings 标签",
            "Overview 展示总打开数、独立收件人、平均阅读时长、下载数",
            "Pages 标签展示每页平均停留时间与跳出率"
        ],
        "validation": [
            "在浏览器中打开 Documents 页面可见已上传文档",
            "点击文档进入 Document Detail 后数据加载正常"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md US-001, UI/page-prototypes.md Section 6-7"
    },
    {
        "id": 6,
        "title": "智能链接创建与权限配置后端",
        "type": "backend",
        "priority": "high",
        "dependencies": "Issue 3",
        "source": "US-002, FR-2~FR-10",
        "risk_class": "build_failure",
        "description": "实现 Smart Link 的创建、命名、slug 生成、访问模式、过期、下载策略、水印开关、密码、白名单等权限配置后端。",
        "acceptance_criteria": [
            "可为文档生成一个或多个唯一 slug 链接",
            "支持 access_mode: public / email_verification / allowlist / password / approval_required",
            "支持 expires_at、revoked_at、download_policy、watermark_enabled",
            " allowlist 支持邮箱域名或具体邮箱列表",
            "密码模式必须设置 password_hash",
            "链接状态包含 active / expired / revoked"
        ],
        "validation": [
            "API 创建链接后 smart_links 表生成记录",
            "过期链接自动返回 expired 状态",
            "撤销链接后 status 变为 revoked"
        ],
        "hard_constraints": ["访问控制必须在文档内容加载前执行"],
        "prd_reference": "PRD.md Section 7.1, US-002, FR-2~FR-10"
    },
    {
        "id": 7,
        "title": "智能链接创建表单 UI",
        "type": "frontend",
        "priority": "high",
        "dependencies": "Issue 6",
        "source": "US-002, UI/page-prototypes.md Section 8",
        "risk_class": "test_failure",
        "description": "实现创建 Smart Link 的表单界面，包含链接命名、收件人邮箱、访问预设、安全控件、接收方摩擦提示与创建结果展示。",
        "acceptance_criteria": [
            "用户可选择 Fast Share / Balanced / High Security 预设",
            "表单显示每个安全选项对接收方摩擦的影响",
            "支持开关：邮箱验证、下载、水印、NDA、过期时间",
            "创建成功后显示可复制链接",
            "在桌面和移动端浏览器中验证可用"
        ],
        "validation": [
            "在浏览器中创建链接后可复制 slug URL",
            "选择 High Security 预设时 recipient friction 显示为 High"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md US-002, UI/page-prototypes.md Section 8"
    },
    {
        "id": 8,
        "title": "链接详情与管理页面",
        "type": "frontend",
        "priority": "high",
        "dependencies": "Issue 6",
        "source": "US-002, UI/page-prototypes.md Section 9",
        "risk_class": "test_failure",
        "description": "实现 Link Detail 页面，展示链接状态、安全模式、复制链接、撤销链接、收件人活动摘要等。",
        "acceptance_criteria": [
            "页面头部展示链接名称、文档名、状态、安全模式",
            "支持一键复制链接与撤销链接",
            "展示意图评分卡片（占位或真实数据）",
            "展示最近活动摘要",
            "撤销后链接状态立即更新"
        ],
        "validation": [
            "点击 Revoke 后链接状态变为 revoked 且 viewer 访问被拒绝",
            "浏览器中 Link Detail 页面加载无报错"
        ],
        "hard_constraints": ["访问控制必须在文档内容加载前执行"],
        "prd_reference": "PRD.md US-002, UI/page-prototypes.md Section 9"
    },
    {
        "id": 9,
        "title": "阅读器访问控制与邮件验证",
        "type": "fullstack",
        "priority": "high",
        "dependencies": "Issue 6",
        "source": "US-003, FR-6, FR-29, UI/page-prototypes.md Section 10.1",
        "risk_class": "test_failure",
        "description": "实现公开访问链接的入口校验、过期/撤销检测、邮箱验证流程、允许列表/密码校验以及被阻止状态的友好提示页。",
        "acceptance_criteria": [
            "打开有效链接时直接进入 viewer",
            "过期或撤销链接显示清晰原因与联系发送方入口",
            "email_verification 模式发送一次性验证邮件/验证码",
            "allowlist 模式拒绝非允许邮箱并提示",
            "password 模式要求输入密码后方可查看",
            "所有访问检查在返回文档内容前完成"
        ],
        "validation": [
            "直接访问已撤销链接返回 403/阻止页面，不暴露文档内容",
            "邮箱验证通过后可在同一浏览器会话内查看",
            "移动端浏览器中验证通过"
        ],
        "hard_constraints": [
            "访问控制必须在文档内容加载前执行",
            "收件人只有在发送方策略明确要求时才需要创建账户"
        ],
        "prd_reference": "PRD.md US-003, FR-6, FR-29, UI/page-prototypes.md Section 10.1"
    },
    {
        "id": 10,
        "title": "文档阅读器渲染与页面导航",
        "type": "frontend",
        "priority": "high",
        "dependencies": "Issue 4, Issue 9",
        "source": "US-003, UI/page-prototypes.md Section 10.2",
        "risk_class": "test_failure",
        "description": "实现文档阅读器前端，支持 PDF 页面渲染、翻页、缩放、大纲导航、移动端适配，并在允许时提供下载按钮。",
        "acceptance_criteria": [
            "桌面与移动端均可打开并阅读文档",
            "支持上一页/下一页、页码指示器、缩略图/大纲",
            "支持 pinch zoom / 双击适配宽度",
            "下载按钮仅在 download_policy 为 allowed 时显示",
            " viewer 首屏渲染在 2 秒内完成（典型 PDF）"
        ],
        "validation": [
            "在浏览器中打开 Smart Link 可逐页浏览 PDF",
            "下载禁用时 viewer 不显示下载入口",
            "Chrome DevTools 移动视图下页面导航可用"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md US-003, UI/page-prototypes.md Section 10.2"
    },
    {
        "id": 11,
        "title": "页面级阅读事件采集后端",
        "type": "backend",
        "priority": "high",
        "dependencies": "Issue 9, Issue 10",
        "source": "US-004, FR-11, FR-12",
        "risk_class": "test_failure",
        "description": "实现 view_sessions、page_view_events、activity_events 的写入接口，记录首次打开、页面停留时长、翻页、关闭等行为。",
        "acceptance_criteria": [
            " viewer 打开时创建 view_sessions 记录",
            "每页可见时间记录到 page_view_events（duration_ms）",
            "activity_events 记录 document_opened / document_closed / page_viewed 等事件",
            "事件写入延迟在正常条件下小于 10 秒",
            "失败写入支持重试或降级记录"
        ],
        "validation": [
            "打开文档并浏览 3 页后，page_view_events 出现 3 条记录",
            "activity_events 按 workspace_id + occurred_at 可查询"
        ],
        "hard_constraints": ["分析事件表为 append-only，不得更新历史记录"],
        "prd_reference": "PRD.md US-004, FR-11, FR-12"
    },
    {
        "id": 12,
        "title": "下载事件与访问拒绝事件采集",
        "type": "backend",
        "priority": "high",
        "dependencies": "Issue 9",
        "source": "US-004, FR-13, FR-29",
        "risk_class": "test_failure",
        "description": "记录下载尝试与结果、访问被拒绝/过期/撤销的原因，为审计与风险分析提供数据。",
        "acceptance_criteria": [
            "成功下载记录到 download_events（status=allowed）",
            "下载被禁用时记录 blocked 与原因",
            "访问被拒绝、过期、撤销分别记录 activity_events",
            "事件包含 actor_email、ip_address、watermarked 等上下文",
            "支持通过 API 查询下载事件列表"
        ],
        "validation": [
            "关闭下载后点击下载按钮，download_events 出现 blocked 记录",
            "访问过期链接产生 access_denied 事件"
        ],
        "hard_constraints": ["分析事件表为 append-only，不得更新历史记录"],
        "prd_reference": "PRD.md US-004, FR-13, FR-29"
    },
    {
        "id": 13,
        "title": "收件人活动时间线与分析展示",
        "type": "fullstack",
        "priority": "high",
        "dependencies": "Issue 11, Issue 12",
        "source": "US-004, FR-14, FR-15, UI/page-prototypes.md Section 9",
        "risk_class": "test_failure",
        "description": "在 Link Detail 等页面展示收件人活动时间线、页面级分析、转发/新收件人检测、下载事件等。",
        "acceptance_criteria": [
            "Link Detail 展示 Activity Timeline：打开、页面浏览、转发、下载",
            "Page Analytics 展示每页平均停留时间",
            "识别并展示新收件人/转发信号",
            "数据更新延迟不超过 10 秒",
            "在浏览器中验证数据准确性"
        ],
        "validation": [
            "打开链接并翻页后，Link Detail 时间线出现对应事件",
            "多一个邮箱打开同一链接时，显示转发/新收件人提示"
        ],
        "hard_constraints": ["分析事件表为 append-only，不得更新历史记录"],
        "prd_reference": "PRD.md US-004, FR-14, FR-15, UI/page-prototypes.md Section 9"
    },
    {
        "id": 14,
        "title": "意图评分计算与解释",
        "type": "backend",
        "priority": "high",
        "dependencies": "Issue 11, Issue 12",
        "source": "US-005, FR-16, FR-17",
        "risk_class": "test_failure",
        "description": "基于阅读行为计算 0-100 的意图评分，输出 Cold/Warm/Hot 标签与解释文本，并随新活动更新。",
        "acceptance_criteria": [
            "系统为每个收件人生成 0-100 的 intent score",
            "评分映射为 cold (0-39) / warm (40-69) / hot (70-100)",
            "评分包含自然语言解释（如“重复查看财务页”）",
            "新活动发生后评分在 1 分钟内重新计算",
            "支持 founder / investment_firm / sales 三类评分类型"
        ],
        "validation": [
            "模拟多次打开与关键页停留后，score 从 cold 升至 warm/hot",
            "数据库 intent_scores 记录包含 explanation 与 factors"
        ],
        "hard_constraints": ["分析事件表为 append-only，不得更新历史记录"],
        "prd_reference": "PRD.md Section 7.4, US-005, FR-16, FR-17"
    },
    {
        "id": 15,
        "title": "仪表盘与热信号 UI",
        "type": "frontend",
        "priority": "high",
        "dependencies": "Issue 13, Issue 14",
        "source": "US-005, US-008, UI/page-prototypes.md Section 5",
        "risk_class": "test_failure",
        "description": "实现登录后首屏 Dashboard，展示今日热信号、推荐跟进、最近打开、活跃数据室、风险提醒与表现最佳内容。",
        "acceptance_criteria": [
            "首屏展示 Hot Signals / Opens / Risks 汇总卡片",
            "Recommended Follow-ups 列表展示高意图收件人与建议动作",
            "Recent Activity 展示最近事件",
            "风险面板展示过期/可疑/被阻止访问",
            "支持创始人/基金/销售三种 segment 的文案变体"
        ],
        "validation": [
            "在浏览器打开 Dashboard 可见热信号卡片",
            "有 Hot score 事件时 Recommended Follow-ups 出现对应条目"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md US-005, US-008, UI/page-prototypes.md Section 5"
    },
    {
        "id": 16,
        "title": "基础动态水印",
        "type": "backend",
        "priority": "medium",
        "dependencies": "Issue 10",
        "source": "US-007, FR-10",
        "risk_class": "unknown",
        "description": "在文档 viewer 中叠加动态水印，显示收件人邮箱与访问时间，用于威慑与追溯；MVP 阶段 viewer 层水印即可。",
        "acceptance_criteria": [
            "watermark_enabled=true 时 viewer 显示半透明水印",
            "水印内容包含收件人邮箱与当前时间戳",
            "水印不遮挡文档主体内容",
            "水印设置状态在 Link Detail 中可见",
            "下载时至少标记是否含水印（MVP 可仅在 viewer 层实现）"
        ],
        "validation": [
            "启用水印后 viewer 截图可见邮箱与时间",
            "关闭水印后 viewer 不再显示水印"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md US-007, FR-10"
    },
    {
        "id": 17,
        "title": "邮件提醒系统",
        "type": "backend",
        "priority": "medium",
        "dependencies": "Issue 14",
        "source": "US-008, FR-22",
        "risk_class": "test_failure",
        "description": "实现邮件通知队列，支持首次打开、Hot score 等事件的邮件提醒，并链接到对应分析页。",
        "acceptance_criteria": [
            "用户可配置是否接收邮件提醒",
            "首次打开文档时向发送方发送邮件",
            "Hot score 事件触发邮件提醒",
            "邮件包含链接到 Link Detail 的按钮",
            "通知发送失败进入重试队列"
        ],
        "validation": [
            "收件人打开链接后，发送方邮箱收到 first-open 邮件",
            "模拟高意图行为后发送方收到 hot-score 邮件"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md US-008, FR-22"
    },
    {
        "id": 18,
        "title": "基础数据室后端",
        "type": "backend",
        "priority": "medium",
        "dependencies": "Issue 3, Issue 2",
        "source": "US-006, FR-18~FR-21",
        "risk_class": "build_failure",
        "description": "实现 Deal Room 的创建、文件夹结构、文件关联、外部成员邀请、访问规则与活动日志。",
        "acceptance_criteria": [
            "可创建 Deal Room 并设置名称、类型、默认权限",
            "支持嵌套文件夹（deal_room_folders）",
            "可向文件夹添加文档版本（deal_room_files）",
            "可邀请外部收件人为 room members",
            "支持按 contact / account / domain 设置访问规则",
            "记录 room 级 activity_events"
        ],
        "validation": [
            "创建 room 后 deal_rooms、deal_room_folders、deal_room_members 生成记录",
            "外部成员访问 room 时按访问规则校验"
        ],
        "hard_constraints": [
            "所有多租户表必须按 workspace_id 隔离",
            "访问控制必须在文档内容加载前执行"
        ],
        "prd_reference": "PRD.md Section 7.5, US-006, FR-18~FR-21"
    },
    {
        "id": 19,
        "title": "数据室创建与管理 UI",
        "type": "frontend",
        "priority": "medium",
        "dependencies": "Issue 18",
        "source": "US-006, UI/page-prototypes.md Section 12-14",
        "risk_class": "test_failure",
        "description": "实现 Deal Rooms 列表、创建流程、Room Detail（Overview/Files/Recipients/Activity/Q&A/Settings）等管理界面。",
        "acceptance_criteria": [
            "Deal Rooms 列表展示名称、模板、成员数、热信号、最后活动",
            "创建流程支持选择模板、命名、添加文档、邀请成员",
            "Room Detail Files 标签展示文件夹树与文件列表",
            "Recipients 标签展示成员权限与活动",
            "Activity 标签展示时间线",
            "Q&A 支持提问与回答（MVP 可简化）"
        ],
        "validation": [
            "在浏览器中创建 Seed Fundraising Room 后可在列表看到",
            "邀请成员后 Room Detail Recipients 标签显示该成员"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md US-006, UI/page-prototypes.md Section 12-14"
    },
    {
        "id": 20,
        "title": "CSV 导出功能",
        "type": "backend",
        "priority": "medium",
        "dependencies": "Issue 13",
        "source": "FR-30",
        "risk_class": "test_failure",
        "description": "为链接、文档、数据室的分析数据提供 CSV 导出能力，方便用户离线使用与周会汇报。",
        "acceptance_criteria": [
            "Link Detail 支持导出该链接的收件人活动 CSV",
            "Document Detail 支持导出文档表现 CSV",
            "Room Detail 支持导出 room activity CSV",
            "CSV 包含时间、收件人、事件类型、页面、时长等字段",
            "导出响应在合理时间内完成（< 10 秒对于千行数据）"
        ],
        "validation": [
            "点击导出后生成可下载 CSV 文件",
            "CSV 内容包含预期列且编码正确"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md FR-30"
    },
    {
        "id": 21,
        "title": "高级水印模板",
        "type": "backend",
        "priority": "medium",
        "dependencies": "Issue 16",
        "source": "P1: Advanced watermark templates",
        "risk_class": "unknown",
        "description": "扩展水印能力，支持自定义水印文本、位置、透明度、颜色，以及下载文件的水印嵌入。",
        "acceptance_criteria": [
            "支持配置水印内容模板（邮箱、时间、IP、自定义文本）",
            "支持调整水印位置与样式",
            "下载 PDF 时可在文件上嵌入水印",
            "不同链接可使用不同水印模板"
        ],
        "validation": [
            "配置自定义水印后 viewer 与下载文件均显示对应水印"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P1"
    },
    {
        "id": 22,
        "title": "Slack 提醒集成",
        "type": "backend",
        "priority": "medium",
        "dependencies": "Issue 17",
        "source": "P1: Slack alerts, FR-23",
        "risk_class": "test_failure",
        "description": "连接 Slack workspace，将首次打开、Hot score、转发检测等事件发送到指定频道。",
        "acceptance_criteria": [
            "用户可通过 OAuth 连接 Slack",
            "可配置提醒事件类型与目标频道",
            "Hot score 事件触发 Slack 消息",
            "消息包含链接到 DealSignal 的按钮"
        ],
        "validation": [
            "配置后模拟 Hot score 事件，Slack 频道收到消息"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 7.8, FR-23, Section 11 P1"
    },
    {
        "id": 23,
        "title": "HubSpot / Salesforce 连接",
        "type": "backend",
        "priority": "medium",
        "dependencies": "Issue 2",
        "source": "US-009, FR-24",
        "risk_class": "test_failure",
        "description": "实现 CRM 集成连接，支持 OAuth 授权并存储 access token，建立 DealSignal 对象与 CRM 对象的映射。",
        "acceptance_criteria": [
            "支持连接 HubSpot 与 Salesforce",
            "存储加密后的 integration credentials",
            "支持将 contact / account / smart_link / deal_room 映射到 CRM 对象",
            "连接状态可显示在设置页"
        ],
        "validation": [
            "完成 OAuth 后 integrations 表生成 connected 记录",
            "crm_mappings 可保存对象映射"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 7.8, US-009, FR-24"
    },
    {
        "id": 24,
        "title": "CRM 活动同步",
        "type": "backend",
        "priority": "medium",
        "dependencies": "Issue 23, Issue 11",
        "source": "US-009, FR-25",
        "risk_class": "test_failure",
        "description": "将文档打开、高意图等事件写入 CRM timeline，并在启用时自动创建跟进任务。",
        "acceptance_criteria": [
            "Smart Link 可与 CRM deal / contact 关联",
            "文档打开事件写入关联 CRM 对象 timeline",
            "Hot score 事件触发 CRM task 创建（可配置）",
            "失败同步进入重试队列"
        ],
        "validation": [
            "模拟文档打开后，HubSpot/Salesforce timeline 出现对应事件"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md US-009, FR-25"
    },
    {
        "id": 25,
        "title": "数据室模板",
        "type": "fullstack",
        "priority": "medium",
        "dependencies": "Issue 18",
        "source": "P1: Deal Room templates",
        "risk_class": "test_failure",
        "description": "为 Seed Fundraising、Series A、LP Update、M&A Diligence、Enterprise Sales 等场景预置数据室模板与默认文件夹。",
        "acceptance_criteria": [
            "创建 room 时可选择模板",
            "模板自动创建默认文件夹结构",
            "模板附带推荐的默认权限与安全设置",
            "模板可在设置中维护"
        ],
        "validation": [
            "选择 Seed Fundraising 模板后自动创建 Pitch/Financials/Legal 等文件夹"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 7.5, Section 11 P1"
    },
    {
        "id": 26,
        "title": "内容库后端",
        "type": "backend",
        "priority": "medium",
        "dependencies": "Issue 3",
        "source": "US-010, FR-26, FR-28",
        "risk_class": "build_failure",
        "description": "实现内容库的数据模型，支持文档状态 Draft / In Review / Approved / Archived、集合管理与使用统计。",
        "acceptance_criteria": [
            "library_collections 与 library_items 表可用",
            "文档状态可在 Draft / In Review / Approved / Archived 间切换",
            "支持将文档加入集合",
            "可追踪文档被使用的链接数与打开数"
        ],
        "validation": [
            "标记文档为 Approved 后状态更新并记录审批人"
        ],
        "hard_constraints": ["所有多租户表必须按 workspace_id 隔离"],
        "prd_reference": "PRD.md Section 7.7, US-010, FR-26, FR-28"
    },
    {
        "id": 27,
        "title": "内容库 UI",
        "type": "frontend",
        "priority": "medium",
        "dependencies": "Issue 26",
        "source": "US-010, UI/page-prototypes.md Section 17",
        "risk_class": "test_failure",
        "description": "实现 Content Library 页面，支持按 Approved / Drafts / Archived / Templates 分类查看、审批、归档与使用统计。",
        "acceptance_criteria": [
            "内容库页面展示文档状态与集合",
            "Admin 可审批或归档文档",
            "可配置仅允许从 Approved 内容创建 Smart Link",
            "展示文档内容表现（链接数、打开数、转化率）"
        ],
        "validation": [
            "在浏览器中打开 Content Library 可看到文档列表",
            "审批后该文档状态变为 Approved"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md US-010, UI/page-prototypes.md Section 17"
    },
    {
        "id": 28,
        "title": "行动助手推荐",
        "type": "backend",
        "priority": "medium",
        "dependencies": "Issue 14",
        "source": "P1: Action Assistant recommendations, PRD.md Section 7.6",
        "risk_class": "unknown",
        "description": "基于意图评分与行为模式生成下一步行动建议（如跟进时机、推荐材料、建议会议），并展示在 Dashboard 与 Link Detail。",
        "acceptance_criteria": [
            "检测高意图、停滞、异常访问等模式",
            "生成推荐标题、正文与建议动作",
            "推荐展示在 Dashboard 与 Link Detail",
            "用户可 dismiss 或 mark done"
        ],
        "validation": [
            "模拟高意图行为后 Dashboard 出现跟进建议",
            "点击 mark done 后 recommendations 状态更新"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 7.6, Section 11 P1"
    },
    {
        "id": 29,
        "title": "品牌化阅读器",
        "type": "frontend",
        "priority": "low",
        "dependencies": "Issue 10",
        "source": "P1: Branded viewer",
        "risk_class": "test_failure",
        "description": "允许工作区在文档 viewer 中展示自定义 logo、品牌色与发送方信息，提升专业形象。",
        "acceptance_criteria": [
            "工作区可上传 logo 与设置主色",
            "viewer 顶部栏展示工作区品牌",
            "品牌设置不遮挡文档内容",
            "移动端 viewer 同步展示品牌"
        ],
        "validation": [
            "配置品牌后 viewer 页面显示自定义 logo"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P1, UI/page-prototypes.md Section 10.2"
    },
    {
        "id": 30,
        "title": "AI 跟进邮件草稿",
        "type": "backend",
        "priority": "low",
        "dependencies": "Issue 28",
        "source": "P2: AI follow-up drafts",
        "risk_class": "unknown",
        "description": "根据收件人行为自动生成个性化跟进邮件草稿，供发送方一键复制或编辑后发送。",
        "acceptance_criteria": [
            "基于行为摘要生成邮件主题与正文",
            "支持创始人/基金/销售三种语气",
            "用户可在 Link Detail 查看并复制草稿",
            "草稿明确标注为 AI 生成，需人工审核后发送"
        ],
        "validation": [
            "高意图收件人详情页展示可用的跟进邮件草稿"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P2, Section 7.6"
    },
    {
        "id": 31,
        "title": "LP 门户",
        "type": "fullstack",
        "priority": "low",
        "dependencies": "Issue 18, Issue 29",
        "source": "P2: LP Portal",
        "risk_class": "unknown",
        "description": "为投资机构提供品牌化 LP 门户，LP 可登录查看 fund deck、季度报告、税务文件等聚合材料。",
        "acceptance_criteria": [
            "可创建 LP Update Room 类型的门户",
            "LP 按账户/联系人权限看到不同内容",
            "门户首页展示最新报告与未读内容",
            "支持通知 LP 新内容上线"
        ],
        "validation": [
            "LP 登录门户后可见被授权的报告列表"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P2"
    },
    {
        "id": 32,
        "title": "自定义域名",
        "type": "infra",
        "priority": "low",
        "dependencies": "Issue 10, Issue 31",
        "source": "P2: Custom domain",
        "risk_class": "unknown",
        "description": "支持工作区绑定自定义域名（如 investor.fund.com），使阅读器与门户展示企业自有域名。",
        "acceptance_criteria": [
            "工作区可配置自定义域名",
            "提供 DNS 验证指引",
            "viewer 链接可通过自定义域名打开",
            "HTTPS 证书自动申请或支持上传"
        ],
        "validation": [
            "配置自定义域名后，Smart Link 可通过该域名访问"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P2"
    },
    {
        "id": 33,
        "title": "高级审计导出",
        "type": "backend",
        "priority": "low",
        "dependencies": "Issue 20",
        "source": "P2: Advanced audit export",
        "risk_class": "test_failure",
        "description": "提供合规级审计导出，包含完整访问日志、IP、设备、下载记录、权限变更等，支持 PDF/CSV。",
        "acceptance_criteria": [
            "可按时间范围导出完整审计日志",
            "导出包含 IP、设备、邮箱、事件类型、结果",
            "支持 tamper-evident 摘要或签名（可选）",
            "导出文件包含工作区与生成时间元数据"
        ],
        "validation": [
            "导出审计日志后文件包含所有事件类型"
        ],
        "hard_constraints": ["分析事件表为 append-only，不得更新历史记录"],
        "prd_reference": "PRD.md Section 11 P2"
    },
    {
        "id": 34,
        "title": "SSO 单点登录",
        "type": "backend",
        "priority": "low",
        "dependencies": "Issue 2",
        "source": "P2: SSO",
        "risk_class": "unknown",
        "description": "支持 SAML / OIDC 单点登录，满足企业客户对工作区成员统一身份管理的需求。",
        "acceptance_criteria": [
            "支持 SAML 2.0 与 OIDC 身份提供商",
            "管理员可配置 SSO 元数据",
            "SSO 用户首次登录自动加入工作区",
            "支持强制 SSO 登录"
        ],
        "validation": [
            "通过 SSO 登录后成功进入工作区"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P2"
    },
    {
        "id": 35,
        "title": "SCIM 用户同步",
        "type": "backend",
        "priority": "low",
        "dependencies": "Issue 34",
        "source": "P2: SCIM",
        "risk_class": "unknown",
        "description": "提供 SCIM 2.0 接口，允许企业通过身份提供商自动同步用户、分配角色、禁用账户。",
        "acceptance_criteria": [
            "实现 SCIM /Users 与 /Groups 端点",
            "支持创建、更新、停用用户",
            "支持通过 group 映射工作区角色",
            "同步事件记录审计日志"
        ],
        "validation": [
            "从 IdP 推送用户后 DealSignal 工作区出现对应成员"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P2"
    },
    {
        "id": 36,
        "title": "数据保留策略",
        "type": "backend",
        "priority": "low",
        "dependencies": "Issue 11, Issue 12",
        "source": "P2: Data retention policies",
        "risk_class": "unknown",
        "description": "允许企业工作区配置数据保留周期，自动清理过期事件、IP 地址、已删除文件等。",
        "acceptance_criteria": [
            "管理员可设置文档、事件、IP 的保留期限",
            "系统按策略自动匿名化或删除过期数据",
            "保留策略变更前通知管理员",
            "支持 GDPR 删除请求工作流"
        ],
        "validation": [
            "设置 30 天事件保留后，过期事件被清理"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P2, Section 10 Privacy"
    },
    {
        "id": 37,
        "title": "高级工作流自动化",
        "type": "backend",
        "priority": "low",
        "dependencies": "Issue 28",
        "source": "P3: Advanced workflow automation",
        "risk_class": "unknown",
        "description": "支持用户自定义触发器与动作，如特定页面访问后自动发送邮件、进入数据室后创建 CRM 任务等。",
        "acceptance_criteria": [
            "可视化或配置化规则编辑器",
            "支持事件触发器：打开、Hot score、下载、进入 room",
            "支持动作：发送邮件、创建任务、邀请成员、更新 CRM",
            "规则执行记录可查询"
        ],
        "validation": [
            "配置规则后触发事件自动执行对应动作"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P3"
    },
    {
        "id": 38,
        "title": "数据驻留",
        "type": "infra",
        "priority": "low",
        "dependencies": "Issue 1",
        "source": "P3: Data residency",
        "risk_class": "unknown",
        "description": "支持企业客户选择数据存储区域（如 US/EU/Asia），满足合规与本地化要求。",
        "acceptance_criteria": [
            "企业工作区可选择数据驻留区域",
            "文档、事件、数据库按区域隔离",
            "跨区域访问遵循策略限制"
        ],
        "validation": [
            "选择 EU 区域后，该工作区数据存储在 EU"
        ],
        "hard_constraints": ["所有多租户表必须按 workspace_id 隔离"],
        "prd_reference": "PRD.md Section 11 P3"
    },
    {
        "id": 39,
        "title": "深度 BI 报表",
        "type": "backend",
        "priority": "low",
        "dependencies": "Issue 13, Issue 14",
        "source": "P3: Deep BI reporting",
        "risk_class": "unknown",
        "description": "提供多维度 BI 报表：内容转化漏斗、团队表现、账户级 engagement、 cohort 分析等，支持导出与嵌入。",
        "acceptance_criteria": [
            "提供漏斗、趋势、对比等报表视图",
            "支持按时间、segment、内容类型筛选",
            "支持导出报表为 CSV/PDF",
            "性能可支持百万级事件"
        ],
        "validation": [
            "生成月度内容表现报表并导出"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P3"
    },
    {
        "id": 40,
        "title": "SOC 2 支持工作流",
        "type": "docs",
        "priority": "low",
        "dependencies": "Issue 33, Issue 36",
        "source": "P3: SOC 2 support workflows",
        "risk_class": "unknown",
        "description": "整理并实施 SOC 2 合规所需的政策、控制、证据收集与审计导出模板。",
        "acceptance_criteria": [
            "制定访问控制、变更管理、事件响应等政策文档",
            "实现审计日志不可篡改与导出",
            "建立定期访问复核工作流",
            "提供审计师只读导出接口"
        ],
        "validation": [
            "可生成 SOC 2 所需的审计证据包"
        ],
        "hard_constraints": ["分析事件表为 append-only，不得更新历史记录"],
        "prd_reference": "PRD.md Section 11 P3"
    },
    {
        "id": 41,
        "title": "企业 DLP 集成",
        "type": "backend",
        "priority": "low",
        "dependencies": "Issue 36",
        "source": "P3: Enterprise DLP integrations",
        "risk_class": "unknown",
        "description": "与常见 DLP/CASB 方案集成，支持内容扫描、敏感数据检测、外发策略联动等企业安全需求。",
        "acceptance_criteria": [
            "提供 API 或 webhook 供 DLP 系统查询/扫描内容",
            "支持上传前敏感信息扫描",
            "支持按 DLP 策略阻止下载或分享",
            "记录 DLP 相关审计事件"
        ],
        "validation": [
            "上传含敏感信息文档时触发 DLP 策略"
        ],
        "hard_constraints": [],
        "prd_reference": "PRD.md Section 11 P3"
    }
]


def slugify(text):
    return text.lower().replace(" ", "-").replace("/", "-").replace("（", "").replace("）", "").replace("、", "-")[:60]


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
    ])
    return "\n".join(lines)


def main():
    for issue in issues:
        slug = slugify(issue["title"])
        filename = f"issue-{issue['id']:03d}-{slug}.md"
        filepath = ISSUES_DIR / filename
        filepath.write_text(build_body(issue), encoding="utf-8")
        print(f"Created {filepath}")


if __name__ == "__main__":
    main()

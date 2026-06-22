---
id: "IM-2024-002"
version: "v2.1.2"
status: "已批准"
owner: "技术负责人 / 项目经理"
linked_docs:
  - "docs/IMPLEMENTATION-PLAN-v2.1.2.md"
  - "docs/PRD-v2.1.0.md"
  - "docs/TDD-v2.1.0.md"
  - "docs/ARCHITECTURE-v2.1.0.md"
  - "docs/database-model-v2.1.0.md"
  - "docs/API-SPEC-v2.1.0.md"
  - "docs/tasks/agent-tasks-v2.1.2/*.md"
---

# DealSignal v2.1.2 开发执行计划 Issue 拆分清单

> **资源编号**：`IM-2024-002`  
> **版本**：`v2.1.2`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`技术负责人 / 项目经理`  
> **编写日期**：`2026-06-21`  
> **关联资源**：
> - `docs/IMPLEMENTATION-PLAN-v2.1.2.md`
> - `docs/PRD-v2.1.0.md`
> - `docs/TDD-v2.1.0.md`
> - `docs/ARCHITECTURE-v2.1.0.md`
> - `docs/database-model-v2.1.0.md`
> - `docs/API-SPEC-v2.1.0.md`
> - `docs/templates/CODE-REVIEW-template-v1.md`
> - `docs/tasks/agent-tasks-v2.1.2/*.md`
> **评审人**：`技术负责人、项目经理、产品负责人`  
> **执行状态（IMPLEMENTATION-PLAN 专用）**：`待开始`

---

## 1. 资源说明

本资源是 `IMPLEMENTATION-PLAN-v2.1.2.md` 的下游拆分产物，用于把 v2.1.2 阶段的前端收尾与后端 MVP 进一步拆成可进入 Sprint、可分配、可跟踪、可验收的 issues/tickets。

**核心目标**：
- 每个 issue 只负责一个可独立合并的交付单元。
- 每个 issue 都能追溯到 `TASK / PRD / TDD / API / 测试用例`。
- issue 的版本、优先级、状态、风险、依赖一目了然。
- 补齐 v2.1.0 issue 清单中未被 AGENT-TASK 覆盖的缺口（`DS-004`、`DS-014`、`DS-018`、`DS-023`、`DS-024`、`DS-025` 等）。

**与 IMPLEMENTATION-PLAN 的关系**：

```text
IMPLEMENTATION-PLAN-v2.1.2（TASK 层）
        │
        ▼
ISSUE-MANIFEST v2.1.2（issue 层）
        │
        ▼
Sprint Board → 分支 → PR → 测试 → 上线
```

---

## 2. 字段规范

### 2.1 Issue 编号

格式：`DS-{NNN}`

| 字段 | 示例 | 说明 |
|------|------|------|
| 项目前缀 | `DS` | DealSignal 英文缩写 |
| 序号 | `001` | 自增，三位 |

### 2.2 版本（Version）

| 版本 | 目标 |
|------|------|
| `v2.1.2` | 前端收尾 + 后端 MVP 核心链路可跑通 |

### 2.3 优先级（Priority）

| 优先级 | 说明 |
|--------|------|
| `P0` | 阻塞里程碑，必须完成 |
| `P1` | 重要，尽量在当前版本完成 |
| `P2` | 有价值，可延期到后续版本 |

### 2.4 状态（Status）

| 状态 | 说明 |
|------|------|
| `待创建` | 已规划，尚未在 issue tracker 创建 |
| `待开始` | 已创建，未进入开发 |
| `开发中` | 工程师正在实现 |
| `代码审查中` | PR 已提交，等待 review |
| `测试中` | QA 验证中 |
| `已验收` | 通过所有质量门禁 |
| `已上线` | 已合并发布 |
| `阻塞` | 因依赖/问题暂停 |

### 2.5 类型（Type）

| 类型 | 说明 |
|------|------|
| `backend` | 后端服务/API |
| `frontend` | Web/前端 |
| `fullstack` | 前后端联动 |
| `infra` | 基础设施/运维/部署 |
| `ai` | AI/LLM/解析相关 |
| `security` | 安全/合规 |
| `docs` | 资源/规范 |
| `test` | 测试工程 |

### 2.6 风险等级（Risk Class）

| 风险 | 说明 |
|------|------|
| `build_failure` | 高风险，可能导致构建/流水线失败 |
| `test_failure` | 中等风险，可能影响测试稳定性 |
| `unknown` | 风险未知，需要技术预研 |

---

## 3. Issue 清单

### 3.1 清单总表

| Issue ID | 标题 | 版本 | 优先级 | 状态 | 类型 | 风险 | 依赖 | 关联 TASK | PRD | TDD | API | 负责人 |
|----------|------|------|--------|------|------|------|------|-----------|-----|-----|-----|--------|
| DS-001 | 工程脚手架与项目初始化 | v2.1.2 | P0 | 待创建 | infra | build_failure | - | TASK-BACKEND-001 | - | 3.2 / 9.1 | - | `{待分配}` |
| DS-002 | 用户认证、租户与 Workspace 模块 | v2.1.2 | P0 | 待创建 | backend | build_failure | DS-001 | TASK-BACKEND-002 | FR-01 ~ FR-02 | 6.1 | API-01 ~ API-04 | `{待分配}` |
| DS-003 | 对象存储与后端签名 URL | v2.1.2 | P0 | 待创建 | infra | build_failure | DS-001 | TASK-BACKEND-003 | FR-03 | 7.3 ~ 7.4 | API-06 | `{待分配}` |
| DS-004 | 子域名/自定义域名与 SSL 自动签发 | v2.1.2 | P1 | 待创建 | infra | build_failure | DS-001 | TASK-BACKEND-007 | FR-03 | 7.4 | API-15 | `{待分配}` |
| DS-005 | 文档上传 API | v2.1.2 | P0 | 待创建 | backend | build_failure | DS-002, DS-003 | TASK-BACKEND-003 | FR-02 | 6.1 | API-05 | `{待分配}` |
| DS-006 | PDF Pipeline（bbox + webp） | v2.1.2 | P0 | 待创建 | ai | unknown | DS-005 | TASK-BACKEND-003 | FR-02 | 6.2 | - | `{待分配}` |
| DS-007 | Office Pipeline（OnlyOffice 转 PDF） | v2.1.2 | P0 | 待创建 | ai | unknown | DS-006 | TASK-BACKEND-003 | FR-02 | 6.2 | - | `{待分配}` |
| DS-008 | 数据库迁移与搜索索引 | v2.1.2 | P0 | 待创建 | backend | build_failure | DS-006 | TASK-BACKEND-004 | FR-02 | 4.x / 8.x | - | `{待分配}` |
| DS-009 | 签名 URL 与权限校验 | v2.1.2 | P0 | 待创建 | backend | build_failure | DS-003, DS-008 | TASK-BACKEND-005 | FR-03 | 6.3 / 7.4 | API-06 ~ API-08 | `{待分配}` |
| DS-010 | Viewer Canvas 前端 | v2.1.2 | P0 | 待创建 | frontend | test_failure | DS-009 | TASK-FRONTEND-002 | FR-03 ~ FR-04 | 6.3 | API-06 ~ API-08 | `{待分配}` |
| DS-011 | Search Service | v2.1.2 | P0 | 待创建 | backend | test_failure | DS-008 | TASK-BACKEND-004 | FR-05 | 6.4 | API-09 | `{待分配}` |
| DS-012 | Evidence Service | v2.1.2 | P0 | 待创建 | backend | test_failure | DS-011 | TASK-BACKEND-004 | FR-05 | 6.5 | API-10 | `{待分配}` |
| DS-013 | Assistant Service | v2.1.2 | P0 | 待创建 | backend | unknown | DS-012 | TASK-BACKEND-004 | FR-05 ~ FR-06 | 6.6 | API-11 ~ API-12 | `{待分配}` |
| DS-014 | 悬浮 AI 助手前端 | v2.1.2 | P0 | 待创建 | frontend | test_failure | DS-010, DS-013 | TASK-FRONTEND-004 | FR-05 ~ FR-06 | 6.3 / 6.6 | API-11 ~ API-12 | `{待分配}` |
| DS-015 | 智能链接与权限 | v2.1.2 | P0 | 待创建 | backend | build_failure | DS-009 | TASK-BACKEND-005 | FR-07 ~ FR-09 | 6.7 | API-13 ~ API-14 | `{待分配}` |
| DS-016 | Dashboard 前端 | v2.1.2 | P1 | 待创建 | frontend | test_failure | DS-015 | TASK-FRONTEND-005 | FR-10 ~ FR-11 | 11.2 | API-16 ~ API-18 | `{待分配}` |
| DS-017 | 热度评分与 Analytics | v2.1.2 | P0 | 待创建 | backend | test_failure | DS-010 | TASK-BACKEND-005 | FR-10 | 6.8 | API-16 | `{待分配}` |
| DS-018 | 行为提醒与跟进建议 | v2.1.2 | P1 | 待创建 | backend | test_failure | DS-017 | TASK-BACKEND-008 | FR-11 | 6.8 | API-17 | `{待分配}` |
| DS-019 | 数据室模块 | v2.1.2 | P0 | 待创建 | fullstack | build_failure | DS-008, DS-015 | TASK-BACKEND-006 | FR-12 ~ FR-13 | 6.10 | API-19 ~ API-22 | `{待分配}` |
| DS-020 | 邮件通知系统 | v2.1.2 | P1 | 待创建 | backend | test_failure | DS-001 | TASK-BACKEND-009 | FR-14 | 6.9 | API-23 | `{待分配}` |
| DS-021 | CRM 集成（HubSpot/Salesforce） | v2.1.2 | P2 | 待创建 | backend | test_failure | DS-017 | TASK-BACKEND-009 | FR-15 | 6.11 | API-24 | `{待分配}` |
| DS-022 | Slack 集成 | v2.1.2 | P2 | 待创建 | backend | test_failure | DS-017 | TASK-BACKEND-009 | FR-16 | 6.11 | API-25 | `{待分配}` |
| DS-023 | 测试用例与自动化 | v2.1.2 | P0 | 待创建 | test | test_failure | 功能开发完成 | TASK-TEST-001 | AC-01 ~ AC-32 | 10.x | - | `{待分配}` |
| DS-024 | 性能压测与优化 | v2.1.2 | P1 | 待创建 | test | test_failure | 功能开发完成 | TASK-TEST-002 | 第 9 节 NFR | 8.x | - | `{待分配}` |
| DS-025 | 安全扫描与修复 | v2.1.2 | P0 | 待创建 | security | build_failure | 开发完成 | TASK-BACKEND-010 | 第 17 节 | 7.x | - | `{待分配}` |
| DS-026 | 前端质量收尾 | v2.1.2 | P1 | 待创建 | frontend | test_failure | - | TASK-FRONTEND-001 | FR-设置/AI | 6.3 | - | `{待分配}` |
| DS-027 | 前端-后端集成层 | v2.1.2 | P0 | 待创建 | frontend | build_failure | DS-002（契约） | TASK-FRONTEND-003 | FR-API | 5.x | §2/§3 | `{待分配}` |

### 3.2 按版本分组

#### v2.1.2 — 前端收尾 + 后端 MVP 核心链路

| Issue ID | 标题 | 优先级 | 状态 | 负责人 |
|----------|------|--------|------|--------|
| DS-001 | 工程脚手架与项目初始化 | P0 | 待创建 | `{待分配}` |
| DS-002 | 用户认证、租户与 Workspace 模块 | P0 | 待创建 | `{待分配}` |
| DS-003 | 对象存储与后端签名 URL | P0 | 待创建 | `{待分配}` |
| DS-004 | 子域名/自定义域名与 SSL 自动签发 | P1 | 待创建 | `{待分配}` |
| DS-005 | 文档上传 API | P0 | 待创建 | `{待分配}` |
| DS-006 | PDF Pipeline（bbox + webp） | P0 | 待创建 | `{待分配}` |
| DS-007 | Office Pipeline（OnlyOffice 转 PDF） | P0 | 待创建 | `{待分配}` |
| DS-008 | 数据库迁移与搜索索引 | P0 | 待创建 | `{待分配}` |
| DS-009 | 签名 URL 与权限校验 | P0 | 待创建 | `{待分配}` |
| DS-010 | Viewer Canvas 前端 | P0 | 待创建 | `{待分配}` |
| DS-011 | Search Service | P0 | 待创建 | `{待分配}` |
| DS-012 | Evidence Service | P0 | 待创建 | `{待分配}` |
| DS-013 | Assistant Service | P0 | 待创建 | `{待分配}` |
| DS-014 | 悬浮 AI 助手前端 | P0 | 待创建 | `{待分配}` |
| DS-015 | 智能链接与权限 | P0 | 待创建 | `{待分配}` |
| DS-016 | Dashboard 前端 | P1 | 待创建 | `{待分配}` |
| DS-017 | 热度评分与 Analytics | P0 | 待创建 | `{待分配}` |
| DS-018 | 行为提醒与跟进建议 | P1 | 待创建 | `{待分配}` |
| DS-019 | 数据室模块 | P0 | 待创建 | `{待分配}` |
| DS-020 | 邮件通知系统 | P1 | 待创建 | `{待分配}` |
| DS-021 | CRM 集成（HubSpot/Salesforce） | P2 | 待创建 | `{待分配}` |
| DS-022 | Slack 集成 | P2 | 待创建 | `{待分配}` |
| DS-023 | 测试用例与自动化 | P0 | 待创建 | `{待分配}` |
| DS-024 | 性能压测与优化 | P1 | 待创建 | `{待分配}` |
| DS-025 | 安全扫描与修复 | P0 | 待创建 | `{待分配}` |
| DS-026 | 前端质量收尾 | P1 | 待创建 | `{待分配}` |
| DS-027 | 前端-后端集成层 | P0 | 待创建 | `{待分配}` |

---

## 4. Issue 详情模板

每个 issue 应保存为 `docs/tasks/issues-v2.1.2/issue-{NNN}-{slug}.md`。

```markdown
# DS-NNN {标题}

## 元数据

- **版本**: `v2.1.2`
- **优先级**: `P0`
- **状态**: `待开始`
- **类型**: `backend`
- **风险**: `build_failure`
- **依赖**: `DS-001, DS-002`
- **关联 TASK**: `TASK-XXX-001`
- **PRD**: `FR-XX`
- **TDD**: `X.X`
- **API**: `API-XX`
- **测试**: `TC-XXX-001 ~ TC-XXX-004`
- **埋点**: `EVT-XX`
- **负责人**: `{姓名}`

## 背景

{这个 issue 要解决什么问题，引用 IMPLEMENTATION-PLAN 和上游资源。}

## 目标

1. {目标 1}
2. {目标 2}

## 验收标准

- [ ] {验收项 1}
- [ ] {验收项 2}
- [ ] {验收项 3}

## 实现提示

- {关键实现点}
- {可能的技术坑}

## 相关资源

- `docs/PRD-v2.1.0.md#...`
- `docs/TDD-v2.1.0.md#...`
- `docs/ARCHITECTURE-v2.1.0.md#...`
```

---

## 5. 按 Sprint 初步分组建议

### Sprint 1（2026-07-07 ~ 2026-07-20）：脚手架 + 认证 + 对象存储

- DS-001 工程脚手架与项目初始化
- DS-002 用户认证、租户与 Workspace 模块
- DS-003 对象存储与后端签名 URL
- DS-026 前端质量收尾

### Sprint 2（2026-07-21 ~ 2026-08-03）：上传与解析

- DS-005 文档上传 API
- DS-006 PDF Pipeline（bbox + webp）
- DS-007 Office Pipeline（OnlyOffice 转 PDF）
- DS-008 数据库迁移与搜索索引
- DS-027 前端-后端集成层（契约层）

### Sprint 3（2026-08-04 ~ 2026-08-17）：Viewer 与 AI

- DS-009 签名 URL 与权限校验
- DS-010 Viewer Canvas 前端
- DS-011 Search Service
- DS-012 Evidence Service
- DS-013 Assistant Service

### Sprint 4（2026-08-18 ~ 2026-08-31）：智能、链接、Dashboard、AI 前端

- DS-014 悬浮 AI 助手前端
- DS-015 智能链接与权限
- DS-016 Dashboard 前端
- DS-017 热度评分与 Analytics

### Sprint 5（2026-09-01 ~ 2026-09-14）：分析、数据室、集成、测试

- DS-018 行为提醒与跟进建议
- DS-019 数据室模块
- DS-020 邮件通知系统
- DS-023 测试用例与自动化

### Sprint 6（2026-09-15 ~ 2026-09-28）：集成、压测、安全、发布

- DS-004 子域名/自定义域名与 SSL 自动签发（P1，可延后）
- DS-021 CRM 集成（HubSpot/Salesforce）
- DS-022 Slack 集成
- DS-024 性能压测与优化
- DS-025 安全扫描与修复

---

## 6. 检查清单

- [x] 所有 issue 可追溯到 TASK / PRD / TDD / API
- [x] 所有 issue 有优先级、类型、风险、依赖
- [x] issue 编号统一为 `DS-{NNN}`
- [x] 关键路径上的 P0 issue 已识别
- [x] 补齐了 v2.1.0 issue 清单中的覆盖缺口（DS-004、DS-014、DS-018、DS-023、DS-024、DS-025）
- [x] Sprint 分组与里程碑日期一致
- [x] 新增前端 v2.1.2 issue（DS-026、DS-027）并映射到真实 TASK

---

> **模板版本**：v1  
> **Issue 清单版本**：v2.1.2  
> **状态**：已批准  
> **最后更新**：2026-06-21

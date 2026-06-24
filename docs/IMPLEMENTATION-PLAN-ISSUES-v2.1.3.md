---
id: "IM-2024-003"
version: "v2.1.3"
status: "已批准"
owner: "技术负责人 / 项目经理"
linked_docs:
  - "docs/IMPLEMENTATION-PLAN-v2.1.3.md"
  - "docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md"
  - "docs/PRD-v2.1.0.md"
  - "docs/TDD-v2.1.0.md"
  - "docs/API-SPEC-v2.1.0.md"
  - "docs/ARCHITECTURE-v2.1.0.md"
  - "docs/database-model-v2.1.0.md"
  - "docs/HEAT-SCORE-ALGORITHM-v2.1.1.md"
  - "docs/templates/CODE-REVIEW-template-v1.md"
  - "docs/tasks/agent-tasks-v2.1.3/*.md"
---

# DealSignal v2.1.3 开发执行计划 Issue 拆分清单

> **资源编号**：`IM-2024-003`  
> **版本**：`v2.1.3`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`技术负责人 / 项目经理`  
> **编写日期**：`2026-06-24`  
> **关联资源**：
> - `docs/IMPLEMENTATION-PLAN-v2.1.3.md`
> - `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md`
> - `docs/tasks/agent-tasks-v2.1.3/*.md`
> - `docs/PROJECT-PROGRESS.md`
> **评审人**：`技术负责人、项目经理、产品负责人`
> **执行状态（IMPLEMENTATION-PLAN 专用）**：`执行中`

---

## 1. 资源说明

本资源是 `IMPLEMENTATION-PLAN-v2.1.3.md` 的下游拆分产物，用于把 v2.1.3 阶段的「前端审计优化 + 后端加固」进一步拆成可进入 Sprint、可分配、可跟踪、可验收的 issues/tickets。

**核心目标**：
- 每个 issue 只负责一个可独立合并的交付单元。
- 每个 issue 都能追溯到 `TASK / PRD / TDD / API / 测试用例`。
- issue 的版本、优先级、状态、风险、依赖一目了然。

**与 IMPLEMENTATION-PLAN 的关系**：

```text
IMPLEMENTATION-PLAN-v2.1.3（TASK 层）
        │
        ▼
ISSUE-MANIFEST v2.1.3（issue 层）
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
| `v2.1.3` | 前端阻塞项清零、API/算法对齐、数据层统一、后端改动落库、文档同步 |

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
| DS-028 | 前端阻塞按钮与即时反馈清零 | v2.1.3 | P0 | 待开始 | frontend | test_failure | - | TASK-FRONTEND-006 | FR-设置/AI/链接 | 6.3 | - | `{待分配}` |
| DS-029 | 表单提交反馈、删除确认与账户菜单 | v2.1.3 | P0 | 待开始 | frontend | test_failure | DS-028 | TASK-FRONTEND-007 | FR-设置/链接/文档 | 6.3 | - | `{待分配}` |
| DS-030 | 前端文案与中英混杂清理 | v2.1.3 | P1 | 待开始 | frontend | test_failure | - | TASK-FRONTEND-008 | FR-设置/文档/联系人 | 6.3 | - | `{待分配}` |
| DS-031 | API 请求层修复与真实后端适配 | v2.1.3 | P0 | 待开始 | frontend | build_failure | - | TASK-FRONTEND-009 | FR-API | 5.x | §2/§3 | `{待分配}` |
| DS-032 | heatScore topKeyPages 算法对齐 | v2.1.3 | P0 | 待开始 | frontend | test_failure | - | TASK-FRONTEND-010 | FR-10 | 6.8 | API-16 | `{待分配}` |
| DS-033 | 统一数据层与 oversized 组件拆分 | v2.1.3 | P1 | 待开始 | frontend | build_failure | DS-031 | TASK-FRONTEND-011 | FR-仪表板/文档/链接 | 6.3/11.2 | - | `{待分配}` |
| DS-034 | UI/UX 细节打磨 | v2.1.3 | P1 | 待开始 | frontend | test_failure | DS-033 | TASK-FRONTEND-012 | FR-设置/洞察/联系人 | 6.3/11.2 | - | `{待分配}` |
| DS-035 | 前端单元与组件测试补强 | v2.1.3 | P1 | 待开始 | test | test_failure | DS-031, DS-033 | TASK-FRONTEND-013 | - | 10.x | - | `{待分配}` |
| DS-036 | 后端未落库改动整理与接口稳定 | v2.1.3 | P0 | 待开始 | backend | build_failure | - | TASK-BACKEND-011 | FR-02/03/05/07 | 6.x/7.x | 全部 | `{待分配}` |
| DS-037 | 后端中间件与基础模块补全 | v2.1.3 | P0 | 待开始 | backend | build_failure | DS-036 | TASK-BACKEND-012 | FR-安全/性能 | 7.x | - | `{待分配}` |
| DS-038 | E2E 与契约测试 | v2.1.3 | P0 | 待开始 | test | test_failure | DS-031, DS-036 | TASK-TEST-003 | AC-01 ~ AC-32 | 10.x | - | `{待分配}` |
| DS-039 | v2.1.3 文档基线同步 | v2.1.3 | P1 | 待开始 | docs | build_failure | 功能开发完成 | TASK-DOCS-001 | - | - | - | `{待分配}` |

### 3.2 按版本分组

#### v2.1.3 — 前端审计优化 + 后端加固

| Issue ID | 标题 | 优先级 | 状态 | 负责人 |
|----------|------|--------|------|--------|
| DS-028 | 前端阻塞按钮与即时反馈清零 | P0 | 待开始 | `{待分配}` |
| DS-029 | 表单提交反馈、删除确认与账户菜单 | P0 | 待开始 | `{待分配}` |
| DS-030 | 前端文案与中英混杂清理 | P1 | 待开始 | `{待分配}` |
| DS-031 | API 请求层修复与真实后端适配 | P0 | 待开始 | `{待分配}` |
| DS-032 | heatScore topKeyPages 算法对齐 | P0 | 待开始 | `{待分配}` |
| DS-033 | 统一数据层与 oversized 组件拆分 | P1 | 待开始 | `{待分配}` |
| DS-034 | UI/UX 细节打磨 | P1 | 待开始 | `{待分配}` |
| DS-035 | 前端单元与组件测试补强 | P1 | 待开始 | `{待分配}` |
| DS-036 | 后端未落库改动整理与接口稳定 | P0 | 待开始 | `{待分配}` |
| DS-037 | 后端中间件与基础模块补全 | P0 | 待开始 | `{待分配}` |
| DS-038 | E2E 与契约测试 | P0 | 待开始 | `{待分配}` |
| DS-039 | v2.1.3 文档基线同步 | P1 | 待开始 | `{待分配}` |

---

## 4. Issue 详情模板

每个 issue 应保存为 `docs/tasks/issues-v2.1.3/issue-{NNN}-{slug}.md`。

```markdown
# DS-NNN {标题}

## 元数据

- **版本**: `v2.1.3`
- **优先级**: `P0`
- **状态**: `待开始`
- **类型**: `frontend`
- **风险**: `test_failure`
- **依赖**: `DS-028, DS-031`
- **关联 TASK**: `TASK-FRONTEND-00x`
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

## 测试策略

- 单元测试：`{覆盖哪些函数}`
- 集成测试：`{覆盖哪些接口}`
- E2E：`{覆盖哪些用户路径}`

## 回滚/风险

- {变更失败如何回滚}
- {对现有功能的影响}
```

---

## 5. 从 Issue 到 Agent Task（最小可执行单元）

> `DS-xxx` issue 是**功能级**单元；进入 AI 编码/开发前，应再拆成**一次 PR 可完成**的 `AGENT-TASK`。

### 5.1 拆分原则

- 一个 `AGENT-TASK` 只交付一个可独立合并的代码单元。
- 推荐粒度：一个 API endpoint、一张表的 migration、一个核心组件、一个工具函数补全。
- 如果一个 issue 预计超过 3~5 天或 400 行代码，必须拆分。

### 5.2 Agent Task 模板

详见 `docs/templates/AGENT-TASK-template-v2.md`。

### 5.3 示例映射

| 父 Issue | Agent Task | task_id | parent_issue | 说明 |
|----------|------------|---------|--------------|------|
| DS-028 | 前端阻塞按钮与即时反馈清零 | TASK-FRONTEND-006 | DS-028 | Security/Insights/TopNav 铃铛/SmartLinkCreator 复制反馈 |
| DS-029 | 表单提交反馈、删除确认与账户菜单 | TASK-FRONTEND-007 | DS-029 | Settings 表单 toast、Dialog 删除、TopNav 账户菜单 |
| DS-030 | 前端文案与中英混杂清理 | TASK-FRONTEND-008 | DS-030 | 移除 views/Deal Rooms/360° 视图等残留文案 |
| DS-031 | API 请求层修复与真实后端适配 | TASK-FRONTEND-009 | DS-031 | FormData Content-Type、token、idempotency、BaseResponse |
| DS-032 | heatScore topKeyPages 算法对齐 | TASK-FRONTEND-010 | DS-032 | 按算法文档修复 topKeyPages |
| DS-033 | 统一数据层与 oversized 组件拆分 | TASK-FRONTEND-011 | DS-033 | useAsyncData + 组件拆分 + 集中格式化 |
| DS-034 | UI/UX 细节打磨 | TASK-FRONTEND-012 | DS-034 | 空状态/键盘可达/移动端/视觉统一 |
| DS-035 | 前端单元与组件测试补强 | TASK-FRONTEND-013 | DS-035 | heatScore/formatters/calculations 单元测试 + 组件测试 |
| DS-036 | 后端未落库改动整理与接口稳定 | TASK-BACKEND-011 | DS-036 | 整理现有 backend diff，确保测试通过 |
| DS-037 | 后端中间件与基础模块补全 | TASK-BACKEND-012 | DS-037 | 限流/幂等中间件 + logger/mailer/redis 测试 |
| DS-038 | E2E 与契约测试 | TASK-TEST-003 | DS-038 | 真实后端 E2E + API 契约测试 |
| DS-039 | v2.1.3 文档基线同步 | TASK-DOCS-001 | DS-039 | API-SPEC/database-model/ARCHITECTURE/README 同步 |

---

## 6. 检查清单

- [x] 所有 issue 可追溯到 TASK / PRD / TDD / API
- [x] 所有 issue 有优先级、类型、风险、依赖
- [x] issue 编号统一为 `DS-{NNN}`，从 DS-028 连续递增
- [x] 关键路径上的 P0 issue 已识别
- [x] Sprint 分组与里程碑日期一致
- [x] 已补齐 v2.1.3 审计计划中的覆盖项

---

> **模板版本**：v1  
> **Issue 清单版本**：v2.1.3  
> **状态**：已批准  
> **最后更新**：2026-06-24

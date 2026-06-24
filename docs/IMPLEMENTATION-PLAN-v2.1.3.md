---
id: "IP-2024-003"
version: "v2.1.3"
status: "已批准"
owner: "技术负责人 / 项目经理"
linked_docs:
  - "docs/PRD-v2.1.0.md"
  - "docs/TDD-v2.1.0.md"
  - "docs/API-SPEC-v2.1.0.md"
  - "docs/ARCHITECTURE-v2.1.0.md"
  - "docs/database-model-v2.1.0.md"
  - "docs/HEAT-SCORE-ALGORITHM-v2.1.1.md"
  - "docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md"
  - "docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.3.md"
  - "docs/tasks/agent-tasks-v2.1.3/*.md"
---

# DealSignal v2.1.3 实施计划

> **文档编号**：`IP-2024-003`  
> **版本**：`v2.1.3`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`技术负责人 / 项目经理`  
> **编写日期**：`2026-06-24`  
> **关联资源**：
> - `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md`
> - `docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.3.md`
> - `docs/tasks/agent-tasks-v2.1.3/*.md`
> - `docs/PROJECT-PROGRESS.md`
> **评审人**：`CTO、技术负责人、产品经理、测试负责人`

---

## 1. 当前状态

- **v2.1.2 已发布**：2026-06-22 完成并打 tag `v2.1.2`。
- **v2.1.3 工作区已预热**：`main` 领先 tag 14 个 commit，存在 72 个修改文件 + 35 个未跟踪文件，主要集中在前端 API 层、上传/Viewer、后端 ingestion/integration/search/workspace 等模块。
- **文档滞后**：`docs/` 目录当前无 diff，API-SPEC、database-model、ARCHITECTURE、README 与代码现状存在偏差。

本计划目标是把 `FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md` 中的阻塞项与工程债务拆成可执行任务，完成 v2.1.3 发布。

---

## 2. 目标

1. **清零前端阻塞项**：所有可见按钮必须有响应或明确不可用说明；复制、保存、删除等操作提供统一即时反馈。
2. **修复 API 层与算法偏差**：`request` 支持 FormData；路径/认证/响应格式向 API-SPEC 对齐；`heatScore` `topKeyPages` 按算法文档实现。
3. **统一数据层与拆分组件**：引入 `useAsyncData` 减少重复 fetch 样板；拆分 oversized 组件；集中格式化/计算逻辑。
4. **补齐测试与文档**：为核心工具函数补单元测试；更新 API-SPEC/database-model/ARCHITECTURE/README；完成 E2E 与契约测试。
5. **稳定后端未落库改动**：整理并提交当前后端 diff，补全新增中间件与基础模块的测试。

---

## 3. 范围

### 3.1 In Scope

- Phase A：无响应按钮、复制反馈、表单提交反馈、删除确认 Dialog、账户菜单、Brand Logo 持久化、文案清理。
- Phase B：`api.ts` / `apiClient.ts` FormData 与认证适配、`heatScore.ts` 算法修复。
- Phase C：`useAsyncData` 统一数据层、组件拆分、重复逻辑集中化、单元测试。
- Phase D：空状态统一、键盘可达、移动端适配、视觉一致性。
- 后端：落库现有改动、补全新中间件（限流/幂等）/基础模块（logger/mailer/redis）测试。
- 文档：同步 API-SPEC、database-model、ARCHITECTURE、README、PROJECT-PROGRESS。

### 3.2 Out of Scope / Deferred

- 全新业务功能（如计费、审计日志详情页、高级 AI agent 工具调用）。
- 大规模重构未在审计计划中列出的模块。
- 自定义域名/CNAME/SSL 的进一步自动化（已在 v2.1.2 完成基础能力）。

---

## 4. 任务总览

### 4.1 前端任务

| Task ID | 标题 | 优先级 | 工作量 | Parent Issue | 依赖 |
|---|---|---|---|---|---|
| TASK-FRONTEND-006 | 前端阻塞按钮与即时反馈清零 | P0 | S | DS-028 | - |
| TASK-FRONTEND-007 | 表单提交反馈、删除确认与账户菜单 | P0 | M | DS-029 | TASK-FRONTEND-006 |
| TASK-FRONTEND-008 | 前端文案与中英混杂清理 | P1 | S | DS-030 | - |
| TASK-FRONTEND-009 | API 请求层修复与真实后端适配 | P0 | M | DS-031 | - |
| TASK-FRONTEND-010 | heatScore topKeyPages 算法对齐 | P0 | S | DS-032 | - |
| TASK-FRONTEND-011 | 统一数据层与 oversized 组件拆分 | P1 | L | DS-033 | TASK-FRONTEND-009 |
| TASK-FRONTEND-012 | UI/UX 细节打磨 | P1 | M | DS-034 | TASK-FRONTEND-011 |
| TASK-FRONTEND-013 | 前端单元与组件测试补强 | P1 | M | DS-035 | TASK-FRONTEND-009 / 011 |

### 4.2 后端任务

| Task ID | 标题 | 优先级 | 工作量 | Parent Issue | 依赖 |
|---|---|---|---|---|---|
| TASK-BACKEND-011 | 后端未落库改动整理与接口稳定 | P0 | L | DS-036 | - |
| TASK-BACKEND-012 | 后端中间件与基础模块补全 | P0 | M | DS-037 | TASK-BACKEND-011 |

### 4.3 测试与文档任务

| Task ID | 标题 | 优先级 | 工作量 | Parent Issue | 依赖 |
|---|---|---|---|---|---|
| TASK-TEST-003 | E2E 与契约测试 | P0 | M | DS-038 | TASK-FRONTEND-009 / TASK-BACKEND-011 |
| TASK-DOCS-001 | v2.1.3 文档基线同步 | P1 | M | DS-039 | 功能开发完成 |

---

## 5. 执行顺序与依赖

```text
Sprint 1（阻塞项清零）
├── TASK-FRONTEND-006  前端阻塞按钮与即时反馈清零
├── TASK-FRONTEND-007  表单提交反馈、删除确认与账户菜单
└── TASK-FRONTEND-008  前端文案与中英混杂清理

Sprint 2（API/算法/数据层）
├── TASK-FRONTEND-009  API 请求层修复与真实后端适配
├── TASK-FRONTEND-010  heatScore topKeyPages 算法对齐
└── TASK-BACKEND-011   后端未落库改动整理与接口稳定

Sprint 3（架构与测试）
├── TASK-FRONTEND-011  统一数据层与 oversized 组件拆分
├── TASK-FRONTEND-013  前端单元与组件测试补强
└── TASK-BACKEND-012   后端中间件与基础模块补全

Sprint 4（体验、E2E、文档、发布）
├── TASK-FRONTEND-012  UI/UX 细节打磨
├── TASK-TEST-003      E2E 与契约测试
└── TASK-DOCS-001      v2.1.3 文档基线同步
```

### 关键路径

```text
TASK-FRONTEND-006 → TASK-FRONTEND-007
        │
        ▼
TASK-FRONTEND-009 ────────────────────→ TASK-FRONTEND-011 → TASK-FRONTEND-012
        │                                      │
        ▼                                      ▼
TASK-FRONTEND-010                    TASK-FRONTEND-013
        │
        ▼
TASK-BACKEND-011 → TASK-BACKEND-012
        │
        ▼
TASK-TEST-003 → TASK-DOCS-001
```

---

## 6. 验收标准

- [ ] 前端 `pnpm lint && pnpm test && pnpm build` 全绿；新增测试覆盖核心工具函数与关键组件。
- [ ] 后端 `make lint && make test && make build` 全绿；`docker compose up --build` 可启动。
- [ ] 0 个既无 `onClick` 也无 `disabled` 的可见按钮。
- [ ] 0 处 `window.confirm`；所有删除/危险操作使用 shadcn Dialog。
- [ ] 所有 clipboard 调用统一走 `copyToClipboard` 并带 toast/图标反馈。
- [ ] `request` 支持 FormData；未配置 `Content-Type: application/json` 时不上传失败。
- [ ] `heatScore.ts` `topKeyPages` 与 `HEAT-SCORE-ALGORITHM-v2.1.1.md` 一致。
- [ ] 后端新增中间件/模块均有单元测试。
- [ ] 全量文档（API-SPEC/database-model/ARCHITECTURE/README/PROJECT-PROGRESS）与代码一致。
- [ ] E2E 覆盖登录、上传、Viewer、链接创建、Dashboard 等 P0 路径。

---

## 7. 风险与应对

| 风险 | 影响 | 应对 |
|---|---|---|
| 当前未落库 diff 过大 | 拆分/提交困难，易冲突 | 由 TASK-BACKEND-011 和 TASK-FRONTEND-009 先整理主干，再按任务切分支 |
| API 路径/响应最终方案未定 | 前端集成反复 | TASK-FRONTEND-009 必须输出明确迁移方案，TASK-TEST-003 用契约测试锁定 |
| 组件拆分引入回归 | UI 行为异常 | 每拆一个组件补一次测试，先复制再替换 |
| 文档同步耗时 | 阻塞发布 | TASK-DOCS-001 与功能开发并行，每完成一个任务同步相关文档 |

---

## 8. 文档同步清单

- [ ] `docs/API-SPEC-v2.1.0.md`：路径、认证、错误码、响应 envelope、缺失端点。
- [ ] `docs/database-model-v2.1.0.md`：新迁移字段、中间件/基础模块依赖的表。
- [ ] `docs/ARCHITECTURE-v2.1.0.md`：新增中间件、logger/mailer/redis 位置。
- [ ] `docs/README.md`：安装命令改为 `pnpm`，补充 v2.1.3 快速链接。
- [ ] `docs/PROJECT-PROGRESS.md`：随任务完成逐项更新状态。
- [ ] `docs/CHANGELOG.md`：v2.1.3 发布时补充变更条目。

---

## 9. 实际落地备注（§10 预留）

> 本计划在执行过程中产生的偏差、范围变更、技术决策将记录于此。

---

> **模板版本**：v1  
> **实施计划版本**：v2.1.3  
> **状态**：已批准  
> **最后更新**：2026-06-24

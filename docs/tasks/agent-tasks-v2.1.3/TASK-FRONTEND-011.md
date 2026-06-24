---
task_id: "TASK-FRONTEND-011"
parent_issue: "DS-033"
agent_task_id: "AGENT-TASK-033"
version: "v2.1.3"
priority: "P1"
status: "待执行"
type: "frontend"
effort: "L"
branch: "feat/agent-task-033-data-layer"
estimated_files: "12"
max_lines: "800"
project_stack: "React 19 + TypeScript + Vite 8 + TanStack Query / Zustand"
ai_red_flags:
  - "不得一次替换所有组件，必须逐步验证"
  - "拆分后必须保持现有 UI 行为一致"
  - "集中化函数不得引入副作用"
  - "敏感数据不得发送给 LLM"
ai_confidence: "medium"
pending_confirmation:
  - "是否引入 TanStack Query 作为统一数据层，还是用本地 useAsyncData hook"
  - "SmartLinkCreator 拆分子组件的命名约定"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

# TASK-FRONTEND-011 统一数据层与 oversized 组件拆分

> **父 Issue**：`DS-033`

---

## 1. 目标

- 引入 `useAsyncData` hook（或 TanStack Query 封装）替代 18+ 处重复 fetch 样板。
- 拆分 oversized 组件：`SmartLinkCreator`、`DocumentsTable`、`DocumentDetail`、`DashboardPage`。
- 集中格式化/计算逻辑：`formatShortDate`、`getActivityLabel`、`groupLogsByDay`。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 前端审计计划 | `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md` §2.3 / §3 Phase C |
| PRD | `docs/PRD-v2.1.0.md` §8.2、§11.2 |
| 父 Issue | `DS-033` |
| 依赖 | `DS-031`（TASK-FRONTEND-009） |

### 2.1 已有代码

- `apps/web/src/components/links/SmartLinkCreator.tsx`（~486 行）
- `apps/web/src/components/documents/DocumentsTable.tsx`（~317 行）
- `apps/web/src/components/documents/DocumentDetail.tsx`（~311 行）
- `apps/web/src/routes/dashboard.tsx`（~322 行）
- `apps/web/src/lib/formatters.ts`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| hook 接口 | 稳定 | `useAsyncData(queryFn, deps)` 返回 `{ data, loading, error, refetch, cancel }` |
| 组件拆分 | 行为一致 | 拆分后 UI、状态、事件不变 |
| 工具函数 | 无副作用 | 纯函数，集中放在 `lib/formatters.ts` / `lib/calculations.ts` |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/hooks/useAsyncData.ts` | 新增 | 统一数据获取 hook |
| `src/lib/calculations.ts` | 新增/修改 | 日趋势聚合、事件映射等 |
| `src/lib/formatters.ts` | 修改 | 增加 `formatShortDate`、`getActivityLabel` |
| `components/links/SmartLinkCreator.tsx` | 修改/拆分 | 拆出 PermissionPanel、SecurityOptions、ScoreDisplay、LinkPreview |
| `components/documents/DocumentsTable.tsx` | 修改/拆分 | 拆出 `columns.tsx` |
| `components/documents/DocumentDetail.tsx` | 修改/拆分 | 拆出 aggregateVisitors、heatDistribution、LinksList |
| `routes/dashboard.tsx` | 修改 | 使用 useAsyncData |
| `routes/documents.tsx` | 修改 | 使用 useAsyncData |
| `routes/links.tsx` | 修改 | 使用 useAsyncData |
| `routes/contacts.tsx` | 修改 | 使用 useAsyncData |
| `routes/deal-rooms.tsx` | 修改 | 使用 useAsyncData |

---

## 5. 验收标准

- [ ] `useAsyncData` hook 稳定可用，覆盖至少 50% 重复 fetch 样板。
- [ ] `SmartLinkCreator` / `DocumentsTable` / `DocumentDetail` / `DashboardPage` 拆分完成。
- [ ] `formatShortDate`、`getActivityLabel`、`groupLogsByDay` 集中且无重复实现。
- [ ] 现有页面行为无回归；`pnpm test` 全绿。
- [ ] 拆分出的子组件有基本单元测试或至少通过现有 E2E。

---

## 6. 实现步骤建议

1. 实现 `useAsyncData` hook 并补测试。
2. 优先在 DashboardPage 替换，验证稳定。
3. 依次替换 Documents/Links/Contacts/DealRooms 列表页。
4. 拆分 `DocumentsTable` 的 columns。
5. 拆分 `DocumentDetail` 的子模块。
6. 拆分 `SmartLinkCreator` 的子组件。
7. 运行全量测试。

---

## 7. 测试验证

```bash
cd apps/web
pnpm test useAsyncData
pnpm test DocumentsTable
pnpm test DocumentDetail
pnpm test SmartLinkCreator
pnpm test
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 不要一次改动所有列表页；每替换一个必须跑通测试。
- 拆分不得改变现有 props 接口（可内部转发）。
- 不要引入未使用的抽象；保持简单。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-033`

---

## 10. Agent 备注

- 如果团队已使用 TanStack Query，建议直接基于它封装 `useAsyncData`，避免重复造轮子。
- 拆分 oversized 组件时，先复制旧组件为子组件，再在原组件中引用，降低回归风险。

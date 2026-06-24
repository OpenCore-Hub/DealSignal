---
task_id: "TASK-FRONTEND-012"
parent_issue: "DS-034"
agent_task_id: "AGENT-TASK-034"
version: "v2.1.3"
priority: "P1"
status: "待执行"
type: "frontend"
effort: "M"
branch: "feat/agent-task-034-ui-ux-polish"
estimated_files: "10"
max_lines: "400"
project_stack: "React 19 + TypeScript + Vite 8 + Tailwind CSS 4 + shadcn/ui + Phosphor Icons"
ai_red_flags:
  - "不得破坏现有交互行为"
  - "空状态组件必须可复用"
  - "键盘可达不得依赖鼠标事件"
  - "敏感数据不得发送给 LLM"
ai_confidence: "high"
pending_confirmation:
  - "EmptyState 组件的插画/图标风格（Phosphor vs 自定义）"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

# TASK-FRONTEND-012 UI/UX 细节打磨

> **父 Issue**：`DS-034`

---

## 1. 目标

统一空状态、补齐键盘可达性、优化移动端适配、收敛圆角/阴影等视觉细节。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 前端审计计划 | `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md` §2.4 / §3 Phase D |
| 设计令牌 | `docs/DESIGN-TOKENS-v2.1.1.md` |
| 父 Issue | `DS-034` |
| 依赖 | `DS-033`（TASK-FRONTEND-011） |

### 2.1 已有代码

- `apps/web/src/routes/insights/pages.tsx`
- `apps/web/src/components/documents/DocumentContent.tsx`
- `apps/web/src/routes/contacts/detail.tsx`
- `apps/web/src/components/insights/HeatMap.tsx`
- `apps/web/src/components/ai/AIAssistant.tsx`
- `apps/web/src/components/ai/AIChat.tsx`
- `apps/web/src/components/layout/Sidebar.tsx`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 空状态 | 统一 `EmptyState` 组件 | 不内联、不使用 emoji |
| 键盘可达 | Tab/Enter/Space | 所有可点击元素 |
| 移动端 | 不固定死高度 | AI 面板使用 `max-h-[calc(100dvh-2rem)]` |
| 视觉 | 统一 rounded-xl + shadow-lg | AI 面板一致 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/components/common/EmptyState.tsx` | 新增 | 可复用空状态 |
| `src/routes/insights/pages.tsx` | 修改 | emoji 改 Phosphor 图标 |
| `src/components/documents/DocumentContent.tsx` | 修改 | 无 pageCount 空态 |
| `src/routes/contacts/detail.tsx` | 修改 | 空态与键盘可达 |
| `src/components/insights/HeatMap.tsx` | 修改 | 键盘操作 |
| `src/components/ai/AIAssistant.tsx` | 修改 | 高度、圆角、阴影 |
| `src/components/ai/AIChat.tsx` | 修改 | 高度、圆角、阴影 |
| `src/components/layout/Sidebar.tsx` | 修改 | 折叠态 Tooltip |
| 各表格组件 | 修改 | `min-w-[640px]` 保证小屏滚动 |

---

## 5. 验收标准

- [ ] 所有空状态使用统一 `EmptyState` 组件，无 emoji。
- [ ] HeatMap 项与 ContactDetail 文档列表可键盘操作。
- [ ] AIAssistant/AIChat 高度在移动端适配。
- [ ] 表格在小屏下可横向滚动。
- [ ] AI 面板圆角/阴影一致。
- [ ] Sidebar 折叠态图标有 Tooltip。
- [ ] `pnpm lint && pnpm test` 全绿。

---

## 6. 实现步骤建议

1. 创建 `EmptyState` 组件。
2. 替换各页面内联空态。
3. 为 HeatMap / ContactDetail 列表项加 `role`/`tabIndex` 或改为 `Link`。
4. 调整 AI 面板高度与圆角。
5. 为表格容器加 `overflow-x-auto` 与 `min-w`。
6. 为 Sidebar 折叠图标加 Tooltip。

---

## 7. 测试验证

```bash
cd apps/web
pnpm test EmptyState
pnpm test HeatMap
pnpm test Sidebar
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 不要为视觉统一而破坏现有响应式布局。
- 键盘事件必须触发与 click 相同的动作。
- 不要引入新的未使用图标库。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-034`

---

## 10. Agent 备注

- `EmptyState` 建议支持 `icon`、`title`、`description`、`action` props。
- 移动端高度优先使用 `dvh` 单位，避免 `100vh` 在 Safari 地址栏收缩时异常。

---
task_id: "TASK-FRONTEND-006"
parent_issue: "DS-028"
agent_task_id: "AGENT-TASK-028"
version: "v2.1.3"
priority: "P0"
status: "已完成"
type: "frontend"
effort: "S"
branch: "feat/agent-task-028-blocker-buttons"
estimated_files: "8"
max_lines: "300"
project_stack: "React 19 + TypeScript + Vite 8 + Tailwind CSS 4 + shadcn/ui + Zustand"
ai_red_flags:
  - "不得引入新的无响应按钮"
  - "不得破坏现有 toast/复制反馈行为"
  - "禁用按钮必须提供 Tooltip 或 title 说明"
  - "敏感数据不得发送给 LLM"
ai_confidence: "high"
pending_confirmation:
  - "SmartLinkCreator 复制成功后期望的 toast 文案与图标状态"
  - "Security/Insights 按钮 disabled 时的提示文案是否可接受占位说明"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-FRONTEND-006` |
> | `parent_issue` | `DS-028` |
> | **版本** | `v2.1.3` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `待执行` |
> | **类型** | `frontend` |
> | **预计工作量** | `S` |
> | **分支名** | `feat/agent-task-028-blocker-buttons` |
> | **预计修改文件数上限** | `8` |
> | **建议最大变更行数** | `300` |
> | **AI 置信度** | `high` |
> | **待人工确认事项** | 文案与图标状态 |

# TASK-FRONTEND-006 前端阻塞按钮与即时反馈清零

> **父 Issue**：`DS-028`  
> **版本**：`v2.1.3`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`待执行`  
> **类型**：`frontend`  
> **预计工作量**：`S`  
> **分支名**：`feat/agent-task-028-blocker-buttons`

---

## 1. 目标

消除前端「无响应按钮」：为 `Security` 2FA 配置、`CreateLinkSheet` 管理/预览按钮补充 `disabled` + title；为 `CreateLinkSheet` 复制按钮增加图标状态反馈；`Security` 审计日志、`InsightsSuggestions` 写跟进邮件、`TopNav` 通知铃铛此前已完成 title 提示，本次补齐 i18n 与剩余缺口。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 前端审计计划 | `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md` §2.1 / §3 Phase A |
| PRD | `docs/PRD-v2.1.0.md` §11.2（Settings/Insights） |
| TDD | `docs/TDD-v2.1.0.md` §6.3 |
| 父 Issue | `DS-028` |

### 2.1 已有代码

- `apps/web/src/routes/settings/security.tsx:139`
- `apps/web/src/routes/insights/suggestions.tsx:94-97`
- `apps/web/src/components/layout/TopNav.tsx:41-45`
- `apps/web/src/components/deal-rooms/CreateLinkSheet.tsx`（SmartLinkCreator 在当前代码库中已不存在，对应复制/管理/预览按钮在 CreateLinkSheet 中处理）
- `apps/web/src/lib/formatters.ts`（含 `copyToClipboard`）

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 可见按钮 | 必须有 `onClick` 或 `disabled` | 不允许两者皆无 |
| disabled 按钮 | 必须有提示 | 通过 shadcn `Tooltip` 或原生 `title` |
| 复制操作 | 必须走 `copyToClipboard` | 统一 toast 反馈与图标状态 |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| Security 审计日志按钮无响应 | 页面渲染后点击 | 显示 disabled + Tooltip「审计日志需后端支持」 |
| Insights 写跟进邮件无响应 | 页面渲染后点击 | 显示 disabled + Tooltip「邮件发送需后端支持」 |
| TopNav 铃铛无响应 | 页面渲染后点击 | 显示 disabled + Tooltip「通知中心即将上线」 |
| SmartLinkCreator 复制无反馈 | 点击复制 | 出现 toast + 图标变为已复制状态，2s 后恢复 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `routes/settings/security.tsx` | 修改 | 2FA 配置按钮 disabled + title |
| `components/deal-rooms/CreateLinkSheet.tsx` | 修改 | 管理/预览按钮 disabled + title；复制按钮增加图标反馈 |
| `routes/insights/suggestions.tsx` | 无需修改 | 写跟进邮件按钮已具备 disabled + title |
| `components/layout/TopNav.tsx` | 无需修改 | 通知铃铛已具备 disabled + title |
| `components/ui/TooltipButton.tsx`（可选） | 新增 | 封装 disabled + Tooltip 按钮 |

### 4.2 行为定义

- 所有目标按钮在 disabled 时仍保持可见样式，hover 显示原因。
- `copyToClipboard` 成功后显示 sonner toast「链接已复制」，图标切换为 `Check`，2 秒后恢复。

---

## 5. 验收标准

- [x] 0 个既无 `onClick` 也无 `disabled` 的可见 Button/菜单项。
- [x] Security/Insights/TopNav 铃铛均有 disabled + 明确提示。
- [x] CreateLinkSheet 复制后 toast + 图标反馈正确。
- [x] `pnpm lint && pnpm test` 全绿。
- [x] 无新增 `console.log` 或 TODO。

---

## 6. 实现步骤建议

1. 搜索项目中所有 `<Button` / `<button` 渲染点，确认无遗漏无响应项。
2. 为 Security/Insights/TopNav 铃铛添加 `disabled` + `Tooltip`/`title`。
3. 在 SmartLinkCreator 中用 `copyToClipboard` 替换 `navigator.clipboard.writeText`，增加本地 `copied` 状态。
4. 运行 lint 与相关测试。

---

## 7. 测试验证

```bash
cd apps/web
pnpm lint
pnpm test SmartLinkCreator
pnpm test --run TopNav
```

---

## 8. 约束与红线

- 不得引入新的无响应按钮。
- disabled 必须有明确原因提示，不能让用户困惑。
- 复制反馈必须复用现有 `copyToClipboard`，不重复实现。
- 保持现有路由与组件接口不变。

---

## 9. Definition of Done

- [x] 代码实现完成
- [x] 测试通过
- [x] lint / typecheck 通过
- [x] 与父 Issue 验收标准对齐
- [x] PR 已关联父 Issue：`Closes #DS-028`（PR #89）

---

## 10. Agent 备注

- 如果 shadcn `Tooltip` 在 disabled button 上无法触发，可用 `<span>` 包裹 Button 再套 Tooltip，或回退到原生 `title`。
- 图标反馈建议用 `Check` / `Copy` 图标对，2s 后自动恢复。

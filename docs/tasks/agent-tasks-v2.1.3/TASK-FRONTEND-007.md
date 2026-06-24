---
task_id: "TASK-FRONTEND-007"
parent_issue: "DS-029"
agent_task_id: "AGENT-TASK-029"
version: "v2.1.3"
priority: "P0"
status: "待执行"
type: "frontend"
effort: "M"
branch: "feat/agent-task-029-forms-delete-account-menu"
estimated_files: "10"
max_lines: "500"
project_stack: "React 19 + TypeScript + Vite 8 + Tailwind CSS 4 + shadcn/ui + Zustand"
ai_red_flags:
  - "不得使用 window.confirm"
  - "所有 async 提交必须有 try/catch + toast"
  - "不得泄露敏感错误信息到 toast"
  - "敏感数据不得发送给 LLM"
ai_confidence: "high"
pending_confirmation:
  - "删除 Dialog 的确认文案（单文档 / 链接）"
  - "账户菜单占位项文案与 disabled 原因"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

# TASK-FRONTEND-007 表单提交反馈、删除确认与账户菜单

> **父 Issue**：`DS-029`

---

## 1. 目标

为 SettingsGeneral/Brand/Integrations 等表单的保存/切换操作提供统一的 `try/catch + toast` 反馈；将 `DocumentDetail` 与 `LinksTable` 的 `window.confirm` 删除替换为 shadcn Dialog；为 `TopNav` 增加真实头像/账户下拉菜单（占位项 disabled + title）。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 前端审计计划 | `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md` §2.2 / §3 Phase A |
| PRD | `docs/PRD-v2.1.0.md` §6.3、§11.2 |
| 父 Issue | `DS-029` |

### 2.1 已有代码

- `apps/web/src/routes/settings/general.tsx`
- `apps/web/src/routes/settings/brand.tsx`
- `apps/web/src/routes/settings/integrations.tsx`
- `apps/web/src/components/documents/DocumentDetail.tsx`
- `apps/web/src/components/links/LinksTable.tsx`
- `apps/web/src/components/layout/TopNav.tsx`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 删除确认 | 必须使用 shadcn Dialog | 0 处 `window.confirm` |
| 表单提交 | 必须 try/catch + toast | 成功/失败均反馈 |
| 账户菜单 | 头像显示 workspace 首字母 | 菜单项暂 disabled |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 设置保存失败 | API 500 | toast 显示「保存失败：{message}」 |
| 删除文档取消 | 点击 Dialog 取消 | 不调用删除 API，无 toast |
| 删除文档确认 | 点击 Dialog 确认 | 调用删除 API，成功后 toast 并跳转 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `routes/settings/general.tsx` | 修改 | 保存反馈与错误处理 |
| `routes/settings/brand.tsx` | 修改 | 保存反馈与错误处理 |
| `routes/settings/integrations.tsx` | 修改 | 切换/保存反馈与错误处理 |
| `components/documents/DocumentDetail.tsx` | 修改 | 删除改用 Dialog |
| `components/links/LinksTable.tsx` | 修改 | 删除改用 Dialog |
| `components/layout/TopNav.tsx` | 修改 | 头像首字母 + 账户下拉菜单 |
| `components/layout/AccountMenu.tsx`（可选） | 新增 | 账户菜单抽离 |

---

## 5. 验收标准

- [ ] 0 处 `window.confirm`。
- [ ] SettingsGeneral/Brand/Integrations 保存/切换均有成功/失败 toast。
- [ ] DocumentDetail/LinksTable 删除使用 shadcn Dialog 二次确认。
- [ ] TopNav 头像显示当前 workspace 首字母（或用户邮箱首字母），下拉菜单占位项 disabled + title。
- [ ] `pnpm lint && pnpm test` 全绿。

---

## 6. 实现步骤建议

1. 全局搜索 `window.confirm`，全部替换为 `AlertDialog`。
2. 为三个 Settings 表单统一封装 `onSubmit` 包装器：try/catch + sonner toast。
3. 在 `TopNav` 读取 `currentWorkspace.name` 首字母作为头像；增加 `DropdownMenu` 账户菜单。
4. 运行相关测试与 lint。

---

## 7. 测试验证

```bash
cd apps/web
pnpm lint
pnpm test DocumentDetail
pnpm test LinksTable
pnpm test TopNav
```

---

## 8. 约束与红线

- 严禁 `window.confirm`。
- toast 错误文案不得暴露堆栈或敏感信息。
- 账户菜单占位项不能无响应。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-029`

---

## 10. Agent 备注

- 若 workspace 名称取首字母出现 emoji 或空字符，可回退到默认「我」。
- 删除 Dialog 建议使用 `AlertDialog` 组件，危险按钮用 `variant="destructive"`。

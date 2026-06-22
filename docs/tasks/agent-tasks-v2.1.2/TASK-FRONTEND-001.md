---
task_id: "TASK-FRONTEND-001"
parent_issue: "DS-026"
agent_task_id: "AGENT-TASK-001"
version: "v2.1.2"
priority: "P1"
status: "已完成"
type: "frontend"
effort: "S"
branch: "feat/agent-task-001-frontend-quality"
estimated_files: "8"
max_lines: "400"
project_stack: "React 19 / React Router 8 / Vite 8 / TypeScript / Tailwind CSS 4 / Base UI / i18next / Vitest"
ai_red_flags:
  - "不得修改范围外文件"
  - "不得破坏现有 53 个测试"
  - "不得引入新的 console 警告"
  - "测试数据不得使用真实域名/邮箱"
ai_confidence: "high"
pending_confirmation: []
available_tools:
  - "test"
  - "lint"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-FRONTEND-001` |
> | `parent_issue` | `DS-026` |
> | `agent_task_id` | `AGENT-TASK-001` |
> | **版本** | `v2.1.2` |
> | **模板版本** | `v2` |
> | **优先级** | `P1` |
> | **状态** | `待执行` |
> | **类型** | `frontend` |
> | **预计工作量** | `S` |
> | **分支名** | `feat/agent-task-001-frontend-quality` |
> | **AI 置信度** | `high` |
> | **依赖** | `-` |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint` |

# TASK-FRONTEND-001 前端质量收尾：act 警告、AI 关键词、workspace settings name

> **父 Issue**：`DS-026`  
> **版本**：`v2.1.2`  
> **模板版本**：`v2`  
> **优先级**：`P1`  
> **状态**：`待执行`  
> **类型**：`frontend`  
> **预计工作量**：`S`  
> **分支名**：`feat/agent-task-001-frontend-quality`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

清理 v2.1.1 遗留的前端不一致项：消除测试 `act(...)` 警告、将 AI 自动回复关键词/建议默认改为英文、让 `workspaceSettings.name` 可配置而不再硬编码为 "Acme Capital"；同时统一 mock handler 响应结构，为接入 `BaseResponse` 做准备。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §11 Settings、§6.3 Workspace 切换 |
| TDD | `docs/TDD-v2.1.0.md` §6.3 Viewer Frontend |
| ARCHITECTURE | `docs/ARCHITECTURE-v2.1.0.md` §3 Frontend |
| 父 Issue | `DS-026` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 本任务范围小，可直接加载全部相关文件与测试。 |
| **必读顺序** | front matter → `ai_red_flags` → 目标 → 边界条件 → 输出。 |
| **分块策略** | 不需要分块；一次读取相关源文件和测试即可。 |

### 3.1 已有代码/表（执行前必须阅读）

- `apps/web/src/components/layout/WorkspaceSwitcher.test.tsx`
- `apps/web/src/components/layout/WorkspaceSwitcher.tsx`
- `apps/web/src/stores/aiStore.ts`
- `apps/web/src/lib/mocks/handlers.ts`
- `apps/web/src/i18n/locales/en/ai.json`
- `apps/web/src/i18n/locales/zh-CN/ai.json`

### 3.2 数据模型/接口

```typescript
// AI 建议项
interface AiSuggestion {
  text: string;
  confidence: number;
}

// workspaceSettings mock 结构
interface WorkspaceSettings {
  name: string;
  slug: string;
  brandColor: string;
  viewerDomain: string;
  logoUrl: string;
}
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 测试稳定性 | 53 个测试全部通过 | 不得引入新失败 |
| 语言一致性 | 默认 `en` | AI 关键词与建议文案优先英文 |
| 可编辑字段 | `workspaceSettings.name` 为 plain text | 不改为 i18n key，但改为可配置默认值 |
| 最大变更行数 | ≤ 400 | 超出需拆分 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| AI 建议中文 | 命中中文关键词 | 默认返回英文 suggestion |
| 测试 act 警告 | 运行 `pnpm test` | 无 `act(...)` 警告 |
| workspace name 硬编码 | Settings 页面加载 | 显示来自 mock 的默认值，可被更新覆盖 |
| lint 失败 | `pnpm lint` | 0 errors |

### 3.5 测试数据 / Mock / Fixture

```json
{
  "mockWorkspaceSettings": {
    "name": "Demo Workspace",
    "slug": "demo-workspace",
    "brandColor": "#0f172a",
    "viewerDomain": "",
    "logoUrl": ""
  },
  "mockUser": {
    "email": "user-001@example.test"
  }
}
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/web/src/components/layout/WorkspaceSwitcher.test.tsx` | 修改 | 消除 `act(...)` 警告 |
| `apps/web/src/stores/aiStore.ts` | 修改 | AI 关键词与 suggestion 改为英文 |
| `apps/web/src/lib/mocks/handlers.ts` | 修改 | `workspaceSettings.name` 改为从 query/seed 读取，支持更新 |
| `apps/web/src/i18n/locales/en/ai.json` | 修改 | 补充 suggestion 英文文案 |
| `apps/web/src/i18n/locales/zh-CN/ai.json` | 修改 | 保留中文翻译 |

### 4.2 行为定义

- `pnpm test` 输出干净，无 React `act(...)` 警告。
- AI store 默认匹配英文关键词并返回英文 suggestion；中文仅作为辅助或保留在 zh-CN 翻译中。
- Mock `workspaceSettings.name` 默认使用 "Demo Workspace" 或 seed 值，且 `PATCH /api/workspace/settings` 可更新它。

---

## 5. 验收标准

- [x] `pnpm test` 无 `act(...)` 警告
- [x] `pnpm lint` 0 errors
- [x] `pnpm build` 成功
- [x] AI 助手 suggestion 默认返回英文
- [x] Settings 页面 workspace name 不再是写死 "Acme Capital"
- [x] 现有 75 个测试全部通过

---

## 6. 实现步骤建议

1. 阅读 `WorkspaceSwitcher.test.tsx`，定位 `act` 警告来源（通常是异步状态更新未包裹 `waitFor`）。
2. 修复测试，确保所有交互和异步断言使用 `waitFor` / `findBy*`。
3. 打开 `aiStore.ts`，将关键词映射和建议文案改为英文，必要时拆分到 i18n 文件。
4. 在 `handlers.ts` 中将 `workspaceSettings.name` 改为变量/seed，支持 `PATCH` 更新。
5. 运行 `pnpm lint && pnpm test && pnpm build`。
6. 提交 PR。

---

## 7. 测试验证

### 7.1 单元测试

```bash
cd apps/web && pnpm test -- WorkspaceSwitcher
```

### 7.2 集成/回归测试

```bash
cd apps/web && pnpm lint && pnpm test && pnpm build
```

### 7.3 手动验证

```bash
# 启动开发服务器
cd apps/web && pnpm dev
# 打开 Settings > General，确认 workspace name 不再是 Acme Capital
# 打开 AI Assistant，输入英文关键词，确认返回英文 suggestion
```

### 7.4 回归测试命令

```bash
cd apps/web && pnpm lint && pnpm test && pnpm build
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`。
- **范围锁定**：不得修改本任务范围外的文件；如必须修改，需在 Agent 备注中说明理由并征得审批。
- **保持现有测试通过**：运行全量回归测试命令全绿才能提交。
- **不要提前实现**：范围外的功能（如真实后端接口）不要碰。
- **禁止把敏感数据发送给 LLM**：Token、密码、密钥、PII、生产数据一律不得出现在 prompt、日志或测试数据中。
- **代码风格**：遵循项目已有目录和命名约定。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

| CODE-REVIEW 3.11 检查项 | 本任务自检要求 |
|------------------------|----------------|
| 无硬编码示例域名/邮箱/密码 | 测试数据使用 `example.test` 域名，默认 workspace name 不使用真实公司名。 |
| 无未清理的 TODO / FIXME / placeholder | 实现完成后全局搜索 `TODO`、`FIXME`、`placeholder`、`XXX`，确保无残留。 |
| 无幻觉常量 | AI 关键词、i18n key 从现有文件引用或新增后同步。 |
| 错误处理不过度 try-catch，不吞掉异常 | 测试断言不吞异常；mock 更新失败时返回合适状态码。 |
| 未引入未使用的依赖或代码 | 提交前运行 lint，删除无用 import/变量。 |
| 未擅自实现范围外功能 | 严格按第 4.1 节文件列表实现。 |
| 测试数据与生产数据隔离 | mock 数据为 fixture，不引用生产 schema 或真实用户数据。 |

---

## 10. Definition of Done

- [x] 代码实现完成
- [x] 测试通过（单元 + 集成）
- [x] lint / typecheck / build 通过
- [ ] 代码审查通过
- [x] 与父 Issue 的验收标准对齐
- [ ] PR 已关联父 Issue：`Relates to #DS-026`
- [x] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- `WorkspaceSwitcher.test.tsx` 的警告通常来自 Base UI 或 React 19 的异步 portal 渲染；优先使用 `@testing-library/react` 的 `waitFor` 包裹断言。
- AI store 的中文关键词可以保留在 `zh-CN/ai.json` 中作为翻译，但匹配逻辑默认以英文为准。
- 若修改 `handlers.ts` 时发现 `workspaceSettings` 被多处引用，保持接口结构不变，仅替换默认值与更新逻辑。

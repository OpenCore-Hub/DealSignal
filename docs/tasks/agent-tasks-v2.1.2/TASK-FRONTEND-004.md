---
task_id: "TASK-FRONTEND-004"
parent_issue: "DS-014"
agent_task_id: "AGENT-TASK-014"
version: "v2.1.2"
priority: "P0"
status: "已完成"
type: "frontend"
effort: "M"
branch: "feat/agent-task-014-floating-ai-assistant"
estimated_files: "8"
max_lines: "500"
project_stack: "React 19 / TypeScript / Tailwind CSS 4 / Base UI / i18next"
ai_red_flags:
  - "必须调用真实 /assistant/chat 与 /search"
  - "evidence 点击必须跳转页面并高亮 bbox"
  - "不得继续本地正则回复"
  - "AI 回答必须附带 evidence"
ai_confidence: "high"
pending_confirmation: []
available_tools:
  - "test"
  - "lint"
  - "browse"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-FRONTEND-004` |
> | `parent_issue` | `DS-014` |
> | `agent_task_id` | `AGENT-TASK-014` |
> | **版本** | `v2.1.2` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `待执行` |
> | **类型** | `frontend` |
> | **预计工作量** | `M` |
> | **分支名** | `feat/agent-task-014-floating-ai-assistant` |
> | **AI 置信度** | `high` |
> | **依赖** | `TASK-FRONTEND-002`, `TASK-BACKEND-004` |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint / browse` |

# TASK-FRONTEND-004 悬浮 AI 助手前端

> **父 Issue**：`DS-014`  
> **版本**：`v2.1.2`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`待执行`  
> **类型**：`frontend`  
> **预计工作量**：`M`  
> **分支名**：`feat/agent-task-014-floating-ai-assistant`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

将全局悬浮 AI 助手从本地正则回复改造为调用后端 `/search` 与 `/assistant/chat`，实现 evidence 展示、页面跳转、bbox 高亮、多轮会话。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8.5、§8.6 |
| TDD | `docs/TDD-v2.1.0.md` §6.6 |
| API 契约 | `docs/API-SPEC-v2.1.0.md` API-06、API-07 |
| 父 Issue | `DS-014` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.1 关键接口

```typescript
interface ChatMessage {
  role: "user" | "assistant";
  content: string;
  evidence?: Evidence[];
}

interface Evidence {
  chunk_id: string;
  quote: string;
  page_number: number;
  boxes: { x: number; y: number; w: number; h: number }[];
  score: number;
}
```

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 上下文 | 保留最近 N 条消息 | 默认 10 条 |
| evidence | 必须可点击跳转 | 跳转到对应 page 并高亮 bbox |
| 空结果 | 无相关文档 |  assistant 说明未找到依据 |
| 最大变更行数 | ≤ 500 | |

---

## 4. 输出

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/stores/aiStore.ts` | 重写 | 调用真实 API、管理会话 |
| `src/components/ai/AIAssistant.tsx` | 修改 | 接入 API、evidence 卡片 |
| `src/components/viewer/AIChat.tsx` | 修改 | 接入 API、跳转高亮 |
| `src/types/index.ts` | 修改 | 对齐 Evidence 字段 |
| `src/components/ai/AIAssistant.test.tsx` | 新增 | API 交互测试 |

---

## 5. 验收标准

- [ ] 用户提问调用 `/assistant/chat`
- [ ] 回答附带 evidence 列表
- [ ] evidence 点击跳转并高亮对应 bbox
- [ ] 多轮会话保留上下文
- [ ] `pnpm test` 通过
- [ ] `pnpm lint` 0 errors
- [ ] `pnpm build` 成功

---

## 6. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / build 通过
- [ ] PR 已关联父 Issue：`Closes #DS-014`

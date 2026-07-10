---
task_id: TASK-SHARE-SHORT-008
parent_issue: DS-SHARE-020
agent_task_id: AGENT-TASK-SHARE-020
version: v1.1.0
priority: P0
status: 已完成
type: fullstack
effort: M
branch: feat/share-short-008-ai-and-visitor-qa
estimated_files: '18'
max_lines: '800'
project_stack: Go 1.25 + Gin + PostgreSQL + React 19 + TypeScript
dependencies:
- INFRA-001
- TASK-SHARE-SHORT-001
- TASK-SHARE-SHORT-006
ai_red_flags:
- AI Assistant 与 Visitor Q&A 必须后端隔离：AI 走现有 assistant service；人工问答走独立表
- 公共端点必须校验 X-Link-Session，访客只能看到/操作自己的问答
- owner 回复通知必须异步，不能阻塞提交接口
- 禁止把访客的 PII 在 AI 上下文中泄露
- UI 必须区分 AI 回答与人工回答，避免访客误以为 AI 回答是 owner 回复
ai_confidence: medium
pending_confirmation:
- AI Assistant 与 Ask owner 是否合并为一个侧边栏 tab，还是两个独立 tab？
- owner 回复入口放在 Analytics Tab 还是独立 Questions Tab？
- 是否允许 owner 在回复时引用文档证据（类似 AI evidence）？
available_tools:
- test
- lint
- browse
---

# TASK-SHARE-SHORT-008 AI Assistant 与 Visitor Q&A 整合

> **父 Issue**：`DS-SHARE-020`  
> **版本**：`v1.1.0`  
> **优先级**：`P0`  
> **类型**：`fullstack`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-short-008-ai-and-visitor-qa`

---

## 1. 目标

把 Access Tab 中两个重叠的开关拆分为**两个独立且真实的能力**，并在公共 Viewer 中整合到同一面板：

- **AI Assistant**（`aiCopilotEnabled`）：已有公共 AI 问答能力，负责“辅助访客快速搜索/掌握资料内容”。
- **Visitor Q&A**（`qaEnabled`）：新增人工问答能力，负责“访客向 owner 在线提问”。

Access Tab 保留两个独立开关；公共 Viewer 侧边栏出现统一的 **“Q&A”** 面板，内部通过子 tab 切换 AI Assistant / Ask owner。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 功能设计 | `/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §8.7.6 |
| 已有实现 | `apps/api/internal/assistant/service.go` `PublicChat`；`apps/web/src/components/viewer/SidebarAIChat.tsx` |
| 相关任务 | TASK-SHARE-SHORT-001、TASK-SHARE-SHORT-009 |

### 2.1 当前问题

- `qaEnabled` 没有数据库字段，也没有独立后端，属于纯占位开关。
- AI Copilot 与 Q&A Conversations 被误认为是同一能力；实际上前者是 AI 问答，后者是人工问答。

---

## 3. 输入

### 3.1 数据模型

```sql
ALTER TABLE links ADD COLUMN IF NOT EXISTS qa_enabled boolean NOT NULL DEFAULT false;

CREATE TABLE link_visitor_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    visitor_id TEXT NOT NULL,
    visitor_email TEXT,
    question TEXT NOT NULL,
    answer TEXT,
    answered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','answered')),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_link_visitor_questions_link_id ON link_visitor_questions(link_id);
CREATE INDEX idx_link_visitor_questions_visitor ON link_visitor_questions(visitor_id);
```

### 3.2 API 契约

**公共端点（访客）**

```http
POST /api/v1/public/links/:token/questions
X-Link-Session: <session-token>
Content-Type: application/json

{ "question": "请问 2025 年营收确认口径是什么？" }
```

```http
GET /api/v1/public/links/:token/questions/me
X-Link-Session: <session-token>
```

**Owner 端点**

```http
GET /api/v1/links/:id/questions
```

```http
PATCH /api/v1/links/:id/questions/:questionId/answer
Content-Type: application/json

{ "answer": "我们按权责发生制确认，详见财务页第 3 页。" }
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 功能开关 | `links.qa_enabled = true` | 关闭时公共端点返回 `403 qa_disabled`，面板隐藏 Ask owner |
| 会话隔离 | 按 `link_id + visitor_id` | 访客只能看到自己的问题 |
| 问题长度 | 1~500 字符 | 过短/过长均返回 `400` |
| 通知 | 异步 | 提交接口只写 DB + 发事件 |
| AI 隔离 | AI Assistant 独立开关 | 关闭 qa_enabled 不影响 AI Assistant |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| qa_enabled 关闭 | `qa_enabled=false` | `403 qa_disabled` |
| 无 session | 无 `X-Link-Session` | `401 session_required` |
| 问题为空 | `question=""` | `400 question_required` |
| owner 查看非本 workspace | 越权 link id | `403 forbidden` |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/050_link_flags_qa_file_requests.up.sql` | 新增 | `links.qa_enabled`（由 INFRA-001 统一编排） |
| `apps/api/internal/db/migrations/050_link_flags_qa_file_requests.down.sql` | 新增 | 回滚 |
| `apps/api/internal/db/queries.sql` | 修改 | CreateLink/UpdateLink 增加 `qa_enabled`；新增 question CRUD |
| `apps/api/internal/db/queries.sql.go` | 重新生成 | sqlc |
| `apps/api/internal/db/models.go` | 重新生成 | sqlc |
| `apps/api/internal/link/service.go` | 修改 | 新增 `CreateVisitorQuestion`、`ListVisitorQuestions`、`AnswerVisitorQuestion` |
| `apps/api/internal/link/handler.go` | 修改 | 注册公共 + owner API |
| `apps/web/src/lib/api.ts` | 新增 | `createVisitorQuestion`、`listVisitorQuestions`、`answerVisitorQuestion` |
| `apps/web/src/types/index.ts` | 修改 | 补充 `VisitorQuestion` |
| `apps/web/src/components/links/share/AccessTab.tsx` | 修改 | 保留 `aiCopilotEnabled` 与 `qaEnabled` 两个开关，label 清晰区分 |
| `apps/web/src/components/viewer/RightSidebar.tsx` | 修改 | 用统一 "Q&A" tab 替代现有 "AI" tab |
| `apps/web/src/components/viewer/QAAssistantPanel.tsx` | 新增 | 统一 Q&A 面板，显示 AI/人工消息流 |
| `apps/web/src/components/viewer/QAComposer.tsx` | 新增 | 底部输入框 + 左侧模式选择器（AI / Ask owner） |
| `apps/web/src/components/viewer/VisitorQAForm.tsx` | 新增 | 访客提问 + 问答列表（Ask owner 模式） |
| `apps/web/src/components/links/share/AnalyticsTab.tsx` 或新 Tab | 修改 | Owner 回复入口 |
| `apps/web/src/i18n/locales/en/linkShare.json` | 修改 | 文案 |
| `apps/web/src/i18n/locales/zh-CN/linkShare.json` | 修改 | 文案 |

### 4.2 行为定义

```text
Access Tab / Advanced
├── AI Assistant [开关]
│   开启后，Q&A 面板支持 AI Assistant 模式。
└── Visitor Q&A [开关]
    开启后，Q&A 面板支持 Ask owner 模式。

公共 Viewer 侧边栏
└── Q&A tab
    - 消息流混合显示 AI 回答与 owner 回复
    - 每条消息带来源标签：AI / Owner
    - 底部输入框左侧有模式选择器：
      ┌─────────────┬──────────────────────────┐
      │ AI Assistant│ Ask owner                │
      └─────────────┴──────────────────────────┘
    - 选择 AI Assistant：消息发给公共 AI 端点，即时返回答案
    - 选择 Ask owner：消息作为 Visitor Question 提交，状态 pending，owner 回复后显示
```

---

## 5. 验收标准

- [ ] 后端新增 `links.qa_enabled` 列与 `link_visitor_questions` 表。
- [ ] Access Tab 同时存在 AI Assistant 与 Visitor Q&A 两个独立开关。
- [ ] 公共 Viewer 侧边栏出现统一 Q&A 面板，输入框左侧支持 AI Assistant / Ask owner 模式切换。
- [ ] 访客提交问题后，owner 收到异步通知；owner 可回复，访客可见。
- [ ] AI Assistant 关闭不影响 Visitor Q&A，反之亦然。
- [ ] 前端 `pnpm lint` / `pnpm typecheck` / `pnpm test` 全绿。
- [ ] 后端 `go test ./internal/link/...` 全绿。

---

## 6. 实现步骤建议

1. 基于 INFRA-001 已创建的 050 migration 修改代码，**本任务不再新增 migration**。
2. 后端 service/handler 实现 Visitor Q&A CRUD + session 校验。
3. 前端 API 封装。
4. 重构 `RightSidebar.tsx`：将 AI tab 升级为 Q&A tab。
5. 新增 `QAAssistantPanel.tsx`，内部集成 `SidebarAIChat` 与 `VisitorQAForm`。
   - 消息流统一渲染，带来源标签。
   - 新增 `QAComposer.tsx`：输入框左侧放置模式选择器（segmented/dropdown），根据模式调用不同 API。
6. 在 Analytics/管理区域实现 owner 回复入口。
7. 接入异步通知。
8. 补测试。

---

## 7. 测试验证

```bash
# 后端
cd apps/api
go test ./internal/link/...
./e2e-test.sh

# 前端
cd apps/web
pnpm test QAAssistantPanel VisitorQAForm RightSidebar AnalyticsTab
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- AI Assistant 与 Visitor Q&A **必须**后端隔离，不能共用同一张表或同一个端点。
- 访客只能看到/操作自己的问题，禁止通过遍历 question id 查看他人问题。
- owner 回复必须异步通知访客（至少站内/邮件一种）。
- UI 必须明确标识 AI 回答与人工回答，禁止混淆。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 单元测试 + e2e P0 通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-020`

---
task_id: "TASK-SHARE-SHORT-001"
parent_issue: "DS-SHARE-001"
agent_task_id: "AGENT-TASK-SHARE-001"
version: "v1.0.0"
priority: "P0"
status: "已完成"
type: "fullstack"
effort: "M"
branch: "feat/share-short-001-public-ai-copilot"
estimated_files: "12"
max_lines: "600"
project_stack: "Go 1.25 + Gin + PostgreSQL + React 19 + TypeScript"
ai_red_flags:
  - "公共 AI 端点必须验证 link session，不能泄露工作区认证 token"
  - "必须按 link + visitor 隔离会话，防止跨链接/跨用户看到聊天记录"
  - "不得把用户 PII 发送给 LLM"
  - "前端必须根据 aiCopilotEnabled 条件渲染 AI tab"
ai_confidence: "medium"
pending_confirmation:
  - "公共 AI 端点是否复用现有 assistant service 还是新建 public assistant service？"
  - "AI 会话是按 visitor_id 还是按设备 session 隔离？"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

# TASK-SHARE-SHORT-001 公共 Viewer AI Copilot 权限与端点修复

> **父 Issue**：`DS-SHARE-001`  
> **版本**：`v1.0.0`  
> **优先级**：`P0`  
> **类型**：`fullstack`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-short-001-public-ai-copilot`

---

## 1. 目标

修复公共文档查看器中 AI Copilot 的权限与功能缺陷：
- 后端新增仅限有效 link session 访问的公共 AI 问答端点。
- 前端根据 `aiCopilotEnabled` 条件渲染 AI tab，未启用时隐藏。
- AI 会话按 `(link_id, visitor_id)` 隔离并持久化到 `assistant_sessions.link_id/document_id`。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.3 |
| PRD | `docs/backup/PRD-v2.1.0.md` FR-06 |
| API 契约 | `docs/backup/API-SPEC-v2.1.0.md` API-06 / API-07 |
| TDD | `docs/backup/TDD-v2.1.0.md` §6.6 ~ §6.8 |

### 2.1 已有代码

- `apps/api/internal/assistant/service.go` — 认证 AI 助手服务
- `apps/api/internal/search/service.go` — hybrid search
- `apps/api/internal/link/session.go` — HMAC 签名 link session
- `apps/web/src/components/viewer/RightSidebar.tsx` — 公共 viewer 侧边栏
- `apps/web/src/components/viewer/SidebarAIChat.tsx` — 公共 viewer AI 聊天
- `apps/web/src/stores/aiStore.ts` — AI 状态管理

### 2.2 当前缺陷

- `RightSidebar.tsx` 始终显示 AI tab，未读取 `aiCopilotEnabled`。
- `SidebarAIChat.tsx` 调用认证 `/search` 端点，匿名用户会 401。
- `assistant.Service.resolveSession` 未写入 `link_id` / `document_id`。

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 访问权限 | 必须持有有效 `X-Link-Session` | 无效/过期 session 返回 `401 session_required` |
| AI 开关 | `links.ai_copilot_enabled = true` | 后端公共端点需再次校验 |
| 文档范围 | 限制在当前 link 的 documents | 不能跨 link 搜索其他文档 |
| 上下文长度 | 最多 20 条历史消息 | 与现有 `maxContextMessages` 一致 |
| 证据数量 | 最多 5 条 | 与现有 `defaultSearchResults` 一致 |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 无 session | 请求无 `X-Link-Session` | `401 session_required` |
| 非法 session | session 签名错误 | `401 invalid_session` |
| AI 未启用 | `ai_copilot_enabled=false` | `403 ai_copilot_disabled` |
| 链接已过期 | `expires_at` 已过去 | `410 link_expired` |
| 超出最大访问次数 | `access_count >= max_access_count` | `403 access_limit_reached` |
| 搜索无结果 | 查询无匹配 chunk | 返回 `answer: "未找到相关依据"`，`evidence: []` |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/assistant/public_handler.go` | 新增 | 公共 AI 端点 `POST /api/v1/public/assistant/chat` |
| `apps/api/internal/assistant/service.go` | 修改 | `resolveSession` 支持 `link_id` + `document_id` |
| `apps/api/internal/search/service.go` | 修改（可选） | 支持按 document IDs 过滤 search |
| `apps/api/internal/server/routes.go` | 修改 | 注册公共 AI 路由 |
| `apps/api/internal/db/queries.sql` | 修改 | 更新 assistant session 查询以支持 link_id |
| `apps/web/src/components/viewer/RightSidebar.tsx` | 修改 | 条件渲染 AI tab |
| `apps/web/src/components/viewer/SidebarAIChat.tsx` | 修改 | 调用公共 AI 端点并传入 session |
| `apps/web/src/lib/api.ts` | 新增 | `publicAssistantChat` API 封装 |
| `apps/web/src/types/index.ts` | 修改（可选） | 补充 public chat 类型 |
| `apps/web/src/stores/aiStore.ts` | 修改 | 支持公共上下文与 session |

### 4.2 接口契约

**请求**
```http
POST /api/v1/public/assistant/chat
X-Link-Session: <session-token>
Content-Type: application/json

{
  "message": "公司收入是多少？",
  "session_id": "uuid-or-null"
}
```

**响应**
```json
{
  "session_id": "uuid",
  "answer": "根据财务页...",
  "evidence": [
    {
      "document_id": "uuid",
      "page_number": 5,
      "quote": "2025 年收入...",
      "boxes": [{"x": 0.1, "y": 0.2, "w": 0.3, "h": 0.1}]
    }
  ]
}
```

---

## 5. 验收标准

- [ ] 公共 viewer 仅在 `aiCopilotEnabled=true` 时显示 AI tab。
- [ ] `POST /api/v1/public/assistant/chat` 仅接受有效 link session。
- [ ] AI 回答限制在当前 link 的 documents 范围内。
- [ ] `assistant_sessions.link_id` / `document_id` 被正确写入。
- [ ] 不同 link 或不同 visitor 的会话相互隔离。
- [ ] 后端单元测试覆盖公共 handler 的认证与隔离逻辑。
- [ ] 前端 `pnpm test` / `pnpm lint` / `pnpm typecheck` 全绿。

---

## 6. 实现步骤建议

1. 修改 `assistant.Service.resolveSession` 接受 `linkID` / `documentID` 参数。
2. 新增 `PublicChat` service 方法，使用 session 中的 `link_id` 限制搜索范围。
3. 新增 `public_handler.go` 并注册到 `/api/v1/public/assistant/chat`。
4. 在 `RightSidebar.tsx` 读取 `aiCopilotEnabled` 并条件渲染。
5. 新增 `publicAssistantChat` API 并在 `SidebarAIChat.tsx` 替换认证端点。
6. 更新 `aiStore` 以区分公共/认证上下文。
7. 补测试。

---

## 7. 测试验证

```bash
# 后端
cd apps/api
go test ./internal/assistant/...
go test ./internal/link/...
make lint

# 前端
cd apps/web
pnpm test SidebarAIChat
pnpm test RightSidebar
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 公共端点**不得复用**需要 workspace auth 的 `/assistant/chat`。
- 禁止把 `public_token` 或 session secret 打印到日志。
- 禁止在公共 AI 中暴露非当前 link 的文档内容。
- 前端禁用 AI tab 时必须同时移除相关路由/状态，避免残留。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 单元测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-001`

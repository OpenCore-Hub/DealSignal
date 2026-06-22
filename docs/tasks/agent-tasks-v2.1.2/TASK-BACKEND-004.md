---
task_id: "TASK-BACKEND-004"
parent_issue: "DS-011 / DS-012 / DS-013"
agent_task_id: "AGENT-TASK-007"
version: "v2.1.0"
priority: "P0"
status: "已完成"
type: "backend"
effort: "L"
branch: "feat/agent-task-007-search-ai"
estimated_files: "12"
max_lines: "800"
project_stack: "Go 1.22+ / Gin / PostgreSQL / pgvector / OpenAI / Redis"
ai_red_flags:
  - "AI 回答必须附带 evidence"
  - "禁止凭空生成内容"
  - "向量搜索必须带 workspace_id 过滤"
  - "LLM API key 不得硬编码"
ai_confidence: "medium"
pending_confirmation:
  - "使用 OpenAI API 还是自托管 embedding/LLM？"
  - "embedding 模型版本"
available_tools:
  - "test"
  - "lint"
  - "docker"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-BACKEND-004` |
> | `parent_issue` | `DS-011 / DS-012 / DS-013` |
> | `agent_task_id` | `AGENT-TASK-007` |
> | **版本** | `v2.1.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `待执行` |
> | **类型** | `backend` |
> | **预计工作量** | `L` |
> | **分支名** | `feat/agent-task-007-search-ai` |
> | **AI 置信度** | `medium` |
> | **依赖** | `TASK-BACKEND-003` |
> | **待人工确认事项** | `LLM/embedding 提供商与模型版本` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-BACKEND-004 Search、Evidence 与 Assistant 服务

> **父 Issue**：`DS-011 / DS-012 / DS-013`  
> **版本**：`v2.1.0`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`待执行`  
> **类型**：`backend`  
> **预计工作量**：`L`  
> **分支名**：`feat/agent-task-007-search-ai`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

实现文档全文/向量搜索、evidence 聚合、AI 助手对话，覆盖 API-09 ~ API-12；要求 AI 回答必须附带可追溯的 evidence；`assistant_sessions.link_id` 改为 nullable 以支持内部文档 AI 问答。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8.5、§8.6 |
| TDD | `docs/TDD-v2.1.0.md` §6.4、§6.5、§6.6 |
| API 契约 | `docs/API-SPEC-v2.1.0.md` API-09 ~ API-12 |
| DB | `docs/database-model-v2.1.0.md` |
| 算法 | `docs/HEAT-SCORE-ALGORITHM-v2.1.1.md` |
| 父 Issue | `DS-011 / DS-012 / DS-013` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 大；优先读 API-09~12 与 PRD AI 相关章节。 |
| **必读顺序** | front matter → `ai_red_flags` → 目标 → 边界条件 → 输出。 |
| **分块策略** | 若上下文受限，先读 Assistant API 请求/响应与 chunks 表结构。 |

### 3.1 已有代码/表（执行前必须阅读）

- `apps/api/internal/db/queries.sql`（来自 TASK-BACKEND-003）
- `apps/api/internal/middleware/auth.go`
- `docs/API-SPEC-v2.1.0.md` API-09 ~ API-12
- `docs/database-model-v2.1.0.md`

### 3.2 数据模型/接口

```sql
-- 新增向量索引（pgvector）
CREATE INDEX idx_chunks_embedding ON chunks
USING hnsw (embedding vector_cosine_ops);

CREATE TABLE assistant_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    user_id UUID NOT NULL REFERENCES users(id),
    title TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE assistant_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES assistant_sessions(id),
    role TEXT NOT NULL CHECK (role IN ('user','assistant')),
    content TEXT NOT NULL,
    evidence JSONB,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 搜索范围 | 仅当前 workspace | 向量与全文查询均带 `workspace_id` |
| top-k | 默认 5，最大 20 | 可配置 |
| 上下文长度 | ≤ 4000 tokens | 超过截断 |
| evidence | 必须包含 pageNumber、bbox、text | 无 evidence 时回答 "未找到依据" |
| LLM 超时 | ≤ 30s | 超时返回 504 |
| 最大变更行数 | ≤ 400 | 超出需拆分 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 越权搜索 | 查询其他 workspace | 403 `forbidden` |
| 无相关文档 | 问题与资料无关 | 返回 `answer` 说明未找到依据，evidence 为空 |
| LLM 超时 | 复杂问题 | 504 `upstream_timeout` |
| 上下文过长 | 历史消息过多 | 截断旧消息，保留系统提示与最近轮次 |
| 向量服务不可用 | pgvector 异常 | 500 并记录，不返回堆栈 |

### 3.5 测试数据 / Mock / Fixture

```json
{
  "question": "What was the Q3 revenue growth?",
  "expectedEvidence": {
    "pageNumber": 3,
    "bbox": {"x": 50, "y": 100, "w": 200, "h": 30},
    "text": "Revenue grew 3x YoY."
  }
}
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/internal/db/migrations/003_search_assistant.up.sql` | 新增 | chunks vector 索引 / assistant_sessions / assistant_messages |
| `apps/api/internal/db/migrations/003_search_assistant.down.sql` | 新增 | 回滚 |
| `apps/api/internal/db/queries.sql` | 修改 | 新增 search / evidence / session 查询 |
| `apps/api/internal/search/service.go` | 新增 | hybrid search（向量 + 全文） |
| `apps/api/internal/evidence/service.go` | 新增 | quote + bbox 聚合 |
| `apps/api/internal/assistant/service.go` | 新增 | LLM 调用与会话管理 |
| `apps/api/internal/assistant/handler.go` | 新增 | 路由 handler |
| `apps/api/internal/llm/client.go` | 新增 | OpenAI/自托管 LLM 客户端 |
| `apps/api/internal/server/routes.go` | 修改 | 注册 assistant 路由 |

### 4.2 行为定义

- `POST /api/assistant/chat` 接收 `sessionId?` 与 `message`，返回 answer + evidence 数组。
- `GET /api/search?q=...` 返回匹配的 chunks 列表（带 evidence）。
- 向量搜索使用 `pgvector`，必须按 `workspace_id` 过滤。
- 多轮对话保留最近 N 条消息上下文。

---

## 5. 验收标准

- [x] `/api/assistant/chat` 返回答案 + evidence
- [x] evidence 包含 pageNumber、bbox、text
- [x] 多轮对话保留上下文
- [x] 越权访问返回 403（由 workspace 鉴权中间件保证）
- [x] 无相关文档时 assistant 不编造内容
- [x] `go test ./...` 通过
- [x] `make lint` 通过

---

## 6. 实现步骤建议

1. 编写 migration `003_search_assistant`。
2. 更新 `queries.sql` 与 sqlc 生成。
3. 实现 `internal/search/service.go`（hybrid search）。
4. 实现 `internal/evidence/service.go`（格式化 evidence）。
5. 实现 `internal/llm/client.go` 封装 OpenAI chat completion。
6. 实现 `internal/assistant/service.go` 与 `handler.go`。
7. 注册路由。
8. 编写测试（可 mock LLM client）。
9. 运行 `make lint && make test`。
10. 提交 PR。

---

## 7. 测试验证

### 7.1 单元测试

```bash
cd apps/api && go test ./internal/search/... ./internal/evidence/... ./internal/assistant/...
```

### 7.2 集成测试

```bash
cd apps/api && docker compose up -d
go test ./tests/integration/... -tags integration
docker compose down
```

### 7.3 手动验证

```bash
curl -X POST http://localhost:8080/api/assistant/chat \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"message":"What was Q3 revenue?"}'
```

### 7.4 回归测试命令

```bash
cd apps/api && make lint && make test
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`。
- **范围锁定**：聚焦 search + evidence + assistant；不做多模态、agent 工具调用、RAG 重排序高级策略。
- **租户隔离**：所有数据库查询必须带 `workspace_id`。
- **AI 安全**：禁止凭空生成内容；answer 必须基于检索到的 chunks。
- **不要提前实现**：范围外的功能（如 public viewer AI）不要碰。
- **禁止把敏感数据发送给 LLM**：Token、密码、密钥、PII、生产数据一律不得出现在 prompt、日志或测试数据中。
- **代码风格**：遵循 Go 标准项目布局。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

| CODE-REVIEW 3.11 检查项 | 本任务自检要求 |
|------------------------|----------------|
| 无硬编码示例域名/邮箱/密码 | LLM API key 来自环境变量。 |
| 无未清理的 TODO / FIXME / placeholder | 全局搜索无残留。 |
| 无幻觉常量 | token 限制、top-k 使用配置/常量。 |
| 错误处理不过度 try-catch，不吞掉异常 | LLM 错误返回结构化上游错误。 |
| 未引入未使用的依赖或代码 | `go mod tidy` 与 lint 通过。 |
| 未擅自实现范围外功能 | 严格按 search/evidence/assistant 范围。 |
| 测试数据与生产数据隔离 | fixture 数据不引用生产。 |

---

## 10. Definition of Done

- [x] 代码实现完成
- [x] 测试通过（单元 + 集成待后续 TEST 任务补齐）
- [x] lint / build 通过
- [ ] 代码审查通过
- [x] 与父 Issue 的验收标准对齐
- [ ] PR 已关联父 Issue：`Closes #DS-011` / `Relates to #DS-012 #DS-013`
- [x] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- 推荐 embedding 模型 `text-embedding-3-small`（1536 维）；若自托管需同步维度。
- LLM 客户端应支持可替换 provider，便于测试与切换模型。
- 可用 `github.com/pgvector/pgvector-go` 与 `pgx` 集成向量查询。
- 若文件数超出，可将 LLM client 或 evidence 格式化拆为单独 task。

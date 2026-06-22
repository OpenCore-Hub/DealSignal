---
task_id: "TASK-BACKEND-005"
parent_issue: "DS-009 / DS-015 / DS-017"
agent_task_id: "AGENT-TASK-008"
version: "v2.1.0"
priority: "P0"
status: "已完成"
type: "backend"
effort: "L"
branch: "feat/agent-task-008-links-analytics"
estimated_files: "12"
max_lines: "800"
project_stack: "Go 1.22+ / Gin / PostgreSQL / Redis"
ai_red_flags:
  - "链接权限校验必须原子化"
  - "访问日志必须不可篡改"
  - "热度评分算法必须与前端一致"
  - "公开访问接口不得泄露 workspace 敏感信息"
ai_confidence: "high"
pending_confirmation: []
available_tools:
  - "test"
  - "lint"
  - "docker"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-BACKEND-005` |
> | `parent_issue` | `DS-009 / DS-015 / DS-017` |
> | `agent_task_id` | `AGENT-TASK-008` |
> | **版本** | `v2.1.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `待执行` |
> | **类型** | `backend` |
> | **预计工作量** | `L` |
> | **分支名** | `feat/agent-task-008-links-analytics` |
> | **AI 置信度** | `high` |
> | **依赖** | `TASK-BACKEND-003` |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-BACKEND-005 智能链接、权限、Analytics 与热度评分

> **父 Issue**：`DS-009 / DS-015 / DS-017`  
> **版本**：`v2.1.0`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`待执行`  
> **类型**：`backend`  
> **预计工作量**：`L`  
> **分支名**：`feat/agent-task-008-links-analytics`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

实现智能链接创建、权限校验、公开访问、访问日志、热度评分与 Analytics，覆盖 API-06 ~ API-08、API-13 ~ API-18。数据室已拆分至 `TASK-BACKEND-006`。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8.7 ~ §8.11 |
| TDD | `docs/TDD-v2.1.0.md` §6.3、§6.7、§6.8 |
| API 契约 | `docs/API-SPEC-v2.1.0.md` API-06 ~ API-08、API-13 ~ API-18 |
| 算法 | `docs/HEAT-SCORE-ALGORITHM-v2.1.1.md` |
| DB | `docs/database-model-v2.1.0.md` |
| 父 Issue | `DS-009 / DS-015 / DS-017` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 大；优先读 links/analytics/heat-score 相关 API 与 PRD。 |
| **必读顺序** | front matter → `ai_red_flags` → 目标 → 边界条件 → 输出。 |
| **分块策略** | 若上下文受限，先读 link/room 表结构与 API-06~08、API-13~18 请求响应。 |

### 3.1 已有代码/表（执行前必须阅读）

- `apps/api/internal/db/queries.sql`（来自前面任务）
- `apps/api/internal/middleware/auth.go`
- `docs/API-SPEC-v2.1.0.md` API-06 ~ API-08、API-13 ~ API-18
- `docs/HEAT-SCORE-ALGORITHM-v2.1.1.md`
- `docs/database-model-v2.1.0.md`
- `apps/web/src/lib/heat/heatScore.ts`（前端算法参考）

### 3.2 数据模型/接口

```sql
CREATE TABLE links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    document_id UUID REFERENCES documents(id),
    slug TEXT UNIQUE NOT NULL,
    access_level TEXT NOT NULL CHECK (access_level IN ('public','email','password','nda')),
    allowed_emails JSONB NOT NULL DEFAULT '[]',
    allowed_domains JSONB NOT NULL DEFAULT '[]',
    password_hash TEXT,
    expires_at TIMESTAMPTZ,
    max_access_count INT,
    download_enabled BOOLEAN DEFAULT false,
    watermark_enabled BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE access_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id UUID NOT NULL REFERENCES links(id),
    visitor_email TEXT,
    ip INET,
    user_agent TEXT,
    viewed_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE page_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id UUID NOT NULL REFERENCES links(id),
    page_number INT NOT NULL,
    duration_seconds INT DEFAULT 0,
    scroll_depth DECIMAL(5,2),
    viewed_at TIMESTAMPTZ DEFAULT now()
);
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 链接 slug | 唯一、8~64 字符、URL safe | 非法返回 400 |
| access_level | public / email / password / nda | 校验严格 |
| 密码链接 | password_hash bcrypt 存储 | 公开访问时校验 |
| 热度评分 | 按 `HEAT-SCORE-ALGORITHM-v2.1.1.md` 计算 | 返回完整 7 维 factors |
| 访问日志 | 只追加，不可删除/修改 | 记录 IP/UA/时间 |
| 事件上报 | `link_opened / page_viewed / download_attempted`；后端推导算法事件 | 去重规则见算法文档 |
| 最大变更行数 | ≤ 800 | 超出需拆分 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 链接不存在 | 随机 slug | 404 `link_not_found` |
| 密码错误 | password 链接输入错误 | 401 `invalid_password` |
| 需要邮箱 | email 链接未提供邮箱 | 401 `email_required` |
| NDA 未同意 | nda 链接未勾选 | 403 `nda_required` |
| 链接已过期 | `expires_at` 已过 | 410 `link_expired` |
| 越权创建 | 非 workspace 成员 | 403 `forbidden` |

### 3.5 测试数据 / Mock / Fixture

```json
{
  "link": {
    "documentId": "doc-001",
    "accessLevel": "email",
    "allowedDomains": ["example.test"],
    "expiresAt": "2026-12-31T23:59:59Z"
  },
  "visitor": {
    "email": "visitor@example.test",
    "ip": "203.0.113.1"
  }
}
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/internal/db/migrations/004_links_analytics.up.sql` | 新增 | links / access_logs / page_views |
| `apps/api/internal/db/migrations/004_links_analytics.down.sql` | 新增 | 回滚 |
| `apps/api/internal/db/queries.sql` | 修改 | 新增相关查询 |
| `apps/api/internal/link/service.go` | 新增 | 链接 CRUD 与权限校验 |
| `apps/api/internal/link/handler.go` | 新增 | 路由 handler |
| `apps/api/internal/analytics/service.go` | 新增 | 事件聚合、热度评分 |
| `apps/api/internal/analytics/handler.go` | 新增 | analytics 路由 |
| `apps/api/internal/heat/score.go` | 新增 | Go 版热度评分算法 |
| `apps/api/internal/server/routes.go` | 修改 | 注册 links / analytics 路由 |

### 4.2 行为定义

- `POST /api/links` 创建智能链接，返回 slug 与访问 URL。
- `GET /api/v1/public/links/:slug` 公开访问，按 access_level 校验并记录日志。
- `POST /api/v1/public/events` 接收阅读事件，后端去重并更新热度评分。
- `GET /api/analytics/links/:linkId/score` 返回热度分数与完整 factors。
- 访问日志写入使用事务，确保热度评分与日志记录原子性。

---

## 5. 验收标准

- [x] 链接权限（public/email/password/NDA）校验正确
- [x] 访问后热度评分更新
- [x] 访问日志不可删除/修改（DB trigger 保护）
- [x] 越权访问返回 403
- [x] 热度评分返回完整 7 维 factors
- [x] `go test ./...` 通过
- [x] `make lint` 通过

---

## 6. 实现步骤建议

1. 编写 migration `004_links_analytics`。
2. 更新 `queries.sql` 与 sqlc 生成。
3. 实现 `internal/link/service.go`（权限校验 + slug 生成）。
4. 实现 `internal/link/handler.go`。
5. 实现 `internal/analytics/service.go`（事件写入 + 热度评分）。
6. 实现 `internal/heat/score.go`，与前端 `heatScore.ts` 数值一致。
7. 实现 `internal/analytics/handler.go`。
8. 注册路由。
9. 编写测试。
10. 运行 `make lint && make test`。
11. 提交 PR。

---

## 7. 测试验证

### 7.1 单元测试

```bash
cd apps/api && go test ./internal/link/... ./internal/analytics/... ./internal/heat/...
```

### 7.2 集成测试

```bash
cd apps/api && docker compose up -d
go test ./tests/integration/... -tags integration
docker compose down
```

### 7.3 手动验证

```bash
# 创建链接
curl -X POST http://localhost:8080/api/links \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"documentId":"doc-001","accessLevel":"email","allowedDomains":["example.test"]}'

# 公开访问
curl -i http://localhost:8080/api/v1/public/links/{slug}?email=visitor@example.test
```

### 7.4 回归测试命令

```bash
cd apps/api && make lint && make test
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`。
- **范围锁定**：聚焦 links + analytics + heat score；数据室在 `TASK-BACKEND-006`。
- **租户隔离**：所有数据库查询必须带 `workspace_id`。
- **公开接口安全**：不暴露 workspace/user 敏感字段。
- **不要提前实现**：范围外的功能不要碰。
- **禁止把敏感数据发送给 LLM**：Token、密码、密钥、PII、生产数据一律不得出现在 prompt、日志或测试数据中。
- **代码风格**：遵循 Go 标准项目布局。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

| CODE-REVIEW 3.11 检查项 | 本任务自检要求 |
|------------------------|----------------|
| 无硬编码示例域名/邮箱/密码 | 测试数据使用 `.test`；密码 bcrypt 存储。 |
| 无未清理的 TODO / FIXME / placeholder | 全局搜索无残留。 |
| 无幻觉常量 | 热度评分公式与算法文档一致。 |
| 错误处理不过度 try-catch，不吞掉异常 | 权限错误返回明确 code。 |
| 未引入未使用的依赖或代码 | `go mod tidy` 与 lint 通过。 |
| 未擅自实现范围外功能 | 严格按 links/analytics/heat score 范围。 |
| 测试数据与生产数据隔离 | fixture 数据不引用生产。 |

---

## 10. Definition of Done

- [x] 代码实现完成
- [x] 测试通过（单元 + 集成待后续 TEST 任务补齐）
- [x] lint / build 通过
- [ ] 代码审查通过
- [x] 与父 Issue 的验收标准对齐
- [ ] PR 已关联父 Issue：`Closes #DS-009` / `Relates to #DS-015 #DS-017`
- [x] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- 热度评分算法建议复用前端 `heatScore.ts` 的 Go 版本，确保数值一致。
- 访问日志写入使用事务，确保热度评分计算与日志记录原子性。
- 若文件数超出，可将 analytics/heat score 拆为单独 task。

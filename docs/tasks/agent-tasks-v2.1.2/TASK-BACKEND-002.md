---
task_id: "TASK-BACKEND-002"
parent_issue: "DS-002"
agent_task_id: "AGENT-TASK-005"
version: "v2.1.0"
priority: "P0"
status: "已完成"
type: "backend"
effort: "L"
branch: "feat/agent-task-005-auth-workspace"
estimated_files: "12"
max_lines: "800"
project_stack: "Go 1.22+ / Gin / PostgreSQL / sqlc / pgx / JWT"
ai_red_flags:
  - "密码必须 bcrypt 存储"
  - "所有查询必须带 tenant_id + workspace_id"
  - "JWT secret 不得硬编码"
  - "邀请 token 必须有时效与单次使用校验"
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
> | `task_id` | `TASK-BACKEND-002` |
> | `parent_issue` | `DS-002` |
> | `agent_task_id` | `AGENT-TASK-005` |
> | **版本** | `v2.1.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `待执行` |
> | **类型** | `backend` |
> | **预计工作量** | `L` |
> | **分支名** | `feat/agent-task-005-auth-workspace` |
> | **AI 置信度** | `high` |
> | **依赖** | `TASK-BACKEND-001` |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-BACKEND-002 认证与 Workspace/租户模块

> **父 Issue**：`DS-002`  
> **版本**：`v2.1.0`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`待执行`  
> **类型**：`backend`  
> **预计工作量**：`L`  
> **分支名**：`feat/agent-task-005-auth-workspace`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

实现用户注册/登录、JWT 认证、租户/Workspace 创建与成员管理，覆盖 API-01 ~ API-04；统一 workspace 角色枚举为 `owner/admin/member/guest`。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §6.3、§8.1 |
| TDD | `docs/TDD-v2.1.0.md` §6.1 |
| ARCHITECTURE | `docs/ARCHITECTURE-v2.1.0.md` §4 |
| API 契约 | `docs/API-SPEC-v2.1.0.md` API-01 ~ API-04 |
| DB | `docs/database-model-v2.1.0.md` |
| 父 Issue | `DS-002` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 较大；优先读 API-SPEC API-01~04 与 database-model。 |
| **必读顺序** | front matter → `ai_red_flags` → 目标 → 边界条件 → 输出。 |
| **分块策略** | 若上下文受限，先读 SQL schema 与 API 请求/响应定义。 |

### 3.1 已有代码/表（执行前必须阅读）

- `apps/api/internal/server/server.go`（来自 TASK-BACKEND-001）
- `apps/api/internal/config/config.go`
- `docs/API-SPEC-v2.1.0.md` API-01 ~ API-04
- `docs/database-model-v2.1.0.md`

### 3.2 数据模型/接口

```sql
-- 核心表（供 sqlc 生成）
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email CITEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    slug CITEXT UNIQUE NOT NULL,
    brand_color TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE workspace_members (
    workspace_id UUID REFERENCES workspaces(id),
    user_id UUID REFERENCES users(id),
    role TEXT NOT NULL CHECK (role IN ('owner','admin','member')),
    joined_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 密码 | bcrypt cost ≥ 10 | 注册/修改密码生效 |
| Workspace slug | 全局唯一、小写、字母数字连字符 | 非法返回 400 |
| 邮箱 | 唯一、格式校验 | 返回明确错误码 |
| 邀请 token | 默认 7 天，最大 30 天 | 超期返回 410 |
| JWT 过期 | 默认 24h | 可配置 |
| 最大变更行数 | ≤ 400 | 超出需拆分 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 邮箱已注册 | 重复注册 | 409 `email_conflict` |
| 密码错误 | 登录 | 401 `unauthorized` |
| 越权访问 | 访问非成员 workspace | 403 `forbidden` |
| token 过期 | 邀请链接过期 | 410 `invite_expired` |
| slug 非法 | `my workspace!` | 400 `invalid_slug` |
| 未认证 | 无 Authorization | 401 `unauthorized` |

### 3.5 测试数据 / Mock / Fixture

```json
{
  "user": {
    "email": "user-001@example.test",
    "password": "correct-horse-battery-staple"
  },
  "workspace": {
    "name": "Demo Capital",
    "slug": "demo-capital"
  }
}
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/internal/db/migrations/001_users_workspaces.up.sql` | 新增 | users / tenants / workspaces / workspace_members 表 |
| `apps/api/internal/db/migrations/001_users_workspaces.down.sql` | 新增 | 回滚 |
| `apps/api/internal/db/queries.sql` | 新增 | sqlc 查询 |
| `apps/api/internal/db/sqlc.yaml` | 新增 | sqlc 配置 |
| `apps/api/internal/auth/service.go` | 新增 | 注册、登录、JWT 签发/校验 |
| `apps/api/internal/auth/handler.go` | 新增 | auth 路由 handler |
| `apps/api/internal/workspace/service.go` | 新增 | workspace CRUD、成员管理 |
| `apps/api/internal/workspace/handler.go` | 新增 | workspace 路由 handler |
| `apps/api/internal/middleware/auth.go` | 新增 | JWT 校验、workspace 注入 |
| `apps/api/internal/server/routes.go` | 修改 | 注册 auth / workspace 路由 |

### 4.2 行为定义

- `POST /api/auth/register` 创建用户并返回 JWT。
- `POST /api/auth/login` 校验密码并返回 JWT。
- `POST /api/workspaces` 创建 workspace，当前用户自动成为 owner。
- `GET /api/workspaces` 返回当前用户所属 workspace 列表。
- `POST /api/workspaces/:id/members` 邀请成员。
- 所有 workspace 路由必须校验用户是否为成员。

---

## 5. 验收标准

- [x] API-01 ~ API-04 通过 curl/测试
- [x] 越权测试返回 403
- [x] 密码以 bcrypt 存储，数据库中不可见明文
- [x] sqlc 生成代码可编译
- [x] `go test ./...` 通过
- [x] `make lint` 通过
- [x] JWT secret 来自环境变量，无硬编码

---

## 6. 实现步骤建议

1. 编写 migration `001_users_workspaces`（含 CITEXT 扩展）。
2. 配置 `sqlc.yaml` 并编写 `queries.sql`。
3. 运行 `sqlc generate` 生成 repository 代码。
4. 实现 `internal/auth/service.go`（bcrypt、JWT）。
5. 实现 `internal/auth/handler.go` 与 `/api/auth/*` 路由。
6. 实现 `internal/workspace/service.go` 与 `handler.go`。
7. 实现 `internal/middleware/auth.go` 注入 `userID` 与 `workspaceID`。
8. 在 `server/routes.go` 注册路由。
9. 编写 service/handler 单元测试与集成测试。
10. 运行 `make lint && make test`。
11. 提交 PR。

---

## 7. 测试验证

### 7.1 单元测试

```bash
cd apps/api && go test ./internal/auth/... ./internal/workspace/...
```

### 7.2 集成测试

```bash
cd apps/api && docker compose up -d
go test ./tests/integration/... -tags integration
docker compose down
```

### 7.3 手动验证

```bash
# 注册
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user-001@example.test","password":"secret123"}'

# 登录
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user-001@example.test","password":"secret123"}'
```

### 7.4 回归测试命令

```bash
cd apps/api && make lint && make test
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`。
- **范围锁定**：仅实现 auth + workspace 核心；不做 OAuth、RBAC 权限细化、邀请邮件发送。
- **租户隔离**：所有数据库查询必须带 `tenant_id` / `workspace_id`。
- **认证授权**：任何敏感操作必须通过 auth middleware。
- **不要提前实现**：范围外的功能（如 billing、integrations）不要碰。
- **禁止把敏感数据发送给 LLM**：Token、密码、密钥、PII、生产数据一律不得出现在 prompt、日志或测试数据中。
- **代码风格**：遵循 Go 标准项目布局。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

| CODE-REVIEW 3.11 检查项 | 本任务自检要求 |
|------------------------|----------------|
| 无硬编码示例域名/邮箱/密码 | 测试数据使用 `.test` 域名；密码不在代码中。 |
| 无未清理的 TODO / FIXME / placeholder | 全局搜索无残留。 |
| 无幻觉常量 | 角色枚举、brypt cost 使用常量或配置。 |
| 错误处理不过度 try-catch，不吞掉异常 | 数据库/密码错误返回结构化错误。 |
| 未引入未使用的依赖或代码 | `go mod tidy` 与 lint 通过。 |
| 未擅自实现范围外功能 | 严格按文件列表实现。 |
| 测试数据与生产数据隔离 | fixture 数据不引用生产。 |

---

## 10. Definition of Done

- [x] 代码实现完成
- [x] 测试通过（单元 + 集成）
- [x] lint / build 通过
- [ ] 代码审查通过
- [x] 与父 Issue 的验收标准对齐
- [ ] PR 已关联父 Issue：`Closes #DS-002`
- [x] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- 推荐使用 `golang.org/x/crypto/bcrypt`、`github.com/golang-jwt/jwt/v5`、`github.com/sqlc-dev/sqlc`。
- 若文件数超过 8，可将 migration/down 文件与 sqlc 配置合并，或将测试拆到单独 task。
- 邀请 token 可先使用随机字符串 + 过期时间字段，不必实现邮件发送。

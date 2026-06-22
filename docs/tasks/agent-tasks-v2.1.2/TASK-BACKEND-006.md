---
task_id: "TASK-BACKEND-006"
parent_issue: "DS-019"
agent_task_id: "AGENT-TASK-009"
version: "v2.1.0"
priority: "P0"
status: "已完成"
type: "backend"
effort: "L"
branch: "feat/agent-task-009-deal-rooms"
estimated_files: "12"
max_lines: "800"
project_stack: "Go 1.22+ / Gin / PostgreSQL / Redis"
ai_red_flags:
  - "数据室成员权限必须原子化"
  - "访问申请日志不可篡改"
  - "NDA 同意记录必须可审计"
  - "文件夹权限必须按成员隔离"
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
> | `task_id` | `TASK-BACKEND-006` |
> | `parent_issue` | `DS-019` |
> | `agent_task_id` | `AGENT-TASK-009` |
> | **版本** | `v2.1.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `待执行` |
> | **类型** | `backend` |
> | **预计工作量** | `L` |
> | **分支名** | `feat/agent-task-009-deal-rooms` |
> | **AI 置信度** | `high` |
> | **依赖** | `TASK-BACKEND-005` |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-BACKEND-006 数据室模块

> **父 Issue**：`DS-019`  
> **版本**：`v2.1.0`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`待执行`  
> **类型**：`backend`  
> **预计工作量**：`L`  
> **分支名**：`feat/agent-task-009-deal-rooms`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

实现数据室创建、成员管理、访问申请/审批、文件夹权限、NDA 开关、Q&A 基础能力，覆盖 API-19 ~ API-22。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8.12 ~ §8.13 |
| TDD | `docs/TDD-v2.1.0.md` §6.10 |
| API 契约 | `docs/API-SPEC-v2.1.0.md` API-19 ~ API-22 |
| DB | `docs/database-model-v2.1.0.md` |
| 父 Issue | `DS-019` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 大；优先读 API-19~22 与 PRD 数据室章节。 |
| **必读顺序** | front matter → `ai_red_flags` → 目标 → 边界条件 → 输出。 |
| **分块策略** | 若上下文受限，先读数据室表结构与 API 请求/响应。 |

### 3.1 已有代码/表（执行前必须阅读）

- `apps/api/internal/db/queries.sql`（来自前面任务）
- `apps/api/internal/middleware/auth.go`
- `docs/API-SPEC-v2.1.0.md` API-19 ~ API-22
- `docs/database-model-v2.1.0.md`

### 3.2 数据模型/接口

```sql
CREATE TABLE deal_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    slug TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    template_type TEXT NOT NULL,
    requires_nda BOOLEAN DEFAULT false,
    requires_approval BOOLEAN DEFAULT false,
    settings JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE deal_room_members (
    room_id UUID NOT NULL REFERENCES deal_rooms(id),
    user_id UUID REFERENCES users(id),
    email TEXT,
    role TEXT NOT NULL CHECK (role IN ('admin','member')),
    status TEXT NOT NULL DEFAULT 'active',
    joined_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (room_id, email)
);

CREATE TABLE room_access_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES deal_rooms(id),
    email TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','cancelled','revoked')),
    requested_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE room_member_folder_permissions (
    room_id UUID NOT NULL,
    member_email TEXT NOT NULL,
    folder_path TEXT NOT NULL,
    permission TEXT NOT NULL CHECK (permission IN ('view','download','none')),
    PRIMARY KEY (room_id, member_email, folder_path)
);
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| slug | 唯一、URL safe | 非法返回 400 |
| template_type | seed/series_a/lp_update/sales_proposal 等 | 统一大小写并归一化 |
| NDA | 必须记录同意时间/IP | 可审计 |
| 审批 | `requires_approval=true` 时未审批成员不可访问 | 状态 pending/approved/rejected |
| folder 权限 | 默认继承，可单独覆盖 | none 表示禁止访问 |
| 最大变更行数 | ≤ 800 | 超出需拆分 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 数据室不存在 | 随机 slug | 404 |
| 未审批访问 | request 状态 pending | 403 |
| NDA 未同意 | requires_nda=true | 403 |
| 越权邀请 | 非数据室 admin | 403 |
| folder 无权限 | permission=none | 403 |

### 3.5 测试数据 / Mock / Fixture

```json
{
  "dealRoom": {
    "title": "Series A Room",
    "slug": "series-a-room",
    "templateType": "series_a",
    "requiresNda": true,
    "requiresApproval": true
  }
}
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/internal/db/migrations/005_deal_rooms.up.sql` | 新增 | deal_rooms / members / access_requests / folder_permissions |
| `apps/api/internal/db/migrations/005_deal_rooms.down.sql` | 新增 | 回滚 |
| `apps/api/internal/db/queries.sql` | 修改 | 新增数据室相关查询 |
| `apps/api/internal/dealroom/service.go` | 新增 | 数据室 CRUD、成员、审批、权限 |
| `apps/api/internal/dealroom/handler.go` | 新增 | 路由 handler |
| `apps/api/internal/server/routes.go` | 修改 | 注册 deal-rooms 路由 |

### 4.2 行为定义

- `POST /api/deal-rooms` 创建数据室。
- `POST /api/deal-rooms/:id/members` 邀请成员。
- `POST /api/deal-rooms/:id/access-requests/:id/approve` 审批访问申请。
- `GET /api/v1/public/deal-rooms/:slug` 公开/半公开访问入口。

---

## 5. 验收标准

- [x] 数据室创建、成员邀请、审批流程可用
- [x] NDA 同意记录可审计且不可覆盖
- [x] folder 权限按成员隔离
- [x] 越权访问返回 403
- [x] `go test ./...` 通过
- [x] `make lint` 通过

---

## 6. 实现步骤建议

1. 编写 migration `005_deal_rooms`。
2. 更新 `queries.sql` 与 sqlc 生成。
3. 实现 `internal/dealroom/service.go`。
4. 实现 `internal/dealroom/handler.go`。
5. 注册路由。
6. 编写测试。
7. 运行 `make lint && make test`。
8. 提交 PR。

---

## 7. 测试验证

### 7.1 单元测试

```bash
cd apps/api && go test ./internal/dealroom/...
```

### 7.2 集成测试

```bash
cd apps/api && docker compose up -d
go test ./tests/integration/... -tags integration
docker compose down
```

### 7.3 手动验证

```bash
curl -X POST http://localhost:8080/api/deal-rooms \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"title":"Series A Room","slug":"series-a-room","templateType":"series_a"}'
```

### 7.4 回归测试命令

```bash
cd apps/api && make lint && make test
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`。
- **范围锁定**：聚焦数据室核心；Q&A 可最小化实现。
- **租户隔离**：所有数据库查询必须带 `workspace_id`。
- **不要提前实现**：范围外的功能不要碰。
- **禁止把敏感数据发送给 LLM**：Token、密码、密钥、PII、生产数据一律不得出现在 prompt、日志或测试数据中。
- **代码风格**：遵循 Go 标准项目布局。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

| CODE-REVIEW 3.11 检查项 | 本任务自检要求 |
|------------------------|----------------|
| 无硬编码示例域名/邮箱/密码 | 测试数据使用 `.test`。 |
| 无未清理的 TODO / FIXME / placeholder | 全局搜索无残留。 |
| 无幻觉常量 | 角色/状态枚举使用常量/配置。 |
| 错误处理不过度 try-catch，不吞掉异常 | 权限错误返回明确 code。 |
| 未引入未使用的依赖或代码 | `go mod tidy` 与 lint 通过。 |
| 未擅自实现范围外功能 | 严格按数据室范围。 |
| 测试数据与生产数据隔离 | fixture 数据不引用生产。 |

---

## 10. Definition of Done

- [x] 代码实现完成
- [x] 测试通过（单元 + 集成待后续 TEST 任务补齐）
- [x] lint / build 通过
- [ ] 代码审查通过
- [x] 与父 Issue 的验收标准对齐
- [ ] PR 已关联父 Issue：`Closes #DS-019`
- [x] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- 若文件数超出，可将 folder 权限或 access requests 拆为单独 task。
- NDA 记录建议单独表 `room_nda_agreements`，关联 visitor email + IP + timestamp。

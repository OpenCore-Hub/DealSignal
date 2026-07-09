---
task_id: TASK-SHARE-SHORT-005
parent_issue: DS-SHARE-015
agent_task_id: AGENT-TASK-SHARE-015
version: v1.0.0
priority: P0
status: 已完成
type: backend
effort: L
branch: feat/share-short-005-link-sharing-core
estimated_files: '16'
max_lines: '800'
project_stack: Go 1.25 + Gin + PostgreSQL + sqlc + bcrypt
dependencies:
- INFRA-001
ai_red_flags:
- 邀请 token 必须 hash 存储，不能明文保存在 DB
- Access rules 全量替换时必须保留审计快照或行级变更记录
- 密码 hash 必须使用 bcrypt，常量时间比较
- Deal Room link 与 document link 必须互斥，应用层 + DB 双重校验
- 规则评估必须 fail-closed：block 优先、allow 必须命中
ai_confidence: high
pending_confirmation:
- 访问请求（request access）是否允许匿名访客提交？
- session 失效是否引入 security_version 字段替代 updated_at 比较？
available_tools:
- test
- lint
---

# TASK-SHARE-SHORT-005 Deal Room / 文档链接分享后端核心

> **父 Issue**：`DS-SHARE-015`  
> **版本**：`v1.0.0`  
> **优先级**：`P0`  
> **状态**：`部分完成`  
> **类型**：`backend`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-short-005-link-sharing-core`

---

## 1. 目标

实现 Deal Room / 文档链接的统一分享后端核心：
- 在 `links` 表支持 `deal_room_id` 与 `password_hash`。
- 提供 `link_access_rules` 与 `link_invitations` 的 CRUD 与规则评估引擎。
- 改造 `Service.Access()`，集成 invite token 解析、access rules、密码校验、OTP/NDA 门控。
- 提供 workspace 级 API：`/deal-rooms/:id/links`、 `/links/:id/access-rules`、 `/links/:id/invitations`、 `/links/:id/invitations/:invitationId/revoke`。

当前已实现主体功能并 E2E 通过，剩余安全加固与审计能力需收尾。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 功能设计 | `/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §5 / §6 / §7 |
| 对齐报告 | ../../reviews/DESIGN-ALIGNMENT-huntress-spectre-falcon.md |
| 最终评审 | ../../reviews/FINAL-REVIEW.md §3.2 |
| 已有代码 | `apps/api/internal/link/service.go` / `handler.go` / `session.go` |
| 迁移 | `apps/api/internal/db/migrations/042_deal_room_sharing.up.sql` |

---

## 3. 输入

### 3.1 数据模型

```sql
-- links 新增
ALTER TABLE links ADD COLUMN deal_room_id UUID;
ALTER TABLE links ADD COLUMN require_password BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE links ADD COLUMN password_hash TEXT;

-- link_access_rules
CREATE TABLE link_access_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    rule_type TEXT NOT NULL CHECK (rule_type IN ('email','domain')),
    value TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('allow','block')),
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (link_id, rule_type, value, action)
);

-- link_invitations
CREATE TABLE link_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token_hash TEXT,              -- HMAC-SHA256 hash；原始 token 仅返回一次
    token TEXT,                   -- 保留用于 lazy backfill，新行可为 NULL
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','opened','verified','expired','revoked')),
    expires_at TIMESTAMPTZ,
    used_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (link_id, email)
);
```

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| link 类型互斥 | `document_id` 与 `deal_room_id` 只能一个非空 | DB check + 应用层校验 |
| allow rules 强制 require_email | 存在 allow 规则时 `require_email=true` | 创建/更新时校验 |
| 密码 | `require_password=true` 时 `password_hash` 非空 | bcrypt cost ≥ 10 |
| 邀请 token | 原始 token 仅返回一次；DB 存 hash | ✅ 已实现 HMAC-SHA256 hash |
| session 失效 | 规则/密码/过期变更后旧 session 失效 | ✅ 已改用 `links.security_version` |

### 3.3 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 同时设置 document_id 与 deal_room_id | 创建 link 请求 | `400 invalid_link_type` |
| allow rule 存在但 require_email=false | 更新 access rules | `400 allow_requires_email` |
| 密码长度不足 | `require_password=true` + 密码 "123" | `400 password_min_length` |
| 重复 invite 同一邮箱 | 已存在 pending/verified 邀请 | 幂等：不重复创建，可重发邮件 |
| invite token 被篡改 | 访问 `/l/:token?inviteToken=xxx` | `403 invite_token_invalid` |
| 规则变更后旧 session 访问 | 已签发 session | `401 session_invalidated` |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/047_invitation_token_hash_and_security_version.up.sql` | 新增 | token hash + `security_version`（由 INFRA-001 统一编排） |
| `apps/api/internal/link/service.go` | 修改 | token hash、规则评估、session 失效、access request 接口 |
| `apps/api/internal/link/handler.go` | 修改 | 新增 access request API |
| `apps/api/internal/link/session.go` | 修改 | 支持 `security_version` |
| `apps/api/internal/db/queries.sql` | 修改 | 按 token hash 查询、access requests CRUD |
| `apps/api/internal/link/service_integration_test.go` | 修改 | 补充 token hash、access request 测试 |

### 4.2 行为定义

- `InviteViewers` 生成随机 token，返回原始值；DB 存 hash。
- `ResolveInviteToken` 按 hash 查询。
- `UpdateAccessRules` 保留旧规则快照到 `link_access_rule_revisions`。
- 规则/密码变更时递增 `links.security_version`，旧 session 失效。
- 匿名访客可提交 `link_access_requests`（email + reason），创建者收到通知。

---

## 5. 验收标准

- [x] `link_invitations.token_hash` 列存在且生效；不再明文存储 token。
- [x] 邀请链接 `/l/:token?inviteToken=xxx` 按 hash 解析与访问。
- [x] 存在 allow 规则时 `require_email` 自动开启并校验。
- [x] 规则变更后旧 `LinkSession` 失效（security_version）。
- [x] 密码变更后旧 `LinkSession` 失效（`UpdateLink` 中 bump `security_version`）。
- [x] `link_access_requests` 表支持创建、列出、审批/拒绝。
- [x] `go test ./internal/link/...` 全绿。
- [ ] `apps/api/e2e-test.sh` 全绿（需补充邀请/请求访问场景）。

---

## 6. 实现步骤建议

1. 基于 INFRA-001 已创建的 047/048/049 migration 修改代码，**本任务不再新增 migration**。
2. 修改 `InviteViewers`：生成 token，存 hash，返回原始 token。
3. 修改 `ResolveInviteToken`：按 hash 查询。
4. 在 `UpdateAccessRules` 中写入 `link_access_rule_revisions` 快照。
5. 使用 `links.security_version` 替换 `updated_at` session 失效逻辑。
6. 实现 `link_access_requests` CRUD 与通知。
7. 补测试。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/link/...
./e2e-test.sh
make lint
```

---

## 8. 约束与红线

- 不得明文存储邀请 token。
- 不得删除或修改 `security_events` 已有记录。
- 访问规则变更必须触发旧 session 失效。
- 所有 DB 查询必须带 `tenant_id` / `workspace_id` 隔离。

---

## 9. Definition of Done

- [x] 代码实现完成
- [x] 单元/集成测试通过
- [ ] E2E 通过（待补充邀请/请求访问场景后执行）
- [x] lint 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-015`

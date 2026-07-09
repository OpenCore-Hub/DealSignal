---
task_id: TASK-SHARE-SHORT-007
parent_issue: DS-SHARE-017
agent_task_id: AGENT-TASK-SHARE-017
version: v1.0.0
priority: P1
status: 完成
type: fullstack
effort: M
branch: feat/share-short-007-invite-email-access-request
estimated_files: '14'
max_lines: '600'
project_stack: Go 1.25 + Gin + PostgreSQL + React 19 + TypeScript
dependencies:
- INFRA-001
- INFRA-002
- TASK-SHARE-SHORT-002
- TASK-SHARE-SHORT-005
- TASK-SHARE-SHORT-006
ai_red_flags:
- 邀请邮件链接必须包含 inviteToken，且不能泄露 workspace 认证信息
- 访问请求审批必须校验创建者权限，防止越权
- 邮件发送必须异步，不能阻塞 API
- 审批通过后必须自动加入 allow list 并发送邀请邮件
- 不得把访客 PII 写入公开可访问的日志
ai_confidence: medium
pending_confirmation:
- 访问请求由创建者手动审批，审批通过后异步通知请求者。
- 审批通过后同时创建 allow-rule 与 invitation（含 inviteToken），并发送邀请邮件。
available_tools:
- test
- lint
- browse
---

# TASK-SHARE-SHORT-007 邀请邮件、访问通知与请求访问闭环

> **父 Issue**：`DS-SHARE-017`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **状态**：`完成`  
> **类型**：`fullstack`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-short-007-invite-email-access-request`

---

## 1. 目标

补齐分享链路中的邮件与请求闭环：
- 邀请邮件：调用 `InviteViewers` 时发送含 `/l/:token?inviteToken=xxx` 的邮件。
- 访问通知：访客成功访问后异步通知创建者（按 `link.created_by` 的邮箱，尊重 `email_enabled`）。
- 请求访问：被 block / not allowed 的访客可点击 "Request access"，创建者收到审批通知，审批通过后自动加入 allow list 并发送邀请邮件。

当前邀请邮件已发送、访问通知已实现，但请求访问后端流程缺失，且通知邮件仍同步发送。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 功能设计 | `/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §6.7 / §8.3 |
| 对齐报告 | ../../reviews/DESIGN-ALIGNMENT-huntress-spectre-falcon.md |
| 最终评审 | ../../reviews/FINAL-REVIEW.md §2.2 / §3.2 |
| 已有代码 | `apps/api/internal/link/service.go`、`apps/api/internal/notification/service.go`、`apps/web/src/components/viewer/PublicViewerPage.tsx` |

---

## 3. 输入

### 3.1 数据模型

```sql
CREATE TABLE link_access_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    reason TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 邮件异步 | 所有邮件通过 `notifications` 表 + worker 发送 | 避免阻塞 API |
| 收件人 | 通知邮件发给 `link.created_by` 对应 `users.email` | 无邮箱或 `email_enabled=false` 时跳过 |
| 请求访问 | 匿名访客可提交 email + reason | 需人机验证或限流防滥用 |
| 审批联动 | approved 后自动创建 allow rule + invitation | 邮件通知请求者 |

### 3.3 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 创建者邮箱未验证 | 通知触发 | 跳过邮件，记录 warn |
| 邮箱已存在于 block list | 提交访问请求 | 提示该邮箱无法请求访问 |
| 重复提交请求 | 同一 email 已 pending | 幂等，不重复创建 |
| 非创建者审批 | 其他用户调用 approve | `403 forbidden` |
| 邮件服务失败 | SMTP 超时 | worker 重试，入死信 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/049_link_access_requests.up.sql` | 新增 | 访问请求表（由 INFRA-001 统一编排） |
| `apps/api/internal/link/service.go` | 修改 | 访问请求 CRUD、审批联动 |
| `apps/api/internal/link/handler.go` | 修改 | 公共访问请求端点、审批端点 |
| `apps/api/internal/notification/service.go` | 修改 | email 统一入队，不再同步发送 |
| `apps/api/internal/mailer/template/` | 修改/新增 | 邀请邮件、访问通知、请求访问邮件模板 |
| `apps/web/src/components/viewer/PublicViewerPage.tsx` | 修改 | 请求访问表单 |
| `apps/web/src/components/links/share/InviteTab.tsx` | 修改 | 展示/审批访问请求（可选） |

### 4.2 行为定义

- `InviteViewers` 创建邀请后发送邮件，邮件类型 `EmailTypeLinkInvite`。
- 访问成功后，创建 pending 通知，worker 发送邮件给创建者。
- 访客在 blocked/not_allowed 错误页点击 "Request access"，提交 `email` + `reason`。
- 创建者在 Invite Tab 或通知中审批；approved 后自动添加 allow rule 并发送邀请邮件。

---

## 5. 验收标准

- [x] 邀请邮件包含正确的 `?inviteToken=` 链接，通过 `notification.Service.EnqueueEmailJob` 入队。
- [x] 访问通知邮件异步发送，不阻塞 API。
- [x] `link_access_requests` 表支持创建、列出、审批/拒绝。
- [x] 审批通过后自动加入 allow list 并发送邀请邮件。
- [x] `email_enabled=false` 时不发送邮件，Slack 不受影响。
- [x] `go test ./internal/link/...`、`go test ./internal/notification/...` 全绿。
- [x] 公共访问请求端点 `POST /api/v1/public/links/:token/access-requests` 已挂载并接入限流。
- [x] 工作区审批端点 `POST /links/:id/access-requests/:requestId/approve|reject` 已挂载。
- [x] 前端 PublicViewerPage 请求访问表单已实现并跑通测试。

---

## 6. 实现步骤建议

1. 基于 INFRA-001 已创建的 049 migration 修改代码，**本任务不再新增 migration**。
2. 修改 `notification.Service.Enqueue`：email 统一入队，worker 发送。
3. 在 `link.Service` 新增 `RequestAccess`、`ApproveAccessRequest` 方法。
4. 新增公共端点 `POST /api/v1/public/links/:token/access-requests`。
5. 新增认证端点 `POST /links/:id/access-requests/:requestId/approve`。
6. 前端 PublicViewerPage 增加请求访问表单。
7. 补测试。

---

## 7. 测试验证

```bash
# 后端
cd apps/api
go test ./internal/link/...
go test ./internal/notification/...
./e2e-test.sh
make lint

# 前端
cd apps/web
pnpm lint
pnpm typecheck
pnpm test PublicViewerPage
```

---

## 8. 约束与红线

- 不得同步发送邮件阻塞 API。
- 不得把邀请 token 明文写入日志。
- 审批操作必须校验用户为 link 创建者或 workspace 管理员。
- 访问请求提交已接入限流：每 IP 每 link 5 次/小时（Redis 滑动窗口，Redis 故障时 fail-open）。

---

## 9. Definition of Done

- [x] 后端代码实现完成
- [x] 单元/集成测试通过
- [x] 前端请求访问 UI 实现完成
- [ ] E2E 通过（依赖本地 Docker 全栈，未在本轮运行）
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-017`
- [x] lint / typecheck 通过
- [x] 后端 `go test ./...` 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-017`

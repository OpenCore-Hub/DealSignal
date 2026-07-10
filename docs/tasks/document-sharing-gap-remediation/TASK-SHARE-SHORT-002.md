---
task_id: "TASK-SHARE-SHORT-002"
parent_issue: "DS-SHARE-002"
agent_task_id: "AGENT-TASK-SHARE-002"
version: "v1.0.0"
priority: "P0"
status: "已完成"
type: "backend"
effort: "S"
branch: "feat/share-short-002-notification-recipient"
estimated_files: "8"
max_lines: "300"
project_stack: "Go 1.25 + Gin + PostgreSQL + React 19 + TypeScript"
ai_red_flags:
  - "通知收件人必须来自 users 表，不能是环境变量 SMTP_USER"
  - "前端必须暴露 email_enabled 开关"
  - "不得泄露用户邮箱给无权限用户"
  - "保持现有 Slack 通知逻辑不变"
ai_confidence: "high"
pending_confirmation:
  - "email_enabled 默认值是否为 true？"
  - "当 link creator 邮箱未验证时，是否仍发送通知？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-SHORT-002 通知收件人与邮件开关修复

> **父 Issue**：`DS-SHARE-002`  
> **版本**：`v1.0.0`  
> **优先级**：`P0`  
> **类型**：`backend`  
> **预计工作量**：`S`  
> **分支名**：`feat/share-short-002-notification-recipient`

---

## 1. 目标

修复通知系统的两个关键缺陷：
- 将通知收件人从硬编码的 `SMTP_USER` 改为根据 `link.created_by` 查询 `users.email`。
- 在前端集成设置中暴露 `email_enabled` 开关，并正确控制邮件通知发送。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.5 |
| PRD | `docs/backup/PRD-v2.1.0.md` FR-14 |
| API 契约 | `docs/backup/API-SPEC-v2.1.0.md` API-15 / API-16 |

### 2.1 已有代码

- `apps/api/internal/notification/service.go` — `Enqueue` / `SendPending`
- `apps/api/internal/suggestions/service.go` — 生成 `hot_signal` 后调用 `Notifier.Enqueue`
- `apps/api/internal/integration/service.go` — 集成设置 CRUD
- `apps/web/src/components/integrations/` — 前端集成设置页面（需确认）

### 2.2 当前缺陷

- `notification/service.go` 中存在 TODO：实际应查用户邮箱。
- 当前通知发送到 `SMTP_USER`（环境变量）。
- 前端 `IntegrationStatus` 未暴露 `email_enabled`。

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 收件人 | `users.email` | 由 `link.created_by` 关联 |
| 开关 | `notification_settings.email_enabled` | false 时不发送邮件 |
| 兜底 | 若用户无邮箱或邮箱未验证 | 记录 warn，不发送 |
| Slack | 不受 `email_enabled` 影响 | 保持独立 |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 用户关闭邮件通知 | `email_enabled=false` | 邮件通知不发送，Slack 可继续发送 |
| 创建者无邮箱 | `users.email` 为空 | 记录 warn，标记通知失败原因 |
| 创建者邮箱未验证 | `email_verified=false` | 暂不发送或发送（按业务决策） |
| 用户不存在 | `link.created_by` 无对应 user | 记录 error，不 panic |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/notification/service.go` | 修改 | `SendPending` 中根据 `user_id` 查 `users.email` |
| `apps/api/internal/db/queries.sql` | 新增 | `GetUserEmailByID` |
| `apps/api/internal/integration/service.go` | 修改 | 确保 `email_enabled` 可读写 |
| `apps/web/src/types/index.ts` | 修改 | `IntegrationStatus` 增加 `emailEnabled` |
| `apps/web/src/components/integrations/IntegrationSettings.tsx` | 修改 | 增加邮件通知开关 |
| `apps/web/src/lib/apiAdapters.ts` | 修改 | 映射 `email_enabled` ↔ `emailEnabled` |

### 4.2 行为定义

- `Notifier.Enqueue(..., userID, ...)` 保持入队时只存 `user_id`。
- Worker 发送时查询 `users.email`；若 `email_enabled=false` 或邮箱为空，则跳过。
- 前端集成设置页面显示“邮件通知”开关，与后端同步。

---

## 5. 验收标准

- [ ] `hot_signal` 邮件发送到 `link.created_by` 对应的真实邮箱。
- [ ] `email_enabled=false` 时邮件不发送，Slack 不受影响。
- [ ] 前端集成设置可读取/修改 `email_enabled`。
- [ ] 创建者邮箱为空时记录 warn，不 panic。
- [ ] 后端单元测试覆盖收件人查询与开关控制。

---

## 6. 实现步骤建议

1. 新增 `GetUserEmailByID` sqlc 查询。
2. 修改 `notification.Service.SendPending`：按 `user_id` 查邮箱，结合 `email_enabled` 决定是否发送。
3. 确保 `integration.Service` 正确处理 `email_enabled`。
4. 前端 `IntegrationStatus` 与设置表单增加 `emailEnabled`。
5. 补测试。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/notification/...
go test ./internal/integration/...
make lint

cd apps/web
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 不得把 `SMTP_USER` 继续作为收件人。
- 查询 `users.email` 时必须带租户隔离（`tenant_id`）。
- 不得在日志中打印完整邮箱地址。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-002`

---
task_id: "TASK-SHARE-INFRA-002"
parent_issue: "DS-SHARE-INFRA-002"
agent_task_id: "AGENT-TASK-SHARE-INFRA-002"
version: "v1.0.0"
priority: "P0"
status: "已完成"
type: "infra"
effort: "L"
branch: "feat/share-infra-002-async-email-worker"
estimated_files: "12"
max_lines: "700"
project_stack: "Go 1.25 + PostgreSQL + Redis"
dependencies:
  - INFRA-001
ai_red_flags:
  - "邮件必须写入 notifications 表持久化，不能直接 goroutine 发 mailer"
  - "worker 必须具备重试、死信、幂等机制"
  - "不得阻塞 API 请求等待邮件发送"
  - "Slack 通知逻辑保持现状或统一入队"
ai_confidence: "high"
pending_confirmation:
  - "worker 采用轮询还是 Redis 队列驱动？"
  - "死信是否需要人工重发入口？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-INFRA-002 可靠异步通知 worker

> **父 Issue**：`DS-SHARE-INFRA-002`  
> **版本**：`v1.0.0`  
> **优先级**：`P0`  
> **类型**：`infra`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-infra-002-async-email-worker`

---

## 1. 目标

改造当前邮件发送双路径（`notification.Service.Enqueue` 同步发送、link 服务 goroutine 直接发 mailer）为统一持久化队列 + worker：
- 所有 email 类型通知写入 `notifications` 表。
- 后台 worker 轮询/消费并异步发送，支持重试与死信。
- `SMTP_USER` 不再作为收件人兜底。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 完成度追踪 | [`COMPLETION-TRACKING.md`](./COMPLETION-TRACKING.md) §3.6 |
| 相关任务 | SHORT-002、SHORT-007、MID-003 |

### 2.1 当前问题

- `apps/api/internal/notification/service.go:75–87`：`Enqueue("email")` 直接调用 `sendEmail`。
- `apps/api/internal/link/service.go:1234–1256`：访问通知直接 goroutine 调用 mailer。
- `notifications` 表已有，但 worker 只处理 Slack。
- `SMTP_USER` 仍是 fallback 收件人。

---

## 3. 数据模型与状态机

`notifications` 表扩展/使用现有字段：

| status | 含义 | 转移条件 |
|---|---|---|
| `pending` | 等待发送 | 创建时 |
| `processing` | worker 锁定 | worker 取出时 |
| `sent` | 发送成功 | SMTP 返回成功 |
| `failed` | 可重试失败 | 发送失败且重试次数 < max |
| `dead` | 死信 | 重试次数耗尽 |

新增字段（若不存在）：
- `retry_count INT NOT NULL DEFAULT 0`
- `next_attempt_at TIMESTAMPTZ`
- `last_error TEXT`
- `provider_message_id TEXT`

---

## 4. Worker 设计

```go
type Worker struct {
    store  NotificationStore
    mailer Mailer
    cfg    WorkerConfig
}

func (w *Worker) Run(ctx context.Context) error
func (w *Worker) processOne(ctx context.Context, n Notification) error
```

- 轮询间隔：5–10s（可配置）。
- 每次批量取 `processing=false` 且 `next_attempt_at <= now()` 的 `pending`/`failed` 通知。
- 使用 `SELECT FOR UPDATE SKIP LOCKED` 防止多 worker 冲突。
- 重试策略：指数退避，最大 5 次。
- 超过最大重试 → `dead`。

---

## 5. 输出

### 5.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/0XX_notifications_retry.up.sql` | 新增 | retry_count / next_attempt_at / last_error |
| `apps/api/internal/notification/service.go` | 修改 | `Enqueue("email")` 仅写 DB，移除同步发送 |
| `apps/api/internal/notification/worker.go` | 修改/新增 | email worker |
| `apps/api/internal/notification/deadletter.go` | 新增 | 死信处理与告警 |
| `apps/api/internal/link/service.go` | 修改 | 访问通知、邀请邮件统一调用 `notification.Service.Enqueue` |
| `apps/api/internal/server/server.go` | 修改 | 启动 notification worker |
| `apps/api/internal/db/queries.sql` | 新增 | 取待发送、更新状态、死信查询 |

### 5.2 行为定义

- `notification.Service.Enqueue(channel, userID, payload)` 写 `notifications` 表，`channel='email'` 不立即发送。
- worker 异步发送，更新 `status`/`sent_at`/`provider_message_id`。
- 收件人解析：查询 `users.email`；若为空/未验证/无邮箱 → 跳过并记录 warn。
- 移除 `SMTP_USER` fallback。

---

## 6. 验收标准

- [x] 所有邮件通过 `notifications` 表 + worker 发送。
- [x] `Enqueue` 不阻塞 API。
- [x] 失败邮件按指数退避重试，最终入死信。
- [x] `SMTP_USER` 不再作为收件人。
- [x] link 服务的访问通知、邀请邮件都改为 `Enqueue`/`EnqueueEmailJob`。
- [x] worker 在多实例下通过 DB 锁避免重复发送。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/notification/...
go test ./internal/link/...
make lint
```

---

## 8. 约束与红线

- 禁止在业务代码中直接调用 `mailer.Send`。
- 禁止把 `SMTP_USER` 作为收件人兜底。
- 重试不能导致通知重复发送给最终用户。
- 死信必须可观测（日志/metrics/告警）。

---

## 9. Definition of Done

- [x] 代码实现完成
- [x] 单元/集成测试通过
- [x] lint 通过（本任务新增代码无新增告警）
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-INFRA-002`

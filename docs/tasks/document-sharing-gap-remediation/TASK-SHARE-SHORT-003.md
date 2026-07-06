---
task_id: "TASK-SHARE-SHORT-003"
parent_issue: "DS-SHARE-003"
agent_task_id: "AGENT-TASK-SHARE-003"
version: "v1.0.0"
priority: "P0"
status: "待执行"
type: "backend"
effort: "M"
branch: "feat/share-short-003-security-audit-events"
estimated_files: "10"
max_lines: "400"
project_stack: "Go 1.25 + Gin + PostgreSQL + Redis"
ai_red_flags:
  - "安全审计事件必须追加写入，不可修改或删除"
  - "异常告警规则必须可配置阈值，避免误报"
  - "失败事件不得泄露内部实现细节给客户端"
  - "IP 与用户代理需符合隐私合规要求"
ai_confidence: "high"
pending_confirmation:
  - "异常访问告警是否发送邮件/Slack，还是仅写入 alerts 表？"
  - "是否需要为安全事件单独建表，还是复用 access_logs？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-SHORT-003 安全审计事件记录

> **父 Issue**：`DS-SHARE-003`  
> **版本**：`v1.0.0`  
> **优先级**：`P0`  
> **类型**：`backend`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-short-003-security-audit-events`

---

## 1. 目标

补齐文档分享链路的安全审计能力：
- 记录安全门失败事件（密码错误、验证码错误、NDA 拒绝、白名单拒绝）。
- 记录异常访问事件（过期链接访问、超出最大次数、被撤销/删除链接访问）。
- 提供基础异常访问告警（如 1 小时内多地区访问、短时间内大量失败尝试）。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.1 / §4.7 |
| PRD | `docs/backup/PRD-v2.1.0.md` EVT-14 ~ EVT-18 |
| TDD | `docs/backup/TDD-v2.1.0.md` §7.4 |

### 2.1 已有代码

- `apps/api/internal/link/handler.go` — `Access`, `SendEmailVerificationCode`, `RecordEvent`
- `apps/api/internal/link/service.go` — 密码/验证码/白名单/NDA 校验
- `apps/api/internal/db/migrations/004_links_analytics.up.sql` — `access_logs` 表

### 2.2 当前缺陷

- 仅成功事件（`link_opened`, `download_attempted`）被记录。
- 安全失败与异常访问只返回错误，未写入审计日志。

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 事件类型 | 追加写入 | 不可修改或删除 |
| 存储位置 | 复用 `access_logs` 或新建 `security_events` | 需决策 |
| 保留期限 | ≥ 90 天 | 建议可配置 |
| 告警阈值 | 可配置 | 如 5 次失败/15min/IP |
| 隐私 | 存储 IP/UA | 符合最小必要原则 |

### 3.2 新增事件类型

| 事件 | 触发场景 |
|---|---|
| `security_gate_failed` | 密码/验证码/白名单/NDA 校验失败 |
| `expired_link_accessed` | 访问已过期链接 |
| `max_access_reached` | 链接访问次数已达上限 |
| `revoked_link_accessed` | 访问被撤销/删除链接 |
| `abnormal_access_pattern` | 聚合判定：多地区/高频失败 |

### 3.3 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 密码错误 | 提交错误密码 | 返回 `401 invalid_password`，记录 `security_gate_failed` |
| 验证码错误 | 提交错误 code | 返回 `401 invalid_code`，记录 `security_gate_failed` |
| 白名单拒绝 | 邮箱不在 allowed_emails | 返回 `403 not_allowed`，记录 `security_gate_failed` |
| 过期访问 | `expires_at` 已过 | 返回 `410 link_expired`，记录 `expired_link_accessed` |
| 高频失败 | 同一 IP 5 分钟内 10 次失败 | 记录并触发 `abnormal_access_pattern` |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/0XX_security_events.up.sql` | 新增 | 安全事件表（若选择新建） |
| `apps/api/internal/db/queries.sql` | 新增/修改 | 安全事件写入与查询 |
| `apps/api/internal/analytics/service.go` | 修改 | 新增 `RecordSecurityEvent` |
| `apps/api/internal/link/handler.go` | 修改 | 失败分支调用安全事件记录 |
| `apps/api/internal/link/service.go` | 修改 | 校验失败处传入失败原因 |
| `apps/api/internal/alert/service.go` | 新增（可选） | 异常模式检测与告警 |
| `apps/api/internal/alert/rules.go` | 新增（可选） | 阈值规则 |

### 4.2 行为定义

- 所有安全相关失败在返回错误前写入审计表。
- 审计记录包含：`link_id`, `event_type`, `visitor_id`, `email`, `ip`, `user_agent`, `reason`, `created_at`。
- 异常模式检测在事件写入后异步执行（或在读取时聚合）。

---

## 5. 验收标准

- [ ] 密码/验证码/白名单/NDA 失败被记录。
- [ ] 过期/撤销/超限链接访问被记录。
- [ ] 异常访问模式可触发告警（邮件/Slack 或 alerts 表）。
- [ ] 安全事件表/字段有适当索引。
- [ ] 后端单元测试覆盖主要失败场景。
- [ ] `go test ./...` 与 `make lint` 全绿。

---

## 6. 实现步骤建议

1. 决策：复用 `access_logs`（扩展 event_type 枚举）或新建 `security_events` 表。
2. 新增 migration 与 sqlc 查询。
3. 在 `analytics.Service` 增加 `RecordSecurityEvent`。
4. 在 `link.Handler` / `link.Service` 所有失败分支调用记录。
5. 实现异常模式检测（可选，可放到 TASK-SHARE-MID-003 通知规则引擎中复用）。
6. 补测试。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/link/...
go test ./internal/analytics/...
make lint
```

---

## 8. 约束与红线

- 审计事件必须追加写入，禁止 UPDATE/DELETE。
- 不得因为审计写入失败而阻塞正常错误响应。
- 失败原因字段枚举化，避免自由文本导致分析困难。
- 告警阈值必须可配置，不能硬编码。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-003`

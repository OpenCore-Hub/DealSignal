---
task_id: "TASK-SHARE-MID-003"
parent_issue: "DS-SHARE-007"
agent_task_id: "AGENT-TASK-SHARE-007"
version: "v1.0.0"
priority: "P1"
status: "待执行"
type: "backend"
effort: "L"
branch: "feat/share-mid-003-notification-rules"
estimated_files: "14"
max_lines: "800"
project_stack: "Go 1.25 + Gin + PostgreSQL + Redis"
ai_red_flags:
  - "规则引擎必须幂等，避免重复通知"
  - "事件合并窗口必须可配置"
  - "安全相关通知不可退订"
  - "规则变更后不影响已入队通知"
ai_confidence: "medium"
pending_confirmation:
  - "通知规则存储在 DB 还是配置文件？"
  - "事件合并是按 visitor 合并还是按 link 合并？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-MID-003 通知规则引擎与事件合并

> **父 Issue**：`DS-SHARE-007`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **类型**：`backend`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-mid-003-notification-rules`

---

## 1. 目标

实现可配置的通知规则引擎与事件合并机制：
- 支持首次打开、重复关键页、多人转发、异常访问等规则触发通知。
- 默认 10 分钟事件合并窗口，避免通知轰炸。
- 支持每日摘要开关；安全通知不可退订。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.5 / §4.6 |
| PRD | `docs/backup/PRD-v2.1.0.md` FR-14 / §8.2.7 |

### 2.1 已有代码

- `apps/api/internal/notification/service.go` — 通知入队/发送
- `apps/api/internal/suggestions/service.go` — 仅 `hot_signal` 触发邮件
- `apps/api/internal/db/migrations/008_notify_integrations.up.sql` — `notification_settings`

---

## 3. 输入

### 3.1 规则类型

| 规则 | 触发条件 | 默认启用 | 可退订 |
|---|---|---|---|
| `first_open` | link 首次被打开 | 是 | 是 |
| `repeat_key_page` | 同一 visitor 24h 内多次查看关键页 | 是 | 是 |
| `forward_signal` | 新 visitor 数在 1h 内达到阈值 | 是 | 是 |
| `abnormal_access` | 多地区/高频失败 | 是 | 否 |
| `hot_signal` | 热度 hot + opens≥2 | 是 | 是 |
| `daily_digest` | 每日汇总 | 否 | 是 |

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 合并窗口 | 10 分钟 | 同规则同 link 的事件合并为一条通知 |
| 去重 | 24h | 同规则同 link 每天最多一次（除 abnormal） |
| 渠道 | email / slack | 规则可单独配置 |
| 安全规则 | 不可退订 | abnormal_access 始终发送 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/0XX_notification_rules.up.sql` | 新增 | `notification_rules` 表 |
| `apps/api/internal/db/queries.sql` | 新增 | 规则 CRUD 与合并查询 |
| `apps/api/internal/notification/rules.go` | 新增 | 规则引擎核心 |
| `apps/api/internal/notification/service.go` | 修改 | 支持按规则入队与合并 |
| `apps/api/internal/notification/worker.go` | 修改 | 发送时读取规则配置 |
| `apps/api/internal/suggestions/service.go` | 修改 | 触发点改为调用规则引擎 |
| `apps/api/internal/integration/service.go` | 修改 | 规则设置 CRUD |

### 4.2 行为定义

- 事件发生后，规则引擎评估所有匹配规则。
- 若命中合并窗口内已存在的通知，则合并内容而非新建。
- 否则创建 pending 通知，由 worker 发送。

---

## 5. 验收标准

- [ ] `notification_rules` 表支持创建/读取/更新/删除。
- [ ] 首次打开、重复关键页、转发、异常访问等规则可触发通知。
- [ ] 10 分钟合并窗口生效。
- [ ] 安全规则不可退订。
- [ ] 每日摘要可开关。
- [ ] 后端单元测试覆盖规则匹配与合并。

---

## 6. 实现步骤建议

1. 设计 `notification_rules` schema（rule_type, event_type, conditions, channels, enabled, unsubscribable）。
2. 新增 migration 与 sqlc 查询。
3. 实现 `notification.RuleEngine`：匹配事件、检查合并窗口、创建/合并通知。
4. 替换 `suggestions/service.go` 中硬编码的 `hot_signal` 通知触发。
5. 修改 `notification.Worker` 读取规则配置。
6. 补测试。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/notification/...
go test ./internal/suggestions/...
make lint
```

---

## 8. 约束与红线

- 规则引擎必须幂等：同一事件多次处理不产生重复通知。
- 合并窗口内通知内容更新不能重置发送时间。
- 安全规则（abnormal_access）不受 `email_enabled` 退订影响。
- 不得破坏现有 `hot_signal` 通知行为。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-007`

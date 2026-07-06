---
task_id: "TASK-SHARE-LONG-004"
parent_issue: "DS-SHARE-013"
agent_task_id: "AGENT-TASK-SHARE-013"
version: "v1.0.0"
priority: "P2"
status: "待执行"
type: "backend"
effort: "L"
branch: "feat/share-long-004-crm-deep-integration"
estimated_files: "14"
max_lines: "800"
project_stack: "Go 1.25 + PostgreSQL + HubSpot/Salesforce API"
ai_red_flags:
  - "CRM 同步必须异步执行，不能阻塞主流程"
  - "失败必须有重试与死信机制"
  - "不得删除或覆盖 CRM 中已有数据"
  - "同步记录必须可审计"
ai_confidence: "low"
pending_confirmation:
  - "优先 HubSpot 还是 Salesforce？"
  - "timeline activity 的格式与字段如何映射？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-LONG-004 CRM 深度集成（Timeline / Deal Stage / Task）

> **父 Issue**：`DS-SHARE-013`  
> **版本**：`v1.0.0`  
> **优先级**：`P2`  
> **类型**：`backend`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-long-004-crm-deep-integration`

---

## 1. 目标

将文档分享的热度、信号、活动深度同步到 CRM：
- HubSpot：写入 Contact Timeline Activity、更新 Deal Stage、创建 Task。
- Salesforce：写入 Activity/Task、更新 Opportunity Stage、更新 Contact。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.5 |
| PRD | `docs/backup/PRD-v2.1.0.md` FR-15 |

### 2.1 已有代码

- `apps/api/internal/integration/service.go` — 已有 HubSpot 联系人与 deal 同步

---

## 3. 输入

### 3.1 同步内容

| 触发条件 | CRM 动作 |
|---|---|
| 新 visitor 打开 link | HubSpot: create/update contact；Salesforce: create/update lead/contact |
| 热度达到 hot | 更新 deal/opportunity stage → "Hot Lead" |
| 生成信号 | 创建 timeline activity / task："Follow up on Q3 pitch" |
| 查看关键页 | 添加 timeline note："Viewed pricing page for 45s" |
| AI 高意向问题 | 创建 task："Answer pricing question" |

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 异步 | 消息队列或 worker | 不阻塞 API |
| 幂等 | 外部 ID 映射 | 避免重复创建 |
| 失败重试 | 3 次指数退避 | 后入死信 |
| 字段映射 | 可配置 | 不同客户字段不同 |
| 审计 | `crm_sync_logs` 表 | 记录每次同步结果 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/0XX_crm_sync_logs.up.sql` | 新增 | `crm_sync_logs` 表 |
| `apps/api/internal/integration/crm_sync.go` | 新增 | 通用 CRM 同步器 |
| `apps/api/internal/integration/hubspot.go` | 修改 | timeline / deal / task 同步 |
| `apps/api/internal/integration/salesforce.go` | 新增（可选） | Salesforce 同步 |
| `apps/api/internal/integration/worker.go` | 新增 | 异步同步 worker |
| `apps/api/internal/signal/service.go` | 修改 | 信号生成后 enqueue CRM 同步 |
| `apps/api/internal/contact/service.go` | 修改 | 联系人更新后 enqueue 同步 |

### 4.2 行为定义

- 信号/关键事件产生后，写入 `crm_sync_queue`（或 Redis 队列）。
- worker 消费并调用对应 CRM API。
- 结果写入 `crm_sync_logs`。

---

## 5. 验收标准

- [ ] HubSpot timeline 能记录查看事件与信号。
- [ ] HubSpot deal stage 能随热度变化更新。
- [ ] HubSpot task 能随信号创建。
- [ ] 同步失败可重试，并有死信记录。
- [ ] 同步日志可审计。

---

## 6. 实现步骤建议

1. 设计 `crm_sync_logs` 与队列表。
2. 抽象 `CRMProvider` 接口（HubSpot / Salesforce）。
3. 实现 HubSpot timeline / deal / task API 调用。
4. 实现异步 worker。
5. 在 signal/contact 关键节点 enqueue。
6. 补测试（大量 mock）。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/integration/...
make lint
```

---

## 8. 约束与红线

- 同步必须异步，不能阻塞 API 响应。
- 必须处理 CRM API 限流（429）。
- 不得覆盖 CRM 中用户手动修改的数据。
- 测试必须使用 CRM sandbox/mock，禁止调用生产 API。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-013`

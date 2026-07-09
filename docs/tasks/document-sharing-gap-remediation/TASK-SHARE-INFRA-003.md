---
task_id: "TASK-SHARE-INFRA-003"
parent_issue: "DS-SHARE-INFRA-003"
agent_task_id: "AGENT-TASK-SHARE-INFRA-003"
version: "v1.0.0"
priority: "P1"
status: "待执行"
type: "infra"
effort: "M"
branch: "feat/share-infra-003-event-retention"
estimated_files: "10"
max_lines: "500"
project_stack: "Go 1.25 + PostgreSQL"
dependencies:
  - INFRA-001
  - TASK-SHARE-SHORT-003
  - TASK-SHARE-SHORT-004
ai_red_flags:
  - "retention 策略必须可配置，不能硬编码"
  - "清理任务必须按 tenant 分批，避免长事务锁表"
  - "不得删除或修改仍在合规保留期内的数据"
  - "分区方案必须兼容现有查询与索引"
ai_confidence: "medium"
pending_confirmation:
  - "采用表分区还是按日期归档到 cold storage？"
  - " retention 默认值：access_logs 90 天，page_views 90 天，security_events 180 天？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-INFRA-003 事件与 Analytics Retention

> **父 Issue**：`DS-SHARE-INFRA-003`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **类型**：`infra`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-infra-003-event-retention`

---

## 1. 目标

为高频写入的事件表建立 retention / partitioning 策略，防止 `access_logs`、`page_views`、`security_events` 无限增长拖垮查询与存储。

---

## 2. 上下文

| 表 | 当前状态 | 风险 |
|---|---|---|
| `access_logs` | 每次 link 打开写入 | 高频 |
| `page_views` | 每次页面切换写入 | 高频 |
| `security_events` | 每次失败/异常写入 | 中频但需长期保留 |

---

## 3. 策略

| 表 | 默认保留 | 方案 |
|---|---|---|
| `access_logs` | 90 天 | 按 `created_at` 月分区 + cron 删除过期分区 |
| `page_views` | 90 天 | 按 `created_at` 月分区 + cron 删除过期分区 |
| `security_events` | 180 天 | 按 `created_at` 月分区，单独备份 |

配置项：
- `ACCESS_LOGS_RETENTION_DAYS`
- `PAGE_VIEWS_RETENTION_DAYS`
- `SECURITY_EVENTS_RETENTION_DAYS`

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/0XX_event_tables_partition.up.sql` | 新增 | 为上述表建分区（或重建成分区表） |
| `apps/api/internal/cron/retention.go` | 新增 | 每日清理过期分区/行 |
| `apps/api/internal/config/config.go` | 修改 | retention 配置 |
| `apps/api/internal/db/queries.sql` | 修改（可选） | 确保分区裁剪有效 |
| `apps/api/internal/analytics/service.go` | 修改（可选） | 写入时无需变更 |

### 4.2 行为定义

- 新数据写入当前分区。
- 每日 cron 删除超过 retention 的最旧分区（或按行删除若未分区）。
- 清理前可导出到 S3/MinIO 归档（可选）。

---

## 5. 验收标准

- [ ] 三个表按时间分区（或等效 retention 方案）。
- [ ] cron 每日清理过期数据，不锁表。
- [ ] retention 天数可配置。
- [ ] 现有查询性能不下降（分区裁剪生效）。
- [ ] 清理操作可审计（记录删除行数/分区）。

---

## 6. 测试验证

```bash
cd apps/api
go test ./internal/cron/...
go test ./internal/analytics/...
make lint
```

---

## 7. 约束与红线

- 不得删除未达 retention 期限的数据。
- 清理任务必须可观测（日志/metrics）。
- 必须考虑多租户：不能误删其他 tenant 数据。
- 分区改造必须有回滚方案。

---

## 8. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-INFRA-003`

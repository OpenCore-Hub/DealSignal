---
task_id: "TASK-SHARE-SHORT-004"
parent_issue: "DS-SHARE-004"
agent_task_id: "AGENT-TASK-SHARE-004"
version: "v1.0.0"
priority: "P1"
status: "已完成"
type: "backend"
effort: "M"
branch: "feat/share-short-004-event-deduplication"
estimated_files: "10"
max_lines: "500"
project_stack: "Go 1.25 + Gin + PostgreSQL + Redis"
ai_red_flags:
  - "去重不能丢失首次事件的真实时间戳"
  - "必须兼容已有数据，避免破坏历史统计"
  - "去重窗口必须可配置"
  - "Redis 不可用时必须有 DB 兜底，且兜底行为一致"
  - "并发场景下必须使用原子操作避免重复事件"
ai_confidence: "medium"
pending_confirmation:
  - "去重是否回溯历史数据，还是仅作用于新事件？"
  - "visitor_id 没有邮箱时（公开链接）是否仍按 IP+UA 去重？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-SHORT-004 访问与页面浏览基础去重

> **父 Issue**：`DS-SHARE-004`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **类型**：`backend`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-short-004-event-deduplication`

---

## 1. 目标

实现设计文档定义的去重规则：
- **30 分钟会话去重**：同一 visitor 30 分钟内多次 `link_opened` 只计一次 open。
- **5 分钟页面去重**：同一 visitor 5 分钟内重复查看同一页只计一次 page view。

**去重存储采用 Redis 为主、DB 兜底的双层架构**。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.4 |
| 算法文档 | `docs/backup/HEAT-SCORE-ALGORITHM-v2.1.1.md` §3.1 |

### 2.1 已有代码

- `apps/api/internal/analytics/service.go` — `RecordLinkOpened`, `RecordPageView`
- `apps/api/internal/db/queries.sql` — 事件写入查询
- `apps/api/internal/link/session.go` — 15 分钟滑动 session
- `apps/api/internal/redis/` 或 `apps/api/internal/config/config.go` — Redis 连接

### 2.2 当前缺陷

- 每次成功访问都写入 `link_opened`。
- 每次页面切换都写入 `page_viewed`。
- 无显式去重逻辑，刷新/重复点击会夸大热度。

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| Open 去重窗口 | 30 分钟 | 同一 `(link_id, visitor_id)` 窗口内只计一次 |
| Page view 去重窗口 | 5 分钟 | 同一 `(link_id, visitor_id, page_number)` 窗口内只计一次 |
| 主存储 | Redis | 使用 TTL key 标记窗口 |
| 兜底存储 | PostgreSQL | Redis 不可用时查询最近事件 |
| 时间基准 | 事件创建时间 | 以最近一次成功事件为准 |
| 兼容性 | 仅作用于新事件 | 历史数据不回溯（默认） |
| 并发控制 | Redis 原子操作 | `SET NX EX` 或 Lua 脚本 |

### 3.2 Redis Key 设计

| 事件 | Key | TTL | Value |
|---|---|---|---|
| `link_opened` | `dedup:link_open:{link_id}:{visitor_id}` | 30min | 首次事件发生时间戳（ISO 8601） |
| `page_viewed` | `dedup:page_view:{link_id}:{visitor_id}:{page_number}` | 5min | 首次事件发生时间戳（ISO 8601） |

**Key 构造示例**：
```
dedup:link_open:550e8400-e29b-41d4-a716-446655440000:visitor_abc123
dedup:page_view:550e8400-e29b-41d4-a716-446655440000:visitor_abc123:3
```

### 3.3 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 30 分钟内重复打开 | 同一 visitor 10 分钟内再次 open | Redis key 存在，不计为新 open，不递增 `access_count` |
| 5 分钟内重复查看同页 | 同一 visitor 2 分钟内再次查看第 3 页 | Redis key 存在，不计为新 page view |
| 跨页快速切换 | 1 分钟内从第 1 页切到第 2 页 | 两页都计，因 page_number 不同 |
| Redis 故障 | Redis 连接断开 | 降级为 DB 查询去重，保证可用性 |
| 多实例并发写入 | 同一 visitor 同时请求两个实例 | `SET NX EX` 原子操作保证仅一个成功 |
| visitor_id 缺失 | 无邮箱且 UA 相同 | 按派生 visitor_id 去重 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/analytics/dedup.go` | 新增 | 去重器接口与 Redis 实现 |
| `apps/api/internal/analytics/dedup_test.go` | 新增 | 去重器单元测试 |
| `apps/api/internal/analytics/service.go` | 修改 | `RecordLinkOpened` / `RecordPageView` 先去重 |
| `apps/api/internal/redis/redis.go` | 修改（可选） | 若不存在则新增基础 Redis 操作封装 |
| `apps/api/internal/db/queries.sql` | 新增 | `GetLastLinkOpenByVisitor`, `GetLastPageViewByVisitorPage`（DB 兜底用） |
| `apps/api/internal/config/config.go` | 修改 | 新增去重窗口配置与 Redis 兜底开关 |
| `apps/api/internal/analytics/service_test.go` | 修改 | 补去重测试 |

### 4.2 去重器接口设计

```go
package analytics

type DedupChecker interface {
    // IsDuplicateOpen 判断指定 link + visitor 在给定窗口内是否已打开
    IsDuplicateOpen(ctx context.Context, linkID, visitorID string, window time.Duration) (bool, error)
    
    // IsDuplicatePageView 判断指定 link + visitor + page 在给定窗口内是否已查看
    IsDuplicatePageView(ctx context.Context, linkID, visitorID string, pageNumber int, window time.Duration) (bool, error)
    
    // MarkOpen / MarkPageView 在成功写入事件后标记去重窗口
    MarkOpen(ctx context.Context, linkID, visitorID string, window time.Duration) error
    MarkPageView(ctx context.Context, linkID, visitorID string, pageNumber int, window time.Duration) error
}
```

### 4.3 行为定义

1. **Redis 正常时**：
   - 调用 `IsDuplicateOpen` / `IsDuplicatePageView`。
   - 若返回 `true`：跳过事件写入与 `access_count` 递增。
   - 若返回 `false`：正常写入事件，然后调用 `MarkOpen` / `MarkPageView`。

2. **Redis 故障时**：
   - 降级为 DB 查询最近事件。
   - 行为与 Redis 一致：窗口内存在则跳过，不存在则写入。

3. **窗口过期后**：
   - Redis TTL 自然过期；DB 查询作为兜底仍可正确判断。

4. **`access_count` 保护**：
   - 去重命中时不递增 `access_count`。

---

## 5. 验收标准

- [ ] 30min 内同一 visitor 的重复 `link_opened` 被 Redis 去重。
- [ ] 5min 内同一 visitor 同一页的重复 `page_viewed` 被 Redis 去重。
- [ ] Redis 故障时自动降级为 DB 去重，行为一致。
- [ ] `links.access_count` 不被重复消耗。
- [ ] 并发场景下无重复事件写入。
- [ ] 去重窗口可配置。
- [ ] 单元测试覆盖 Redis 命中、Redis 未命中、DB 兜底、并发场景。
- [ ] `go test ./...` 与 `make lint` 全绿。

---

## 6. 实现步骤建议

1. 确认/完善 `internal/redis` 包，提供 `SET NX EX`、`GET`、`Exists` 等基础操作。
2. 新增 `internal/analytics/dedup.go`：
   - 定义 `DedupChecker` 接口。
   - 实现 `RedisDedupChecker`（主）。
   - 实现 `DBDedupChecker`（兜底）。
   - 实现 `FailoverDedupChecker` 组合两者。
3. 在 `RecordLinkOpened` / `RecordPageView` 中注入 `DedupChecker`。
4. 修改事件写入流程：先检查去重，再写入，成功后标记 Redis。
5. 新增 sqlc 查询用于 DB 兜底。
6. 增加配置项：
   - `LINK_OPEN_DEDUP_WINDOW_MINUTES`（默认 30）
   - `PAGE_VIEW_DEDUP_WINDOW_MINUTES`（默认 5）
   - `DEDUP_REDIS_ENABLED`（默认 true）
7. 补单元测试，使用 miniredis 或 Redis mock。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/analytics/...
make lint
```

### 7.1 关键测试场景

| 场景 | 预期 |
|---|---|
| 首次 open | 写入事件，Redis 设置 TTL key |
| 30min 内重复 open | 不写入事件，Redis key 仍存在 |
| 30min 后再次 open | 允许写入，重新设置 TTL key |
| Redis 断开 + 10min 内重复 open | DB 兜底查询，不写入事件 |
| 并发重复 open | 仅一个写入成功 |

---

## 8. 约束与红线

- 去重不得删除或修改已有事件。
- 必须保持 `access_count` 与 `access_logs` 一致性。
- 公开/认证 viewer 两套事件入口都要加去重。
- Redis 故障时不得拒绝服务，必须降级到 DB。
- 配置默认值必须与文档一致（30min / 5min）。
- Redis key 必须设置 TTL，避免长期堆积。
- 不得在 Redis key 中写入 PII 明文（visitor_id 已脱敏，符合现有设计）。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] Redis 去重 + DB 兜底均通过测试
- [ ] lint 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-004`

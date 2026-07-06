---
task_id: "TASK-SHARE-LONG-003"
parent_issue: "DS-SHARE-012"
agent_task_id: "AGENT-TASK-SHARE-012"
version: "v1.0.0"
priority: "P2"
status: "待执行"
type: "fullstack"
effort: "L"
branch: "feat/share-long-003-realtime-push"
estimated_files: "18"
max_lines: "1000"
project_stack: "Go 1.25 + Redis + React 19 + WebSocket/SSE"
ai_red_flags:
  - "WebSocket/SSE 必须带认证，防止未授权订阅"
  - "必须实现连接断线重连"
  - "消息必须按 workspace 隔离"
  - "高并发下不能拖垮后端"
ai_confidence: "low"
pending_confirmation:
  - "采用 WebSocket 还是 SSE？"
  - "消息流是否用 Redis Pub/Sub 还是 Kafka？"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

# TASK-SHARE-LONG-003 实时事件推送与 Dashboard 更新

> **父 Issue**：`DS-SHARE-012`  
> **版本**：`v1.0.0`  
> **优先级**：`P2`  
> **类型**：`fullstack`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-long-003-realtime-push`

---

## 1. 目标

实现实时事件推送基础设施：
- 后端通过 WebSocket 或 SSE 向在线 sharer 推送查看事件、信号、通知。
- Dashboard 与 Signals 页面实时更新，无需手动刷新。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.2 / §4.6 |
| PRD | `docs/backup/PRD-v2.1.0.md` FR-14 |
| ARCHITECTURE | `docs/backup/ARCHITECTURE-v2.1.0.md` §6.2 |

### 2.1 已有代码

- `apps/api/internal/notification/worker.go` — 30s 轮询
- `apps/web/src/stores/signalStore.ts` — 信号状态
- `apps/web/src/components/dashboard/DashboardPage.tsx` — 仪表盘

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 协议 | WebSocket 或 SSE | 推荐 SSE（兼容性好、易调试） |
| 认证 | JWT / session cookie | 连接建立时验证 |
| 频道 | 按 workspace | 用户只能订阅自己 workspace |
| 消息类型 | event / signal / notification | 前端根据类型更新状态 |
| 重连 | 指数退避 | 断线后自动重连 |
| 心跳 | 30s | 保活与检测死连接 |

### 3.2 消息格式

```json
{
  "type": "signal",
  "payload": {
    "id": "uuid",
    "link_id": "uuid",
    "type": "hot_signal",
    "priority": "high",
    "created_at": "2026-07-05T10:00:00Z"
  }
}
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/realtime/hub.go` | 新增 | 连接管理与广播 |
| `apps/api/internal/realtime/handler.go` | 新增 | WebSocket/SSE 端点 |
| `apps/api/internal/realtime/redis.go` | 新增 | Redis Pub/Sub 桥接 |
| `apps/api/internal/server/routes.go` | 修改 | 注册实时路由 |
| `apps/api/internal/analytics/service.go` | 修改 | 事件写入后 publish |
| `apps/api/internal/signal/service.go` | 修改 | 信号生成后 publish |
| `apps/api/internal/notification/service.go` | 修改 | 通知入队后 publish |
| `apps/web/src/lib/realtime.ts` | 新增 | 前端实时连接 SDK |
| `apps/web/src/stores/signalStore.ts` | 修改 | 监听实时 signal |
| `apps/web/src/stores/dashboardStore.ts` | 修改（可选） | 监听实时 dashboard 更新 |
| `apps/web/src/components/dashboard/DashboardPage.tsx` | 修改 | 实时更新 UI |

---

## 5. 验收标准

- [ ] 在线用户可实时收到查看事件、信号、通知推送。
- [ ] Dashboard 在收到推送后自动更新。
- [ ] 断线后自动重连并恢复订阅。
- [ ] 用户只能订阅自己 workspace 的频道。
- [ ] 后端负载测试：1k 并发连接稳定。

---

## 6. 实现步骤建议

1. 选择协议（推荐 SSE 先做 MVP）。
2. 实现后端 `realtime.Hub` 与 handler。
3. 集成 Redis Pub/Sub 支持多实例广播。
4. 在事件/信号/通知产生处 publish 消息。
5. 前端实现 `realtime.ts` SDK 与重连逻辑。
6. 在 Dashboard/Signals store 中订阅并更新。
7. 补测试与压测。

---

## 7. 测试验证

```bash
# 后端
cd apps/api
go test ./internal/realtime/...
make lint

# 前端
cd apps/web
pnpm test realtime
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 实时连接必须认证，未认证直接断开。
- 必须限制单个用户连接数，防止资源耗尽。
- 消息 payload 不得包含敏感 PII。
- 多实例部署必须使用 Redis Pub/Sub，不能仅内存广播。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-012`

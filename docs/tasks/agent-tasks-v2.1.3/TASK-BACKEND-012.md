---
task_id: "TASK-BACKEND-012"
parent_issue: "DS-037"
agent_task_id: "AGENT-TASK-037"
version: "v2.1.3"
priority: "P0"
status: "待执行"
type: "backend"
effort: "M"
branch: "feat/agent-task-037-middleware-modules"
estimated_files: "12"
max_lines: "600"
project_stack: "Go 1.25 + Gin + Redis + PostgreSQL + Docker"
ai_red_flags:
  - "限流/幂等中间件必须附带单元测试"
  - "不得在生产配置中硬编码敏感信息"
  - "logger/mailer/redis 模块必须可 mock"
  - "敏感数据不得发送给 LLM"
ai_confidence: "high"
pending_confirmation:
  - "限流策略：按 IP / 用户 / 接口维度？"
  - "幂等 key 存储位置：Redis / DB / 内存？"
available_tools:
  - "test"
  - "lint"
---

# TASK-BACKEND-012 后端中间件与基础模块补全

> **父 Issue**：`DS-037`

---

## 1. 目标

完成并测试当前未落库的新增中间件与基础模块：
- 限流中间件（`internal/middleware/ratelimit.go`）
- 幂等中间件（`internal/middleware/idempotency.go`）
- auth 内存 store（`internal/auth/memory_store.go`）
- logger / mailer / redis 基础模块

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 实施计划 | `docs/IMPLEMENTATION-PLAN-v2.1.3.md` |
| ARCHITECTURE | `docs/ARCHITECTURE-v2.1.0.md` §8 |
| TDD | `docs/TDD-v2.1.0.md` §7.x |
| 父 Issue | `DS-037` |
| 依赖 | `DS-036`（TASK-BACKEND-011） |

### 2.1 已有代码

- `apps/api/internal/middleware/idempotency.go`（未跟踪）
- `apps/api/internal/middleware/idempotency_test.go`（未跟踪）
- `apps/api/internal/middleware/ratelimit.go`（未跟踪）
- `apps/api/internal/middleware/ratelimit_test.go`（未跟踪）
- `apps/api/internal/auth/memory_store.go`（未跟踪）
- `apps/api/internal/auth/verification_test.go`（未跟踪）
- `apps/api/internal/logger/`（未跟踪）
- `apps/api/internal/mailer/`（未跟踪）
- `apps/api/internal/redis/`（未跟踪）

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 限流 | 默认按 IP | 可配置按 user / endpoint |
| 幂等 | 24h TTL | key 来自 `Idempotency-Key` header |
| logger | structured JSON | 与现有 gin logger 兼容 |
| mailer | 可 mock | 开发环境不真发邮件 |
| redis | connection pool | 支持本地 docker-compose |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 限流超限 | 同一 IP 1 秒内 100 次 | 返回 `429 rate_limited` |
| 幂等重复 | 相同 Idempotency-Key | 返回第一次结果，不重复执行 |
| 无效幂等 key | key 为空或过长 | 返回 `400 invalid_idempotency_key` |
| mailer mock | 测试环境 | 写入日志而非真实发送 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/middleware/ratelimit.go` | 完成 | 限流逻辑 |
| `internal/middleware/ratelimit_test.go` | 完成 | 单元测试 |
| `internal/middleware/idempotency.go` | 完成 | 幂等逻辑 |
| `internal/middleware/idempotency_test.go` | 完成 | 单元测试 |
| `internal/auth/memory_store.go` | 完成 | 内存 verification store |
| `internal/auth/verification_test.go` | 完成 | 测试 |
| `internal/logger/*.go` | 完成 | structured logger |
| `internal/mailer/*.go` | 完成 | mailer 接口 + mock |
| `internal/redis/*.go` | 完成 | redis client wrapper |
| `internal/server/server.go` | 修改 | 注册新中间件 |
| `internal/config/config.go` | 修改 | 新增配置项 |

---

## 5. 验收标准

- [ ] 限流/幂等中间件单元测试全绿。
- [ ] auth memory store 测试全绿。
- [ ] logger/mailer/redis 模块可在测试中 mock。
- [ ] 中间件在 server 路由中正确注册。
- [ ] `go test ./...` 与 `make lint` 全绿。

---

## 6. 实现步骤建议

1. 完善 `ratelimit.go`：基于 Redis 或内存滑动窗口。
2. 完善 `idempotency.go`：Redis TTL + 响应缓存。
3. 完善 `memory_store.go`：verification code 存取。
4. 完善 logger/mailer/redis wrapper。
5. 在 server 中按路由注册中间件（敏感写操作先加幂等）。
6. 补测试。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/middleware/...
go test ./internal/auth/...
go test ./internal/logger/...
go test ./internal/mailer/...
go test ./internal/redis/...
make lint
```

---

## 8. 约束与红线

- 限流/幂等中间件必须可配置开关，测试环境可关闭。
- mailer 必须能在测试中替换为 mock。
- redis wrapper 必须处理连接失败降级（不阻塞启动）。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / build 通过
- [ ] PR 已关联父 Issue：`Closes #DS-037`

---

## 10. Agent 备注

- 若 Redis 不可用，限流/幂等可先用内存实现兜底，但需说明线程安全。
- mailer mock 建议写入 logger 以便测试断言。

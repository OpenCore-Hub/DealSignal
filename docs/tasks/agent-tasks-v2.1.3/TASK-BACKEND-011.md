---
task_id: "TASK-BACKEND-011"
parent_issue: "DS-036"
agent_task_id: "AGENT-TASK-036"
version: "v2.1.3"
priority: "P0"
status: "待执行"
type: "backend"
effort: "L"
branch: "feat/agent-task-036-backend-stabilize"
estimated_files: "20"
max_lines: "1200"
project_stack: "Go 1.25 + Gin + pgx + sqlc + Redis + MinIO + Docker"
ai_red_flags:
  - "不得引入破坏性 schema 变更而不写迁移"
  - "所有改动必须通过 go test ./..."
  - "不得泄露 JWT/密钥/S3 凭证到日志或测试"
  - "敏感数据不得发送给 LLM"
ai_confidence: "medium"
pending_confirmation:
  - "当前 backend diff 是否应一次性提交还是拆为多个 PR"
  - "新增迁移 013~016 的执行顺序与合并策略"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

# TASK-BACKEND-011 后端未落库改动整理与接口稳定

> **父 Issue**：`DS-036`

---

## 1. 目标

整理并落库当前工作区中 `apps/api` 的未提交改动，确保 ingestion/integration/search/upload/workspace/server 模块编译通过、测试全绿、接口稳定。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 实施计划 | `docs/IMPLEMENTATION-PLAN-v2.1.3.md` |
| API 规范 | `docs/API-SPEC-v2.1.0.md` |
| TDD | `docs/TDD-v2.1.0.md` §6.x / §7.x |
| 父 Issue | `DS-036` |

### 2.1 已有代码/改动

- `apps/api/internal/ingestion/pdf.go`（+365 行）
- `apps/api/internal/integration/*`
- `apps/api/internal/search/*`
- `apps/api/internal/upload/handler.go`
- `apps/api/internal/workspace/*`
- `apps/api/internal/server/*`
- `apps/api/internal/db/migrations/013~016*.sql`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 编译 | `go build ./...` 全绿 | 不能引入未使用依赖 |
| 测试 | `go test ./...` 全绿 | race detector 通过 |
| 迁移 | 可顺序执行 | 013~016 依赖清晰 |
| 接口 | 不破坏 v2.1.2 已发布契约 | 路径/字段保持兼容 |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 迁移冲突 | 重复执行 migration | 幂等或报错明确 |
| 接口返回字段缺失 | 旧前端调用 | 保持 v2.1.2 字段 |
| 测试 race | `go test -race ./...` | 无 data race |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/ingestion/pdf.go` | 整理 | 确保边界、测试通过 |
| `internal/integration/*` | 整理 | HubSpot/Slack 逻辑稳定 |
| `internal/search/*` | 整理 | handler/service 重构稳定 |
| `internal/upload/handler.go` | 整理 | 与前端 FormData 兼容 |
| `internal/workspace/*` | 整理 | middleware/service 扩展 |
| `internal/server/*` | 整理 | 路由与启动逻辑 |
| `internal/db/migrations/013~016*.sql` | 整理 | 顺序、回滚、幂等 |
| `internal/db/queries.sql` | 可能修改 | 匹配新迁移字段 |
| `internal/db/queries.sql.go` | 重新生成 | `sqlc generate` |
| `e2e-test.sh` / `e2e-full.sh` | 整理 | 验证 P0 路径 |

---

## 5. 验收标准

- [ ] `go test ./...` 全绿（含 `-race`）。
- [ ] `make lint && make build && make security` 全绿。
- [ ] 新增迁移可在干净数据库顺序执行并回滚。
- [ ] P0 E2E 脚本通过。
- [ ] 不破坏 v2.1.2 前端调用。

---

## 6. 实现步骤建议

1. 审查当前 backend diff，识别变更主题。
2. 按模块分组提交（ingestion、integration、search、upload/workspace/server）。
3. 检查迁移脚本顺序与依赖。
4. 运行 `go test ./...` 与 race test。
5. 运行 E2E 脚本。

---

## 7. 测试验证

```bash
cd apps/api
go test ./...
go test -race ./...
make lint
make build
make security
./e2e-test.sh
```

---

## 8. 约束与红线

- 不得引入未审批的 schema 变更。
- 不得删除 v2.1.2 已有接口字段。
- 不得将密钥写入日志。
- 所有新增文件必须包含 package comment 与基础测试（Go 习惯）。

---

## 9. Definition of Done

- [ ] 代码整理完成
- [ ] 测试通过
- [ ] lint / build / security 通过
- [ ] E2E 通过
- [ ] PR 已关联父 Issue：`Closes #DS-036`

---

## 10. Agent 备注

- 当前 diff 较大，建议先 `git add -p` 分主题审查。
- 若发现某模块改动超出本任务范围，应拆分为独立任务。

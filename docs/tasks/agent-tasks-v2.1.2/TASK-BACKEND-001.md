---
task_id: "TASK-BACKEND-001"
parent_issue: "DS-001"
agent_task_id: "AGENT-TASK-004"
version: "v2.1.0"
priority: "P0"
status: "已完成"
type: "backend"
effort: "M"
branch: "feat/agent-task-004-backend-scaffold"
estimated_files: "10"
max_lines: "600"
project_stack: "Go 1.22+ / Gin / Docker / PostgreSQL / Redis"
ai_red_flags:
  - "不得提交敏感配置到仓库"
  - "Dockerfile 不得使用 root 运行服务"
  - "日志不得打印 token/password"
  - "所有配置必须支持环境变量注入"
ai_confidence: "high"
pending_confirmation: []
available_tools:
  - "test"
  - "lint"
  - "docker"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-BACKEND-001` |
> | `parent_issue` | `DS-001` |
> | `agent_task_id` | `AGENT-TASK-004` |
> | **版本** | `v2.1.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `待执行` |
> | **类型** | `backend` |
> | **预计工作量** | `M` |
> | **分支名** | `feat/agent-task-004-backend-scaffold` |
> | **AI 置信度** | `high` |
> | **依赖** | `-` |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-BACKEND-001 Go 后端脚手架：工程结构、配置、HTTP server、Docker

> **父 Issue**：`DS-001`  
> **版本**：`v2.1.0`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`待执行`  
> **类型**：`backend`  
> **预计工作量**：`M`  
> **分支名**：`feat/agent-task-004-backend-scaffold`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

创建 `apps/api` Go 服务骨架，包括标准目录结构、环境变量配置加载、Gin HTTP server、健康检查端点、Docker 多阶段构建与 docker-compose 本地开发栈（Postgres + Redis + API）；预留自定义域名/SSL 接口占位。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8、§16 |
| TDD | `docs/TDD-v2.1.0.md` §3.2 / §9.1 |
| ARCHITECTURE | `docs/ARCHITECTURE-v2.1.0.md` §4 |
| 父 Issue | `DS-001` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 中等；先读 ARCHITECTURE §4 与 TDD §3.2。 |
| **必读顺序** | front matter → `ai_red_flags` → 目标 → 边界条件 → 输出。 |
| **分块策略** | 不需要分块；本任务以新增文件为主。 |

### 3.1 已有代码/表（执行前必须阅读）

- `docs/ARCHITECTURE-v2.1.0.md` §4
- `docs/TDD-v2.1.0.md` §3.2、§9.1
- 现有 `apps/web/src/lib/api.ts`（了解前端调用约定）

### 3.2 数据模型/接口

```go
// Config 结构体示例
type Config struct {
    Port        string `env:"PORT" envDefault:"8080"`
    DatabaseURL string `env:"DATABASE_URL"`
    RedisURL    string `env:"REDIS_URL"`
    JWTSecret   string `env:"JWT_SECRET"`
    LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
}
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| Go 版本 | ≥ 1.22 | 使用标准 toolchain |
| 端口 | 默认 `8080`，可通过 `PORT` 覆盖 | |
| 配置缺失 | 必填项未设置时服务拒绝启动 | 明确返回错误日志 |
| 日志 | JSON 格式 | 方便日志聚合 |
| 最大变更行数 | ≤ 400 | 超出需拆分 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 缺少 DATABASE_URL | 未设置 | 启动失败，日志提示配置缺失 |
| 端口被占用 | `PORT=8080` 已被占用 | 启动失败，日志提示 bind error |
| 健康检查 | `GET /healthz` | 返回 200 + `{"status":"ok"}` |
| 未定义路由 | `GET /not-found` | 返回 404 JSON 错误 |

### 3.5 测试数据 / Mock / Fixture

```bash
# 健康检查响应示例
$ curl -s http://localhost:8080/healthz | jq .
{
  "status": "ok",
  "version": "v2.1.0"
}
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/go.mod` | 新增 | Go 模块定义 |
| `apps/api/cmd/server/main.go` | 新增 | 入口：加载配置、启动 server |
| `apps/api/internal/config/config.go` | 新增 | 环境变量配置与校验 |
| `apps/api/internal/server/server.go` | 新增 | Gin 路由、中间件、健康检查 |
| `apps/api/internal/server/routes.go` | 新增 | 路由注册（暂不实现业务） |
| `apps/api/Dockerfile` | 新增 | 多阶段构建，非 root 运行 |
| `apps/api/docker-compose.yml` | 新增 | Postgres + Redis + API |
| `apps/api/Makefile` | 新增 | 常用命令：build / test / lint / migrate |

### 4.2 行为定义

- `go run ./cmd/server` 启动服务，监听 `PORT`。
- `GET /healthz` 返回 200 JSON。
- 未匹配路由返回统一 JSON 错误格式 `{ "code": "not_found", "message": "..." }`。
- Docker 镜像使用非 root 用户运行。
- `docker compose up --build` 能同时启动 Postgres、Redis 与 API。

---

## 5. 验收标准

- [x] `go build ./cmd/server` 成功
- [x] `GET /healthz` 返回 200
- [x] `docker compose up --build` 能启动 API + Postgres + Redis
- [x] 必填配置缺失时服务明确退出并打印原因
- [x] 无敏感信息（密码/secret）硬编码在源码或镜像中
- [x] `make lint`（或 `golangci-lint run`）通过
- [x] 至少包含一个 server 启动测试

---

## 6. 实现步骤建议

1. 在 `apps/api/` 初始化 Go module。
2. 添加 `internal/config/config.go`，使用 `envconfig` 或 `caarlos0/env` 加载环境变量。
3. 添加 `internal/server/server.go`，初始化 Gin，配置 recovery、logger、CORS、请求 ID。
4. 注册 `GET /healthz` 与 404 handler。
5. 编写 `cmd/server/main.go` 组合 config 与 server。
6. 编写 `Dockerfile`（builder + distroless/non-root runner）。
7. 编写 `docker-compose.yml` 启动 Postgres、Redis、API。
8. 编写 `Makefile` 常用命令。
9. 添加启动/健康检查测试。
10. 运行 `make lint && make test && make build`。
11. 提交 PR。

---

## 7. 测试验证

### 7.1 单元测试

```bash
cd apps/api && go test ./internal/server/...
```

### 7.2 集成测试

```bash
cd apps/api && docker compose up --build -d
sleep 5
curl -s http://localhost:8080/healthz
docker compose down
```

### 7.3 手动验证

```bash
cd apps/api && go run ./cmd/server
# 另一终端
curl -i http://localhost:8080/healthz
```

### 7.4 回归测试命令

```bash
cd apps/api && make lint && make test && make build
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`。
- **范围锁定**：不得实现业务接口（auth/upload/AI 等），本任务只做脚手架。
- **保持现有测试通过**：运行全量回归测试命令全绿才能提交。
- **不要提前实现**：范围外的功能不要碰。
- **禁止把敏感数据发送给 LLM**：Token、密码、密钥、PII、生产数据一律不得出现在 prompt、日志或测试数据中。
- **代码风格**：遵循 Go 标准项目布局与项目命名约定。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

| CODE-REVIEW 3.11 检查项 | 本任务自检要求 |
|------------------------|----------------|
| 无硬编码示例域名/邮箱/密码 | 配置通过环境变量注入；Dockerfile 不设置默认密码。 |
| 无未清理的 TODO / FIXME / placeholder | 实现完成后全局搜索，确保无残留。 |
| 无幻觉常量 | 端口、超时等常量从 config 读取。 |
| 错误处理不过度 try-catch，不吞掉异常 | 配置错误直接返回并退出。 |
| 未引入未使用的依赖或代码 | 提交前运行 `go mod tidy` 与 lint。 |
| 未擅自实现范围外功能 | 仅实现脚手架。 |
| 测试数据与生产数据隔离 | 测试使用 sqlite/memory/docker 本地服务。 |

---

## 10. Definition of Done

- [x] 代码实现完成
- [x] 测试通过（单元 + 集成）
- [x] lint / build 通过
- [ ] 代码审查通过
- [x] 与父 Issue 的验收标准对齐
- [ ] PR 已关联父 Issue：`Closes #DS-001`
- [x] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- 推荐使用 `github.com/gin-gonic/gin` + `github.com/caarlos0/env/v11`。
- Dockerfile 可参考 `gcr.io/distroless/static-debian12:nonroot`。
- 如需 CORS，先允许前端开发地址 `http://localhost:5173`。
- 如果超出 8 个文件，优先将 Makefile / docker-compose 拆到后续 task。

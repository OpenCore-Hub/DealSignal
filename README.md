# DealSignal

智能文档分享与交易信号平台（Signal-First Document Intelligence）。

## 项目结构

```
├── apps/web          # React + Vite 前端应用
├── apps/api          # Go + Gin 后端服务
├── docs/             # PRD、TDD、设计文档、实施计划
├── CHANGELOG.md      # 版本变更日志
├── VERSION           # 当前版本
└── README.md         # 本文件
```

## 技术栈

### 前端

- React 19 + TypeScript
- React Router 8
- Vite 8
- Tailwind CSS 4
- Base UI + 自定义 shadcn 风格组件
- Zustand
- Motion（Framer Motion）
- TanStack Table
- MSW（开发环境 Mock）
- Vitest + React Testing Library
- Playwright（E2E）

### 后端

- Go 1.25
- Gin
- PostgreSQL（pgx / sqlc）
- Redis
- S3-compatible object storage
- OpenAI API
- JWT authentication

## 快速开始

### 前端

```bash
cd apps/web
pnpm install

# 启动开发服务器（含 MSW Mock）
pnpm dev

# 类型检查
pnpm typecheck

# 代码检查
pnpm lint

# 单元测试
pnpm test

# E2E 测试
pnpm exec playwright install --with-deps chromium
pnpm test:e2e

# 构建
pnpm build
```

### 后端

```bash
cd apps/api

# 启动本地依赖（PostgreSQL + Redis + MinIO + OnlyOffice）
docker-compose up -d

# 运行测试
make test

# 代码检查
make lint

# 漏洞扫描
make security

# 构建
make build

# 启动服务（需配置环境变量）
go run ./cmd/server
```

### 全栈（Docker Compose）

```bash
cd apps/api
docker-compose up --build
```

然后访问 `http://localhost:5173`（前端开发服务器）或 `http://localhost:8080`（API）。

## 环境变量

- 前端：`apps/web/.env.example`
- 后端：`apps/api/.env.example`

关键变量：

- `VITE_API_BASE_URL`：前端调用的 API 地址；留空则回退到 MSW。
- `DATABASE_URL`、`REDIS_URL`、`JWT_SECRET`、`S3_*`：后端必需配置。
- `OPENAI_API_KEY`：后端可选；留空则禁用向量搜索与 AI assistant。
- `OPENAI_BASE_URL`、`OPENAI_REFERER`、`OPENAI_APP_TITLE`：OpenAI-compatible / OpenRouter 配置。

## 设计文档

- [实施计划 v2.1.2](./docs/IMPLEMENTATION-PLAN-v2.1.2.md)
- [Issue 拆分清单 v2.1.2](./docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.2.md)
- [前端审计与优化计划 v2.1.3](./docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md)
- [性能报告 v2.1.2](./docs/PERFORMANCE-REPORT-v2.1.2.md)
- [安全策略](./docs/SECURITY.md)
- [产品设计 v2.1.1](./docs/PRODUCT-DESIGN-v2.1.1-REFINED.md)
- [交互规范 v2.1.1](./docs/INTERACTION-SPEC-v2.1.1-REFINED.md)
- [设计令牌 v2.1.1](./docs/DESIGN-TOKENS-v2.1.1.md)
- [API 规范 v2.1.0](./docs/API-SPEC-v2.1.0.md)
- [架构设计 v2.1.0](./docs/ARCHITECTURE-v2.1.0.md)

## 安全扫描

安全策略与风险接受项见 [`docs/SECURITY.md`](./docs/SECURITY.md)。

常用命令：

```bash
# Go 漏洞扫描
cd apps/api && make security

# Trivy 文件系统扫描
cd apps/api && make trivy-fs

# 前端依赖审计
cd apps/web && pnpm security
```

## 端到端验证

```bash
# 后端 P0 E2E（无需 AI key）
cd apps/api && ./e2e-test.sh

# 后端 P0 + AI E2E（本地 mock LLM）
cd apps/api && ./e2e-ai.sh

# 前端真实后端 E2E
cd apps/web && ./e2e-real-backend.sh
```

## 持续集成

- `.github/workflows/ci.yml`：前端类型检查 / Lint / 单元测试 / E2E，后端构建 / 测试 / Lint / 安全扫描。
- `.github/workflows/security.yml`：govulncheck、Trivy fs/image、gitleaks、`pnpm audit`。

## 版本

当前版本见 [`VERSION`](./VERSION)。变更历史见 [`CHANGELOG.md`](./CHANGELOG.md)。

## 贡献指南

1. 遵循 `apps/web/src/index.css` 中的语义化 Token。
2. 所有新增可点击元素必须支持键盘操作（Tab + Enter/Space）。
3. 所有复制、删除、保存等操作必须提供即时反馈。
4. 不要在生产代码中直接引用 `mock*` 数据；Mock 仅用于 MSW handler。
5. 后端修改需通过 `make lint && make test && make security` 方可提交。

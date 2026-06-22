# Agent 工作规范

## 项目背景

DealSignal 是一个面向创始人、投资人和销售团队的智能文档分享与交易信号平台。

- **前端**：React 19 + Vite 8 + Tailwind CSS 4 + Base UI + Zustand。
- **后端**：Go 1.25 + Gin + PostgreSQL（pgx/sqlc）+ Redis + S3-compatible 存储 + OpenAI。
- **当前版本**：`v2.1.2`（见 `VERSION` / `CHANGELOG.md`）。

## 工作原则

1. **设计优先**：任何 UI 改动前，先对照 `docs/DESIGN-TOKENS-v2.1.1.md` 与 `docs/INTERACTION-SPEC-v2.1.1-REFINED.md`。
2. **零假交互**：不要保留 `onClick={() => {}}`。未实现的功能应 `disabled` 并加 Tooltip 说明，或从 UI 移除。
3. **即时反馈**：复制、删除、保存、重置等操作必须提供图标变化、toast 或二次确认。
4. **键盘可达**：所有可点击元素必须可通过 Tab 聚焦，Enter/Space 触发。
5. **中文 SaaS 语境**：界面标签、微文案、数据单位使用中文。
6. **不要引用 Mock**：业务组件应调用 `src/lib/api.ts` 中的 API，不要直接 import `src/lib/mocks/data.ts`。
7. **MSW 仅开发**：生产构建不要启动 MSW；Mock 逻辑只写在 `src/lib/mocks/` 中。
8. **后端安全第一**：Go 依赖和容器镜像需通过 `make security` / Trivy 扫描；不得为绕过扫描而放宽规则。
9. **测试门禁**：前端改动通过 `pnpm test`，后端改动通过 `make test`（含 `-race`）后方可提交。

## 目录约定

### 前端

- `apps/web/src/routes/`：页面入口，尽量保持薄。
- `apps/web/src/components/`：业务与通用组件。
- `apps/web/src/components/ui/`：基础 UI 组件，基于 Base UI 封装。
- `apps/web/src/lib/`：API 客户端、工具函数、类型。
- `apps/web/src/stores/`：Zustand 状态管理。

### 后端

- `apps/api/cmd/server/`：服务入口。
- `apps/api/internal/server/`：HTTP 路由、中间件。
- `apps/api/internal/{domain,upload,search,link,dealroom,...}/`：按业务域组织的 handler / service / repository。
- `apps/api/internal/db/`：sqlc 生成的模型与查询。
- `apps/api/internal/db/migrations/`：数据库迁移文件。
- `apps/api/scripts/loadtest/`：k6 压测脚本。

## 代码风格

### 前端

- 使用 TypeScript 严格模式。
- Tailwind 类名优先使用语义化 Token，避免任意值（如 `text-[10px]`）。
- 组件 props 必须显式类型化。
- 异步请求必须包含错误处理。

### 后端

- 遵循 Go 标准项目布局。
- Handler 只负责 HTTP 转换；业务逻辑放在 service。
- 数据库访问通过 sqlc 生成的 repository，避免手写裸 SQL。
- 错误返回统一 JSON 格式 `{ "code": "...", "message": "..." }`。
- 配置通过环境变量注入，禁止硬编码密钥或默认密码。

## 当前重点

- v2.1.2 已合入 `main`：后端 MVP、前端集成、测试自动化、安全扫描均已就绪。
- 下一步参见 [前端审计与优化计划 v2.1.3](./docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md) 与 backlog issue。

# DealSignal 项目完成进度追踪

> 本文件用于持续跟踪 DealSignal 各版本的实际落地进度。  
> 最后更新：`2026-06-24`  
> 当前分支：`main`  
> 当前版本：`v2.1.2`（已发布），`main` 领先 tag `v2.1.2` 14 个 commit（`v2.1.2-14-g3447cfd`）

---

## 1. 执行摘要

| 维度 | 结论 |
|---|---|
| **v2.1.2 里程碑** | 已 100% 完成并发布（CHANGELOG 日期 2026-06-22） |
| **v2.1.3 工作区** | 存在大量未提交改动：72 个修改文件、35 个未跟踪文件、代码净增约 +4,451/-718 行 |
| **当前阶段** | v2.1.3 前端审计优化 + 后端加固已进入实现，但尚未形成新的 task/issue 清单 |
| **最大风险** | 大段未落库 diff、文档滞后于代码、v2.1.3 阻塞项与工程债务仍需收尾 |

---

## 2. v2.1.2 里程碑（已完成）

| 任务/问题 | 标题 | 优先级 | 状态 |
|---|---|---|---|
| DS-001 | 工程脚手架与项目初始化 | P0 | ✅ 已上线 |
| DS-002 | 用户认证、租户与 Workspace 模块 | P0 | ✅ 已上线 |
| DS-003 | 对象存储与后端签名 URL | P0 | ✅ 已上线 |
| DS-004 | 子域名/自定义域名与 SSL 自动签发 | P1 | ✅ 已上线 |
| DS-005 | 文档上传 API | P0 | ✅ 已上线 |
| DS-006 | PDF Pipeline（bbox + webp） | P0 | ✅ 已上线 |
| DS-007 | Office Pipeline（OnlyOffice 转 PDF） | P0 | ✅ 已上线 |
| DS-008 | 数据库迁移与搜索索引 | P0 | ✅ 已上线 |
| DS-009 | 签名 URL 与权限校验 | P0 | ✅ 已上线 |
| DS-010 | Viewer Canvas 前端 | P0 | ✅ 已上线 |
| DS-011 | Search Service | P0 | ✅ 已上线 |
| DS-012 | Evidence Service | P0 | ✅ 已上线 |
| DS-013 | Assistant Service | P0 | ✅ 已上线 |
| DS-014 | 悬浮 AI 助手前端 | P0 | ✅ 已上线 |
| DS-015 | 智能链接与权限 | P0 | ✅ 已上线 |
| DS-016 | Dashboard 前端 | P1 | ✅ 已上线 |
| DS-017 | 热度评分与 Analytics | P0 | ✅ 已上线 |
| DS-018 | 行为提醒与跟进建议 | P1 | ✅ 已上线 |
| DS-019 | 数据室模块 | P0 | ✅ 已上线 |
| DS-020 | 邮件通知系统 | P1 | ✅ 已上线 |
| DS-021 | CRM 集成（HubSpot/Salesforce） | P2 | ✅ 已上线 |
| DS-022 | Slack 集成 | P2 | ✅ 已上线 |
| DS-023 | 测试用例与自动化 | P0 | ✅ 已上线 |
| DS-024 | 性能压测与优化 | P1 | ✅ 已上线 |
| DS-025 | 安全扫描与修复 | P0 | ✅ 已上线 |
| DS-026 | 前端质量收尾 | P1 | ✅ 已上线 |
| DS-027 | 前端-后端集成层 | P0 | ✅ 已上线 |

### v2.1.2 质量门禁

- [x] 前端 `pnpm lint && pnpm test && pnpm build` 全绿
- [x] 后端 `make lint && make test && make build` 全绿
- [x] `docker compose up --build` 可启动
- [x] `VITE_API_BASE_URL` 可切换真实后端，未配置时回退 MSW
- [x] `make security` 无 HIGH/CRITICAL 漏洞
- [x] 文档与代码在字段、枚举、路径、依赖上保持一致（发布时）

---

## 3. 当前工作区状态（v2.1.3 进行中）

### 3.1 改动统计

| 类别 | 数量 | 备注 |
|---|---|---|
| 已修改文件 | 72 | 均为 `apps/api` 与 `apps/web` 代码 |
| 未跟踪文件 | 35 | 含迁移脚本、新模块、前端新页面、环境文件等 |
| 代码净增量 | +4,451 / -718 | 基于 `git diff --stat` |
| 文档改动 | 0 | `docs/` 目录无 diff，文档滞后于代码 |

### 3.2 后端改动地图

| 模块 | 主要变更 | 状态 |
|---|---|---|
| `internal/ingestion/pdf.go` | PDF bbox/webp 解析大幅扩展 | 🟡 进行中 |
| `internal/integration/*` | HubSpot/Slack 集成逻辑增强 | 🟡 进行中 |
| `internal/search/*` | 搜索 service/handler 重构与测试补强 | 🟡 进行中 |
| `internal/server/*` | 路由与服务端初始化调整 | 🟡 进行中 |
| `internal/upload/handler.go` | 上传接口改造 | 🟡 进行中 |
| `internal/workspace/*` | workspace handler/middleware/service 扩展 | 🟡 进行中 |
| `internal/middleware/*` | 新增限流、幂等中间件（未跟踪文件） | 🟡 进行中 |
| `internal/auth/memory_store.go` | 新增内存 store | 🟡 进行中 |
| `internal/logger/` / `internal/mailer/` / `internal/redis/` | 新增基础模块 | 🟡 进行中 |
| 数据库迁移 | 新增 013~016 号迁移 | 🟡 进行中 |

### 3.3 前端改动地图

| 模块 | 主要变更 | 状态 |
|---|---|---|
| `src/lib/apiClient.ts` + `.test.ts` | 新增 apiClient 测试与改造 | 🟡 进行中 |
| `src/lib/api.ts` | request/Content-Type/FormData 适配 | 🟡 进行中 |
| `src/lib/mocks/handlers.ts` | mock handler 大幅扩展 | 🟡 进行中 |
| `components/upload/Uploader.tsx` | 上传组件重写 | 🟡 进行中 |
| `components/documents/DocumentContent.tsx` | 文档内容展示增强 | 🟡 进行中 |
| `components/layout/TopNav.tsx` | TopNav 微调 | 🟡 进行中 |
| `routes/settings/integrations.tsx` | 集成设置页增强 | 🟡 进行中 |
| `e2e/*` | P0 + real-backend E2E 用例更新 | 🟡 进行中 |
| `router.tsx` | 路由调整 | 🟡 进行中 |
| `login.tsx` / `register.tsx` / `verify-email.tsx` | 新增认证页面（未跟踪） | 🟡 进行中 |

---

## 4. v2.1.3 剩余工作清单

依据 [`docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md`](./FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md)。

### Phase A：阻塞项清零

| # | 问题 | 位置/影响 | 状态 | 备注 |
|---|---|---|---|---|
| A1 | Security「查看审计日志」按钮无响应 | `routes/settings/security.tsx` | 🔲 待处理 | 未在 diff 中观察到改动 |
| A2 | InsightsSuggestions「写跟进邮件」按钮无响应 | `routes/insights/suggestions.tsx` | 🔲 待处理 | 未在 diff 中观察到改动 |
| A3 | TopNav 通知铃铛无响应 | `components/layout/TopNav.tsx` | 🔲 待处理 | TopNav 有微调，需验证 |
| A4 | SmartLinkCreator 复制后无 toast/图标反馈 | `components/links/SmartLinkCreator.tsx` | 🔲 待处理 | 未在 diff 中观察到改动 |
| A5 | SettingsGeneral/Brand/Integrations 保存无 toast/catch | `routes/settings/general.tsx`、`brand.tsx`、`integrations.tsx` | 🔲 待处理 | integrations 有改动，需验证 |
| A6 | Brand Logo 仅本地预览，未持久化 | `routes/settings/brand.tsx` | 🔲 待处理 | 未在 diff 中观察到改动 |
| A7 | TopNav 头像硬编码 "JD"，无账户菜单 | `components/layout/TopNav.tsx` | 🔲 待处理 | 需验证当前改动是否覆盖 |
| A8 | DocumentDetail/LinksTable 仍使用 `window.confirm` | `DocumentDetail.tsx`、`LinksTable.tsx` | 🔲 待处理 | 未确认 |
| A9 | 界面文案中英混杂 | 多处 | 🔲 待处理 | 未确认 |

### Phase B：API 层与算法修复

| # | 问题 | 状态 | 备注 |
|---|---|---|---|
| B1 | `request` 强制 `Content-Type: application/json`，与 FormData 冲突 | 🟡 进行中 | `api.ts` 与 `Uploader.tsx` 已有改动 |
| B2 | API 路径、认证头、分页/幂等未对齐 API-SPEC | 🟡 进行中 | `apiClient.ts` 与 `api.ts` 已有改动 |
| B3 | `heatScore.ts` `topKeyPages` 用页码字符串匹配，与算法文档不符 | 🔲 待处理 | 未在 diff 中观察到改动 |

### Phase C：统一数据层与组件拆分

| # | 问题 | 状态 | 备注 |
|---|---|---|---|
| C1 | 18+ 处重复 fetch 样板 | 🔲 待处理 | 需引入 `useAsyncData` 或类似 hook |
| C2 | `api.ts` 仍 re-export 工具函数 | 🔲 待处理 | 职责需解耦 |
| C3 | oversized 组件拆分 | 🔲 待处理 | SmartLinkCreator / DocumentsTable / DocumentDetail / DashboardPage |
| C4 | 日期格式化、事件映射、日趋势聚合重复逻辑集中化 | 🔲 待处理 | 抽离到 `formatters.ts` / `calculations.ts` |
| C5 | 核心工具函数补单元测试 | 🔲 待处理 | `heatScore.ts`、`formatters.ts`、`calculations.ts` |

### Phase D：UI/UX 细节打磨

| # | 问题 | 状态 | 备注 |
|---|---|---|---|
| D1 | 空状态统一使用 `EmptyState` | 🔲 待处理 | DocumentDetail、contacts/detail、insights/pages |
| D2 | HeatMap / ContactDetail 键盘可达性 | 🔲 待处理 | 需加 `role`/`tabIndex` 或改为 `<Link>` |
| D3 | AIAssistant/AIChat 移动端高度适配 | 🔲 待处理 | 改为 `max-h-[calc(100dvh-2rem)]` |
| D4 | 表格小屏横向滚动 | 🔲 待处理 | 设置 `min-w-[640px]` |
| D5 | AI 面板圆角/阴影统一 | 🔲 待处理 | 统一 `rounded-xl` + `shadow-lg` |
| D6 | Sidebar 折叠态图标 Tooltip | 🔲 待处理 | 未确认 |

---

## 5. 风险与下一步行动

| 风险 | 影响 | 下一步行动 |
|---|---|---|
| 大段未提交 diff | 丢失/冲突风险高 | 将当前 72+35 个文件按功能切分为多个 PR 提交 |
| 文档滞后 | 后续维护与审计困难 | 同步更新 API-SPEC、database-model、ARCHITECTURE、README |
| v2.1.3 缺 task/issue 清单 | 进度不可追踪 | 将本清单中的 Phase A~D 拆分为 `DS-028+` issue 与 `TASK-*-v2.1.3` 任务文件 |
| API 路径/响应未最终统一 | 前后端联调可能返工 | 在 `TASK-FRONTEND-003` 后续工作中明确 `/api${path}` → `/{ws}/api/v1/*` 迁移方案 |
| 测试覆盖不足 | 回归风险高 | 随 Phase C 为核心工具函数与组件补测试 |

---

## 6. 更新记录

| 日期 | 版本 | 更新内容 | 更新人 |
|---|---|---|---|
| 2026-06-24 | v2.1.3 | 初始进度追踪，汇总 v2.1.2 完成情况与 v2.1.3 工作区状态 | Kimi Code CLI |

---

## 7. 使用说明

1. 每次阶段性工作后，更新「当前版本」「工作区状态」「剩余工作清单」中的状态列。
2. 状态图例：
   - `✅ 已完成`
   - `🟡 进行中`
   - `🔲 待处理`
   - `⏸️ 阻塞/延后`
3. 建议在完成 v2.1.3 规划后，将本文件中的 `Phase A~D` 条目同步到 `docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.3.md` 与 `docs/tasks/agent-tasks-v2.1.3/`。

---

## 8. 正式 issue/task 文件

| 文档 | 说明 |
|---|---|
| `docs/IMPLEMENTATION-PLAN-v2.1.3.md` | v2.1.3 实施计划 |
| `docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.3.md` | v2.1.3 issue 拆分清单（DS-028 ~ DS-039） |
| `docs/tasks/agent-tasks-v2.1.3/README.md` | v2.1.3 可执行 task 总览 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-FRONTEND-006.md` | 前端阻塞按钮与即时反馈清零 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-FRONTEND-007.md` | 表单提交反馈、删除确认与账户菜单 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-FRONTEND-008.md` | 前端文案与中英混杂清理 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-FRONTEND-009.md` | API 请求层修复与真实后端适配 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-FRONTEND-010.md` | heatScore topKeyPages 算法对齐 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-FRONTEND-011.md` | 统一数据层与 oversized 组件拆分 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-FRONTEND-012.md` | UI/UX 细节打磨 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-FRONTEND-013.md` | 前端单元与组件测试补强 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-BACKEND-011.md` | 后端未落库改动整理与接口稳定 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-BACKEND-012.md` | 后端中间件与基础模块补全 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-TEST-003.md` | E2E 与契约测试 |
| `docs/tasks/agent-tasks-v2.1.3/TASK-DOCS-001.md` | v2.1.3 文档基线同步 |

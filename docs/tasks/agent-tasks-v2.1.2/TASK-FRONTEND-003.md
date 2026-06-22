---
task_id: "TASK-FRONTEND-003"
parent_issue: "DS-027"
agent_task_id: "AGENT-TASK-003"
version: "v2.1.2"
priority: "P0"
status: "已完成"
type: "frontend"
effort: "M"
branch: "feat/agent-task-003-api-integration"
estimated_files: "10"
max_lines: "600"
project_stack: "React 19 / TypeScript / Vite 8 / i18next / MSW / Vitest"
ai_red_flags:
  - "不得在生产代码中硬编码后端 URL"
  - "认证 token 不得打印到日志"
  - "MSW fallback 必须可配置开关"
  - "错误响应必须结构化，不得吞掉异常"
  - "所有请求必须携带 workspaceSlug 与版本段"
ai_confidence: "high"
pending_confirmation: []
available_tools:
  - "test"
  - "lint"
  - "browse"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-FRONTEND-003` |
> | `parent_issue` | `DS-027` |
> | `agent_task_id` | `AGENT-TASK-003` |
> | **版本** | `v2.1.2` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `待执行` |
> | **类型** | `frontend` |
> | **预计工作量** | `M` |
> | **分支名** | `feat/agent-task-003-api-integration` |
> | **AI 置信度** | `high` |
> | **依赖** | `TASK-BACKEND-002`（Auth/Workspace 契约）；完整端到端验证依赖 `TASK-BACKEND-003`~`006` |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint / browse` |

# TASK-FRONTEND-003 前端-后端集成层：API base URL、workspace 上下文、认证、BaseResponse、错误处理、MSW fallback

> **父 Issue**：`DS-027`  
> **版本**：`v2.1.2`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`待执行`  
> **类型**：`frontend`  
> **预计工作量**：`M`  
> **分支名**：`feat/agent-task-003-api-integration`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

改造 `apps/web` 的 API 层，使其可切换真实后端与 MSW mock；统一注入 `workspaceSlug`、API 版本、认证 token、`Accept-Language`、幂等键；解析后端统一 `BaseResponse` 并抛出结构化错误；为 `createLink`、`createDealRoom` 等写操作增加前端到 API schema 的 mapper。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8、§16 |
| TDD | `docs/TDD-v2.1.0.md` §5.x |
| API-SPEC | `docs/API-SPEC-v2.1.0.md` §2.2、§2.4、§2.5、§2.6、§2.7、§3.2 |
| PLAN | `docs/IMPLEMENTATION-PLAN-v2.1.2.md` §4.1、§5 |
| ISSUES | `docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.2.md` DS-027 |
| 前端对齐评审 | `docs/reviews/frontend-implementation-doc-alignment-v2.1.2.md` §6.2 |
| 父 Issue | `DS-027` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 中等；先读 `api.ts`、路由、`main.tsx`、mock handlers。 |
| **必读顺序** | front matter → `ai_red_flags` → 目标 → 边界条件 → 输出。 |
| **分块策略** | 不需要分块；本任务以改造 `api.ts` 与 MSW 启用逻辑为主。 |

### 3.1 已有代码/表（执行前必须阅读）

- `apps/web/src/lib/api.ts`
- `apps/web/src/main.tsx`
- `apps/web/src/router.tsx`
- `apps/web/src/lib/mocks/browser.ts`
- `apps/web/src/lib/mocks/handlers.ts`
- `apps/web/src/stores/uiStore.ts`
- `apps/web/src/types/index.ts`
- `apps/web/.env`（如存在）

### 3.2 数据模型/接口

```typescript
// 统一 API 错误
interface ApiError {
  code: string;
  message: string;
  details?: Array<{ field: string; issue: string }>;
  requestId?: string;
  status: number;
}

// 后端 BaseResponse
interface BaseResponse<T> {
  code: "ok" | string;
  message: string;
  request_id: string;
  data?: T;
  pagination?: Pagination;
}

interface RequestOptions extends RequestInit {
  token?: string;
  idempotencyKey?: string;
  headers?: Record<string, string> | Headers;
}
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 环境变量 | `VITE_API_BASE_URL` 为空时启用 MSW | 空字符串视为未配置 |
| 路径注入 | 内部请求带 `/{workspaceSlug}/api/v1` | 公开请求带 `/api/v1/public` |
| 认证头 | 存在 token 时自动附加 `Authorization: Bearer <token>` | token 不记录日志 |
| 语言头 | 自动附加 `Accept-Language: <i18n.language>` | 支持 en / zh-CN |
| 幂等键 | 写操作可选 `Idempotency-Key` | 由调用方生成 UUID |
| 请求 ID | 自动生成 `X-Request-ID` | UUID |
| 错误解析 | 非 2xx 时解析 `code/message/details/request_id` | 抛出 `ApiError` |
| 最大变更行数 | ≤ 600 | 超出需拆分 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 后端 401 | token 过期 | 抛出结构化错误，code=`unauthorized` |
| 后端 403 | 越权 workspace | 抛出结构化错误，code=`forbidden` |
| 后端 5xx | 服务不可用 | 抛出结构化错误，code=`internal_error` |
| 网络异常 | fetch 失败 | 抛出结构化错误，code=`network_error` |
| BaseResponse code != ok | 业务错误 | 抛出结构化错误，使用返回的 code/message |
| 无 base URL | 未配置且未启用 MSW | 请求相对路径 `/api/*` 并失败 |

### 3.5 测试数据 / Mock / Fixture

```json
{
  "mockToken": "dummy-token-for-local-development-only",
  "mockHeaders": {
    "Accept-Language": "en",
    "X-Request-ID": "550e8400-e29b-41d4-a716-446655440000",
    "Idempotency-Key": "660e8400-e29b-41d4-a716-446655440000"
  }
}
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/web/src/lib/apiClient.ts` | 新增 | 底层 fetch 封装：路径注入、header 注入、BaseResponse 拆包、统一错误解析 |
| `apps/web/src/lib/api.ts` | 修改 | 使用 `apiClient`；支持 `VITE_API_BASE_URL`；为写操作增加 camelCase→snake_case mapper |
| `apps/web/src/lib/apiAdapters.ts` | 新增 | `createLink`、`createDealRoom` 等前端配置到 API payload 的转换 |
| `apps/web/src/main.tsx` | 修改 | 按环境变量决定是否启用 MSW |
| `apps/web/src/lib/mocks/browser.ts` | 修改 | 导出条件启用函数 |
| `apps/web/src/lib/mocks/server.ts` | 新增 | Node/Playwright/CI 用的 MSW server setup |
| `apps/web/src/lib/mocks/handlers.ts` | 修改 | 统一响应为 `BaseResponse` 结构；补充 auth/workspace 占位 |
| `apps/web/.env.example` | 新增 | 示例环境变量 |
| `apps/web/src/lib/apiClient.test.ts` | 新增 | 错误解析、header 注入、BaseResponse 拆包测试 |
| `apps/web/src/lib/apiAdapters.test.ts` | 新增 | mapper 单元测试 |

### 4.2 行为定义

- 配置 `VITE_API_BASE_URL=https://api.example.test` 后，请求按 `https://api.example.test/{workspaceSlug}/api/v1/*` 发送。
- 未配置且启用 MSW 时，请求相对路径 `/api/*` 并由 MSW 拦截。
- 统一错误对象包含 `code`、`message`、`status`、`requestId`、`details`。
- 认证、语言、请求 ID、幂等头自动注入；调用方无需重复设置。
- 写操作 payload 自动从 camelCase 转换为 snake_case（保留内部 TypeScript 类型习惯）。

---

## 5. 验收标准

- [x] 无 `VITE_API_BASE_URL` 时走 MSW
- [x] 配置 `VITE_API_BASE_URL` 后请求真实后端，且路径带 `workspaceSlug`
- [x] 自动携带 `Authorization`、`Accept-Language`、`X-Request-ID`、`Idempotency-Key`
- [x] 成功响应按 `BaseResponse` 拆包返回 `data`
- [x] 错误统一抛出结构化 `ApiError`
- [x] `createLink` / `createDealRoom` payload 与 API-SPEC 字段对齐
- [x] `pnpm test` 通过
- [x] `pnpm lint` 0 errors
- [x] `pnpm build` 成功
- [x] token 不出现在日志或错误信息中

---

## 6. 实现步骤建议

1. 阅读现有 `api.ts`、`router.tsx`、`main.tsx` 的 MSW 初始化逻辑。
2. 创建 `apiClient.ts`，实现 `request<T>`、BaseResponse 拆包、错误解析、header 注入、workspaceSlug/版本路径注入。
3. 创建 `apiAdapters.ts`，实现 `toCreateLinkPayload`、`toCreateDealRoomPayload` 等。
4. 修改 `api.ts` 使用 `apiClient` 与 adapters，移除重复逻辑。
5. 修改 `main.tsx`：仅在未配置 `VITE_API_BASE_URL` 或显式启用 mock 时启动 MSW。
6. 新增 `lib/mocks/server.ts` 用于 Node/CI。
7. 调整 `handlers.ts` 统一返回 `{ data: ... }` 结构，并补充 `/auth/*`、workspace 占位。
8. 新增 `.env.example`。
9. 新增 `apiClient.test.ts` 与 `apiAdapters.test.ts`。
10. 运行 `pnpm lint && pnpm test && pnpm build`。
11. 提交 PR。

---

## 7. 测试验证

### 7.1 单元测试

```bash
cd apps/web && pnpm test -- apiClient apiAdapters
```

### 7.2 集成/回归测试

```bash
cd apps/web && pnpm lint && pnpm test && pnpm build
```

### 7.3 手动验证

```bash
# 使用真实后端
cp .env.example .env
# 编辑 .env 设置 VITE_API_BASE_URL=http://localhost:8080
cd apps/web && pnpm dev

# 使用 MSW（不设置 VITE_API_BASE_URL）
cd apps/web && pnpm dev
```

### 7.4 回归测试命令

```bash
cd apps/web && pnpm lint && pnpm test && pnpm build
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`。
- **范围锁定**：不得修改本任务范围外的业务组件；如必须修改，需在 Agent 备注中说明理由并征得审批。
- **保持现有测试通过**：运行全量回归测试命令全绿才能提交。
- **不要提前实现**：范围外的功能（如 OAuth 登录、token 刷新）不要碰。
- **禁止把敏感数据发送给 LLM**：Token、密码、密钥、PII、生产数据一律不得出现在 prompt、日志或测试数据中。
- **代码风格**：遵循项目已有目录和命名约定。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

| CODE-REVIEW 3.11 检查项 | 本任务自检要求 |
|------------------------|----------------|
| 无硬编码示例域名/邮箱/密码 | `.env.example` 使用 `https://api.example.test`。 |
| 无未清理的 TODO / FIXME / placeholder | 实现完成后全局搜索，确保无残留。 |
| 无幻觉常量 | HTTP 状态码、错误 code 与 API-SPEC 对齐。 |
| 错误处理不过度 try-catch，不吞掉异常 | 异常统一封装后抛出，不静默吞掉。 |
| 未引入未使用的依赖或代码 | 提交前运行 lint。 |
| 未擅自实现范围外功能 | 仅做 API 客户端、adapters 与 MSW 开关。 |
| 测试数据与生产数据隔离 | fixture/mock 数据不引用生产系统。 |

---

## 10. Definition of Done

- [x] 代码实现完成
- [x] 测试通过（单元 + 集成）
- [x] lint / typecheck / build 通过
- [ ] 代码审查通过
- [x] 与父 Issue 的验收标准对齐
- [ ] PR 已关联父 Issue：`Relates to #DS-027`
- [x] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- Vite 环境变量必须以 `VITE_` 开头才能在客户端使用。
- MSW 的 `worker.start()` 是异步的，确保在 `main.tsx` 中 await 后再挂载 React。
- `workspaceSlug` 可从 `useParams` 或 UI store 获取；推荐在 `apiClient` 中通过读取当前 URL 或接受显式参数。
- 若测试中需要覆盖 fetch，建议在 `apiClient.test.ts` 使用 `vi.stubGlobal('fetch', ...)`。
- 如果文件数超过 10，可将 adapters 拆到对应业务 task（如 `TASK-BACKEND-005`）。

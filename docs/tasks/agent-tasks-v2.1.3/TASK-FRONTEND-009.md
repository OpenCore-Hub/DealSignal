---
task_id: "TASK-FRONTEND-009"
parent_issue: "DS-031"
agent_task_id: "AGENT-TASK-031"
version: "v2.1.3"
priority: "P0"
status: "待执行"
type: "frontend"
effort: "M"
branch: "feat/agent-task-031-api-client-fix"
estimated_files: "10"
max_lines: "500"
project_stack: "React 19 + TypeScript + Vite 8 + TanStack Query / Zustand"
ai_red_flags:
  - "不得破坏现有 /api 路径的 MSW 调用"
  - "FormData 上传不得被覆盖 Content-Type"
  - "所有敏感 header 必须在服务端安全注入"
  - "敏感数据不得发送给 LLM"
ai_confidence: "medium"
pending_confirmation:
  - "是否保留 /api${path} 作为短期兼容，还是直接切到 /{ws}/api/v1/*"
  - "token 来源：localStorage / cookie / 后端注入的 VITE_*"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

# TASK-FRONTEND-009 API 请求层修复与真实后端适配

> **父 Issue**：`DS-031`

---

## 1. 目标

修复 `apps/web/src/lib/api.ts` 与 `apiClient.ts`：
- `Content-Type: application/json` 不再覆盖 FormData。
- 支持可选 `token`、预留 `requestId` / `idempotencyKey`。
- 统一解析 `BaseResponse` 并做结构化错误处理。
- 保持 `/api${path}` 短期兼容，但注释说明向 `/{ws}/api/v1/*` 迁移方式。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 前端审计计划 | `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md` §2.2 / §6.2 |
| API 规范 | `docs/API-SPEC-v2.1.0.md` §2.2 / §2.5 / §2.7 |
| PRD | `docs/PRD-v2.1.0.md` §10.2 |
| 父 Issue | `DS-031` |

### 2.1 已有代码

- `apps/web/src/lib/api.ts`
- `apps/web/src/lib/apiClient.ts`
- `apps/web/src/lib/apiClient.test.ts`（新增未落库）
- `apps/web/src/lib/mocks/handlers.ts`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| Content-Type | 非 FormData 才设置 `application/json` | FormData 由浏览器自动设置 boundary |
| token | 可选 | 未提供时不加 Authorization |
| BaseResponse | 必须校验 `code === "ok"` | 错误时抛出结构化错误 |
| 路径 | 短期兼容 `/api${path}` | 长期迁移到 `/{ws}/api/v1/*` |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| FormData 上传 | `POST /api/documents` with `FormData` | 不设置 Content-Type，上传成功 |
| 401 响应 | 后端返回 401 | 抛出 `{ code: 'unauthorized', message, requestId }` |
| BaseResponse code != ok | `code: 'invalid_request'` | 抛出结构化错误，不直接返回 data |
| 无 token 请求公开接口 | 不带 token | 不附加 Authorization |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/lib/api.ts` | 修改 | Content-Type 条件、token、BaseResponse、错误处理 |
| `src/lib/apiClient.ts` | 修改 | 适配 api.ts 新签名 |
| `src/lib/apiClient.test.ts` | 修改/新增 | 覆盖 FormData、BaseResponse、错误 |
| `src/lib/mocks/handlers.ts` | 修改 | 统一返回 BaseResponse 结构 |
| `src/types/index.ts` | 修改 | ApiError / BaseResponse 类型 |

---

## 5. 验收标准

- [ ] FormData 上传不再被错误 Content-Type 覆盖。
- [ ] 请求可选注入 token、requestId、idempotencyKey。
- [ ] 成功响应统一拆包 `data`；错误抛出结构化 `ApiError`。
- [ ] 现有 `apiClient.test.ts` 与新增测试全通过。
- [ ] MSW handler 返回 BaseResponse，不影响现有前端页面。
- [ ] `pnpm lint && pnpm test` 全绿。

---

## 6. 实现步骤建议

1. 阅读现有 `api.ts` 与 `apiClient.ts`。
2. 修改 `request`：检测 body 是否为 FormData，条件设置 headers。
3. 增加 `requestId` / `idempotencyKey` 可选参数。
4. 解析响应：先 `response.json()`，校验 `code === 'ok'`，返回 `data`。
5. 统一错误：构造 `ApiError` 含 `code/message/details/request_id`。
6. 更新 mock handlers 为 BaseResponse 结构。
7. 补充测试。

---

## 7. 测试验证

```bash
cd apps/web
pnpm test apiClient
pnpm test --run lib
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 不要一次性全量切换到 `/{ws}/api/v1/*`，先保持 `/api` 兼容。
- 不要在请求层耦合 workspace slug 解析（留给上层调用方）。
- 不要吞掉非 2xx 响应体。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-031`

---

## 10. Agent 备注

- 如果 mock handlers 数量过多，可先更新与 upload/search/assistant 相关的核心 handler。
- 错误处理可参考后端返回格式：`{ code, message, details?, request_id }`。

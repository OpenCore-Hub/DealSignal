---
task_id: "TASK-TEST-003"
parent_issue: "DS-038"
agent_task_id: "AGENT-TASK-038"
version: "v2.1.3"
priority: "P0"
status: "待执行"
type: "test"
effort: "M"
branch: "feat/agent-task-038-e2e-contract"
estimated_files: "10"
max_lines: "500"
project_stack: "Go 1.25 + Gin / React 19 + Vite 8 + Playwright + MSW + k6"
ai_red_flags:
  - "E2E 不得依赖真实第三方服务（OpenAI/OnlyOffice 需提供 mock/跳过）"
  - "契约测试必须基于 API-SPEC，不得与实现耦合"
  - "测试数据必须隔离，不得污染生产/共享环境"
  - "敏感数据不得发送给 LLM"
ai_confidence: "medium"
pending_confirmation:
  - "契约测试覆盖范围：P0 接口还是全部接口？"
  - "E2E 是否需要在 CI 中连接真实后端容器？"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

# TASK-TEST-003 E2E 与契约测试

> **父 Issue**：`DS-038`

---

## 1. 目标

- 扩展 Playwright E2E，覆盖登录、上传、Viewer、链接创建、Dashboard 等 P0 路径。
- 建立前端契约测试，验证请求/响应形状与 `docs/API-SPEC-v2.1.0.md` 一致。
- 配置 MSW server setup，支持前端单元/组件测试中的 API mock。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 实施计划 | `docs/IMPLEMENTATION-PLAN-v2.1.3.md` |
| API 规范 | `docs/API-SPEC-v2.1.0.md` |
| TDD | `docs/TDD-v2.1.0.md` §10 |
| 父 Issue | `DS-038` |
| 依赖 | `DS-031`（TASK-FRONTEND-009）、`DS-036`（TASK-BACKEND-011） |

### 2.1 已有代码

- `apps/web/e2e/p0.spec.ts`
- `apps/web/e2e/real-backend.spec.ts`
- `apps/web/src/lib/mocks/handlers.ts`
- `apps/api/e2e-test.sh`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| E2E 环境 | Playwright + 本地后端/MSW | 真实后端 E2E 可配置 |
| 契约测试 | 覆盖 P0 接口 | auth/workspace/documents/links/viewer/events/search/assistant |
| MSW server | 用于 Vitest | 与 browser handler 共享 fixture |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `web/e2e/auth.spec.ts` | 新增 | 登录/注册/邀请 |
| `web/e2e/upload.spec.ts` | 新增 | 文档上传与 ingestion 轮询 |
| `web/e2e/viewer.spec.ts` | 新增 | Viewer 渲染与水印 |
| `web/e2e/links.spec.ts` | 新增 | 创建链接、公开访问、事件上报 |
| `web/e2e/dashboard.spec.ts` | 新增 | Dashboard 信号与热度 |
| `web/src/lib/mocks/server.ts` | 新增 | MSW server for tests |
| `web/src/lib/mocks/contract.ts` | 新增 | 契约校验辅助 |
| `web/vitest.setup.ts` | 修改 | 全局启用 MSW server |
| `web/playwright.config.ts` | 修改 | 真实后端配置 |
| `api/e2e-full.sh` | 整理 | 与前端 E2E 对齐 |

---

## 5. 验收标准

- [ ] Playwright E2E 覆盖登录、上传、Viewer、链接、Dashboard P0 路径。
- [ ] 至少为 5 个核心接口建立契约测试。
- [ ] MSW server 在 Vitest 中可用。
- [ ] CI 中前端 E2E 与后端 E2E 均通过。
- [ ] `pnpm test:e2e` 与 `pnpm test` 全绿。

---

## 6. 实现步骤建议

1. 配置 MSW server for Vitest。
2. 为 handlers 增加契约校验层。
3. 编写前端 P0 E2E。
4. 对齐后端 E2E 脚本与 fixture。
5. 更新 CI workflow。

---

## 7. 测试验证

```bash
cd apps/web
pnpm exec playwright install --with-deps chromium
pnpm test:e2e
pnpm test

cd apps/api
./e2e-test.sh
```

---

## 8. 约束与红线

- E2E 不能依赖真实 OpenAI/OnlyOffice；提供 mock 或跳过开关。
- 契约测试不得读取后端实现，只能读取 API-SPEC。
- 测试账号必须使用 `.test` 域名。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] CI 通过
- [ ] PR 已关联父 Issue：`Closes #DS-038`

---

## 10. Agent 备注

- 真实后端 E2E 可在本地 docker-compose 启动后运行；CI 中可用 service containers。
- 契约测试可用 zod/io-ts 对响应做 shape 校验。

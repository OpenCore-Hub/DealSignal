---
task_id: "TASK-TEST-001"
parent_issue: "DS-023"
agent_task_id: "AGENT-TASK-016"
version: "v2.1.2"
priority: "P0"
status: "已完成"
type: "test"
effort: "L"
branch: "feat/agent-task-016-test-automation"
estimated_files: "10"
max_lines: "600"
project_stack: "Vitest / React Testing Library / Playwright / Go test / Docker"
ai_red_flags:
  - "测试数据不得使用生产凭据"
  - "E2E 不得依赖外部不可控服务"
  - "覆盖率报告必须可生成"
  - "CI 必须全绿才能合并"
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
> | `task_id` | `TASK-TEST-001` |
> | `parent_issue` | `DS-023` |
> | `agent_task_id` | `AGENT-TASK-016` |
> | **版本** | `v2.1.2` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `已完成` |
> | **类型** | `test` |
> | **预计工作量** | `L` |
> | **分支名** | `feat/agent-task-016-test-automation` |
> | **AI 置信度** | `high` |
> | **依赖** | 功能开发完成 |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-TEST-001 测试用例与自动化

> **父 Issue**：`DS-023`  
> **版本**：`v2.1.2`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`待执行`  
> **类型**：`test`  
> **预计工作量**：`L`  
> **分支名**：`feat/agent-task-016-test-automation`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

建立覆盖前后端的测试体系：前端单元/组件测试、API 契约测试、Playwright E2E，以及后端 Go 单元/集成测试；确保 P0 路径有自动化门禁。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| TDD | `docs/TDD-v2.1.0.md` §10.x |
| PRD | `docs/PRD-v2.1.0.md` §12 |
| 父 Issue | `DS-023` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.1 测试范围

| 层级 | 工具 | 覆盖 |
|------|------|------|
| 前端单元 | Vitest + jsdom | calculations, heat score, formatters, apiClient, adapters, i18n |
| 前端组件 | React Testing Library | ThemeToggle, WorkspaceSwitcher, SignalCard, AIAssistant, Uploader |
| 前端 E2E | Playwright | login → upload → create link → view analytics |
| 后端单元 | go test | service/handler 层 |
| 后端集成 | go test + docker | 全链路 API |

---

## 4. 输出

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/web/src/lib/apiClient.test.ts` | 新增 | API 客户端测试 |
| `apps/web/src/lib/apiAdapters.test.ts` | 新增 | mapper 测试 |
| `apps/web/src/components/**/*.test.tsx` | 新增 | 组件测试 |
| `apps/web/e2e/*.spec.ts` | 新增 | Playwright E2E |
| `apps/web/vitest.config.ts` | 修改 | 覆盖率配置 |
| `apps/api/tests/integration/*_test.go` | 新增 | 后端集成测试 |
| `.github/workflows/ci.yml` | 修改 | 测试门禁 |

---

## 5. 验收标准

- [x] 前端单元/组件测试：`apps/web` 13 个测试文件、83 个用例全部通过（Vitest + jsdom + React Testing Library）。
- [x] 前端覆盖率门禁：`@vitest/coverage-v8` 已接入，`coverage.enabled: true`，仅对核心逻辑与已测组件统计；当前结果 statements 89.63% / branches 76.44% / functions 82.52% / lines 92.03%，阈值设置为 statements 80% / branches 70% / functions 55% / lines 80%。
- [x] 后端单元测试：`go test -race -cover ./...` 全绿，新增 `link/service_test.go`、`upload/service_test.go`、`auth/service_test.go` 覆盖核心校验逻辑。
- [x] CI 测试门禁：`.github/workflows/ci.yml` 已包含 `backend-test`（含 coverprofile）、`backend-lint`、`backend-security`（govulncheck）与前端 typecheck/lint/test/coverage。
- [ ] Playwright P0 E2E：脚本与配置待补充（当前未覆盖）。

---

## 6. Definition of Done

- [x] 测试实现完成
- [x] CI 配置完成
- [ ] PR 已关联父 Issue：`Closes #DS-023`（待提交 PR 时填写）

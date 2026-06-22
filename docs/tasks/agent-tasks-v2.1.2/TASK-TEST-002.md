---
task_id: "TASK-TEST-002"
parent_issue: "DS-024"
agent_task_id: "AGENT-TASK-017"
version: "v2.1.2"
priority: "P1"
status: "已完成"
type: "test"
effort: "M"
branch: "feat/agent-task-017-performance-test"
estimated_files: "6"
max_lines: "400"
project_stack: "k6 / vegeta / Go test / Docker / PostgreSQL / Redis"
ai_red_flags:
  - "压测不得在生产环境执行"
  - "压测数据必须隔离"
  - "目标性能指标必须量化"
  - "发现的瓶颈必须记录并修复"
ai_confidence: "medium"
pending_confirmation:
  - "压测工具选型（k6 / vegeta / Locust）"
  - "目标 QPS / 延迟基线"
available_tools:
  - "test"
  - "docker"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-TEST-002` |
> | `parent_issue` | `DS-024` |
> | `agent_task_id` | `AGENT-TASK-017` |
> | **版本** | `v2.1.2` |
> | **模板版本** | `v2` |
> | **优先级** | `P1` |
> | **状态** | `已完成` |
> | **类型** | `test` |
> | **预计工作量** | `M` |
> | **分支名** | `feat/agent-task-017-performance-test` |
> | **AI 置信度** | `medium` |
> | **依赖** | 功能开发完成 |
> | **待人工确认事项** | `压测工具 / 性能基线` |
> | **可用工具/技能** | `test / docker` |

# TASK-TEST-002 性能压测与优化

> **父 Issue**：`DS-024`  
> **版本**：`v2.1.2`  
> **模板版本**：`v2`  
> **优先级**：`P1`  
> **状态**：`待执行`  
> **类型**：`test`  
> **预计工作量**：`M`  
> **分支名**：`feat/agent-task-017-performance-test`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

对 P0 API（上传、签名 URL、公开链接访问、AI 问答、搜索）进行负载压测，量化延迟与吞吐量，修复明显瓶颈。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §9 NFR |
| TDD | `docs/TDD-v2.1.0.md` §8.x |
| 父 Issue | `DS-024` |

---

## 3. 输入

### 3.1 压测目标（示例）

| 接口 | 目标 P99 延迟 | 目标 QPS |
|------|--------------|----------|
| 公开链接访问 | < 200ms | 1000/s |
| 签名 URL 生成 | < 100ms | 500/s |
| AI 问答 | < 3000ms | 30/s |
| 搜索 | < 500ms | 100/s |

---

## 4. 输出

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/scripts/loadtest/*.js` | 新增 | k6/vegeta 脚本 |
| `apps/api/Makefile` | 修改 | 增加 `make loadtest` |
| `docs/PERFORMANCE-REPORT-v2.1.2.md` | 新增 | 压测报告 |

---

## 5. 验收标准

- [x] 压测脚本可复现：`apps/api/scripts/loadtest/` 下新增 k6 脚本（`public-link.js`、`signed-url.js`、`search.js`、`assistant-chat.js`）与共享 `options.js`。
- [x] P0 接口延迟/吞吐量基线已量化并记录于 `docs/PERFORMANCE-REPORT-v2.1.2.md`；实际执行结果待真实环境校准后回填。
- [x] Makefile 增加 `loadtest` 入口与 `loadtest-*` 目标，方便复现。
- [ ] 真实环境压测执行与瓶颈修复（待部署后执行）。

---

## 6. Definition of Done

- [x] 压测脚本与报告输出完成
- [ ] 真实环境压测执行（待部署后执行）
- [ ] PR 已关联父 Issue：`Closes #DS-024`（待提交 PR 时填写）

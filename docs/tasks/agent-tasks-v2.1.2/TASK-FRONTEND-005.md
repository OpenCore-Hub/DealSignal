---
task_id: "TASK-FRONTEND-005"
parent_issue: "DS-016"
agent_task_id: "AGENT-TASK-015"
version: "v2.1.2"
priority: "P1"
status: "已完成"
type: "frontend"
effort: "S"
branch: "feat/agent-task-015-dashboard-frontend"
estimated_files: "6"
max_lines: "300"
project_stack: "React 19 / TypeScript / Tailwind CSS 4 / Base UI / i18next"
ai_red_flags:
  - "不得破坏现有 Dashboard 渲染"
  - "热度评分必须从 API 获取而非 mock"
  - "不得引入新 console 警告"
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
> | `task_id` | `TASK-FRONTEND-005` |
> | `parent_issue` | `DS-016` |
> | `agent_task_id` | `AGENT-TASK-015` |
> | **版本** | `v2.1.2` |
> | **模板版本** | `v2` |
> | **优先级** | `P1` |
> | **状态** | `待执行` |
> | **类型** | `frontend` |
> | **预计工作量** | `S` |
> | **分支名** | `feat/agent-task-015-dashboard-frontend` |
> | **AI 置信度** | `high` |
> | **依赖** | `TASK-BACKEND-005` |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint / browse` |

# TASK-FRONTEND-005 Dashboard 前端完善

> **父 Issue**：`DS-016`  
> **版本**：`v2.1.2`  
> **模板版本**：`v2`  
> **优先级**：`P1`  
> **状态**：`待执行`  
> **类型**：`frontend`  
> **预计工作量**：`S`  
> **分支名**：`feat/agent-task-015-dashboard-frontend`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

完善 Dashboard 前端，接入真实热度评分与 Analytics 数据，补齐 PRD 中要求的 content conversion / team performance 视图（若范围允许），并确保信号流排序与交互规范一致。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8.10、§8.11、§11.2 |
| TDD | `docs/TDD-v2.1.0.md` §11.2 |
| API 契约 | `docs/API-SPEC-v2.1.0.md` API-16 ~ API-18 |
| 父 Issue | `DS-016` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 信号排序 | hot > risk > warm > cold | 默认 |
| 热度来源 | API-10 / analytics overview | 不再使用 mock heatLevel |
| 最大变更行数 | ≤ 300 | |

---

## 4. 输出

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/components/dashboard/DashboardPage.tsx` | 修改 | 接入真实数据 |
| `src/components/dashboard/SignalCard.tsx` | 修改 | aria-expanded、排序适配 |
| `src/components/dashboard/ActionList.tsx` | 修改 | 支持 postpone/ignore |
| `src/routes/insights/*.tsx` | 修改 | 接入 analytics API |

---

## 5. 验收标准

- [ ] Dashboard 使用 API 返回的热度与 signals
- [ ] 信号流排序符合规范
- [ ] ActionList 支持 postpone/ignore
- [ ] `pnpm test` 通过
- [ ] `pnpm lint` 0 errors
- [ ] `pnpm build` 成功

---

## 6. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / build 通过
- [ ] PR 已关联父 Issue：`Closes #DS-016`

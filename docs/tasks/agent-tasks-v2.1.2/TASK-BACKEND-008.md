---
task_id: "TASK-BACKEND-008"
parent_issue: "DS-018"
agent_task_id: "AGENT-TASK-011"
version: "v2.1.0"
priority: "P1"
status: "已完成"
type: "backend"
effort: "M"
branch: "feat/agent-task-011-behavior-reminders"
estimated_files: "8"
max_lines: "500"
project_stack: "Go 1.22+ / Gin / PostgreSQL / Redis"
ai_red_flags:
  - "提醒规则必须可配置"
  - "不得向非授权用户泄露他人行为数据"
  - "建议生成必须基于真实事件，禁止编造"
  - "通知触发必须幂等"
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
> | `task_id` | `TASK-BACKEND-008` |
> | `parent_issue` | `DS-018` |
> | `agent_task_id` | `AGENT-TASK-011` |
> | **版本** | `v2.1.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P1` |
> | **状态** | `待执行` |
> | **类型** | `backend` |
> | **预计工作量** | `M` |
> | **分支名** | `feat/agent-task-011-behavior-reminders` |
> | **AI 置信度** | `high` |
> | **依赖** | `TASK-BACKEND-005` |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-BACKEND-008 行为提醒与跟进建议

> **父 Issue**：`DS-018`  
> **版本**：`v2.1.0`  
> **模板版本**：`v2`  
> **优先级**：`P1`  
> **状态**：`待执行`  
> **类型**：`backend`  
> **预计工作量**：`M`  
> **分支名**：`feat/agent-task-011-behavior-reminders`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

基于热度评分与访问事件，自动生成行为提醒（如高意向信号、风险下降、跟进时机）与跟进建议，覆盖 API-17。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8.11 |
| TDD | `docs/TDD-v2.1.0.md` §6.8 |
| API 契约 | `docs/API-SPEC-v2.1.0.md` API-17 |
| 算法 | `docs/HEAT-SCORE-ALGORITHM-v2.1.1.md` |
| 父 Issue | `DS-018` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.1 数据模型

```sql
CREATE TABLE suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    contact_id UUID REFERENCES contacts(id),
    link_id UUID REFERENCES links(id),
    document_id UUID REFERENCES documents(id),
    type TEXT NOT NULL CHECK (type IN ('follow_up','risk_alert','hot_signal')),
    reason TEXT NOT NULL,
    action TEXT NOT NULL,
    dismissed BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 生成频率 | 按事件触发或定时任务 | 同一 contact/link 24h 内不重复生成同类型 |
| 隐私 | 仅 workspace 成员可见 | 严格 tenant 隔离 |
| 最大变更行数 | ≤ 500 | |

### 3.3 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 无事件 | 新 contact | 不生成建议 |
| 越权 | 非 workspace 成员 | 403 |

---

## 4. 输出

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/internal/db/migrations/007_suggestions.up.sql` | 新增 | suggestions 表 |
| `apps/api/internal/suggestions/service.go` | 新增 | 建议生成逻辑 |
| `apps/api/internal/suggestions/handler.go` | 新增 | 路由 |
| `apps/api/internal/server/routes.go` | 修改 | 注册 suggestions 路由 |

---

## 5. 验收标准

- [x] 热度变化、访问事件触发建议生成（link_opened / download_attempted 后自动触发）
- [x] 建议包含原因与可执行 action
- [x] 越权访问返回 403
- [x] `go test ./...` 通过
- [x] `make lint` 通过

---

## 6. 实现步骤

1. 编写 migration。
2. 实现 suggestion service，订阅 analytics 事件。
3. 实现 handler 与 list/dismiss 接口。
4. 注册路由。
5. 测试。
6. 提交 PR。

---

## 7. Definition of Done

- [x] 代码实现完成
- [x] 测试通过
- [x] lint / build 通过
- [ ] PR 已关联父 Issue：`Closes #DS-018`

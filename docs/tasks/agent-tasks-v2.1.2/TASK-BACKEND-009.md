---
task_id: "TASK-BACKEND-009"
parent_issue: "DS-020 / DS-021 / DS-022"
agent_task_id: "AGENT-TASK-012"
version: "v2.1.0"
priority: "P1"
status: "已完成"
type: "backend"
effort: "L"
branch: "feat/agent-task-012-notify-integrations"
estimated_files: "12"
max_lines: "800"
project_stack: "Go 1.22+ / Gin / PostgreSQL / SMTP / Slack Webhook / OAuth"
ai_red_flags:
  - "第三方 OAuth secret 不得硬编码"
  - "邮件模板不得泄露敏感链接"
  - "通知发送必须异步，不得阻塞主流程"
  - "CRM 同步必须可重试"
ai_confidence: "medium"
pending_confirmation:
  - "邮件服务提供商（SMTP / SendGrid / Resend）"
  - "Slack OAuth 应用信息"
  - "HubSpot 与 Salesforce 是否都实现？"
available_tools:
  - "test"
  - "lint"
  - "docker"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-BACKEND-009` |
> | `parent_issue` | `DS-020 / DS-021 / DS-022` |
> | `agent_task_id` | `AGENT-TASK-012` |
> | **版本** | `v2.1.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P1` |
> | **状态** | `待执行` |
> | **类型** | `backend` |
> | **预计工作量** | `L` |
> | **分支名** | `feat/agent-task-012-notify-integrations` |
> | **AI 置信度** | `medium` |
> | **依赖** | `TASK-BACKEND-005` |
> | **待人工确认事项** | `邮件服务商 / Slack OAuth / CRM 范围` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-BACKEND-009 邮件通知与 Slack/HubSpot/Salesforce 集成

> **父 Issue**：`DS-020 / DS-021 / DS-022`  
> **版本**：`v2.1.0`  
> **模板版本**：`v2`  
> **优先级**：`P1`  
> **状态**：`待执行`  
> **类型**：`backend`  
> **预计工作量**：`L`  
> **分支名**：`feat/agent-task-012-notify-integrations`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

实现邮件通知、Slack/HubSpot 集成接入，覆盖 API-23 ~ API-25；Salesforce 根据资源可延后或同批实现。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8.14、§8.15 |
| TDD | `docs/TDD-v2.1.0.md` §6.9、§6.11 |
| API 契约 | `docs/API-SPEC-v2.1.0.md` API-23 ~ API-25 |
| 父 Issue | `DS-020 / DS-021 / DS-022` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.1 数据模型

```sql
CREATE TABLE notification_settings (
    workspace_id UUID PRIMARY KEY REFERENCES workspaces(id),
    email_enabled BOOLEAN DEFAULT true,
    slack_webhook_url TEXT,
    hubspot_connected BOOLEAN DEFAULT false,
    salesforce_connected BOOLEAN DEFAULT false,
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    user_id UUID,
    channel TEXT NOT NULL CHECK (channel IN ('email','slack')),
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE oauth_states (
    state TEXT PRIMARY KEY,
    workspace_id UUID NOT NULL,
    provider TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
```

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 通知触发 | 热度 ≥ 阈值或关键事件 | 可配置 |
| 重试 | 最多 3 次 | 异步 worker |
| OAuth state | 随机、10 分钟过期 | 防 CSRF |
| 最大变更行数 | ≤ 800 | |

### 3.3 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 未配置邮件服务 | SMTP 未设置 | 通知标记失败 |
| OAuth state 过期 | 超 10 分钟 | 400 `invalid_state` |
| 越权修改设置 | 非成员 | 403 |

---

## 4. 输出

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/internal/db/migrations/008_notify_integrations.up.sql` | 新增 | 通知/集成表 |
| `apps/api/internal/notification/service.go` | 新增 | 邮件/Slack 发送 |
| `apps/api/internal/notification/worker.go` | 新增 | 异步 worker |
| `apps/api/internal/integration/slack.go` | 新增 | Slack OAuth + webhook |
| `apps/api/internal/integration/hubspot.go` | 新增 | HubSpot OAuth + sync |
| `apps/api/internal/integration/salesforce.go` | 新增 | Salesforce OAuth + sync（可延后） |
| `apps/api/internal/integration/handler.go` | 新增 | 集成路由 |
| `apps/api/internal/server/routes.go` | 修改 | 注册路由 |

---

## 5. 验收标准

- [x] 高热度信号触发邮件通知（hot_signal 生成时入队）
- [x] Slack 集成可发送消息
- [x] HubSpot OAuth 可连接；同步保留 stub（真实 CRM API 调用超出当前范围）
- [x] 通知发送不阻塞主流程（异步 worker）
- [x] `go test ./...` 通过
- [x] `make lint` 通过

---

## 6. Definition of Done

- [x] 代码实现完成
- [x] 测试通过
- [x] lint / build 通过
- [ ] PR 已关联父 Issue：`Closes #DS-020` / `Relates to #DS-021 #DS-022`

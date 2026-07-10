---
task_id: TASK-SHARE-SHORT-009
parent_issue: DS-SHARE-021
agent_task_id: AGENT-TASK-SHARE-021
version: v1.0.0
priority: P1
status: 已完成
type: fullstack
effort: M
branch: feat/share-short-009-file-requests
estimated_files: '18'
max_lines: '800'
project_stack: Go 1.25 + Gin + PostgreSQL + React 19 + TypeScript
dependencies:
- INFRA-001
- TASK-SHARE-SHORT-005
- TASK-SHARE-SHORT-006
ai_red_flags:
- 公共端点必须校验 X-Link-Session，不能匿名无限提交
- owner 收到的请求通知必须异步发送，不能阻塞提交接口
- 请求内容必须做长度限制与 XSS 过滤，不能原样回显 HTML
- 一个访客对同一 link 的 pending 请求建议去重或限制频率
- 文件请求状态变更必须有审计日志，便于追踪
ai_confidence: medium
pending_confirmation:
- 请求是否需要支持上传附件（MVP 建议仅文本，附件后续扩展）？
- owner 在 Analytics Tab 还是独立 Tab 查看请求？
- 通知方式是邮件、站内通知还是两者都有？
available_tools:
- test
- lint
- browse
---

# TASK-SHARE-SHORT-009 访客文件请求（File Requests）MVP

> **父 Issue**：`DS-SHARE-021`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **类型**：`fullstack`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-short-009-file-requests`

---

## 1. 目标

实现 **访客 → owner** 的文件/资料请求：访客在访问数据室/文档时发现资料缺失，可向 owner 提交补充请求。这是 Access Tab `fileRequestsEnabled` 开关的真实能力之一。

> 另有一种 **owner → 第三方** 的文件收集意图（让外部人员上传资料），由于需要安全匿名上传链路，复杂度更高，单独拆分为 [TASK-SHARE-MID-009](./TASK-SHARE-MID-009.md)。本任务只覆盖访客发起的 inbound 请求。

- 后端新增 `link_file_requests` 表与 CRUD API。
- `links.file_requests_enabled` 控制该功能是否对访客可见。
- 访客在侧边栏 "Requests" 中提交请求并查看状态。
- Owner 在 Link Analytics / 邀请管理区域看到待处理请求，并可标记为 approved / rejected / fulfilled。
- 新请求产生时，异步通知 link creator / 指定收件人。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 功能设计 | `/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §8.7.6 |
| 相关任务 | TASK-SHARE-SHORT-006、TASK-SHARE-SHORT-007、TASK-SHARE-MID-007、TASK-SHARE-MID-009 |
| 已有代码 | `apps/api/internal/link/service.go`、`apps/web/src/components/viewer/RightSidebar.tsx` |

### 2.1 当前问题

- `fileRequestsEnabled` 只是 `DraftLink` 中的布尔字段，后端没有对应表/列，访客开了也没用。
- 真实场景：投资人看 deal room 时发现缺一份 cap table，希望一键请求 owner 补充，而不是发邮件。
- 明确边界：本任务不包含“owner 向外部第三方发起上传请求”功能，该功能见 MID-009。

---

## 3. 输入

### 3.1 数据模型

```sql
CREATE TABLE link_file_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    visitor_id TEXT,
    visitor_email TEXT,
    message TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','fulfilled')),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_link_file_requests_link_id ON link_file_requests(link_id);
CREATE INDEX idx_link_file_requests_status ON link_file_requests(status);
```

### 3.2 API 契约

**公共端点（访客）**

```http
POST /api/v1/public/links/:token/file-requests
X-Link-Session: <session-token>
Content-Type: application/json

{ "message": "请补充 2025 年 Cap Table" }
```

```http
GET /api/v1/public/links/:token/file-requests/me
X-Link-Session: <session-token>
```

**Owner 端点**

```http
GET /api/v1/links/:id/file-requests
```

```http
PATCH /api/v1/links/:id/file-requests/:requestId/status
Content-Type: application/json

{ "status": "approved" }
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 功能开关 | `links.file_requests_enabled = true` | 关闭时公共端点返回 `403 file_requests_disabled` |
| 防滥用 | 同一 visitor 同一 link 最多 3 条 pending | 超限返回 `429 too_many_requests` |
| 内容长度 | `message` 1~500 字符 | 过短/过长均返回 `400` |
| 通知 | 异步 | 提交接口只写 DB + 发事件，不直接发邮件 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 开关关闭 | `file_requests_enabled=false` | `403 file_requests_disabled` |
| 未登录 session | 无 `X-Link-Session` | `401 session_required` |
| 请求内容为空 | `message=""` | `400 message_required` |
| 超出 pending 上限 | 已有 3 条 pending | `429 too_many_requests` |
| owner 更新非法状态 | `status="deleted"` | `400 invalid_status` |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/050_link_flags_qa_file_requests.up.sql` | 新增 | `links.file_requests_enabled`（由 INFRA-001 统一编排） |
| `apps/api/internal/db/migrations/050_link_flags_qa_file_requests.down.sql` | 新增 | 回滚 |
| `apps/api/internal/db/queries.sql` | 修改 | CreateLink/UpdateLink 增加 `file_requests_enabled` |
| `apps/api/internal/db/queries.sql.go` | 重新生成 | sqlc |
| `apps/api/internal/db/models.go` | 重新生成 | sqlc |
| `apps/api/internal/link/service.go` | 修改 | 新增 `CreateFileRequest`、`ListFileRequests`、`UpdateFileRequestStatus` |
| `apps/api/internal/link/handler.go` | 修改 | 注册公共 + owner API |
| `apps/api/internal/link/access.go`（或 session.go） | 修改 | 公共端点 session 校验 |
| `apps/web/src/lib/api.ts` | 新增 | `createPublicFileRequest`、`listLinkFileRequests`、`updateFileRequestStatus` |
| `apps/web/src/types/index.ts` | 修改 | 补充 `FileRequest` 类型 |
| `apps/web/src/components/links/share/AccessTab.tsx` | 修改 | `fileRequestsEnabled` 开关保持可见 |
| `apps/web/src/components/viewer/RightSidebar.tsx` | 修改 | 增加 "Requests" tab（仅开启时） |
| `apps/web/src/components/viewer/FileRequestPanel.tsx` | 新增 | 提交表单 + 本人请求列表 |
| `apps/web/src/components/links/share/AnalyticsTab.tsx` 或新 Tab | 修改 | Owner 查看 + 状态变更 |
| `apps/web/src/i18n/locales/en/linkShare.json` | 修改 | 文案 |
| `apps/web/src/i18n/locales/zh-CN/linkShare.json` | 修改 | 文案 |

### 4.2 行为定义

```text
Access Tab / Advanced
└── File Requests [开关]
    开启后，公共 Viewer 侧边栏出现 "Requests" tab：
    - 访客可填写请求内容并提交。
    - 访客可看到自己提交过的请求与状态。
    Owner 在 Link Analytics 看到所有请求，可：
    - approved（已同意补充）
    - rejected（拒绝）
    - fulfilled（已上传/完成）
```

---

## 5. 验收标准

- [ ] 后端新增 `link_file_requests` 表与 `links.file_requests_enabled` 列。
- [ ] 公共端点仅在开关开启且 session 有效时接受请求。
- [ ] 访客提交请求后，owner 收到异步通知（邮件/站内通知至少一种）。
- [ ] Owner 可查看请求列表并更新状态。
- [ ] 前端 `pnpm lint` / `pnpm typecheck` / `pnpm test` 全绿。
- [ ] 后端 `go test ./internal/link/...` 全绿。

---

## 6. 实现步骤建议

1. 基于 INFRA-001 已创建的 050 migration 修改代码，**本任务不再新增 migration**。
2. 后端 service/handler 实现 CRUD + session 校验。
3. 前端 API 封装。
4. 实现 `FileRequestPanel.tsx`。
5. 在 `RightSidebar.tsx` 条件渲染 Requests tab。
6. 在 Analytics / 管理区域实现 owner 视图。
7. 接入异步通知（可复用 TASK-SHARE-SHORT-007 的通知机制）。
8. 补测试。

---

## 7. 测试验证

```bash
# 后端
cd apps/api
go test ./internal/link/...
./e2e-test.sh

# 前端
cd apps/web
pnpm test FileRequestPanel RightSidebar AnalyticsTab
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 公共端点**必须**校验 `X-Link-Session`。
- `message` 必须做 HTML 转义，禁止富文本。
- 通知必须异步，不能阻塞请求提交。
- 状态变更必须记录 `updated_at`，建议同时写入安全审计事件。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 单元测试 + e2e P0 通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-021`

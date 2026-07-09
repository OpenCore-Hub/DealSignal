---
task_id: TASK-SHARE-MID-009
parent_issue: DS-SHARE-023
agent_task_id: AGENT-TASK-SHARE-023
version: v1.1.0
priority: P2
status: 待执行
type: fullstack
effort: L
branch: feat/share-mid-009-file-request-links
estimated_files: '22'
max_lines: '1000'
project_stack: Go 1.25 + Gin + PostgreSQL + React 19 + TypeScript + MinIO
dependencies:
- INFRA-001
- TASK-SHARE-SHORT-005
- TASK-SHARE-SHORT-006
- TASK-SHARE-MID-007
ai_red_flags:
- 文件收集链接必须复用现有 link session，禁止自建绕过访问规则的 token
- 上传文件默认 pending_review，owner 审批后才能进入 deal room
- 上传文件必须做类型/大小校验，不能直接写入正式文档库
- 审批通过时必须使用事务：更新 link_uploaded_files + 创建 document + 插入 deal_room_documents
- 通知必须异步，不能阻塞上传接口
- 必须防止上传链接被滥用（次数限制、过期失效、IP/UA 记录）
ai_confidence: medium
pending_confirmation:
- 文件收集链接是否支持密码访问？（技术上直接复用 require_password）
- 是否允许把文件收集链接关联到单个 document 目录，还是必须 deal_room？
- 审批通过后是否立即触发 ingestion pipeline？
available_tools:
- test
- lint
- browse
---

# TASK-SHARE-MID-009 文件收集链接（File Request Links）

> **父 Issue**：`DS-SHARE-023`  
> **版本**：`v1.1.0`  
> **优先级**：`P2`  
> **类型**：`fullstack`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-mid-009-file-request-links`

---

## 1. 目标

让 owner 通过**通用的创建分享链接流程**生成一个“文件收集链接”：

- 该链接复用现有 Access Rules（邮箱、OTP、密码、允许/阻止列表、过期时间）。
- 第三方打开链接后，通过安全验证直接进入**上传区**，无需登录平台。
- 上传的文件先进入待审核区；owner 审批通过后，自动归入指定 deal room 目录。
- 不引入额外的独立工作流或页面，公共 viewer 在检测到链接类型为 `file_request` 时渲染上传面板。

这是 File Requests 的第二种意图（owner → 第三方），与 [TASK-SHARE-SHORT-009](./TASK-SHARE-SHORT-009.md) 的访客→owner 请求互补。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 功能设计 | `/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §8.7.6 |
| 相关任务 | TASK-SHARE-SHORT-005、TASK-SHARE-SHORT-006、TASK-SHARE-SHORT-009、TASK-SHARE-MID-007 |
| 依赖能力 | 现有 link session、Access Rules、公共 viewer 路由、文档 ingestion pipeline、deal room 目录 |

### 2.1 为什么复用 share link 而不是独立 workflow

| 维度 | 独立 Upload Request（旧方案） | 复用 Share Link（本方案） |
|---|---|---|
| 访问控制 | 需要自建 upload token + OTP | 直接复用 `require_email` / `require_email_verification` / `password` |
| 会话管理 | 需要自建 upload session | 直接复用 `X-Link-Session` |
| 创建入口 | 新增 "Request upload" 页面 | 复用 `DealRoomShareDialog` / `LinkShareDialog` |
| 公共页面 | 新增 `/upload/:token` 路由 | 复用 `/l/:token`，按 `link_type` 条件渲染 |
| 安全审计 | 两套逻辑 | 统一走 access_logs / 安全事件 |
| 用户心智 | 多一个概念 | 所有安全 gate 都是 "share link" |

---

## 3. 输入

### 3.1 数据模型

```sql
-- 使用 link_type 区分用途，避免污染 permission_type 维度
ALTER TABLE links ADD COLUMN IF NOT EXISTS link_type TEXT NOT NULL DEFAULT 'share'
    CHECK (link_type IN ('share','file_request'));

-- 文件收集链接的目标目录
ALTER TABLE links ADD COLUMN IF NOT EXISTS target_folder_path TEXT NOT NULL DEFAULT '/Uploads';

-- 第三方上传的文件记录
CREATE TABLE link_uploaded_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    original_filename TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type TEXT NOT NULL,
    uploader_email TEXT,
    uploader_visitor_id TEXT,
    uploader_ip INET,
    uploader_user_agent TEXT,
    status TEXT NOT NULL DEFAULT 'pending_review' CHECK (status IN ('pending_review','approved','rejected')),
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_link_uploaded_files_link ON link_uploaded_files(link_id);
CREATE INDEX idx_link_uploaded_files_status ON link_uploaded_files(status);
```

### 3.2 API 契约

**Owner 端点**

创建文件收集链接（复用现有 link 创建端点，新增字段）：

```http
POST /api/v1/deal-rooms/:id/links
Content-Type: application/json

{
  "name": "收集审计报告",
  "link_type": "file_request",
  "require_email": true,
  "require_email_verification": true,
  "expires_at": "2026-08-01T00:00:00Z",
  "target_folder_path": "/Financials/2025",
  "max_access_count": 5,
  "notify_on_access": true
}
```

```http
GET /api/v1/links/:id/uploaded-files
```

```http
POST /api/v1/links/:id/uploaded-files/:fileId/approve
POST /api/v1/links/:id/uploaded-files/:fileId/reject
```

**公共端点（第三方上传人）**

复用现有 access 流程获取 `X-Link-Session` 后：

```http
POST /api/v1/public/links/:token/upload
X-Link-Session: <session-token>
Content-Type: multipart/form-data

file=<binary>
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 链接类型 | `link_type = 'file_request'` | 必须关联 `deal_room_id`，`document_id` 为 null |
| 访问规则 | 复用现有 Access Rules | email / OTP / password / allow / block / expired |
| 上传次数 | `max_access_count` 或单独计数 | 建议用 `max_access_count` 控制总上传次数 |
| 文件大小 | 默认最大 50MB | 可配置 |
| 文件类型 | 仅允许常见办公格式 | pdf / doc / docx / xls / xlsx / zip |
| 默认目录 | `target_folder_path` | 默认 `/Uploads`；owner 可指定 |
| 审批 | owner 审批后才加入 deal room | 默认 `pending_review` |
| 通知 | 异步 | 上传完成通知 owner；审批结果通知上传人 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 链接类型不是 file_request | `link_type='share'` 调用 upload | `403 not_file_request_link` |
| 无 session | 无 `X-Link-Session` | `401 session_required` |
| 链接过期 | `expires_at` 已过 | `410 link_expired` |
| 超过上传次数 | 已上传 >= max_access_count | `409 upload_limit_reached` |
| 文件过大 | > 50MB | `413 payload_too_large` |
| 文件类型非法 | exe / bat | `415 unsupported_media_type` |
| 审批时链接已失效 | 正常审批 | 仍可审批，但不能再上传 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/053_file_request_links.up.sql` | 新增 | `link_type` + `target_folder_path` + `link_uploaded_files`（由 INFRA-001 统一编排） |
| `apps/api/internal/db/migrations/053_file_request_links.down.sql` | 新增 | 回滚 |
| `apps/api/internal/db/queries.sql` | 修改 | CreateLink/UpdateLink 支持 `link_type='file_request'` 与 `target_folder_path`；新增 uploaded files CRUD |
| `apps/api/internal/db/queries.sql.go` | 重新生成 | sqlc |
| `apps/api/internal/db/models.go` | 重新生成 | sqlc |
| `apps/api/internal/link/service.go` | 修改 | `UploadFileForLink`、`ListUploadedFiles`、`ApproveUploadedFile`、`RejectUploadedFile` |
| `apps/api/internal/link/handler.go` | 修改 | 注册公共 upload 与 owner 审批端点 |
| `apps/api/internal/link/access.go` | 修改（可选） | 确保 Access() 对 file_request 链接返回足够元数据 |
| `apps/web/src/lib/api.ts` | 新增 | `uploadPublicFile`、`listUploadedFiles`、`approveUploadedFile`、`rejectUploadedFile` |
| `apps/web/src/types/index.ts` | 修改 | 补充 `UploadedFile`、`LinkType` |
| `apps/web/src/components/links/share/ShareTab.tsx` | 修改 | 增加 "File request" link type / preset |
| `apps/web/src/components/links/share/AccessTab.tsx` | 修改 | file_request 链接显示 `target_folder_path` 输入；隐藏 inbound fileRequestsEnabled |
| `apps/web/src/components/viewer/PublicViewerPage.tsx` | 修改 | 根据 `permissionType === 'file_request'` 渲染上传面板 |
| `apps/web/src/components/viewer/PublicFileRequestUpload.tsx` | 新增 | 上传区 UI：邮箱/OTP 通过后显示上传表单 |
| `apps/web/src/components/links/share/AnalyticsTab.tsx` | 修改 | owner 查看上传文件列表 + 审批按钮 |
| `apps/web/src/i18n/locales/en/linkShare.json` | 修改 | 文案 |
| `apps/web/src/i18n/locales/zh-CN/linkShare.json` | 修改 | 文案 |

### 4.2 行为定义

```text
Owner 创建文件收集链接
1. 在 DealRoomShareDialog 中选择 Link type = "File request"。
2. 配置 Access Rules：require_email / OTP / password / allowed emails / expires_at。
3. 配置目标目录 target_folder_path（默认 /Uploads）。
4. 保存后系统生成普通 share link（如 /l/abc123）。

第三方访问
1. 打开 /l/abc123。
2. 复用现有 gate：输入邮箱 → OTP → 密码（若开启）。
3. 验证通过后，公共 viewer 不显示文档，而是显示上传区。
4. 上传文件 → 文件进入 pending_review。

Owner 审批
1. 在 Link Analytics Tab 看到 "Uploaded files"。
2. Approve：文件作为新 document 加入 deal_room_documents.target_folder_path。
3. Reject：通知上传人，文件不进入 deal room。
```

---

## 5. 验收标准

- [ ] `links.link_type` 支持 `'file_request'`。
- [ ] 文件收集链接复用现有 Access Rules 与 `X-Link-Session`。
- [ ] 公共 viewer 在 `permissionType === 'file_request'` 时渲染上传区，不显示文档。
- [ ] 第三方上传文件后进入 `pending_review`，owner 可 approve/reject。
- [ ] approve 后文件出现在 deal room 的 `target_folder_path` 目录下。
- [ ] 前端 `pnpm lint` / `pnpm typecheck` / `pnpm test` 全绿。
- [ ] 后端 `go test ./internal/link/...` 全绿。

---

## 6. 实现步骤建议

1. 基于 INFRA-001 已创建的 053 migration 修改代码，**本任务不再新增 migration**。
2. 更新 sqlc 查询与 link service/handler，支持 file_request 链接创建。
3. 后端实现 `UploadFileForLink`（校验类型/大小/次数，写入 MinIO + DB）与审批方法。
4. 审批方法使用事务：更新 `link_uploaded_files.status` + 创建 `documents` + 插入 `deal_room_documents`。
5. 前端 ShareTab/AccessTab 增加 File request 类型与目标目录输入。
6. 前端 `PublicViewerPage` 按 `permissionType` 分支渲染 `PublicFileRequestUpload`。
7. 前端 AnalyticsTab 增加 uploaded files 列表与审批按钮。
8. 接入异步通知。
9. 补测试。

---

## 7. 测试验证

```bash
# 后端
cd apps/api
go test ./internal/link/...
./e2e-test.sh

# 前端
cd apps/web
pnpm test PublicViewerPage PublicFileRequestUpload AnalyticsTab DealRoomShareDialog
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- **必须**复用现有 link session 与 Access Rules，禁止为上传自建绕过访问控制的 token。
- **必须**将上传文件与正式文档库隔离，直到 owner 审批通过。
- 审批通过操作**必须**使用事务，避免 document 与 deal_room_documents 不一致。
- 上传文件必须做类型与大小校验，禁止可执行文件。
- 通知必须异步，不能阻塞上传接口。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 单元测试 + e2e P0 通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-023`

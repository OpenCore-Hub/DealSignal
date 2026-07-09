---
task_id: "TASK-SHARE-INFRA-001"
parent_issue: "DS-SHARE-INFRA-001"
agent_task_id: "AGENT-TASK-SHARE-INFRA-001"
version: "v1.0.0"
priority: "P0"
status: "已完成"
type: "infra"
effort: "L"
branch: "feat/share-infra-001-schema-orchestration"
estimated_files: "14"
max_lines: "600"
project_stack: "Go 1.25 + PostgreSQL + sqlc"
dependencies: []
ai_red_flags:
  - "所有新增 migration 必须统一编号，避免与现有 046_links_document_id_nullable 冲突"
  - "历史 invite token 一次性 hash 迁移必须幂等、可回滚"
  - "links.security_version 必须有 NOT NULL DEFAULT 1"
  - "不能在业务任务中再单独新增 migration"
ai_confidence: "high"
pending_confirmation: []
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-INFRA-001 Schema 统一编排

> **父 Issue**：`DS-SHARE-INFRA-001`  
> **版本**：`v1.0.0`  
> **优先级**：`P0`  
> **类型**：`infra`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-infra-001-schema-orchestration`

---

## 1. 目标

统一产出文档分享业务 v1.4 所需的所有 schema 变更，解决迁移编号冲突，确保下游业务任务（SHORT-005/007/008/009、MID-003/008/009 等）不再各自新增 migration。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 设计文档 | `/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §5 |
| 完成度追踪 | [`COMPLETION-TRACKING.md`](./COMPLETION-TRACKING.md) |
| 现有迁移 | `apps/api/internal/db/migrations/042_deal_room_sharing.up.sql`、 `046_links_document_id_nullable.up.sql` |

### 2.1 当前问题

- 任务计划原定 `046_invitation_token_hash`，但仓库已有 `046_links_document_id_nullable.up.sql`。
- 多个业务任务各自计划新增 migration，编号分散，分支合并极易冲突。
- `links` 表缺少 `security_version`、占位开关字段等共享列。

---

## 3. 迁移编号规划

| 新编号 | 文件 | 覆盖任务 | 说明 |
|---|---|---|---|
| `047` | `047_invitation_token_hash_and_security_version.up.sql` | SHORT-005-A | token hash + security_version |
| `048` | `048_link_access_rule_revisions.up.sql` | SHORT-005-A | 规则变更审计快照 |
| `049` | `049_link_access_requests.up.sql` | SHORT-005-B / SHORT-007 | 访客访问请求 |
| `050` | `050_link_flags_qa_file_requests.up.sql` | SHORT-008 / SHORT-009 | `qa_enabled`、`file_requests_enabled` |
| `051` | `051_link_index_files.up.sql` | MID-008 | 索引文件缓存表 |
| `052` | `052_notification_rules.up.sql` | MID-003 | 通知规则引擎表 |
| `053` | `053_file_request_links.up.sql` | MID-009 | 文件收集链接 |
| `054` | `054_security_events_tenant_workspace.up.sql` | SHORT-003 | 安全事件补租户隔离 |
| `055` | `055_link_status_lifecycle.up.sql` | MID-007 | `links.status`、归档/续期支持 |

> 若未来需要继续扩展，统一从 `056` 起递增，并在此文件更新编号表。

---

## 4. 各 migration 详细 DDL

### 4.1 `047_invitation_token_hash_and_security_version.up.sql`

```sql
-- 1. Add security_version to links for deterministic session invalidation.
ALTER TABLE links
    ADD COLUMN IF NOT EXISTS security_version INTEGER NOT NULL DEFAULT 1;

-- 2. Add token_hash to link_invitations. New invitations write only the hash;
--    the raw token is returned once to the caller. Existing rows are backfilled
--    lazily by the application (see SHORT-005 backfill) before token_hash becomes
--    NOT NULL.
ALTER TABLE link_invitations
    ADD COLUMN IF NOT EXISTS token_hash TEXT;

-- Allow new invitations to leave the legacy token column NULL.
ALTER TABLE link_invitations
    ALTER COLUMN token DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_link_invitations_token_hash ON link_invitations(token_hash);

-- 3. Trigger to bump security_version when access rules change.
CREATE OR REPLACE FUNCTION bump_link_security_version()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE links SET security_version = security_version + 1 WHERE id = OLD.link_id;
        RETURN OLD;
    ELSE
        UPDATE links SET security_version = security_version + 1 WHERE id = NEW.link_id;
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_bump_security_version_on_rule_change ON link_access_rules;
CREATE TRIGGER trg_bump_security_version_on_rule_change
    AFTER INSERT OR UPDATE OR DELETE ON link_access_rules
    FOR EACH ROW EXECUTE FUNCTION bump_link_security_version();
```

### 4.2 `048_link_access_rule_revisions.up.sql`

```sql
CREATE TABLE IF NOT EXISTS link_access_rule_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    rules_snapshot JSONB NOT NULL,
    changed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_link_access_rule_revisions_link_id ON link_access_rule_revisions(link_id);
```

### 4.3 `049_link_access_requests.up.sql`

```sql
CREATE TABLE IF NOT EXISTS link_access_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    reason TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (link_id, email)
);
CREATE INDEX idx_link_access_requests_link_id ON link_access_requests(link_id);
CREATE INDEX idx_link_access_requests_status ON link_access_requests(status);
```

### 4.4 `050_link_flags_qa_file_requests.up.sql`

```sql
ALTER TABLE links
    ADD COLUMN IF NOT EXISTS qa_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS file_requests_enabled BOOLEAN NOT NULL DEFAULT false;
```

### 4.5 `051_link_index_files.up.sql`

```sql
ALTER TABLE links
    ADD COLUMN IF NOT EXISTS index_file_enabled BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS link_index_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL UNIQUE REFERENCES links(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','generating','ready','failed')),
    content_html TEXT,
    error_message TEXT,
    generated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
```

### 4.6 `052_notification_rules.up.sql`

```sql
CREATE TABLE IF NOT EXISTS notification_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    rule_type TEXT NOT NULL CHECK (rule_type IN ('first_open','repeat_key_page','forward_signal','abnormal_access','hot_signal','daily_digest')),
    channels TEXT[] NOT NULL DEFAULT ARRAY['email'],
    enabled BOOLEAN NOT NULL DEFAULT true,
    unsubscribable BOOLEAN NOT NULL DEFAULT true,
    merge_window_minutes INT NOT NULL DEFAULT 10,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, rule_type)
);
```

### 4.7 `053_file_request_links.up.sql`

```sql
-- Use link_type instead of extending permission_type to avoid dimension pollution.
ALTER TABLE links
    ADD COLUMN IF NOT EXISTS link_type TEXT NOT NULL DEFAULT 'share'
    CHECK (link_type IN ('share','file_request'));

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS target_folder_path TEXT NOT NULL DEFAULT '/Uploads';

CREATE TABLE IF NOT EXISTS link_uploaded_files (
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

### 4.8 `054_security_events_tenant_workspace.up.sql`

```sql
ALTER TABLE security_events
    ADD COLUMN IF NOT EXISTS tenant_id UUID,
    ADD COLUMN IF NOT EXISTS workspace_id UUID;

-- Backfill from link_id lookup in a one-time job if needed.
CREATE INDEX IF NOT EXISTS idx_security_events_tenant ON security_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_security_events_workspace ON security_events(workspace_id);
```

### 4.9 `055_link_status_lifecycle.up.sql`

```sql
ALTER TABLE links
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active','archived','expired'));

-- expires_at in past does not automatically flip status; cron or application enforces.
CREATE INDEX idx_links_status ON links(status);
```

---

## 5. 验收标准

- [ ] 所有 047–055 migration 文件存在且通过 `go test ./internal/db/...` / `make migrate-up`。
- [ ] 现有 `046_links_document_id_nullable.up.sql` 未被覆盖。
- [ ] 下游业务任务文件中引用的 migration 编号与本文件一致。
- [x] 历史 `link_invitations.token` 提供 lazy backfill job（应用层在 ResolveInviteToken 时逐步迁移）。
- [ ] `links.security_version` 默认值为 1，规则/密码变更触发器有效。

---

## 6. 实现步骤建议

1. 在 `apps/api/internal/db/migrations/` 创建 047–055 `.up.sql` 与 `.down.sql`。
2. 运行 `make migrate-up` 在本地验证。
3. 更新 `docs/tasks/document-sharing-gap-remediation/COMPLETION-TRACKING.md` 中 migration 编号。
4. 通知所有下游任务：schema 已就绪，开始实现业务逻辑。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/db/...
make migrate-up
make lint
```

---

## 8. 约束与红线

- 禁止在业务任务中再新增 migration；所有 schema 变更回收到 INFRA-001。
- 历史数据迁移必须幂等、可重跑，不能丢失数据。
- 删除或重命名列必须经过双写/双读过渡期（如 token 列）。
- 所有表必须保留 `tenant_id` / `workspace_id` 隔离。

---

## 9. Definition of Done

- [x] 全部 migration 文件创建并通过测试
- [x] lint 通过（现有历史告警未引入新错误）
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-INFRA-001`

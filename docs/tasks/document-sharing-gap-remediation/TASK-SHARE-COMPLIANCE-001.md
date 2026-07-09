---
task_id: "TASK-SHARE-COMPLIANCE-001"
parent_issue: "DS-SHARE-COMPLIANCE-001"
agent_task_id: "AGENT-TASK-SHARE-COMPLIANCE-001"
version: "v1.0.0"
priority: "P1"
status: "待执行"
type: "compliance"
effort: "M"
branch: "feat/share-compliance-001-pii-retention"
estimated_files: "10"
max_lines: "500"
project_stack: "Go 1.25 + PostgreSQL + React 19 + TypeScript"
dependencies:
  - INFRA-001
  - INFRA-003
  - TASK-SHARE-SHORT-005
ai_red_flags:
  - "访客 PII（邮箱、IP）必须最小化存储，IP 必须哈希不可逆"
  - "必须提供数据导出与删除能力"
  - " retention 期满后必须自动匿名化或删除"
  - "合规日志本身不得包含 PII 明文"
ai_confidence: "medium"
pending_confirmation:
  - "删除请求是硬删除还是匿名化？"
  - "导出格式：JSON / CSV / PDF？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-COMPLIANCE-001 Sharing 链路 PII 最小化与 Retention

> **父 Issue**：`DS-SHARE-COMPLIANCE-001`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **类型**：`compliance`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-compliance-001-pii-retention`

---

## 1. 目标

让文档分享链路符合 GDPR/CCPA/数据安全法要求：
- 最小化收集访客 PII（邮箱、IP、UA）。
- IP 地址哈希不可逆。
- 提供 owner/workspace 级别的数据导出与删除接口。
- retention 到期自动匿名化或删除。

---

## 2. 上下文

| 数据 | 当前存储 | 目标 |
|---|---|---|
| 访客邮箱 | `access_logs.email`, `page_views.email` | 保留，用于通知与规则 |
| 访客 IP | `access_logs.ip` 等 | 存储 IP 哈希前 8 位或 HMAC，不存明文 |
| UA | `access_logs.user_agent` | 保留，用于安全分析 |
| AI 问题 | `assistant_messages.content` | 保留，需受 retention 限制 |

---

## 3. 输出

### 3.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/0XX_pii_hashing.up.sql` | 新增 | IP 改为 hash 列，删除明文 IP 列 |
| `apps/api/internal/analytics/service.go` | 修改 | 写入时 hash IP |
| `apps/api/internal/link/service.go` | 修改 | 安全事件记录时 hash IP |
| `apps/api/internal/compliance/` | 新增 | 导出、删除、匿名化服务 |
| `apps/api/internal/compliance/handler.go` | 新增 | workspace 合规端点 |
| `apps/web/src/components/settings/CompliancePanel.tsx` | 新增 | 导出/删除 UI |
| `apps/web/src/lib/api.ts` | 新增 | 合规 API 封装 |

### 3.2 API 契约

```http
GET /api/v1/workspaces/:slug/compliance/export?visitor_email=alice@vc.com
POST /api/v1/workspaces/:slug/compliance/anonymize
{
  "visitor_email": "alice@vc.com"
}
DELETE /api/v1/workspaces/:slug/compliance/data
{
  "visitor_email": "alice@vc.com"
}
```

---

## 4. 验收标准

- [ ] IP 明文不再写入事件/安全表。
- [ ] 提供按邮箱导出访客数据接口。
- [ ] 提供按邮箱匿名化/删除接口。
- [ ] retention 到期后自动清理或匿名化 PII。
- [ ] 操作记录写入审计日志。

---

## 5. 测试验证

```bash
cd apps/api
go test ./internal/compliance/...
go test ./internal/analytics/...
make lint

cd apps/web
pnpm lint
pnpm typecheck
```

---

## 6. 约束与红线

- 禁止在日志中打印完整邮箱或明文 IP。
- 删除/匿名化必须租户隔离，不能影响其他 workspace。
- 导出数据必须加密或临时 token 保护。

---

## 7. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-COMPLIANCE-001`

---
task_id: "TASK-SHARE-MID-005"
parent_issue: "DS-SHARE-009"
agent_task_id: "AGENT-TASK-SHARE-009"
version: "v1.0.0"
priority: "P1"
status: "已完成"
type: "backend"
effort: "M"
branch: "feat/share-mid-005-signed-urls"
estimated_files: "10"
max_lines: "500"
project_stack: "Go 1.25 + Gin + Redis + MinIO"
ai_red_flags:
  - "签名密钥必须独立于 JWT secret"
  - "签名 URL 必须有过期时间，且不可被篡改"
  - "下载签名与页面预览签名可共用一套机制但需区分 scope"
  - "必须防止签名 URL 被转发后长期有效"
ai_confidence: "medium"
pending_confirmation:
  - "签名 URL 有效期：15 分钟还是与 link session 一致？"
  - "MinIO 预签名 URL 是否需要额外 HMAC 签名？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-MID-005 页面与下载签名 URL

> **父 Issue**：`DS-SHARE-009`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **类型**：`backend`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-mid-005-signed-urls`

---

## 1. 目标

为公共 viewer 的页面图片预览 URL 和下载 URL 增加 HMAC 签名保护，防止 URL 被转发、盗链或长期滥用。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.7 |
| TDD | `docs/backup/TDD-v2.1.0.md` §7.4 |

### 2.1 已有代码

- `apps/api/internal/link/handler.go` — `PublicDocumentPages`, `PublicSignedURL`, `PublicDownloadURL`
- `apps/api/internal/storage/service.go` — MinIO 存储操作

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 签名算法 | HMAC-SHA256 | 密钥 `URL_SIGNING_SECRET` |
| 签名内容 | resource + link_token + visitor_id + expires_at | 防止篡改 |
| 有效期 | 15 分钟 | 与 link session 对齐 |
| 单次使用 | 否 | 签名 URL 可重复访问直至过期 |
| 验证 | 后端中间件 | 无效/过期签名返回 `403 forbidden` |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 签名缺失 | URL 无 `sig` | `403 missing_signature` |
| 签名错误 | `sig` 被篡改 | `403 invalid_signature` |
| 签名过期 | `expires` 已过去 | `403 signature_expired` |
| 资源不匹配 | 签名时 resource=A，访问 B | `403 resource_mismatch` |
| 转发给其他用户 | 签名 URL 发给第三方 | 允许访问但受 link 安全门/session 限制（可选） |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/link/signature.go` | 新增 | HMAC 签名/验证工具 |
| `apps/api/internal/link/handler.go` | 修改 | 返回签名 URL；验证签名 |
| `apps/api/internal/config/config.go` | 修改 | 新增 `URL_SIGNING_SECRET` |
| `apps/api/internal/server/routes.go` | 修改 | 静态/文件路由加签名验证中间件 |
| `apps/api/internal/db/migrations/0XX_signed_url.up.sql` | 新增（可选） | 若需记录签名使用日志 |
| `apps/api/.env.example` | 修改 | 增加 `URL_SIGNING_SECRET` |
| `apps/api/docker-compose.yml` | 修改 | 传递环境变量 |

### 4.2 行为定义

- `PublicSignedURL` / `PublicDownloadURL` 返回的 URL 包含 `?expires=ts&sig=hmac`。
- 访问这些 URL 时，中间件验证签名与过期时间。
- 签名失败返回 403，不暴露 MinIO 原始 URL。

---

## 5. 验收标准

- [ ] 页面图片与下载 URL 均带 HMAC 签名。
- [ ] 签名过期后访问返回 403。
- [ ] 签名被篡改后访问返回 403。
- [ ] 环境变量 `URL_SIGNING_SECRET` 已配置。
- [ ] 后端单元测试覆盖签名生成与验证。

---

## 6. 实现步骤建议

1. 新增 `URL_SIGNING_SECRET` 配置。
2. 实现 `link.SignURL(resource, token, visitorID, expires)` 与 `VerifyURL`。
3. 修改 `PublicSignedURL` / `PublicDownloadURL` 返回签名 URL。
4. 在文件服务路由注册签名验证中间件。
5. 更新 `.env.example` 与 `docker-compose.yml`。
6. 补测试。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/link/...
make lint
```

---

## 8. 约束与红线

- 签名密钥必须与 `JWT_SECRET`、`LINK_SESSION_SECRET` 分离。
- 不得在 URL 中携带敏感信息（如邮箱、IP）。
- 验证失败不得回退到无签名访问。
- 签名参数命名避免与现有查询参数冲突。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-009`

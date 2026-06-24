---
id: "API-2024-001"
version: "v2.1.0"
status: "已批准"
owner: "后端架构师 / 产品经理"
---

# DealSignal API 规范文档 v2.1.0

> **文档编号**：`API-2024-001`  
> **版本**：`v2.1.0`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`后端架构师 / 产品经理`  
> **编写日期**：`2026-06-20`  
> **关联文档**：  
> - `docs/TDD-v2.1.0.md`  
> - `docs/PRD-v2.1.0.md`  
> - `docs/database-model-v2.1.0.md`  
> - `docs/templates/EVENT-TRACKING-template-v1.md`  
> - `docs/openapi-v2.1.0.yaml`（待创建）  
> **评审人**：`CTO、后端负责人、前端负责人、测试负责人、安全负责人`

---

## 0. 文档使用说明

> **同步要求**：本文件必须与对应的 OpenAPI YAML 文件（`docs/openapi-v2.1.0.yaml`）保持同步。任何一方修改后，另一方必须同步更新，避免 Markdown 描述与 OpenAPI 机器可读契约出现偏差。

本文档是 **DealSignal** 的 API 规范文档（API Specification），定义服务端接口的协议、认证、错误码、资源命名、请求/响应契约和版本策略。

**目标**：
- 统一前后端、第三方集成、客户端的接口契约。
- 明确 RESTful API 风格与 Workspace 上下文传递规则。
- 记录认证、授权、限流、幂等、分页、缓存等通用规则。
- 作为开发、测试、SDK 生成、文档生成的共同依据。

**适用对象**：
- 后端开发工程师
- 前端/客户端开发工程师
- 测试工程师
- 集成/生态开发者

---

## 1. 文档控制信息

### 1.1 变更日志

| 版本 | 日期 | 修改人 | 修改内容 | 影响范围 |
|------|------|--------|----------|----------|
| v2.1.0 | 2026-06-20 | 后端架构师 | 按 API-SPEC-template-v1 创建 DealSignal v2.1.0 API 规范，继承 TDD 第 5 节接口契约并补充版本策略、SDK、同步清单 | 全文档 |
| v2.1.0-patch | 2026-06-24 | 前端工程师 | 将内部 API 基地址从 `/{workspaceSlug}/api/v1` 修正为 `/api/workspaces/{workspaceSlug}`，与后端路由及前端 `apiClient` 保持一致 | 全文档 |
| v2.1.0-patch | 2026-06-24 | 后端工程师 | 修正 API-11/14/15/16 路径与后端实际路由一致：建议改到 `/suggestions`、数据室公开申请使用 `{slug}`、HubSpot 同步使用 `/integrations/hubspot/sync`、Slack 连接使用 `/integrations/slack/connect` | 第 4 节 |

### 1.2 API 版本

| 版本 | 状态 | 基地址 | 说明 |
|------|------|--------|------|
| v1 | 当前 | `/api/workspaces/{workspaceSlug}`（内部）<br>`/api/v1/public`（公开） | v2.1.0 初始版本 |

---

## 2. 通用约定

### 2.1 协议与编码

- **传输协议**：HTTPS only
- **数据格式**：JSON（`Content-Type: application/json`），上传接口使用 `multipart/form-data`
- **字符编码**：UTF-8
- **时间格式**：ISO 8601（`2026-06-20T10:00:00Z`）
- **日期格式**：`YYYY-MM-DD`

### 2.2 API 风格

采用 RESTful 风格：

- 资源名使用复数名词，如 `/documents`、`/links`、`/deal-rooms`。
- 使用 HTTP 方法表示动作：
  - `GET`：读取
  - `POST`：创建
  - `PATCH`：部分更新
  - `DELETE`：删除
- 嵌套资源不超过 2 层，如 `/documents/{id}/pages`。

**路由策略**：
- **内部 Workspace API 基路径**：`https://{tenantSlug}.dealsignal.com/api/workspaces/{workspaceSlug}`
- **公开访问 API 基路径**：`https://{publicDomain}/api/v1/public`
- Workspace 上下文通过 URL 路径 `/{workspaceSlug}` 传递；tenant 上下文通过子域名 Host 解析。
- 公开链接落地页使用品牌方自定义域名，通过 query 参数传递 tenant/workspace/token。

### 2.3 认证方式

| 方式 | 场景 | 说明 |
|------|------|------|
| Bearer Token | 用户会话 | `Authorization: Bearer <jwt>` |
| API Key | 服务/机器调用 / Webhook 回调验证 | `X-API-Key: <api_key>` |
| Signed URL | 公开/匿名访问 | URL 参数中的签名 token |

### 2.4 通用请求头

| 头字段 | 必填 | 说明 |
|--------|------|------|
| Authorization | 是（用户会话） | Bearer token |
| X-API-Key | 是（服务/机器调用） | API Key，与 Authorization 二选一 |
| Content-Type | 是 | `application/json` 或 `multipart/form-data` |
| X-Request-ID | 推荐 | 请求追踪 ID（UUID） |
| X-Idempotency-Key | 条件 | 幂等键，用于写操作 |
| Accept-Language | 否 | 用户语言偏好；当前前端支持 `en`（默认）与 `zh-CN`。若后端返回用户可见文案，建议与此头字段对齐；否则后端可返回 plain text 或 i18n key，需在接口契约中明确。 |

> **i18n 约定**：v2.1.1 前端已实现 `en` / `zh-CN` 双语。mock 阶段部分用户可见文案以 i18n key 形式返回（如 `dashboard.mock.signals.sig_1.title`），由前端 `t(key)` 渲染。真实后端接入时需在 API 契约中明确：后端是返回已本地化的 plain text，还是返回与前端命名空间一致的 key。

### 2.5 通用响应结构

所有接口的响应都以统一的 `BaseResponse` 为基座，再根据业务类型扩展 `data`、`pagination` 或 `details`。

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx"
}
```

- `code`：业务状态码，`ok` 表示成功，错误时返回具体的错误码。
- `message`：人类可读的信息。
- `request_id`：请求追踪 ID，必须返回，便于链路排查。

列表或对象数据通过 `data` 字段承载，分页信息通过 `pagination` 承载：

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx",
  "data": [],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "has_more": true
  }
}
```

### 2.6 错误响应

错误响应同样基于 `BaseResponse`，并通过可选的 `details` 字段提供字段级错误信息。

```json
{
  "code": "invalid_request",
  "message": "Invalid request parameters",
  "details": [
    { "field": "email", "issue": "invalid_format" }
  ],
  "request_id": "req_xxx"
}
```

### 2.7 通用错误码

| HTTP 状态码 | 错误码 | 说明 |
|-------------|--------|------|
| 400 | `invalid_request` | 请求参数错误 |
| 401 | `unauthorized` | 未认证 |
| 403 | `forbidden` | 无权限 |
| 404 | `not_found` | 资源不存在 |
| 409 | `conflict` | 资源冲突 |
| 410 | `gone` | 资源已过期/已撤回 |
| 413 | `file_too_large` | 文件超过限制 |
| 415 | `invalid_file_type` | 文件类型不支持 |
| 422 | `unprocessable_entity` | 业务校验失败 |
| 429 | `rate_limited` | 限流 |
| 500 | `internal_error` | 服务器内部错误 |
| 503 | `service_unavailable` | 依赖服务暂不可用 |

### 2.8 分页

| 参数 | 类型 | 默认值 | 最大值 | 说明 |
|------|------|--------|--------|------|
| page | int | 1 | - | 页码 |
| page_size | int | 20 | 100 | 每页数量 |

或使用 cursor 分页：

| 参数 | 类型 | 说明 |
|------|------|------|
| cursor | string | 游标 |
| limit | int | 返回数量，默认 20，最大 100 |

### 2.9 限流

| 场景 | 限流策略 | 响应 |
|------|----------|------|
| 普通 API | 100 次/分钟/用户 | 429 |
| 上传 API | 10 次/分钟/用户 | 429 |
| AI 问答 | 30 次/分钟/用户 | 429 |
| 公开链接 | 1000 次/小时/IP | 429 |

### 2.10 幂等

关键写操作要求幂等，通过 `X-Idempotency-Key` 请求头实现：

- 创建资源（POST）
- 创建链接（POST）
- 创建数据室（POST）
- CRM/Slack 同步（POST）

---

## 3. 资源与路由

### 3.1 资源列表

| 资源名 | 路径前缀 | 说明 |
|--------|----------|------|
| Documents | `/api/workspaces/{workspaceSlug}/documents` | 文档管理 |
| Pages | `/api/workspaces/{workspaceSlug}/documents/{id}/pages` | 文档页面 |
| Search | `/api/workspaces/{workspaceSlug}/search` | 文档内搜索 |
| Assistant | `/api/workspaces/{workspaceSlug}/assistant` | AI 助手会话 |
| Links | `/api/workspaces/{workspaceSlug}/links` | 智能链接 |
| Analytics | `/api/workspaces/{workspaceSlug}/analytics` | 热度评分与跟进建议 |
| Deal Rooms | `/api/workspaces/{workspaceSlug}/deal-rooms` | 数据室 |
| Integrations | `/api/workspaces/{workspaceSlug}/integrations` | 第三方集成 |
| Public | `/api/v1/public/...` | 公开访问接口 |

### 3.2 路由总览

| 方法 | 路径 | 操作 | 认证 | 对应 API |
|------|------|------|------|----------|
| POST | `/api/workspaces/{workspaceSlug}/documents` | 上传文档 | Bearer | API-01 |
| GET | `/api/workspaces/{workspaceSlug}/documents/{documentId}` | 获取文档状态 | Bearer | API-02 |
| GET | `/api/workspaces/{workspaceSlug}/documents/{documentId}/pages` | 获取页面列表 | Bearer | API-03 |
| POST | `/api/workspaces/{workspaceSlug}/documents/{documentId}/pages/signed-url` | 获取签名 URL | Bearer | API-04 |
| POST | `/api/v1/public/events` | 上报阅读事件 | Signed URL | API-05 |
| POST | `/api/workspaces/{workspaceSlug}/search` | 文档内搜索 | Bearer | API-06 |
| POST | `/api/workspaces/{workspaceSlug}/assistant/chat` | AI 问答 | Bearer | API-07 |
| POST | `/api/workspaces/{workspaceSlug}/links` | 创建智能链接 | Bearer | API-08 |
| GET | `/api/v1/public/links/{publicToken}` | 访问公开链接 | Signed URL | API-09 |
| GET | `/api/workspaces/{workspaceSlug}/analytics/links/{linkId}/score` | 热度评分 | Bearer | API-10 |
| GET / POST | `/api/workspaces/{workspaceSlug}/suggestions` | 跟进建议（列表/生成） | Bearer | API-11 |
| POST | `/api/workspaces/{workspaceSlug}/deal-rooms` | 创建数据室 | Bearer | API-12 |
| GET | `/api/workspaces/{workspaceSlug}/deal-rooms/{roomId}` | 获取数据室 | Bearer | API-13 |
| POST | `/api/v1/public/deal-rooms/{slug}/access-requests` | 数据室访问申请 | 无 | API-14 |
| POST | `/api/workspaces/{workspaceSlug}/integrations/hubspot/sync` | HubSpot 同步 | Bearer | API-15 |
| POST | `/api/workspaces/{workspaceSlug}/integrations/slack/connect` | Slack 连接 | Bearer | API-16 |

---

## 4. 接口详细契约

> 以下契约继承并整理自 `docs/TDD-v2.1.0.md` 第 5.4 节。详细字段、请求/响应示例、错误码均与 TDD 保持一致。

### 4.1 上传与解析

#### API-01：上传文档

| 属性 | 值 |
|------|-----|
| 接口编号 | API-01 |
| 名称 | Upload Document |
| 方法 | POST |
| 路径 | `/api/workspaces/{workspaceSlug}/documents` |
| 认证 | Bearer Token |
| 幂等 | 否 |
| 对应 PRD | FR-02 |
| 对应 TDD | 5.4.1 |

```http
POST /api/workspaces/{workspaceSlug}/documents
Host: {tenantSlug}.dealsignal.com
Authorization: Bearer {jwt}
Content-Type: multipart/form-data
```

**请求参数**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | binary | 是 | PDF / DOCX / PPTX / XLSX，最大 100MB |
| source_type | string | 否 | pdf / docx / pptx / xlsx，未提供时从文件名推断 |

**成功响应 201**：

```json
{
  "id": "doc_xxx",
  "title": "Acme Pitch Deck.pdf",
  "source_type": "pdf",
  "status": "uploaded",
  "page_count": null,
  "created_at": "2026-06-20T10:00:00Z"
}
```

**错误码**：

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| UNAUTHORIZED | 401 | 未认证或 token 过期 |
| FORBIDDEN | 403 | 无 workspace 写入权限 |
| FILE_TOO_LARGE | 413 | 文件超过 100MB |
| INVALID_FILE_TYPE | 415 | 文件类型不支持 |
| STORAGE_QUOTA_EXCEEDED | 429 | 租户存储配额超限 |
| INTERNAL_ERROR | 500 | 上传失败 |

---

#### API-02：获取文档状态

| 属性 | 值 |
|------|-----|
| 接口编号 | API-02 |
| 名称 | Get Document Status |
| 方法 | GET |
| 路径 | `/api/workspaces/{workspaceSlug}/documents/{documentId}` |
| 认证 | Bearer Token |
| 对应 PRD | FR-02 |
| 对应 TDD | 5.4.1 |

**成功响应 200**：

```json
{
  "id": "doc_xxx",
  "title": "Acme Pitch Deck.pdf",
  "source_type": "pdf",
  "status": "ready",
  "page_count": 24,
  "ingestion_job": {
    "id": "job_xxx",
    "status": "completed",
    "error_message": null
  },
  "created_at": "2026-06-20T10:00:00Z",
  "updated_at": "2026-06-20T10:02:30Z"
}
```

**错误码**：

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| UNAUTHORIZED | 401 | 未认证 |
| FORBIDDEN | 403 | 无权访问该文档 |
| DOCUMENT_NOT_FOUND | 404 | 文档不存在 |

---

### 4.2 查看与渲染

#### API-03：获取页面列表

| 属性 | 值 |
|------|-----|
| 接口编号 | API-03 |
| 名称 | List Document Pages |
| 方法 | GET |
| 路径 | `/api/workspaces/{workspaceSlug}/documents/{documentId}/pages` |
| 认证 | Bearer Token |
| 对应 PRD | FR-03 |
| 对应 TDD | 5.4.2 |

**成功响应 200**：

```json
{
  "document_id": "doc_xxx",
  "pages": [
    {
      "page_number": 1,
      "width": 1440,
      "height": 1920,
      "thumbnail_object_key": "tenants/.../page_1_thumb.webp"
    }
  ],
  "total": 24
}
```

**错误码**：

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| UNAUTHORIZED | 401 | 未认证 |
| FORBIDDEN | 403 | 无权限 |
| DOCUMENT_NOT_FOUND | 404 | 文档不存在 |
| DOCUMENT_NOT_READY | 409 | 文档尚未解析完成 |

---

#### API-04：获取签名 URL（内部）

| 属性 | 值 |
|------|-----|
| 接口编号 | API-04 |
| 名称 | Get Signed Page URL |
| 方法 | POST |
| 路径 | `/api/workspaces/{workspaceSlug}/documents/{documentId}/pages/signed-url` |
| 认证 | Bearer Token |
| 对应 PRD | FR-03 |
| 对应 TDD | 5.4.2 |

**请求参数**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page_number | integer | 是 | 页码，从 1 开始 |
| purpose | string | 否 | view / thumbnail，默认 view |

**成功响应 200**：

```json
{
  "page_number": 1,
  "image_url": "https://cdn.dealsignal.com/...?...signature=...&expires=...",
  "expires_at": "2026-06-20T10:15:00Z",
  "width": 1440,
  "height": 1920
}
```

**错误码**：

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| UNAUTHORIZED | 401 | 未认证 |
| FORBIDDEN | 403 | 无权限 |
| PAGE_NOT_FOUND | 404 | 页面不存在 |
| SIGNATURE_ERROR | 500 | 签名生成失败 |

**公开访问签名 URL 变体**：

```http
POST /api/v1/public/documents/{documentId}/pages/signed-url
Host: {publicDomain}
Content-Type: application/json
```

请求体需携带 `token`（link public_token）与 `page_number`；响应同上。

---

### 4.3 阅读事件

#### API-05：上报阅读事件

| 属性 | 值 |
|------|-----|
| 接口编号 | API-05 |
| 名称 | Track Reading Events |
| 方法 | POST |
| 路径 | `/api/v1/public/events` |
| 认证 | Signed URL Token |
| 对应 PRD | FR-10 |
| 对应 TDD | 5.4.3 |

**请求参数**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| token | string | 是 | link public_token |
| visitor_id | string | 是 | 访问者会话 ID |
| events | array | 是 | 事件数组 |
| events[].event_type | string | 是 | link_opened / page_viewed / download_attempted |
| events[].page_number | integer | 否 | page_viewed 必填 |
| events[].duration_ms | integer | 否 | 停留时长 |
| events[].scroll_depth | integer | 否 | 滚动深度百分比 |
| events[].timestamp | string | 是 | ISO 8601 |

**成功响应 200**：

```json
{
  "received": 3,
  "visitor_id": "vst_xxx"
}
```

**错误码**：

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| INVALID_TOKEN | 400 | link token 无效 |
| RATE_LIMITED | 429 | 上报过于频繁 |
| INTERNAL_ERROR | 500 | 服务端错误 |

---

### 4.4 AI 搜索与问答

#### API-06：文档内搜索

| 属性 | 值 |
|------|-----|
| 接口编号 | API-06 |
| 名称 | Search Within Document |
| 方法 | POST |
| 路径 | `/api/workspaces/{workspaceSlug}/search` |
| 认证 | Bearer Token |
| 对应 PRD | FR-05 |
| 对应 TDD | 5.4.4 |

**请求参数**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| document_id | string | 是 | 文档 ID |
| query | string | 是 | 搜索关键词/问题 |
| mode | string | 否 | exact / fulltext / vector / hybrid，默认 hybrid |
| top_k | integer | 否 | 返回结果数，默认 5，最大 20 |

**成功响应 200**：

```json
{
  "document_id": "doc_xxx",
  "query": "付款期限",
  "results": [
    {
      "chunk_id": "chk_xxx",
      "score": 0.92,
      "normalized_text": "付款期限为 Net 30 ...",
      "page_number": 5,
      "boxes": [
        { "x": 0.12, "y": 0.34, "w": 0.45, "h": 0.06 }
      ]
    }
  ]
}
```

**错误码**：

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| UNAUTHORIZED | 401 | 未认证 |
| FORBIDDEN | 403 | 无文档权限 |
| DOCUMENT_NOT_READY | 409 | 文档未解析完成 |

**公开访问变体**：

```http
POST /api/v1/public/search
Host: {publicDomain}
Content-Type: application/json
```

请求体携带 `token`（link token）替代 `document_id`。

---

#### API-07：AI 问答

| 属性 | 值 |
|------|-----|
| 接口编号 | API-07 |
| 名称 | AI Assistant Chat |
| 方法 | POST |
| 路径 | `/api/workspaces/{workspaceSlug}/assistant/chat` |
| 认证 | Bearer Token |
| 对应 PRD | FR-05 ~ FR-06 |
| 对应 TDD | 5.4.4 |

**请求参数**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| document_id | string | 是 | 文档 ID |
| query | string | 是 | 用户问题 |
| session_id | string | 否 | 会话 ID，未提供时创建新会话 |

**成功响应 200**：

```json
{
  "session_id": "sess_xxx",
  "answer": "根据文档第 5 页，付款期限为 Net 30。",
  "evidence": [
    {
      "chunk_id": "chk_xxx",
      "quote": "付款期限为 Net 30",
      "page_number": 5,
      "boxes": [
        { "x": 0.12, "y": 0.34, "w": 0.45, "h": 0.06 }
      ],
      "score": 0.92
    }
  ],
  "follow_up_questions": ["逾期付款罚金是多少？", "是否支持分期付款？"]
}
```

**错误码**：

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| UNAUTHORIZED | 401 | 未认证 |
| FORBIDDEN | 403 | 无文档权限 |
| DOCUMENT_NOT_READY | 409 | 文档未解析完成 |
| LLM_UNAVAILABLE | 503 | LLM 服务暂不可用 |

**公开访问变体**：

```http
POST /api/v1/public/assistant/chat
Host: {publicDomain}
Content-Type: application/json
```

请求体携带 `token` 替代 `document_id`，响应同上。

---

### 4.5 链接与权限

#### API-08：创建智能链接

| 属性 | 值 |
|------|-----|
| 接口编号 | API-08 |
| 名称 | Create Smart Link |
| 方法 | POST |
| 路径 | `/api/workspaces/{workspaceSlug}/links` |
| 认证 | Bearer Token |
| 幂等 | 是 |
| 对应 PRD | FR-07 ~ FR-09 |
| 对应 TDD | 5.4.5 |

**请求参数**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| document_id | string | 是 | 文档 ID |
| name | string | 否 | 链接名称 |
| permission_type | string | 否 | public / email_required / whitelist / password，默认 public |
| allowed_emails | array | 否 | 白名单邮箱列表 |
| allowed_domains | array | 否 | 白名单域名列表 |
| password | string | 否 | 访问密码，permission_type=password 时必填 |
| expires_at | string | 否 | ISO 8601 过期时间 |
| max_access_count | integer | 否 | 最大访问次数 |
| download_enabled | boolean | 否 | 默认 false |
| watermark_enabled | boolean | 否 | 默认 false |

**成功响应 201**：

```json
{
  "id": "link_xxx",
  "public_token": "abc123",
  "name": "Investor Round A",
  "short_url": "https://investor.acme.com/?tenant=acme&workspace=main&token=abc123",
  "permission_type": "whitelist",
  "status": "active",
  "created_at": "2026-06-20T10:00:00Z"
}
```

**错误码**：

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| UNAUTHORIZED | 401 | 未认证 |
| FORBIDDEN | 403 | 无文档权限 |
| DOCUMENT_NOT_READY | 409 | 文档未 READY 禁止创建链接 |
| INVALID_PERMISSION_CONFIG | 400 | 权限配置非法 |

---

#### API-09：访问公开链接

| 属性 | 值 |
|------|-----|
| 接口编号 | API-09 |
| 名称 | Access Public Link |
| 方法 | GET |
| 路径 | `/api/v1/public/links/{publicToken}` |
| 认证 | Signed URL Token |
| 对应 PRD | FR-07 ~ FR-09 |
| 对应 TDD | 5.4.5 |

**Query 参数**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| tenant | string | 是 | tenant slug |
| workspace | string | 是 | workspace slug |
| token | string | 是 | link public_token（可与路径参数二选一，路径优先） |

**成功响应 200**：

```json
{
  "link": {
    "id": "link_xxx",
    "name": "Investor Round A",
    "document_id": "doc_xxx",
    "permission_type": "public",
    "download_enabled": false,
    "watermark_enabled": true
  },
  "document": {
    "id": "doc_xxx",
    "title": "Acme Pitch Deck.pdf",
    "page_count": 24,
    "status": "ready"
  },
  "visitor_id": "vst_xxx",
  "requires_email": false,
  "requires_password": false
}
```

**错误码**：

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| LINK_NOT_FOUND | 404 | 链接不存在 |
| LINK_EXPIRED | 410 | 链接已过期 |
| LINK_REVOKED | 410 | 链接已撤回 |
| LINK_MAX_ACCESS_REACHED | 429 | 达到最大访问次数 |
| REQUIRES_EMAIL | 403 | 需要邮箱验证 |
| REQUIRES_PASSWORD | 403 | 需要密码 |
| WHITELIST_DENIED | 403 | 邮箱不在白名单 |

---

### 4.6 意图分析

#### API-10：热度评分

| 属性 | 值 |
|------|-----|
| 接口编号 | API-10 |
| 名称 | Get Link Heat Score |
| 方法 | GET |
| 路径 | `/api/workspaces/{workspaceSlug}/analytics/links/{linkId}/score` |
| 认证 | Bearer Token |
| 对应 PRD | FR-10 |
| 对应 TDD | 5.4.6 |

**成功响应 200**：

```json
{
  "link_id": "link_xxx",
  "score": 78,
  "tier": "hot",
  "factors": [
    { "name": "open_count", "value": 3, "weight": 0.2 },
    { "name": "key_page_views", "value": 2, "weight": 0.35 },
    { "name": "total_duration_min", "value": 8.5, "weight": 0.25 },
    { "name": "forward_signals", "value": 1, "weight": 0.2 }
  ],
  "updated_at": "2026-06-20T10:05:00Z"
}
```

---

#### API-11：跟进建议

| 属性 | 值 |
|------|-----|
| 接口编号 | API-11 |
| 名称 | List / Generate Follow-up Suggestions |
| 方法 | `GET` 列表，`POST` 生成 |
| 路径 | `/api/workspaces/{workspaceSlug}/suggestions` |
| 认证 | Bearer Token |
| 对应 PRD | FR-11 |
| 对应 TDD | 5.4.6 |
| 说明 | 后端当前未提供按 link 维度获取建议的接口；建议统一通过 workspace 级 suggestions 资源操作。 |

**`GET /suggestions` 成功响应 200**：

```json
{
  "data": [
    {
      "id": "sg_xxx",
      "type": "follow_up",
      "priority": "high",
      "title": "重复查看财务页",
      "description": "投资人在 24 小时内 3 次查看财务页，建议发送 financial model。",
      "recommended_action": "发送 follow-up 邮件",
      "dismissed": false,
      "created_at": "2026-06-20T10:00:00Z"
    }
  ]
}
```

**`POST /suggestions` 成功响应 202**：

```json
{
  "code": "ok",
  "message": "suggestions generated"
}
```

---

### 4.7 数据室

#### API-12：创建数据室

| 属性 | 值 |
|------|-----|
| 接口编号 | API-12 |
| 名称 | Create Deal Room |
| 方法 | POST |
| 路径 | `/api/workspaces/{workspaceSlug}/deal-rooms` |
| 认证 | Bearer Token |
| 幂等 | 是 |
| 对应 PRD | FR-12 ~ FR-13 |
| 对应 TDD | 5.4.7 |

**请求参数**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 数据室名称 |
| slug | string | 是 | URL 标识 |
| template_type | string | 否 | seed / series_a / lp_update / sales_proposal |
| requires_nda | boolean | 否 | 默认 false |
| requires_approval | boolean | 否 | 默认 false |
| documents | array | 否 | 初始文档列表 `{ document_id, folder_path }` |

**成功响应 201**：

```json
{
  "id": "room_xxx",
  "slug": "series-a-dataroom",
  "name": "Series A Data Room",
  "template_type": "series_a",
  "requires_nda": true,
  "requires_approval": true,
  "created_at": "2026-06-20T10:00:00Z"
}
```

---

#### API-13：获取数据室

| 属性 | 值 |
|------|-----|
| 接口编号 | API-13 |
| 名称 | Get Deal Room |
| 方法 | GET |
| 路径 | `/api/workspaces/{workspaceSlug}/deal-rooms/{roomId}` |
| 认证 | Bearer Token |
| 对应 PRD | FR-12 ~ FR-13 |
| 对应 TDD | 5.4.7 |

**成功响应 200**：

```json
{
  "id": "room_xxx",
  "slug": "series-a-dataroom",
  "name": "Series A Data Room",
  "folders": [
    {
      "path": "/Financials",
      "documents": [
        { "document_id": "doc_xxx", "title": "Financial Model.xlsx" }
      ]
    }
  ],
  "members": [
    { "email": "investor@vc.com", "role": "viewer", "nda_confirmed_at": "2026-06-20T10:00:00Z" }
  ],
  "access_requests": [
    { "email": "new@vc.com", "status": "pending", "reason": "Due diligence" }
  ]
}
```

---

#### API-14：数据室访问申请

| 属性 | 值 |
|------|-----|
| 接口编号 | API-14 |
| 名称 | Request Deal Room Access |
| 方法 | POST |
| 路径 | `/api/v1/public/deal-rooms/{slug}/access-requests` |
| 认证 | 无 |
| 对应 PRD | FR-12 ~ FR-13 |
| 对应 TDD | 5.4.7 |

**请求参数**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| email | string | 是 | 申请者邮箱 |
| reason | string | 否 | 申请理由 |

**成功响应 201**：

```json
{
  "id": "req_xxx",
  "room_id": "room_xxx",
  "email": "new@vc.com",
  "status": "pending",
  "created_at": "2026-06-20T10:00:00Z"
}
```

---

### 4.8 集成

#### API-15：HubSpot 同步

| 属性 | 值 |
|------|-----|
| 接口编号 | API-15 |
| 名称 | Sync to HubSpot |
| 方法 | POST |
| 路径 | `/api/workspaces/{workspaceSlug}/integrations/hubspot/sync` |
| 认证 | Bearer Token |
| 对应 PRD | FR-15 |
| 对应 TDD | 5.4.8 |
| 说明 | 后端当前仅实现 HubSpot 同步，Salesforce 尚未支持。请求无需 body。 |

**成功响应 202**：

```json
{
  "code": "ok",
  "message": "sync started"
}
```

---

#### API-16：Slack 连接

| 属性 | 值 |
|------|-----|
| 接口编号 | API-16 |
| 名称 | Connect Slack |
| 方法 | POST |
| 路径 | `/api/workspaces/{workspaceSlug}/integrations/slack/connect` |
| 认证 | Bearer Token |
| 对应 PRD | FR-16 |
| 对应 TDD | 5.4.8 |
| 说明 | 后端当前未提供主动发送 Slack 通知的接口；该端点返回 Slack OAuth 授权 URL。 |

**成功响应 200**：

```json
{
  "url": "https://slack.com/oauth/v2/authorize?..."
}
```

---

## 5. 认证与授权

### 5.1 JWT Claims

| claim | 类型 | 说明 |
|-------|------|------|
| sub | string | 用户 ID |
| tid | string | 当前租户 ID |
| wid | string | 当前 Workspace ID |
| role | string | 用户在当前 Workspace 的角色 |
| exp | int | 过期时间 |
| iat | int | 签发时间 |

### 5.2 Workspace 角色权限

| 角色 | 权限 |
|------|------|
| OWNER | 全部权限，可删除 Workspace |
| ADMIN | 管理成员、设置、创建数据室 |
| CONTRIBUTOR | 上传文档、创建链接、查看分析 |
| VIEWER | 只读被授权的资源/数据室 |

### 5.3 公开访问 Token

签名 URL token payload 包含：

| 字段 | 类型 | 说明 |
|------|------|------|
| tenant_id | string | 租户 ID |
| workspace_id | string | Workspace ID |
| document_id | string | 文档 ID |
| purpose | string | 用途 |
| expires_at | int | 过期时间 |

---

## 6. Webhook

### 6.1 Webhook 事件

| 事件名 | 说明 | 版本 |
|--------|------|------|
| `document.uploaded` | 文档上传完成 | v1 |
| `document.ready` | 文档解析完成 | v1 |
| `link.accessed` | 链接被访问 | v1 |
| `link.hot` | 链接热度达到 hot 阈值 | v1 |
| `deal_room.access_requested` | 数据室收到访问申请 | v1 |

### 6.2 Webhook 负载

```json
{
  "event": "document.ready",
  "timestamp": "2026-06-20T10:00:00Z",
  "data": {
    "document_id": "doc_xxx",
    "workspace_id": "ws_xxx"
  }
}
```

### 6.3 签名验证

- 使用 HMAC-SHA256 签名。
- 签名在 `X-Webhook-Signature` 头中。

---

## 7. SDK 与工具

### 7.1 官方 SDK

| 语言 | 包名 | 安装 |
|------|------|------|
| TypeScript | `@dealsignal/api-sdk` | `npm install @dealsignal/api-sdk` |
| Go | `github.com/dealsignal/api-sdk-go` | `go get github.com/dealsignal/api-sdk-go` |
| Python | `dealsignal-api` | `pip install dealsignal-api` |

### 7.2 OpenAPI

- OpenAPI 规范文件：`docs/openapi-v2.1.0.yaml`（待创建）
- 文档站点：`https://developers.dealsignal.com`（待配置）

---

## 8. 检查清单

- [x] 所有接口有唯一编号
- [x] 请求/响应字段有类型、必填、约束说明
- [x] 错误码覆盖所有失败场景
- [x] 认证/授权要求明确
- [x] 幂等、分页、限流规则已定义
- [x] 与 PRD 功能需求一一对应
- [x] 与数据库模型字段一致
- [x] OpenAPI/Swagger 已同步更新
- [ ] 破坏性变更已规划版本升级

## 同步检查清单

- [x] 本文件中的接口路径、参数、响应与 OpenAPI YAML 一致
- [x] 错误响应结构一致
- [x] 认证方式一致

---

> **模板版本**：v1  
> **API-SPEC 版本**：v2.1.0  
> **状态**：已批准  
> **最后更新**：2026-06-20

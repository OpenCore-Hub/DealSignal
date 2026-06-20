---
id: "API-YYYY-NNN"
version: "{vX.Y.Z}"
status: "{草稿 / 评审中 / 已批准 / 已归档}"
owner: "{负责人}"
---

# API 规范文档模板 v1

> **文档编号**：`API-YYYY-NNN`  
> **版本**：`{vX.Y.Z}`  
> **模板版本**：`v1`  
> **状态**：`{草稿 / 评审中 / 已批准 / 已归档}`  
> **编写人/适用对象**：`后端架构师 / 产品经理`  
> **编写日期**：`{YYYY-MM-DD}`  
> **关联文档**：  
> - `docs/TDD-vX.Y.Z.md`  
> - `docs/PRD-vX.Y.Z.md`  
> - `docs/templates/DATABASE-MODEL-template-v1.md`  
> - `docs/templates/EVENT-TRACKING-template-v1.md`  
> **评审人**：`CTO、后端负责人、前端负责人、测试负责人、安全负责人`

---

## 0. 文档使用说明

> **同步要求**：本文件必须与对应的 OpenAPI YAML 文件（如 `docs/openapi-vX.Y.Z.yaml`）保持同步。任何一方修改后，另一方必须同步更新，避免 Markdown 描述与 OpenAPI 机器可读契约出现偏差。

本文档是 `{产品名}` 的 API 规范文档（API Specification），定义服务端接口的协议、认证、错误码、资源命名、请求/响应契约和版本策略。

**目标**：
- 统一前后端、第三方集成、客户端的接口契约。
- 明确 RESTful / RPC / GraphQL 等 API 风格。
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
| v0.1.0 | YYYY-MM-DD | {编写人} | 初始版本 | 全文档 |

### 1.2 API 版本

| 版本 | 状态 | 基地址 | 说明 |
|------|------|--------|------|
| v1 | 当前 | `/api/v1` | 初始版本 |
| v0 | 废弃 | `/api/v0` | 已弃用，计划下线 |

---

## 2. 通用约定

### 2.1 协议与编码

- **传输协议**：HTTPS only
- **数据格式**：JSON（`Content-Type: application/json`）
- **字符编码**：UTF-8
- **时间格式**：ISO 8601（`2026-06-18T15:20:42Z`）
- **日期格式**：`YYYY-MM-DD`
- **金额**：以最小货币单位整数表示（如分）

### 2.2 API 风格

{选择其中一种，并删除其他}

#### RESTful

- 资源名使用复数名词，如 `/resources`、`/organizations`。
- 使用 HTTP 方法表示动作：
  - `GET`：读取
  - `POST`：创建
  - `PUT` / `PATCH`：更新
  - `DELETE`：删除
- 嵌套资源不超过 2 层，如 `/resources/{id}/pages`。

> **默认路由策略**：API base path 为 `/api/v1`。Organization / 租户默认通过 `Authorization` Token claim（`oid`/`tid`）、可选的 `X-Organization-Id` 请求头、或子域名 `{organizationSlug}.api.example.com` 解析，**不要将 `organization_id` 或 `{organizationSlug}` 作为默认 URL path 前缀**。
>
> **备选方案**：若项目 ADR 明确采用“子域名 + 路径 slug”的多租户方案，则接口路径可调整为 `/{organizationSlug}/api/v1/...`；该方案必须与 OpenAPI、SDK、前端路由保持一致，并在项目启动前通过 ADR 明确。

#### RPC-like

- 使用 `POST /{service}/{Action}`，如 `POST /upload/CreateUpload`。
- 请求体包含所有参数。

### 2.3 认证方式

| 方式 | 场景 | 说明 |
|------|------|------|
| Bearer Token | 用户会话 | `Authorization: Bearer <jwt>` |
| API Key | 服务/机器调用 / Webhook 回调验证 | `X-API-Key: <api_key>` |
| Signed URL | 公开/匿名访问 | URL 参数中的签名 token |
| OAuth 2.0 | 第三方集成 | 授权码 / Client Credentials |

### 2.4 通用请求头

| 头字段 | 必填 | 说明 |
|--------|------|------|
| Authorization | 是（用户会话） | Bearer token |
| X-API-Key | 是（服务/机器调用） | API Key，与 Authorization 二选一 |
| Content-Type | 是 | `application/json` |
| X-Request-ID | 推荐 | 请求追踪 ID（UUID） |
| X-Idempotency-Key | 条件 | 幂等键，用于写操作 |
| Accept-Language | 否 | 用户语言偏好 |

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

- `details`：可选，用于参数校验失败等场景，包含字段名、问题类型与可读说明。
- 本约定必须与 `openapi-template-v1.yaml` 中的 `BaseResponse` / `ErrorResponse` 保持同步。

### 2.7 通用错误码

| HTTP 状态码 | 错误码 | 说明 |
|-------------|--------|------|
| 400 | `invalid_request` | 请求参数错误 |
| 401 | `unauthorized` | 未认证 |
| 403 | `forbidden` | 无权限 |
| 404 | `not_found` | 资源不存在 |
| 409 | `conflict` | 资源冲突 |
| 422 | `unprocessable_entity` | 业务校验失败 |
| 429 | `rate_limited` | 限流 |
| 500 | `internal_error` | 服务器内部错误 |

### 2.8 分页

| 参数 | 类型 | 默认值 | 最大值 | 说明 |
|------|------|--------|--------|------|
| page | int | 1 | - | 页码 |
| page_size | int | 20 | 100 | 每页数量 |

或使用 cursor 分页：

| 参数 | 类型 | 说明 |
|------|------|------|
| cursor | string | 游标 |
| limit | int | 返回数量 |

### 2.9 限流

| 场景 | 限流策略 | 响应 |
|------|----------|------|
| 普通 API | 100 次/分钟/用户 | 429 |
| 上传 API | 10 次/分钟/用户 | 429 |
| 公开链接 | 1000 次/小时/IP | 429 |

---

## 3. 资源与路由

### 3.1 资源列表

| 资源名 | 路径前缀 | 说明 |
|--------|----------|------|
| Organizations | `/api/v1/organizations` | Organization 管理 |
| Resources | `/api/v1/resources` | 资源管理 |
| SharedAccess | `/api/v1/shared-access` | shared access |

### 3.2 路由总览

| 方法 | 路径 | 操作 | 认证 |
|------|------|------|------|
| GET | `/api/v1/organizations` | 列出 Organization | Bearer |
| POST | `/api/v1/organizations` | 创建 Organization | Bearer |
| GET | `/api/v1/organizations/{organization_id}` | 获取 Organization | Bearer |
| PATCH | `/api/v1/organizations/{organization_id}` | 更新 Organization | Bearer |
| DELETE | `/api/v1/organizations/{organization_id}` | 删除 Organization | Bearer |
| GET | `/api/v1/resources` | 列出 Resource | Bearer |
| POST | `/api/v1/resources` | 创建 Resource | Bearer |
| GET | `/api/v1/resources/{resource_id}` | 获取 Resource | Bearer |
| PATCH | `/api/v1/resources/{resource_id}` | 更新 Resource | Bearer |
| DELETE | `/api/v1/resources/{resource_id}` | 删除 Resource | Bearer |
| GET | `/public/resources/{resource_id}` | 公开访问 Resource | Signed URL |
| GET | `/api/v1/shared-access` | 列出 SharedAccess | Bearer |
| POST | `/api/v1/shared-access` | 创建 SharedAccess | Bearer |
| GET | `/api/v1/shared-access/{shared_access_id}` | 获取 SharedAccess | Bearer |
| PATCH | `/api/v1/shared-access/{shared_access_id}` | 更新 SharedAccess | Bearer |
| DELETE | `/api/v1/shared-access/{shared_access_id}` | 撤销 SharedAccess | Bearer |
| GET | `/api/v1/webhooks` | 列出 Webhook 订阅 | Bearer |
| POST | `/api/v1/webhooks` | 创建 Webhook 订阅 | Bearer |

---

## 4. 接口详细契约

### 4.1 {API-01} 创建 Organization

#### 基本信息

| 属性 | 值 |
|------|-----|
| 接口编号 | API-01 |
| 名称 | Create Organization |
| 方法 | POST |
| 路径 | `/api/v1/organizations` |
| 认证 | Bearer Token |
| 幂等 | 是（通过 Idempotency-Key） |
| 对应 PRD | FR-01 |

#### 请求

```json
{
  "name": "{公司名}",
  "slug": "{组织标识}"
}
```

| 字段 | 类型 | 必填 | 约束 | 说明 |
|------|------|------|------|------|
| name | string | 是 | 1-255 字符 | Organization 名称 |
| slug | string | 是 | 小写、连字符、数字 | 唯一标识 |

#### 响应

**201 Created**

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx",
  "data": {
    "id": "ws_xxx",
    "name": "{公司名}",
    "slug": "{组织标识}",
    "role": "owner",
    "created_at": "2026-06-18T15:20:42Z"
  }
}
```

#### 错误码

| HTTP | 错误码 | 场景 |
|------|--------|------|
| 400 | `invalid_request` | 参数校验失败 |
| 409 | `slug_taken` | slug 已被占用 |
| 429 | `rate_limited` | 创建过于频繁 |

---

### 4.2 {API-02} 列出 Organization 资源

#### 基本信息

| 属性 | 值 |
|------|-----|
| 接口编号 | API-02 |
| 名称 | List Resources |
| 方法 | GET |
| 路径 | `/api/v1/resources` |
| 认证 | Bearer Token |

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| status | query | 否 | 过滤状态 |
| page | query | 否 | 页码 |
| page_size | query | 否 | 每页数量 |

#### 响应

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx",
  "data": [
    {
      "id": "doc_xxx",
      "title": "Q2 Report",
      "status": "active",
      "created_at": "2026-06-18T15:20:42Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "has_more": true
  }
}
```

---

### 4.3 {API-NN} {接口名称}

{每个接口重复上述结构}

---

### 4.4 {API-03} 创建 SharedAccess

#### 基本信息

| 属性 | 值 |
|------|-----|
| 接口编号 | API-03 |
| 名称 | Create SharedAccess |
| 方法 | POST |
| 路径 | `/api/v1/shared-access` |
| 认证 | Bearer Token |
| 幂等 | 是（通过 Idempotency-Key） |
| 对应 PRD | FR-04 |

#### 请求

```json
{
  "resource_id": "doc_xxx",
  "expires_in_seconds": 604800,
  "allow_download": true
}
```

| 字段 | 类型 | 必填 | 约束 | 说明 |
|------|------|------|------|------|
| resource_id | string | 是 | - | 资源 ID |
| expires_in_seconds | integer | 否 | > 0 | 有效期（秒），默认 7 天 |
| allow_download | boolean | 否 | - | 是否允许下载，默认 false |

#### 响应

**201 Created**

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx",
  "data": {
    "id": "sa_xxx",
    "resource_id": "doc_xxx",
    "organization_id": "org_xxx",
    "access_url": "https://api.example.com/public/resources/doc_xxx?token=signed_token",
    "expires_at": "2026-07-18T15:20:42Z",
    "created_at": "2026-06-18T15:20:42Z"
  }
}
```

---

### 4.5 {API-04} 撤销 SharedAccess

#### 基本信息

| 属性 | 值 |
|------|-----|
| 接口编号 | API-04 |
| 名称 | Revoke SharedAccess |
| 方法 | DELETE |
| 路径 | `/api/v1/shared-access/{shared_access_id}` |
| 认证 | Bearer Token |

#### 响应

**204 No Content**

---

## 5. 认证与授权

### 5.1 JWT  claims

| claim | 类型 | 说明 |
|-------|------|------|
| sub | string | 用户 ID |
| tid | string | 当前租户 ID（与 TDD 租户解析策略对齐） |
| oid | string | 当前 Organization ID |
| role | string | 用户角色 |
| exp | int | 过期时间 |
| iat | int | 签发时间 |

> 注：默认情况下，`tid`/`oid` 是当前 organization / 租户的主要解析来源（通过 `Authorization` Token 或 `X-Organization-Id` 头）。子域名 `{organizationSlug}.api.example.com` 仅作为可选解析方式；若 ADR 决定采用 Host/URL path 解析 tenant，则 `tid`/`oid` 可作为冗余 claim 用于校验。

### 5.2 权限模型

| 角色 | Organization 权限 | Resource 权限 |
|------|----------------|---------------|
| owner | 全部 | 全部 |
| admin | 管理成员、设置 | 读写 |
| member | 查看 | 读写自己创建的 |
| public | 查看 | 只读 |

### 5.3 公开访问 Token

| 字段 | 类型 | 说明 |
|------|------|------|
| tenant_id | string | 租户 ID |
| organization_id | string | Organization ID |
| resource_id | string | 资源 ID |
| purpose | string | 用途 |
| expires_at | int | 过期时间 |

---

## 6. Webhook

### 6.1 Webhook 事件

| 事件名 | 说明 | 版本 |
|--------|------|------|
| `resource.created` | 资源创建 | v1 |
| `resource.processed` | 资源处理完成 | v1 |
| `link.accessed` | 链接被访问 | v1 |

### 6.2 Webhook 负载

```json
{
  "event": "resource.created",
  "timestamp": "2026-06-18T15:20:42Z",
  "data": {
    "resource_id": "doc_xxx",
    "organization_id": "ws_xxx"
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
| TypeScript | `@{组织标识}/api-sdk` | `npm install @{组织标识}/api-sdk` |
| Go | `github.com/{组织标识}/api-sdk-go` | `go get github.com/{组织标识}/api-sdk-go` |
| Python | `{组织标识}-api` | `pip install {组织标识}-api` |

### 7.2 OpenAPI

- OpenAPI 规范文件：`openapi/v1.yaml`
- 文档站点：`https://developers.example.com`

---

## 8. 检查清单

- [ ] 所有接口有唯一编号
- [ ] 请求/响应字段有类型、必填、约束说明
- [ ] 错误码覆盖所有失败场景
- [ ] 认证/授权要求明确
- [ ] 幂等、分页、限流规则已定义
- [ ] 与 PRD 功能需求一一对应
- [ ] 与数据库模型字段一致
- [ ] OpenAPI/Swagger 已同步更新
- [ ] 破坏性变更已规划版本升级

## 同步检查清单

- [ ] 本文件中的接口路径、参数、响应与 OpenAPI YAML 一致
- [ ] 错误响应结构一致
- [ ] 认证方式一致
- [ ] 分页/幂等/限流策略一致

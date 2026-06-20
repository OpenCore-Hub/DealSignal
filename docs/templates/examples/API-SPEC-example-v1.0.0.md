---
id: "API-2024-021"
version: "v1.0.0"
status: "已批准"
owner: "后端架构师 / 产品经理"
---
> 本文件为示例，仅用于展示如何填写对应模板。实际项目中请替换为真实内容。

# API 规范：Shared Resource Link 访问分析

> **文档编号**：`API-2024-021`  
> **版本**：`v1.0.0`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`后端架构师 / 产品经理`  
> **编写日期**：`2024-06-18`  
> **关联文档**：  
> - `docs/TDD-v1.0.0.md`  
> - `docs/PRD-v1.0.0.md`  
> - `docs/DATABASE-MODEL-v1.0.0.md`  
> - `docs/templates/EVENT-TRACKING-template-v1.md`  
> **评审人**：`CTO、后端负责人、前端负责人、测试负责人、安全负责人`

---

## 0. 文档使用说明

本文档为 Shared Resource Link 访问分析的 API 规范示例，基于 `API-SPEC-template-v1.md` 与 `openapi-v1.0.0.yaml` 同步维护。任何一方修改后，另一方必须同步更新。

---

## 1. 文档控制信息

### 1.1 变更日志

| 版本 | 日期 | 修改人 | 修改内容 | 影响范围 |
|------|------|--------|----------|----------|
| v0.1.0 | 2024-06-15 | 钱进 | 初始版本 | 全文档 |
| v1.0.0 | 2024-06-18 | 钱进 | 评审通过 | 全文档 |

### 1.2 API 版本

| 版本 | 状态 | 基地址 | 说明 |
|------|------|--------|------|
| v1 | 当前 | `/api/v1` | Shared Resource Link 访问分析接口 |

---

## 2. 通用约定

### 2.1 协议与编码

- **传输协议**：HTTPS only
- **数据格式**：JSON（`Content-Type: application/json`）
- **字符编码**：UTF-8
- **时间格式**：ISO 8601（`2024-06-18T15:20:42Z`）
- **日期格式**：`YYYY-MM-DD`

### 2.2 API 风格

采用 RESTful 风格：

- 资源名使用复数名词，如 `/shared-resource-links`。
- 使用 HTTP 方法表示动作：GET 读取、POST 创建、PATCH 更新、DELETE 删除。
- 嵌套资源不超过 2 层，如 `/shared-resource-links/{id}/analytics`。

### 2.3 认证方式

| 方式 | 场景 | 说明 |
|------|------|------|
| Bearer Token | 用户会话 | `Authorization: Bearer <jwt>` |
| Signed URL | 公开/匿名访问 | URL 参数中的签名 token |

### 2.4 通用请求头

| 头字段 | 必填 | 说明 |
|--------|------|------|
| Authorization | 是 | Bearer token |
| Content-Type | 是 | `application/json` |
| X-Request-ID | 推荐 | 请求追踪 ID（UUID） |
| X-Idempotency-Key | 条件 | 幂等键，用于写操作 |

### 2.5 通用响应结构

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx"
}
```

### 2.6 错误响应

```json
{
  "code": "invalid_request",
  "message": "Invalid request parameters",
  "details": [
    { "field": "eventType", "issue": "invalid_value" }
  ],
  "request_id": "req_xxx"
}
```

### 2.7 通用错误码

| HTTP 状态码 | 错误码 | 说明 |
|-------------|--------|------|
| 400 | `invalid_request` | 请求参数错误 |
| 401 | `unauthorized` | 未认证或 token 过期 |
| 403 | `forbidden` | 无权限访问 |
| 404 | `not_found` | 资源不存在 |
| 409 | `conflict` | 资源冲突 |
| 422 | `unprocessable_entity` | 业务校验失败 |
| 429 | `rate_limited` | 限流 |
| 500 | `internal_error` | 服务器内部错误 |

### 2.8 限流

| 场景 | 限流策略 | 响应 |
|------|----------|------|
| 事件上报 | 每 IP 120 次/分钟 | 429 |
| 分析查询 | 每用户 60 次/分钟 | 429 |

---

## 3. 资源与路由

### 3.1 资源列表

| 资源名 | 路径前缀 | 说明 |
|--------|----------|------|
| Shared Resource Links | `/api/v1/shared-resource-links` | Shared Resource Link 管理 |
| Events | `/api/v1/shared-resource-links/{id}/events` | 访问事件 |
| Analytics | `/api/v1/shared-resource-links/{id}/analytics` | 访问分析 |

### 3.2 路由总览

| 方法 | 路径 | 操作 | 认证 |
|------|------|------|------|
| POST | `/api/v1/shared-resource-links/:linkId/events` | 上报访问事件 | Signed URL / Anonymous |
| GET | `/api/v1/shared-resource-links/:linkId/analytics` | 获取访问分析 | Bearer |

---

## 4. 接口详细契约

### 4.1 API-01 上报 Shared Resource Link 访问事件

#### 基本信息

| 属性 | 值 |
|------|-----|
| 接口编号 | API-01 |
| 名称 | Report Shared Resource Link Event |
| 方法 | POST |
| 路径 | `/api/v1/shared-resource-links/{linkId}/events` |
| 认证 | Signed URL token（通过 query 参数 `token`） |
| 幂等 | 是（通过 `clientEventId`） |
| 对应 PRD | FR-01 |
| 对应 TDD | 第 5.3 节 |

#### 请求

**路径参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| linkId | UUID | 是 | Shared Resource Link ID |

**Query 参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| token | string | 是 | Shared Resource Link 签名访问 token |

**请求体**：

```json
{
  "clientEventId": "evt_xxx",
  "pageId": "page-uuid",
  "eventType": "page_viewed",
  "durationMs": 5200,
  "visitorToken": "anon-token",
  "occurredAt": "2024-06-18T15:20:42Z"
}
```

| 字段 | 类型 | 必填 | 约束 | 说明 |
|------|------|------|------|------|
| clientEventId | string | 是 | 最大 64 字符 | 客户端事件唯一标识，用于幂等 |
| pageId | UUID | 是 | - | 页面 ID |
| eventType | string | 是 | page_viewed / download | 事件类型 |
| durationMs | int | 条件 | ≥ 0，eventType=page_viewed 时必填 | 停留时长（毫秒） |
| visitorToken | string | 是 | 最大 64 字符 | 匿名访客标识 |
| occurredAt | string | 否 | ISO 8601 | 事件发生时间，默认服务端时间 |

#### 响应

**200 OK**

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx",
  "data": {
    "accepted": true,
    "eventId": "evt_yyy"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| accepted | boolean | 是否接收成功 |
| eventId | string | 服务端事件 ID |

#### 错误码

| HTTP | 错误码 | 场景 |
|------|--------|------|
| 400 | `invalid_request` | 参数校验失败，如 eventType 不合法 |
| 401 | `unauthorized` | token 无效或过期 |
| 403 | `forbidden` | link 已撤销或 tracking 已关闭 |
| 404 | `shared_resource_link_not_found` | linkId 不存在 |
| 409 | `duplicate_event` | clientEventId 重复 |
| 429 | `rate_limited` | 超过限流阈值 |

---

### 4.2 API-02 获取 Shared Resource Link 访问分析

#### 基本信息

| 属性 | 值 |
|------|-----|
| 接口编号 | API-02 |
| 名称 | Get Shared Resource Link Analytics |
| 方法 | GET |
| 路径 | `/api/v1/shared-resource-links/{linkId}/analytics` |
| 认证 | Bearer Token |
| 幂等 | 否 |
| 对应 PRD | FR-02、FR-03 |
| 对应 TDD | 第 5.3 节 |

#### 请求

**路径参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| linkId | UUID | 是 | Shared Resource Link ID |

**Query 参数**：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| startDate | string | 否 | 7 天前 | 开始日期，YYYY-MM-DD |
| endDate | string | 否 | 今天 | 结束日期，YYYY-MM-DD |

#### 响应

**200 OK**

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx",
  "data": {
    "linkId": "link-uuid",
    "startDate": "2024-06-11",
    "endDate": "2024-06-18",
    "totalViews": 128,
    "uniqueVisitors": 42,
    "pageStats": [
      {
        "pageId": "p1",
        "pageNumber": 1,
        "avgDurationMs": 8200,
        "views": 96
      },
      {
        "pageId": "p2",
        "pageNumber": 2,
        "avgDurationMs": 4300,
        "views": 72
      }
    ],
    "visitorScores": [
      {
        "visitorToken": "anon-1",
        "heatLevel": "high",
        "totalViews": 8,
        "uniquePages": 3,
        "totalDurationMs": 98000,
        "lastSeenAt": "2024-06-18T10:00:00Z"
      },
      {
        "visitorToken": "anon-2",
        "heatLevel": "low",
        "totalViews": 1,
        "uniquePages": 1,
        "totalDurationMs": 2000,
        "lastSeenAt": "2024-06-17T08:30:00Z"
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| linkId | UUID | Shared Resource Link ID |
| startDate | string | 统计开始日期 |
| endDate | string | 统计结束日期 |
| totalViews | int | 总访问次数 |
| uniqueVisitors | int | 独立访客数 |
| pageStats | array | 每页访问统计 |
| visitorScores | array | 客户兴趣评分列表 |

**pageStats 字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| pageId | UUID | 页面 ID |
| pageNumber | int | 页码 |
| avgDurationMs | int | 平均停留时长（毫秒） |
| views | int | 访问次数 |

**visitorScores 字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| visitorToken | string | 匿名访客标识 |
| heatLevel | string | high / medium / low |
| totalViews | int | 总访问次数 |
| uniquePages | int | 访问的不同页面数 |
| totalDurationMs | int | 总停留时长（毫秒） |
| lastSeenAt | string | 最近访问时间 |

#### 错误码

| HTTP | 错误码 | 场景 |
|------|--------|------|
| 401 | `unauthorized` | 未认证或 token 过期 |
| 403 | `forbidden` | 非 link 所属 organization 成员 |
| 404 | `shared_resource_link_not_found` | linkId 不存在 |
| 422 | `invalid_date_range` | 日期范围不合法 |
| 429 | `rate_limited` | 超过限流阈值 |

---

## 5. 认证与授权

### 5.1 JWT claims

| claim | 类型 | 说明 |
|-------|------|------|
| sub | string | 用户 ID |
| tid | string | 当前租户 ID |
| wid | string | 当前 Organization ID |
| role | string | 用户角色 |
| exp | int | 过期时间 |

### 5.2 权限模型

| 角色 | Shared Resource Link 分析权限 |
|------|----------------------|
| owner | 可查看所有 link 分析 |
| admin | 可查看所有 link 分析 |
| member | 可查看自己创建及授权 link 分析 |
| public | 不可查看分析 |

### 5.3 Signed URL Token

| 字段 | 类型 | 说明 |
|------|------|------|
| link_id | string | Shared Resource Link ID |
| organization_id | string | Organization ID |
| purpose | string | 用途，如 `view` |
| expires_at | int | 过期时间 |

---

## 6. Webhook

### 6.1 Webhook 事件

| 事件名 | 说明 | 版本 |
|--------|------|------|
| `shared_resource_link.event_reported` | 访问事件被上报 | v1 |

### 6.2 Webhook 负载

```json
{
  "event": "shared_resource_link.event_reported",
  "timestamp": "2024-06-18T15:20:42Z",
  "data": {
    "link_id": "link-uuid",
    "organization_id": "ws-uuid",
    "event_type": "page_viewed"
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
| TypeScript | `@exampleorg/web-sdk` | `npm install @exampleorg/web-sdk` |

### 7.2 OpenAPI

- OpenAPI 规范文件：`docs/openapi-v1.0.0.yaml`
- 文档站点：`https://developers.example-org.example.com`

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

## 同步检查清单

- [x] 本文件中的接口路径、参数、响应与 OpenAPI YAML 一致
- [x] 错误响应结构一致
- [x] 认证方式一致
- [x] 分页/幂等/限流策略一致

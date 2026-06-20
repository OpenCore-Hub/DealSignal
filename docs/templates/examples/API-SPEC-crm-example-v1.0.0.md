---
id: "API-CRM-001"
version: "v1.0.0"
status: "已批准"
owner: "Tom Wang"
---

> 本文件为示例，仅用于展示如何填写对应模板。实际项目中请替换为真实内容。

# API 规范：CloudCRM v1

> **文档编号**：`API-CRM-001`  
> **版本**：`v1.0.0`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`Tom Wang / 后端开发、前端开发、集成开发者`  
> **编写日期**：`2024-06-20`  
> **关联文档**：
> - `docs/PRD-v1.0.0.md`
> - `docs/TDD-v1.0.0.md`
> - `docs/DATABASE-MODEL-v1.0.0.md`

---

## 1. 通用约定

- **Base URL**：`https://api.cloudcrm.example.com/api/v1`
- **认证**：Bearer Token（JWT），`Authorization: Bearer <jwt>`
- **数据格式**：JSON
- **通用响应结构**：`{ code, message, request_id, data, pagination }`
- **租户解析**：优先从 JWT 的 `oid` claim 获取 organization，其次 `X-Organization-Id` 请求头。

## 2. 资源与路由

| 资源名 | 路径前缀 | 说明 |
|--------|----------|------|
| Accounts | `/api/v1/accounts` | 客户账户管理 |
| Contacts | `/api/v1/contacts` | 联系人管理 |
| Opportunities | `/api/v1/opportunities` | 商机 Pipeline 管理 |
| Activities | `/api/v1/activities` | 活动记录 |
| Dashboard | `/api/v1/dashboard` | 销售仪表盘 |

## 3. 接口详细契约

### 3.1 创建 Account

| 属性 | 值 |
|------|-----|
| 接口编号 | CRM-01 |
| 名称 | Create Account |
| 方法 | POST |
| 路径 | `/api/v1/accounts` |
| 认证 | Bearer Token |

**请求体**：

```json
{
  "name": "CloudTech Inc.",
  "industry": "SaaS",
  "website": "https://cloudtech.example.com"
}
```

**响应 201**：

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx",
  "data": {
    "id": "acc_xxx",
    "name": "CloudTech Inc.",
    "industry": "SaaS",
    "website": "https://cloudtech.example.com",
    "owner_id": "usr_xxx",
    "created_at": "2024-06-20T10:00:00Z"
  }
}
```

### 3.2 创建 Contact

| 属性 | 值 |
|------|-----|
| 接口编号 | CRM-02 |
| 名称 | Create Contact |
| 方法 | POST |
| 路径 | `/api/v1/contacts` |
| 认证 | Bearer Token |

**请求体**：

```json
{
  "account_id": "acc_xxx",
  "first_name": "张伟",
  "email": "zhangwei@cloudtech.example.com",
  "title": "CEO"
}
```

### 3.3 更新 Opportunity 阶段

| 属性 | 值 |
|------|-----|
| 接口编号 | CRM-03 |
| 名称 | Update Opportunity Stage |
| 方法 | PATCH |
| 路径 | `/api/v1/opportunities/:id/stage` |
| 认证 | Bearer Token |

**请求体**：

```json
{
  "stage": "proposal",
  "reason": "客户确认预算"
}
```

**业务规则**：
- 阶段只能按状态机单向推进。
- 变更原因在 `closed-lost` 阶段必填。

### 3.4 查询 Dashboard Pipeline

| 属性 | 值 |
|------|-----|
| 接口编号 | CRM-04 |
| 名称 | Get Pipeline Dashboard |
| 方法 | GET |
| 路径 | `/api/v1/dashboard/pipeline` |
| 认证 | Bearer Token |

**响应 200**：

```json
{
  "code": "ok",
  "data": {
    "total_amount_cents": 125000000,
    "stages": [
      { "stage": "prospecting", "count": 12, "amount_cents": 20000000 },
      { "stage": "proposal", "count": 5, "amount_cents": 45000000 },
      { "stage": "closed-won", "count": 3, "amount_cents": 60000000 }
    ]
  }
}
```

## 4. 错误码

| HTTP | 错误码 | 说明 |
|------|--------|------|
| 400 | `invalid_request` | 参数校验失败 |
| 401 | `unauthorized` | Token 无效 |
| 403 | `forbidden` | 无权限访问该租户数据 |
| 409 | `invalid_stage_transition` | 商机阶段状态机非法 |
| 422 | `missing_reason` | closed-lost 缺少原因 |

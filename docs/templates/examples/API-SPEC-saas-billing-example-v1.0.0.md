---
id: "API-BILLING-001"
version: "v1.0.0"
status: "已批准"
owner: "David Park"
---

> 本文件为示例，仅用于展示如何填写对应模板。实际项目中请替换为真实内容。

# API 规范：SubFlow Billing v1

> **文档编号**：`API-BILLING-001`  
> **版本**：`v1.0.0`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`David Park / 后端开发、支付集成、财务系统`  
> **编写日期**：`2024-06-20`  
> **关联文档**：
> - `docs/PRD-v1.0.0.md`
> - `docs/TDD-v1.0.0.md`
> - `docs/DATABASE-MODEL-v1.0.0.md`

---

## 1. 通用约定

- **Base URL**：`https://api.subflow.example.com/api/v1`
- **认证**：Bearer Token（JWT）
- **金额单位**：整数分（CNY），例如 `120000` 表示 ¥1,200.00
- **通用响应结构**：`{ code, message, request_id, data, pagination }`

## 2. 资源与路由

| 资源名 | 路径前缀 | 说明 |
|--------|----------|------|
| Plans | `/api/v1/plans` | 套餐配置 |
| Subscriptions | `/api/v1/subscriptions` | 订阅生命周期 |
| SubscriptionItems | `/api/v1/subscription-items` | 订阅项与用量 |
| Invoices | `/api/v1/invoices` | 发票管理 |
| Payments | `/api/v1/payments` | 支付与重试 |

## 3. 接口详细契约

### 3.1 创建 Plan

| 属性 | 值 |
|------|-----|
| 接口编号 | BILL-01 |
| 名称 | Create Plan |
| 方法 | POST |
| 路径 | `/api/v1/plans` |
| 认证 | Bearer Token（admin） |

**请求体**：

```json
{
  "name": "Pro Plan",
  "interval": "year",
  "unit_amount_cents": 120000,
  "currency": "CNY",
  "trial_days": 14
}
```

**响应 201**：

```json
{
  "code": "ok",
  "data": {
    "id": "plan_pro",
    "name": "Pro Plan",
    "interval": "year",
    "unit_amount_cents": 120000,
    "currency": "CNY",
    "trial_days": 14
  }
}
```

### 3.2 创建订阅

| 属性 | 值 |
|------|-----|
| 接口编号 | BILL-02 |
| 名称 | Create Subscription |
| 方法 | POST |
| 路径 | `/api/v1/subscriptions` |
| 认证 | Bearer Token |

**请求体**：

```json
{
  "customer_id": "cus_xxx",
  "plan_id": "plan_pro",
  "interval": "year"
}
```

### 3.3 上报用量

| 属性 | 值 |
|------|-----|
| 接口编号 | BILL-03 |
| 名称 | Report Usage |
| 方法 | POST |
| 路径 | `/api/v1/subscription-items/:id/usage` |
| 认证 | Bearer Token（服务间） |
| 幂等 | 是（X-Idempotency-Key） |

**请求体**：

```json
{
  "quantity": 1500,
  "recorded_at": "2024-06-20T10:00:00Z"
}
```

**响应 201**：

```json
{
  "code": "ok",
  "data": {
    "usage_record_id": "ur_xxx",
    "quantity": 1500,
    "recorded_at": "2024-06-20T10:00:00Z"
  }
}
```

### 3.4 查询发票

| 属性 | 值 |
|------|-----|
| 接口编号 | BILL-04 |
| 名称 | List Invoices |
| 方法 | GET |
| 路径 | `/api/v1/invoices` |
| 认证 | Bearer Token |

**响应 200**：

```json
{
  "code": "ok",
  "data": [
    {
      "id": "inv_xxx",
      "customer_id": "cus_xxx",
      "status": "open",
      "total_cents": 120000,
      "amount_due_cents": 120000,
      "due_date": "2024-07-01"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 1,
    "has_more": false
  }
}
```

## 4. 错误码

| HTTP | 错误码 | 说明 |
|------|--------|------|
| 400 | `invalid_request` | 参数校验失败 |
| 409 | `duplicate_usage_record` | 用量上报 idempotency_key 冲突 |
| 422 | `invalid_plan_transition` | 不允许的订阅变更 |
| 422 | `amount_mismatch` | 发票金额与支付金额不一致 |

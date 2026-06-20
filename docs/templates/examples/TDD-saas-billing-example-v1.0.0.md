---
id: "TDD-2024-BILLING-001"
version: "v1.0.0"
status: "已批准"
owner: "David Park"
linked_prd: "docs/PRD-v1.0.0.md"
linked_architecture: "docs/ARCHITECTURE-v1.0.0.md"
ai_red_flags:
  - "所有金额计算使用整数分，禁止任何浮点数中间结果"
  - "订阅周期起始/结束必须按 UTC 午夜计算，不可依赖运行时本地时区"
  - "用量上报必须幂等，重复上报同一 idempotency_key 不可重复计费"
  - "invoice 一旦 open 后不可修改金额，只允许 credit note 冲正"
ai_confidence: "high"
pending_confirmation: []
---

> 本文件为示例，仅用于展示如何填写对应模板。实际项目中请替换为真实内容。

# TDD：SubFlow SaaS 订阅计费平台

> **文档编号**：`TDD-2024-BILLING-001`  
> **版本**：`v1.0.0`  
> **模板版本**：`v2`  
> **状态**：`已批准`  
> **编写人/适用对象**：`David Park / 计费架构师、后端开发、测试、财务系统负责人`  
> **编写日期**：`2024-06-20`  
> **关联文档**：
> - `docs/PRD-v1.0.0.md`
> - `docs/DATABASE-MODEL-v1.0.0.md`
> - `docs/API-SPEC-v1.0.0.md`
> **评审人**：`CTO、后端负责人、财务负责人、安全负责人`

---

## 1. 概述与目标

### 1.1 设计目标

基于 PRD-2024-BILLING-001，构建高可靠、可审计的订阅计费引擎，支持 Plan/Price 配置、订阅生命周期、用量计费、发票与支付回收。

### 1.2 设计原则

| 原则 | 说明 | 落地方式 |
|------|------|----------|
| 不可变账单 | 已生成 invoice 金额不可改 | 只追加 credit note |
| 幂等性 | 所有资金相关操作幂等 | idempotency_key + 唯一索引 |
| 时区安全 | 所有周期按 UTC 计算 | 专用日期工具库 |

---

## 2. 架构总览

### 2.1 系统架构

```text
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│  Admin UI   │ ───▶ │  Billing API │ ───▶ │ PostgreSQL  │
│  Customer   │      │  (Node.js)   │      │  (Ledger)   │
│  Portal     │      └──────┬───────┘      └─────────────┘
└─────────────┘             │
                            ▼
                     ┌──────────────┐
                     │  Usage Queue │
                     │  (Kafka)     │
                     └──────┬───────┘
                            ▼
                     ┌──────────────┐
                     │  Invoice     │
                     │  Scheduler   │
                     └──────────────┘
```

### 2.2 服务边界

| 服务 | 职责 | 技术栈 | 关键 SLO |
|------|------|--------|----------|
| Billing API | Plan/Subscription/Invoice CRUD | Node.js / Fastify | P99 < 300ms |
| Usage Ingestion | 用量上报、去重、汇总 | Go | 10k events/min |
| Invoice Scheduler | 周期 invoice 生成、续期 | Node.js | 延迟 ≤ 1min |
| Dunning Worker | 失败支付重试 | Node.js | 按策略执行 |

---

## 3. 数据架构

### 3.1 价格表 `prices`

```sql
CREATE TABLE prices (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  plan_id UUID NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('recurring','usage')),
  interval TEXT CHECK (interval IN ('month','year')),
  unit_amount_cents BIGINT,
  currency TEXT NOT NULL DEFAULT 'CNY',
  usage_pricing_model TEXT CHECK (usage_pricing_model IN ('tiered','volume')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.2 订阅表 `subscriptions`

```sql
CREATE TABLE subscriptions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID NOT NULL,
  plan_id UUID NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('trialing','active','past_due','canceled','paused')),
  current_period_start DATE NOT NULL,
  current_period_end DATE NOT NULL,
  cancel_at_period_end BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_subscriptions_customer ON subscriptions (customer_id, status);
CREATE INDEX idx_subscriptions_period_end ON subscriptions (current_period_end);
```

### 3.3 发票表 `invoices`

```sql
CREATE TABLE invoices (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID NOT NULL,
  subscription_id UUID,
  status TEXT NOT NULL CHECK (status IN ('draft','open','paid','void','uncollectible')),
  total_cents BIGINT NOT NULL DEFAULT 0,
  amount_due_cents BIGINT NOT NULL DEFAULT 0,
  due_date DATE,
  paid_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_invoices_customer_status ON invoices (customer_id, status);
```

### 3.4 用量记录表 `usage_records`

```sql
CREATE TABLE usage_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  subscription_item_id UUID NOT NULL,
  quantity BIGINT NOT NULL,
  idempotency_key TEXT NOT NULL UNIQUE,
  recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_usage_records_item ON usage_records (subscription_item_id, recorded_at);
```

---

## 4. 关键接口

### 4.1 创建订阅

```http
POST /api/v1/subscriptions
Authorization: Bearer <jwt>
Content-Type: application/json
```

```json
{
  "customer_id": "cus_xxx",
  "plan_id": "plan_pro",
  "interval": "year",
  "trial_days": 14
}
```

### 4.2 上报用量

```http
POST /api/v1/subscription-items/si_xxx/usage
Authorization: Bearer <jwt>
Content-Type: application/json
X-Idempotency-Key: idem_xxx
```

```json
{
  "quantity": 1500,
  "recorded_at": "2024-06-20T10:00:00Z"
}
```

---

## 5. 周期与金额计算

- 周期起始日为订阅创建当天 UTC 午夜。
- 月订阅周期：`current_period_end = current_period_start + 1 month - 1 day`。
- proration 按剩余天数比例计算，向上取整到分。
- 所有金额中间计算使用 `bigint` 分，最终展示时除以 100。

---

## 6. 安全与合规

- 支付敏感信息不落地，仅保存 payment gateway 返回的 token 与 ID。
- 所有资金写操作记录 `ledger_entries` 不可变流水。
- 角色权限：财务人员可查看 invoice，不可修改 invoice 金额。

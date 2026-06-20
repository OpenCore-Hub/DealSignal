---
id: "DB-BILLING-001"
version: "v1.0.0"
status: "已批准"
owner: "David Park"
---

> 本文件为示例，仅用于展示如何填写对应模板。实际项目中请替换为真实内容。

# 数据库模型：SubFlow Billing v1

> **文档编号**：`DB-BILLING-001`  
> **版本**：`v1.0.0`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`David Park / 后端开发、DBA、财务系统`  
> **编写日期**：`2024-06-20`  
> **关联文档**：
> - `docs/PRD-v1.0.0.md`
> - `docs/TDD-v1.0.0.md`
> - `docs/API-SPEC-v1.0.0.md`

---

## 1. 数据库选型

| 组件 | 选型 | 版本 | 部署方式 | 备注 |
|------|------|------|----------|------|
| 主数据库 | PostgreSQL | 15.x | 阿里云 RDS | 事务型账单数据 |
| 缓存 | Redis | 7.x | Cluster | 幂等键、限流 |
| 消息队列 | Kafka | 3.x | 托管 | 用量上报、调度事件 |

## 2. 领域划分

- **产品域**：plans、prices、price_tiers
- **客户订阅域**：customers、subscriptions、subscription_items
- **用量域**：usage_records
- **账单支付域**：invoices、invoice_line_items、payments、ledger_entries、credit_notes

## 3. 核心表结构

### 3.1 plans

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | TEXT | PK | 套餐 ID，如 plan_pro |
| name | TEXT | NOT NULL | 套餐名称 |
| description | TEXT | - | 描述 |
| active | BOOLEAN | NOT NULL DEFAULT true | 是否可售 |
| created_at | TIMESTAMPTZ | DEFAULT now() | 创建时间 |

### 3.2 prices

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | 价格 ID |
| plan_id | TEXT | FK plans.id | 所属套餐 |
| type | TEXT | NOT NULL CHECK | recurring / usage |
| interval | TEXT | - | month / year |
| unit_amount_cents | BIGINT | - | 单价（分） |
| currency | TEXT | NOT NULL | CNY |
| usage_pricing_model | TEXT | - | tiered / volume |

### 3.3 subscriptions

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | 订阅 ID |
| customer_id | UUID | NOT NULL | 客户 ID |
| plan_id | TEXT | NOT NULL | 套餐 ID |
| status | TEXT | NOT NULL CHECK | 状态 |
| current_period_start | DATE | NOT NULL | 当前周期开始 |
| current_period_end | DATE | NOT NULL | 当前周期结束 |
| cancel_at_period_end | BOOLEAN | DEFAULT false | 周期末取消 |

### 3.4 invoice_line_items

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | 行项 ID |
| invoice_id | UUID | NOT NULL FK | 发票 ID |
| description | TEXT | NOT NULL | 描述 |
| amount_cents | BIGINT | NOT NULL | 金额（分） |
| quantity | BIGINT | - | 数量 |
| period_start | DATE | - | 计费周期开始 |
| period_end | DATE | - | 计费周期结束 |

### 3.5 ledger_entries

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | 流水 ID |
| customer_id | UUID | NOT NULL | 客户 ID |
| invoice_id | UUID | - | 关联发票 |
| type | TEXT | NOT NULL | debit / credit |
| amount_cents | BIGINT | NOT NULL | 金额（分） |
| currency | TEXT | NOT NULL | CNY |
| created_at | TIMESTAMPTZ | DEFAULT now() | 创建时间 |

## 4. 索引策略

- `subscriptions`：`idx_subscriptions_customer_status` (customer_id, status)
- `subscriptions`：`idx_subscriptions_period_end` (current_period_end) 用于续期调度
- `usage_records`：`idx_usage_records_item` (subscription_item_id, recorded_at)
- `invoices`：`idx_invoices_customer_status` (customer_id, status)
- `ledger_entries`：`idx_ledger_customer_created` (customer_id, created_at)

## 5. 数据一致性

- invoice 金额 = 所有 invoice_line_items.amount_cents 之和。
- 支付成功后，payments.amount_cents 累计到 invoices.paid_amount_cents。
- 任何金额修正通过 credit_notes + ledger_entries 完成，不修改原 invoice。

---
id: "TDD-2024-CRM-001"
version: "v1.0.0"
status: "已批准"
owner: "Tom Wang"
linked_prd: "docs/PRD-v1.0.0.md"
linked_architecture: "docs/ARCHITECTURE-v1.0.0.md"
ai_red_flags:
  - "所有客户数据查询必须强制带上 tenant_id 过滤"
  - "商机阶段状态机必须单向推进，closed-lost 也不可回退到 open 阶段"
  - "活动记录不可物理删除，仅允许软删"
ai_confidence: "high"
pending_confirmation: []
---

> 本文件为示例，仅用于展示如何填写对应模板。实际项目中请替换为真实内容。

# TDD：CloudCRM 联系人管理与销售Pipeline

> **文档编号**：`TDD-2024-CRM-001`  
> **版本**：`v1.0.0`  
> **模板版本**：`v2`  
> **状态**：`已批准`  
> **编写人/适用对象**：`Tom Wang / 架构师、后端开发、前端开发、测试、DevOps`  
> **编写日期**：`2024-06-20`  
> **关联文档**：
> - `docs/PRD-v1.0.0.md`
> - `docs/DATABASE-MODEL-v1.0.0.md`
> - `docs/API-SPEC-v1.0.0.md`
> **评审人**：`架构师、后端负责人、前端负责人、安全负责人`

---

## 1. 概述与目标

### 1.1 设计目标

基于 PRD-2024-CRM-001，构建一个多租户 CRM 系统，支持联系人、账户、商机、活动的管理和 Pipeline 分析。

### 1.2 设计原则

| 原则 | 说明 | 落地方式 |
|------|------|----------|
| 租户隔离 | 所有业务数据按 organization 隔离 | 行级 tenant_id/organization_id 过滤 |
| 可审计 | 关键字段变更留痕 | 审计表 + 事件日志 |
| 可扩展 | 阶段和字段可配置 | 元数据表驱动 |

---

## 2. 架构总览

### 2.1 系统架构

```text
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   Web App   │ ───▶ │   API GW     │ ───▶ │  CRM API    │
│  (React)    │      │  (Auth/RBAC) │      │  (Node.js)  │
└─────────────┘      └──────────────┘      └──────┬──────┘
                                                  │
                                                  ▼
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│  Dashboard  │ ◀─── │  Analytics   │ ◀─── │ PostgreSQL  │
│  (Read)     │      │  Worker      │      │             │
└─────────────┘      └──────────────┘      └─────────────┘
```

### 2.2 服务边界

| 服务 | 职责 | 技术栈 | 关键 SLO |
|------|------|--------|----------|
| CRM API | 业务 CRUD、权限校验 | Node.js / Fastify | P99 < 200ms |
| Analytics Worker | 聚合 Pipeline 指标 | Node.js | 延迟 ≤ 5min |
| PostgreSQL | 业务数据持久化 | PostgreSQL 15 | 可用性 99.9% |

---

## 3. 数据架构

### 3.1 账户表 `accounts`

```sql
CREATE TABLE accounts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  organization_id UUID NOT NULL,
  name TEXT NOT NULL,
  industry TEXT,
  website TEXT,
  owner_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_accounts_tenant ON accounts (tenant_id, organization_id);
CREATE INDEX idx_accounts_owner ON accounts (owner_id);
```

### 3.2 联系人表 `contacts`

```sql
CREATE TABLE contacts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  organization_id UUID NOT NULL,
  account_id UUID NOT NULL REFERENCES accounts(id),
  first_name TEXT NOT NULL,
  last_name TEXT,
  email TEXT,
  phone_encrypted TEXT,
  title TEXT,
  owner_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_contacts_account ON contacts (account_id);
CREATE INDEX idx_contacts_tenant_name ON contacts (tenant_id, organization_id, last_name, first_name);
```

### 3.3 商机表 `opportunities`

```sql
CREATE TABLE opportunities (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  organization_id UUID NOT NULL,
  account_id UUID NOT NULL REFERENCES accounts(id),
  name TEXT NOT NULL,
  stage TEXT NOT NULL CHECK (stage IN ('prospecting','qualification','proposal','negotiation','closed-won','closed-lost')),
  amount_cents BIGINT NOT NULL DEFAULT 0,
  expected_close_date DATE,
  owner_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_opportunities_stage ON opportunities (tenant_id, organization_id, stage);
CREATE INDEX idx_opportunities_close_date ON opportunities (expected_close_date);
```

### 3.4 活动表 `activities`

```sql
CREATE TABLE activities (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  organization_id UUID NOT NULL,
  contact_id UUID REFERENCES contacts(id),
  opportunity_id UUID REFERENCES opportunities(id),
  type TEXT NOT NULL CHECK (type IN ('call','email','meeting','task')),
  subject TEXT NOT NULL,
  notes TEXT,
  due_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_activities_contact ON activities (contact_id, created_at DESC);
CREATE INDEX idx_activities_opportunity ON activities (opportunity_id, created_at DESC);
```

---

## 4. 关键接口

### 4.1 创建联系人

```http
POST /api/v1/contacts
Authorization: Bearer {jwt}
Content-Type: application/json
```

```json
{
  "account_id": "acc_xxx",
  "first_name": "张伟",
  "last_name": "",
  "email": "zhangwei@cloudtech.com",
  "title": "CEO"
}
```

### 4.2 更新商机阶段

```http
PATCH /api/v1/opportunities/opp_xxx/stage
Authorization: Bearer {jwt}
Content-Type: application/json
```

```json
{
  "stage": "proposal",
  "reason": "客户确认预算"
}
```

---

## 5. 安全与合规

- 所有查询强制附加 `tenant_id = current_tenant_id()`。
- 手机号使用 AES-256-GCM 加密存储。
- 商机阶段变更写入 `opportunity_stage_history` 审计表。
- 导出功能按角色过滤，禁止导出超出权限范围的数据。

---
id: "DB-CRM-001"
version: "v1.0.0"
status: "已批准"
owner: "Tom Wang"
---

> 本文件为示例，仅用于展示如何填写对应模板。实际项目中请替换为真实内容。

# 数据库模型：CloudCRM v1

> **文档编号**：`DB-CRM-001`  
> **版本**：`v1.0.0`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`Tom Wang / 后端开发、DBA、架构师`  
> **编写日期**：`2024-06-20`  
> **关联文档**：
> - `docs/PRD-v1.0.0.md`
> - `docs/TDD-v1.0.0.md`
> - `docs/API-SPEC-v1.0.0.md`

---

## 1. 数据库选型

| 组件 | 选型 | 版本 | 部署方式 | 备注 |
|------|------|------|----------|------|
| 主数据库 | PostgreSQL | 15.x | 阿里云 RDS | 主业务数据 |
| 缓存 | Redis | 7.x | Cluster | 会话、限流、临时聚合 |
| 搜索引擎 | PostgreSQL tsvector | - | 内置 | 联系人/账户全文检索 |

## 2. 领域划分

- **租户与用户域**：organizations、users、memberships
- **销售域**：accounts、contacts、opportunities、opportunity_stage_history
- **活动域**：activities、activity_reminders

## 3. 核心表结构

### 3.1 accounts

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | 账户 ID |
| tenant_id | UUID | NOT NULL | 租户 ID |
| organization_id | UUID | NOT NULL | 组织 ID |
| name | TEXT | NOT NULL | 公司名称 |
| industry | TEXT | - | 行业 |
| website | TEXT | - | 官网 |
| owner_id | UUID | NOT NULL | 负责销售 |
| created_at | TIMESTAMPTZ | DEFAULT now() | 创建时间 |
| updated_at | TIMESTAMPTZ | DEFAULT now() | 更新时间 |

### 3.2 contacts

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | 联系人 ID |
| tenant_id | UUID | NOT NULL | 租户 ID |
| organization_id | UUID | NOT NULL | 组织 ID |
| account_id | UUID | FK accounts.id | 所属账户 |
| first_name | TEXT | NOT NULL | 名 |
| last_name | TEXT | - | 姓 |
| email | TEXT | - | 邮箱 |
| phone_encrypted | TEXT | - | 加密手机号 |
| title | TEXT | - | 职位 |
| owner_id | UUID | NOT NULL | 负责人 |
| deleted_at | TIMESTAMPTZ | - | 软删时间 |

### 3.3 opportunities

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | 商机 ID |
| tenant_id | UUID | NOT NULL | 租户 ID |
| organization_id | UUID | NOT NULL | 组织 ID |
| account_id | UUID | FK accounts.id | 关联账户 |
| name | TEXT | NOT NULL | 商机名称 |
| stage | TEXT | NOT NULL CHECK | 阶段 |
| amount_cents | BIGINT | NOT NULL DEFAULT 0 | 金额（分） |
| expected_close_date | DATE | - | 预计成交日 |
| owner_id | UUID | NOT NULL | 负责人 |

### 3.4 opportunity_stage_history

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | 审计 ID |
| opportunity_id | UUID | FK | 商机 ID |
| from_stage | TEXT | - | 原阶段 |
| to_stage | TEXT | NOT NULL | 新阶段 |
| reason | TEXT | - | 变更原因 |
| changed_by | UUID | NOT NULL | 操作人 |
| changed_at | TIMESTAMPTZ | DEFAULT now() | 变更时间 |

## 4. 索引策略

- `accounts`：`idx_accounts_tenant` (tenant_id, organization_id)
- `contacts`：`idx_contacts_account` (account_id)；`idx_contacts_tenant_name` 支持按姓名排序
- `opportunities`：`idx_opportunities_stage` (tenant_id, organization_id, stage)
- `activities`：`idx_activities_contact` (contact_id, created_at DESC)

## 5. 迁移策略

- 使用 Atlas 管理 schema 变更。
- 所有新增非空字段必须提供默认值或分两步迁移（先加可空，再填默认值，后改非空）。

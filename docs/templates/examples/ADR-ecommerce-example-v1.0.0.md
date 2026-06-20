---
id: "ADR-ECOMMERCE-001"
version: "v1.0.0"
status: "已批准"
owner: "Bob Smith"
deciders:
  - "Bob Smith"
  - "Carol Lee"
date: "2024-06-20"
---

# ADR-ECOMMERCE-001：订单服务选用 PostgreSQL 作为持久化存储

## 状态

已批准

## 上下文

电商平台订单服务需要支持：
- 高一致性的事务（库存扣减、优惠券核销、支付状态流转）。
- 复杂查询（按用户、按状态、按时间范围、按商户）。
- 未来可能的多租户数据隔离。

候选方案：
- **A：PostgreSQL**（关系型数据库）
- **B：MongoDB**（文档数据库）
- **C：CockroachDB**（分布式 SQL）

## 决策

选用 **PostgreSQL** 作为订单服务主存储。

## 理由

1. **ACID 事务**：订单创建涉及库存、优惠券、支付多表一致性更新，PostgreSQL 提供成熟的事务支持。
2. **复杂查询**：订单列表、商家报表、售后查询需要多表 JOIN 与范围查询，PostgreSQL 优化器成熟。
3. **团队熟悉度**：团队已有 3 年 PostgreSQL 运维经验，能降低学习成本。
4. **生态工具**：支持 pg_dump、逻辑复制、PostGIS（未来物流场景扩展）、丰富 ORM 支持。

## 负面影响

- 水平扩展需通过分库分表或读写分离实现，比 MongoDB/CockroachDB 复杂。
- 大数据量历史订单查询需配合归档/冷热分离策略。

## 替代方案

| 方案 | 优点 | 缺点 | 不选原因 |
|------|------|------|----------|
| MongoDB | 灵活 schema、易水平扩展 | 多文档事务弱于 PostgreSQL | 订单场景强一致性优先 |
| CockroachDB | 原生分布式、强一致性 | 运维复杂度高、成本高 | 当前阶段无需全球分布式 |

## 相关决策

- ADR-ECOMMERCE-002：订单历史数据归档策略
- ADR-ECOMMERCE-003：库存扣减并发控制方案

## 备注

本决策将在日订单量超过 1000 万时重新评估是否需要引入 CockroachDB 或分库分表。

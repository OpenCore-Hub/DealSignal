---
id: "TDD-2024-031"
version: "v1.0.0"
status: "已批准"
owner: "技术负责人"
linked_prd: "docs/PRD-v1.0.0.md"
linked_architecture: "docs/ARCHITECTURE-v1.0.0.md"
ai_red_flags:
  - "公开访问接口必须做签名 token 校验，不能依赖 session"
  - "埋点写入不能阻塞主请求响应"
  - "不得将 organization 级敏感信息写入公开 URL"
ai_confidence: "high"
pending_confirmation:
  - "确认 analytics 聚合是否采用独立只读副本"
---

> 本文件为示例，仅用于展示如何填写对应模板。实际项目中请替换为真实内容。

# TDD：Shared Resource Link 访问分析

> **文档编号**：`TDD-2024-031`  
> **版本**：`v1.0.0`  
> **模板版本**：`v2`  
> **状态**：`已批准`  
> **编写人/适用对象**：`技术团队`  
> **编写日期**：`2024-06-19`  
> **关联文档**：  
> - `docs/PRD-v1.0.0.md`  
> - `docs/DATABASE-MODEL-v1.0.0.md`  
> - `docs/API-SPEC-v1.0.0.md`  
> **评审人**：`架构师、后端负责人、前端负责人、DevOps、安全负责人`

---

## 0. 文档使用说明

本文档为 Shared Resource Link 访问分析的技术设计示例，基于 `TDD-template-v2.md` 填写。目标读者为架构师、后端/前端开发、测试、DevOps、安全。

---

## 1. 文档控制信息

### 1.1 变更日志

| 版本 | 日期 | 修改人 | 修改内容 | 影响范围 |
|------|------|--------|----------|----------|
| v0.1.0 | 2024-06-17 | 赵强 | 初始版本 | 全文档 |
| v1.0.0 | 2024-06-20 | 赵强 | 评审通过 | 全文档 |

### 1.2 关联文档

| 文档类型 | 名称 | 路径 |
|----------|------|------|
| PRD | 《Shared Resource Link 访问分析 PRD》 | `docs/PRD-v1.0.0.md` |
| 数据库模型 | 《Shared Resource Link 数据库模型》 | `docs/DATABASE-MODEL-v1.0.0.md` |
| API 规范 | 《Shared Resource Link API 规范》 | `docs/API-SPEC-v1.0.0.md` |

---

## 2. 概述与目标

### 2.1 设计目标

本文档基于 `PRD-2024-042` 编制，重点解决：

1. **高可靠地采集前端页面浏览事件**：弱网、异常关闭下尽量减少事件丢失。
2. **低延迟地聚合并展示访问分析数据**：5 分钟内完成从事件采集到分析页展示。
3. **保证多租户数据隔离**：不同 organization 的分析数据严格隔离。

### 2.2 设计原则

| 原则 | 说明 | 落地方式 |
|------|------|----------|
| **异步解耦** | 事件采集与聚合解耦 | Redis + Worker |
| **数据安全优先** | 多租户隔离，敏感数据不暴露 | 行级隔离 + 脱敏展示 |
| **可观测性** | 链路延迟、丢失率、错误率可监控 | Prometheus + Grafana |
| **成本可控** | 分析数据使用列式存储 | PostgreSQL 托管实例 |

### 2.3 范围边界

**包含**：前端埋点 SDK、事件接收 API、异步聚合任务、分析查询 API、前端分析页。
**不包含**：实时 WebSocket 推送、邮件/通知系统改造、AI 智能跟进建议。

---

## 3. 架构总览

### 3.1 系统架构图

```text
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│  Web Public │ ───▶ │  Events API  │ ───▶ │   Redis     │
│  (React)    │      │  (Node.js)   │      │             │
└─────────────┘      └──────────────┘      └──────┬──────┘
                                                  │
                                                  ▼
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│  Analytics  │ ◀─── │  Aggregator  │ ◀─── │ PostgreSQL  │
│  Dashboard  │      │  (Worker)    │      │             │
└─────────────┘      └──────────────┘      └─────────────┘
```

### 3.2 服务边界

| 服务 | 职责 | 技术栈 | 关键 SLO |
|------|------|--------|----------|
| Events API | 接收并校验事件，写入 Redis | Node.js / Fastify | P99 < 100ms |
| Aggregator Worker | 消费事件，计算聚合与热度评分 | Node.js | 成功率 > 99% |
| Analytics API | 提供分析数据查询 | Node.js / Fastify | P99 < 500ms |
| PostgreSQL | 时序事件存储与聚合查询 | PostgreSQL 15+ | 可用性 99.9% |

---

## 4. 数据架构

### 4.1 事件表 `shared_resource_link_events`（PostgreSQL）

```sql
CREATE TABLE shared_resource_link_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  link_id UUID NOT NULL,
  page_id UUID NOT NULL,
  visitor_token TEXT NOT NULL,
  event_type TEXT NOT NULL CHECK (event_type IN ('page_viewed', 'download')),
  duration_ms INTEGER,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  tenant_id UUID NOT NULL,
  organization_id UUID NOT NULL,
  user_agent TEXT,
  ip_hash TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_shared_resource_link_events_link_occurred
  ON shared_resource_link_events (link_id, occurred_at);
CREATE INDEX idx_shared_resource_link_events_occurred
  ON shared_resource_link_events (occurred_at);
```

字段说明：`link_id` / `page_id` / `visitor_token` / `event_type` / `duration_ms` / `occurred_at` 为核心事件字段；`tenant_id` / `organization_id` 用于多租户隔离；`user_agent` / `ip_hash` 用于分析且脱敏存储。事件保留 90 天，可通过定时任务或分区策略清理。

### 4.2 聚合表 `shared_resource_link_analytics`（PostgreSQL）

```sql
CREATE TABLE shared_resource_link_analytics (
  link_id UUID NOT NULL,
  date DATE NOT NULL,
  total_views INTEGER NOT NULL DEFAULT 0,
  unique_visitors INTEGER NOT NULL DEFAULT 0,
  page_stats JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (link_id, date)
);

CREATE INDEX idx_shared_resource_link_analytics_date
  ON shared_resource_link_analytics (date);
```

字段说明：`link_id` + `date` 为聚合键；`total_views` 为总访问次数；`unique_visitors` 为独立访客数；`page_stats` 为每页访问统计 JSON。

### 4.3 热度评分表 `shared_resource_link_visitor_scores`（PostgreSQL）

```sql
CREATE TABLE shared_resource_link_visitor_scores (
  link_id UUID NOT NULL,
  visitor_token TEXT NOT NULL,
  score_level TEXT NOT NULL CHECK (score_level IN ('high', 'medium', 'low')),
  calculated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (link_id, visitor_token)
);
```

字段说明：按 `link_id` + `visitor_token` 维护最近 7 天滚动窗口的热度档位。

### 4.4 索引策略

- `shared_resource_link_events` 主键为 `id`；额外建立 `(link_id, occurred_at)` 与 `(occurred_at)` 索引以加速聚合任务与按时间清理。
- `shared_resource_link_analytics` 主键为 `(link_id, date)`；额外建立 `(date)` 索引以支持按日期范围的仪表盘查询。
- `shared_resource_link_visitor_scores` 主键为 `(link_id, visitor_token)`，可直接用于热度筛选。

---

## 5. 接口设计

### 5.1 接口设计原则

- RESTful API，JSON 格式，基路径 `/api/v1`。
- Organization 通过 `Authorization` Token 中的 `wid` claim 解析。
- 写操作接口要求幂等，关键接口携带 `X-Idempotency-Key`。

### 5.2 接口契约

#### API-01：事件上报

```http
POST /api/v1/shared-resource-links/:linkId/events
Authorization: Bearer {jwt}
Content-Type: application/json
```

**请求体**：

```json
{
  "pageId": "page-uuid",
  "eventType": "page_viewed",
  "durationMs": 5200,
  "visitorToken": "anon-token"
}
```

**响应**：

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx",
  "data": { "ok": true }
}
```

#### API-02：分析查询

```http
GET /api/v1/shared-resource-links/:linkId/analytics
Authorization: Bearer {jwt}
```

**响应**：

```json
{
  "code": "ok",
  "message": "success",
  "request_id": "req_xxx",
  "data": {
    "linkId": "link-uuid",
    "totalViews": 128,
    "uniqueVisitors": 42,
    "pageStats": [
      { "pageId": "p1", "avgDurationMs": 8200, "views": 96 }
    ],
    "visitorScores": [
      { "visitorToken": "anon-1", "heatLevel": "high" }
    ]
  }
}
```

### 5.4 错误码汇总

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| UNAUTHORIZED | 401 | 未认证或 token 过期 |
| FORBIDDEN | 403 | 无权限访问该 link |
| shared_resource_link_not_found | 404 | link 不存在 |
| RATE_LIMITED | 429 | 请求过于频繁 |

---

## 6. 核心模块设计

### 6.1 事件采集 SDK

- **职责**：监听文档查看器页面切换、停留、下载事件；生成并维护 visitor_token；批量上报事件，失败时本地缓存。
- **流程**：页面打开启动计时器 → 停留 ≥ 3 秒 / 切换页面 / 点击下载时组装事件入队 → 定时批量上报；成功清空，失败写入 IndexedDB 缓存（最多 50 条）。
- **异常处理**：网络断开时写入 IndexedDB 恢复后重试；429 时指数退避；400 时丢弃并记录日志。

### 6.2 聚合服务

- **职责**：每 5 分钟从 PostgreSQL 读取未聚合事件，计算 total_views、unique_visitors、page_stats 与热度评分，写入聚合表与评分表。
- **流程**：定时触发 → 读取最近 5 分钟窗口事件 → 按 link / page / visitor 分组聚合 → 计算热度评分 → 写入 `shared_resource_link_analytics` 与 `shared_resource_link_visitor_scores`。

---

## 7. 安全与性能设计

### 7.1 安全设计

- **认证与授权**：分析查询 API 必须携带有效 Bearer Token；事件上报接口需校验 Shared Resource Link 是否启用追踪；Repository 层自动注入 `tenant_id` + `organization_id` 过滤条件。
- **多租户隔离**：所有业务表包含 `tenant_id` 和 `organization_id`；用户只能查询其所属 organization 的 Shared Resource Link 分析数据。
- **隐私保护**：客户 IP 仅存储哈希值，UA 仅用于内部分析，均不展示给用户；提供 opt-out 开关，关闭后不采集事件。
- **审计日志**：访问分析页与修改追踪设置均需记录 user_id、link_id、timestamp、ip。

### 7.2 性能设计

| 服务/接口 | SLI | SLO | 测量方式 |
|-----------|-----|-----|----------|
| 事件上报 API | P99 延迟 | < 100ms | Prometheus histogram |
| 分析查询 API | P99 延迟 | < 500ms | Prometheus histogram |
| 聚合任务 | 成功率 | > 99% | 自定义指标 |
| 事件丢失率 | 丢失率 | < 0.1% | 对账指标 |

- **缓存**：聚合结果缓存 Redis 1 分钟，link 元数据缓存 5 分钟。
- **限流**：事件上报每 IP 120 次/分钟，分析查询每用户 60 次/分钟。
- **数据库优化**：PostgreSQL 按 `link_id` + `occurred_at` / `date` 分区；PostgreSQL 查询带 `organization_id` 过滤。

---

## 8. 测试策略

### 8.1 测试分层

| 测试类型 | 覆盖范围 | 工具 | 通过标准 |
|----------|----------|------|----------|
| 单元测试 | 聚合逻辑、热度评分规则 | Jest | 覆盖率 ≥ 70% |
| 集成测试 | API 接口、数据库交互 | Supertest + testcontainers | P0 用例 100% 通过 |
| 接口测试 | API-01、API-02 | Postman | P0 接口 100% 通过 |
| E2E 测试 | 创建链接 → 模拟访问 → 查看分析 | Playwright | P0 路径 100% 通过 |
| 性能测试 | 分析查询 API | k6 | 达到 SLO |

### 8.2 关键测试场景

1. **多租户隔离**：用户 A 无法查询用户 B 的 link 分析数据。
2. **事件幂等**：同一 visitor 30 秒内重复访问同一页面只记录一次。
3. **弱网恢复**：断网后本地缓存事件，恢复后成功上报。
4. **热度评分**：高、中、低三档边界值计算正确。


---

## 9. 部署与运维

### 9.1 部署拓扑

- 事件采集服务：2 实例，无状态。
- 聚合 Worker：1 主 + 1 备，处理 Redis 消费与 PostgreSQL 写入。
- 分析 API：与现有 API 服务同部署，读取 PostgreSQL。

### 9.2 可观测性

- 指标：事件采集 QPS、聚合延迟、PostgreSQL 查询 P99。
- 日志：聚合任务每次批处理记录条数与耗时。
- 告警：聚合延迟 > 5 分钟、PostgreSQL 写入失败率 > 1%。

---

## 10. 测试策略

- 单元测试：覆盖率 ≥ 80%，重点测试事件解析与热度评分规则。
- 集成测试：事件写入 → Redis → 聚合 → PostgreSQL → API 查询全链路。
- 性能测试：模拟 10 万事件/分钟峰值，验证聚合延迟。

---

## 11. 风险与依赖

| 风险 | 缓解措施 |
|------|----------|
| PostgreSQL 运维经验不足 | 先用托管服务，预留两周学习窗口 |
| 事件顺序乱序导致聚合偏差 | 使用事件时间戳 + 5 分钟 watermark |

---

## 12. 决策记录

| 决策 | 原因 |
|------|------|
| 事件采集走 Redis 而非直接写 PostgreSQL | 解耦峰值，保护存储 |
| 热度评分规则化 | 数据量不足，ML 模型不可解释 |

---

## 13. 附录

### 13.1 参考文档

- PRD：`docs/PRD-v1.0.0.md`
- API 规范：`docs/API-SPEC-v1.0.0.md`

---

## 14. 待确认事项

1. PostgreSQL 保留策略是否按 organization 分区。
2. 热度评分权重是否在产品上线后 A/B 测试调整。

---

## 15. 检查清单（文档发布前必须完成）

- [ ] 所有占位符已替换为实际内容
- [ ] 所有 P0 接口已有 OpenAPI / 详细契约
- [ ] 所有 P0 数据表已有 DDL 和索引
- [ ] SLO/SLI 已定义且可测量
- [ ] 与 PRD 的关键决策一致

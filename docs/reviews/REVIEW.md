# DealSignal 文档分享缺口修复任务计划：一致性 Review

**Review 日期**：2026-07-05  
**Reviewer**：Kimi Code CLI  
**Review 对象**：
- `docs/reviews/document-sharing-design-vs-implementation-gap-report.md`
- `docs/tasks/document-sharing-gap-remediation/README.md`
- `docs/tasks/document-sharing-gap-remediation/TASK-SHARE-*.md`（14 个任务文件）
- 与现有 `docs/tasks/agent-tasks-v2.1.3/` 的交叉影响

---

## 1. Review 结论：可开始执行 ✅

任务计划与缺口分析报告**基本一致**，14 个任务覆盖了报告中列出的所有高/中优先级缺口。存在 3 处需要前置协调或边界澄清的事项，但不影响整体启动。

---

## 2. 缺口 → 任务 覆盖度矩阵

### 2.1 高优先级缺口（🔴）

| 缺口报告章节 | 缺口描述 | 覆盖任务 | 覆盖状态 | 备注 |
|---|---|---|---|---|
| §4.1 | 事件体系从 18 种缩水到 3 种 | TASK-SHARE-MID-002 | ✅ 完整覆盖 | 补齐 forward/return/scroll/ai 等事件 |
| §4.1 | 滚动深度字段废弃 | TASK-SHARE-MID-002 | ✅ 完整覆盖 | 新增 `scroll_depth_recorded` 事件与聚合 |
| §4.1 | 缺少安全审计事件 | TASK-SHARE-SHORT-003 | ✅ 完整覆盖 | security_gate_failed / expired / max_access / revoked |
| §4.1 | 缺少 AI 交互事件 | TASK-SHARE-MID-002 | ✅ 完整覆盖 | ai_question_asked / ai_answer_viewed / ai_evidence_clicked |
| §4.2 | 无 WebSocket/SSE 实时推送 | TASK-SHARE-LONG-003 | ✅ 完整覆盖 | 长期实现 |
| §4.2 | 无 10 分钟事件合并 | TASK-SHARE-MID-003 | ✅ 完整覆盖 | 通知规则引擎包含合并逻辑 |
| §4.3 | 公共 viewer AI 标签未按 flag 渲染 | TASK-SHARE-SHORT-001 | ✅ 完整覆盖 | RightSidebar 条件渲染 |
| §4.3 | 无公共 AI 端点 | TASK-SHARE-SHORT-001 | ✅ 完整覆盖 | 新增 `/api/v1/public/assistant/chat` |
| §4.3 | `assistant_sessions.link_id/document_id` 未写入 | TASK-SHARE-SHORT-001 | ✅ 完整覆盖 | resolveSession 扩展 |
| §4.3 | AI 问题未作为意图信号 | TASK-SHARE-LONG-002 | ✅ 完整覆盖 | AI 问答意图分析 |
| §4.4 | Key Page Views 定义偏差（停留 ≥3s） | TASK-SHARE-MID-001 | ✅ 完整覆盖 | 后端关键词匹配 |
| §4.4 | 无 30min/5min 去重 | TASK-SHARE-SHORT-004 | ✅ 完整覆盖 | 基础去重实现 |

### 2.2 中优先级缺口（🟡）

| 缺口报告章节 | 缺口描述 | 覆盖任务 | 覆盖状态 | 备注 |
|---|---|---|---|---|
| §4.4 | 缺少会话模型 (`link_accesses`) | 未单独覆盖 | ⚠️ 部分覆盖 | TASK-SHARE-SHORT-004 去重可复用 `access_logs`；建议补充说明是否引入 `link_accesses` 表 |
| §4.5 | 通知收件人固定为 SMTP_USER | TASK-SHARE-SHORT-002 | ✅ 完整覆盖 | 按 link.created_by 查 users.email |
| §4.5 | 前端无 email_enabled 开关 | TASK-SHARE-SHORT-002 | ✅ 完整覆盖 | IntegrationStatus 暴露 |
| §4.5 | 无通知规则引擎 | TASK-SHARE-MID-003 | ✅ 完整覆盖 | notification_rules 表 + 合并 |
| §4.5 | CRM 只同步 contact/deal，无 timeline/task | TASK-SHARE-LONG-004 | ✅ 完整覆盖 | CRM 深度集成 |
| §4.6 | Dashboard 非实时 | TASK-SHARE-LONG-003 | ✅ 完整覆盖 | WebSocket/SSE |
| §4.6 | 异常访问提醒未实现 | TASK-SHARE-SHORT-003 / TASK-SHARE-MID-003 | ✅ 完整覆盖 | 安全审计 + 规则引擎 |
| §4.6 | 自动生成 follow-up 文案 | 未覆盖 | ❌ 缺口 | 建议新增 TASK-SHARE-MID-006 或在 LONG-002 中扩展 |
| §4.7 | 动态水印未完整实现 | TASK-SHARE-MID-004 | ✅ 完整覆盖 | Canvas 动态水印 |
| §4.7 | 签名 URL 未实现 | TASK-SHARE-MID-005 | ✅ 完整覆盖 | HMAC 签名 URL |
| §4.7 | NDA 文本未版本化存储 | 未覆盖 | ❌ 缺口 | 建议补充到 TASK-SHARE-SHORT-003 或新增小任务 |
| §4.8 | `analytics_jobs` 未使用 | TASK-SHARE-LONG-001 / LONG-003 | ⚠️ 间接覆盖 | 长期任务会引入实时/物化机制，但未直接实现 analytics_jobs |
| §4.8 | 过期链接无后台清理 | 未覆盖 | ❌ 缺口 | 建议新增 TASK-SHARE-MID-006 后台任务 |

### 2.3 低优先级缺口（🟢）

| 缺口报告章节 | 缺口描述 | 覆盖任务 | 覆盖状态 | 备注 |
|---|---|---|---|---|
| §1.1 / §4.7 | 品牌化/自定义域名 | 未覆盖 | ✅ 符合预期 | 明确 out of MVP，可延后 |

---

## 3. 发现的问题与澄清建议

### 3.1 🔴 必须与现有 v2.1.3 任务协调

#### 3.1.1 前置依赖：TASK-BACKEND-011

- **问题**：`agent-tasks-v2.1.3/TASK-BACKEND-011` 正在整理后端未落库改动（migrations 013~016、integration、search、upload 等）。
- **风险**：若我们的短期任务（SHORT-001 ~ SHORT-004）与 BACKEND-011 并行开发，migration 编号会冲突，integration/notification 代码也可能冲突。
- **建议**：
  - **必须在 TASK-BACKEND-011 完成后** 再启动 TASK-SHARE-SHORT-001 / SHORT-002 / SHORT-003。
  - 或在 BACKEND-011 合并后基于最新 main 切出 share-short 分支。

#### 3.1.2 前后端 Key Page 重叠：TASK-FRONTEND-010 vs TASK-SHARE-MID-001

- **问题**：
  - `TASK-FRONTEND-010`（v2.1.3 P0）修复前端 `heatScore.ts` 的 `topKeyPages` 展示逻辑。
  - `TASK-SHARE-MID-001`（中期 P1）修复后端 `key_page_views` 统计逻辑。
- **风险**：两边关键词/相似度算法可能不一致，导致 Dashboard 展示与后端评分不一致。
- **建议**：
  - 在执行 TASK-FRONTEND-010 时，先与 TASK-SHARE-MID-001 对齐关键词集合与匹配策略。
  - 最佳做法：将关键词配置放到后端配置表，前端只负责展示，不参与评分逻辑。

#### 3.1.3 E2E 覆盖重叠：TASK-TEST-003

- **问题**：`TASK-TEST-003` 计划编写 viewer/links/dashboard 的 E2E。
- **风险**：我们的任务会改变 viewer AI、通知、事件去重等行为，导致已写好的 E2E 失效。
- **建议**：
  - 将 `TASK-SHARE-SHORT-001` / `SHORT-002` / `SHORT-004` 作为 `TASK-TEST-003` 的前置输入。
  - 或在 TASK-TEST-003 中预留 AI/notification/dedup 的测试桩，待 share 任务完成后补全。

### 3.2 🟡 任务范围需要澄清

#### 3.2.1 TASK-SHARE-SHORT-001：公共 AI 端点设计

- **待确认**：
  - 是新建 `internal/assistant/public_handler.go` 还是在现有 handler 中增加 public 模式？
  - 公共 AI 是否复用 `assistant.Service.Chat` 还是单独 `PublicChat` 方法？
- **建议**：在任务执行前由后端负责人确认架构方向，避免大范围重构。

#### 3.2.2 TASK-SHARE-SHORT-004：去重实现方式（已确认）

- **决策**：**Redis TTL key 为主，DB 查询兜底**。
- **理由**：
  - `SET NX EX` 提供原子性，天然解决多实例并发重复写入问题。
  - O(1) 内存操作性能远高于 DB 查询，适合事件流场景。
  - 项目已使用 Redis（限流、session），无需引入新基础设施。
- **关键实现**：
  - key：`dedup:link_open:{link_id}:{visitor_id}`（TTL 30min）、`dedup:page_view:{link_id}:{visitor_id}:{page_number}`（TTL 5min）。
  - 仅在事件成功写入 DB 后再标记 Redis，防止 Redis 成功但 DB 失败的“假去重”。
  - Redis 故障时自动降级为 DB 查询，保证可用性。
- **待确认**：去重是否影响 `links.access_count` 的语义？（建议：去重命中时不递增 `access_count`。）

#### 3.2.3 TASK-SHARE-MID-001：关键词来源

- **待确认**：
  - 关键词匹配基于 `documents.title`、`pages.title`、还是 OCR 文本？
  - 当前数据库是否有 `pages.title` 字段？
- **建议**：先基于 `documents.title` + page 序号对应文档内标题（若存在），否则降级到 `page_number` 不做关键词匹配。避免引入 OCR 依赖。

### 3.3 ❌ 未覆盖的次要缺口（建议补充或明确延后）

| 缺口 | 建议处理方式 | 优先级 |
|---|---|---|
| 自动生成 follow-up 邮件草稿 | 新增 `TASK-SHARE-MID-006`（frontend/backend，M） | P2 |
| NDA 文本版本化存储 | 合并到 `TASK-SHARE-SHORT-003` 或新增小任务 | P2 |
| 过期/撤销链接后台清理 | 新增 `TASK-SHARE-MID-007`（backend，S） | P2 |
| `analytics_jobs` 物化任务 | 明确延后到长期，或在 LONG-003 中说明 | P3 |
| 通知 bell UI 未读数 | 可在 TASK-SHARE-MID-003 中扩展，或单独任务 | P2 |

---

## 4. 依赖与可行性评估

### 4.1 内部依赖是否合理

| 依赖路径 | 评估 | 说明 |
|---|---|---|
| SHORT-004 → MID-001 | ✅ 合理 | 去重是基础，Key Page 统计依赖准确的事件数 |
| SHORT-004 → MID-002 | ✅ 合理 | 扩展事件体系需要先建立去重/session 基础 |
| SHORT-002 → MID-003 | ✅ 合理 | 规则引擎依赖正确的通知收件人与开关 |
| SHORT-001 → LONG-002 | ✅ 合理 | AI 意图分析需要先有公共 AI 会话数据 |
| MID-001 → LONG-001 | ✅ 合理 | 时间衰减基于正确的 Key Page 定义 |
| MID-002 → LONG-003 | ✅ 合理 | 实时推送需要完整事件体系 |
| MID-003 → LONG-004 | ✅ 合理 | CRM 同步需要规则引擎产生的丰富信号 |
| LONG-001 + LONG-002 → LONG-005 | ✅ 合理 | 预测模型需要高质量特征 |

### 4.2 与外部 v2.1.3 任务的依赖

| 本计划任务 | 外部依赖 | 关系 |
|---|---|---|
| SHORT-001 ~ SHORT-003 | TASK-BACKEND-011 | 应后置，避免 migration/code 冲突 |
| MID-001 | TASK-FRONTEND-010 | 应协调关键词策略 |
| 所有涉及 viewer/links/dashboard 改动的任务 | TASK-TEST-003 | E2E 应后补或预留桩 |

### 4.3 工作量评估

| 阶段 | 预估总工时 | 风险 |
|---|---|---|
| 短期 | 2-3 人周 | 低，但需等待 BACKEND-011 |
| 中期 | 6-9 人周 | 中，涉及前后端协调与 schema 变更 |
| 长期 | 12-18 人周 | 高，AI/ML/实时化需要额外基础设施 |

---

## 5. 执行前必须确认的事项

### 5.1 架构决策

1. **公共 AI 端点**：新建 handler 还是改造现有 handler？
2. **去重存储**：Redis 为主 + DB 兜底（已确认）。
3. **Key Page 关键词来源**：文档标题 / 页面标题 / OCR？
4. **实时推送协议**：SSE 还是 WebSocket？
5. **AI 意图分析**：外部 LLM 还是本地轻量模型？

### 5.2 与 v2.1.3 的合并策略

1. **必须等 TASK-BACKEND-011 合并后再启动 SHORT-001 ~ SHORT-003**。
2. **TASK-FRONTEND-010 与 TASK-SHARE-MID-001 必须共享关键词配置**。
3. **TASK-TEST-003 的 E2E 用例需要随 share 任务更新**。

### 5.3 数据与隐私

1. 去重与 Key Page 修正是否回溯历史数据？
2. 动态水印中的 IP 哈希算法是否可逆？
3. AI 意图分析是否涉及将用户问题发送给第三方 LLM？

---

## 6. 推荐调整

### 6.1 新增 3 个补充任务（可选）

若希望任务计划更完整，建议补充：

| 新增任务 ID | 标题 | 阶段 | 优先级 | 依赖 | 说明 |
|---|---|---|---|---|---|
| TASK-SHARE-MID-006 | 自动生成 follow-up 邮件草稿 | 中期 | P2 | MID-003 | 基于信号自动生成邮件内容 |
| TASK-SHARE-MID-007 | 过期/撤销链接后台清理 | 中期 | P2 | - | cron 任务清理过期数据 |
| TASK-SHARE-MID-008 | 通知 bell UI 未读数 | 中期 | P2 | MID-003 | 顶部通知图标与未读管理 |

### 6.2 任务合并建议

- `TASK-SHARE-SHORT-003`（安全审计事件）与 `TASK-SHARE-MID-003`（通知规则引擎）可共享异常模式检测逻辑，建议在 MID-003 中复用 SHORT-003 的审计事件。
- `TASK-SHARE-MID-004`（动态水印）与 `TASK-SHARE-MID-005`（签名 URL）都是安全增强，可由同一开发者连续执行。

---

## 7. 最终建议

### 7.1 可以开始执行 ✅

任务计划整体质量较高，覆盖全面，依赖清晰，可直接进入开发。

### 7.2 启动顺序

```text
Step 1: 等待/完成 TASK-BACKEND-011（v2.1.3 后端稳定）
Step 2: 启动 TASK-SHARE-SHORT-002（通知收件人，影响面小，价值高）
Step 3: 并行启动 TASK-SHARE-SHORT-003（安全审计）与 TASK-SHARE-SHORT-004（去重）
Step 4: 启动 TASK-SHARE-SHORT-001（公共 AI，需与前端协调）
Step 5: 进入中期任务前，先确认 TASK-FRONTEND-010 与 TASK-SHARE-MID-001 的关键词策略
```

### 7.3 风险监控

- **代码冲突**：密切关注 TASK-BACKEND-011 的 migration 编号与 integration 改动。
- **数据回溯**：去重与 Key Page 修正会改变历史评分，需提前与产品/业务确认策略。
- **性能**：实时推送与扩展事件体系会增加负载，建议在 LONG-003 前做容量评估。

---

## 8. Review Checklist

- [x] 缺口报告中的每个高优先级缺口都有对应任务
- [x] 任务文件符合 AGENT-TASK-template-v2 格式
- [x] 任务依赖关系合理且无循环依赖
- [x] 已识别与现有 v2.1.3 任务的冲突与重叠
- [x] 已指出未覆盖的次要缺口
- [x] 已给出明确的启动顺序与风险建议
- [x] 结论：可开始执行

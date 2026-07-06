# DealSignal 文档分享缺口修复：架构决策推荐

**文档版本**：v1.0.0  
**日期**：2026-07-05  
**适用任务**：`TASK-SHARE-SHORT-001` / `TASK-SHARE-SHORT-004` / `TASK-SHARE-MID-001` / `TASK-SHARE-LONG-003` / `TASK-SHARE-LONG-002`

---

## 1. 公共 AI 端点：推荐新建独立 Public AI Handler

### 1.1 推荐方案

**新建独立的公共 AI 端点**，而非改造现有 `/assistant/chat`：

```
POST /api/v1/public/assistant/chat
Headers: X-Link-Session: <hmac-session-token>
Body:    { "message": "...", "session_id": "uuid-or-null" }
```

后端新增：
- `apps/api/internal/assistant/public_handler.go` — 公共端点 handler
- `apps/api/internal/assistant/service.go#PublicChat(ctx, session, req)` — 公共会话方法

### 1.2 推荐理由

| 维度 | 独立 Public Handler | 改造现有 /assistant/chat |
|---|---|---|
| **认证边界** | 清晰：只认 `X-Link-Session` | 混乱：需同时支持 workspace auth 和 link session |
| **权限范围** | 天然限制在当前 link 的 documents | 需额外参数和校验防止越权 |
| **审计安全** | 独立日志，便于追踪公开访问者行为 | 与内部用户会话混同，审计困难 |
| **会话隔离** | 按 `(link_id, visitor_id)` 隔离，数据结构清晰 | 需兼容 `user_id` 为主的内部会话模型 |
| **代码维护** | 职责单一，后期扩展公共 AI 功能更灵活 | 条件分支增多，易引入 bug |

### 1.3 实现要点

1. **复用 RAG 核心逻辑**：调用现有 `search.Service.Search` 和 `evidence.Formatter.BuildContext`，但限制 `document_ids` 为当前 link 的 documents。
2. **会话模型扩展**：
   - `assistant_sessions` 新增/使用 `link_id` + `visitor_id`。
   - 当 `session_id` 为空时，按 `(link_id, visitor_id)` 查找或创建会话。
3. **安全校验链**：
   - 验证 `X-Link-Session` 签名与过期时间。
   - 校验 `links.ai_copilot_enabled = true`。
   - 校验 link 未过期、未撤销、未超限。
   - 搜索范围限制在 `link_documents.document_id` 列表。
4. **数据隔离**：公共会话的 `assistant_messages` 不得被 workspace 内部用户看到；反之亦然。

### 1.4 风险与缓解

| 风险 | 缓解措施 |
|---|---|
| 公共端点被刷 | 复用现有 Redis 限流（按 IP + token） |
| LLM 成本上升 | 限制上下文长度（20 条历史）、证据数量（5 条） |
| 越权访问其他文档 | 强制 `document_ids` 过滤，search service 增加 `FilterByDocumentIDs` |

---

## 2. 去重存储：推荐 Redis 为主、DB 兜底

### 2.1 推荐方案

**使用 Redis TTL key 作为主要去重存储，PostgreSQL 查询作为故障兜底**：

```
# link_opened 30min 去重
SET dedup:link_open:{link_id}:{visitor_id} {timestamp} NX EX 1800

# page_viewed 5min 去重
SET dedup:page_view:{link_id}:{visitor_id}:{page_number} {timestamp} NX EX 300
```

- **命中**：key 存在 → 跳过事件写入。
- **未命中**：key 不存在 → 允许写入，然后设置 key。
- **Redis 故障**：降级为 DB 查询 `access_logs` / `page_views` 最近事件。

### 2.2 推荐理由

| 维度 | Redis 去重 | DB 去重 |
|---|---|---|
| **性能** | 极高，O(1) 内存操作 | 依赖索引与磁盘 I/O |
| **并发安全** | `SET NX EX` 原子操作，天然防并发重复 | 应用层先查后写存在竞态窗口 |
| **扩展性** | 轻松支撑高并发事件流 | 高并发下可能成瓶颈 |
| **实现复杂度** | 中，需封装 Redis key 与兜底逻辑 | 低，但并发处理复杂 |
| **可靠性** | 中，需 DB 兜底防 Redis 故障 | 高，不依赖额外组件 |
| **调试审计** | 较弱，TTL 过期后无记录 | 强，历史事件可追溯 |

**综合判断**：Redis + DB 兜底结合了两者的优点，是当前阶段最佳方案。

### 2.3 需要存储/判断的内容

#### 2.3.1 `link_opened` 去重

| 字段 | 作用 |
|---|---|
| `link_id` | 区分不同分享链接 |
| `visitor_id` | 区分不同访问者 |
| `event_type = 'link_opened'` | 只匹配打开事件 |
| `created_at` | 首次事件发生时间戳（value） |

**Redis key**：`dedup:link_open:{link_id}:{visitor_id}`  
**TTL**：30 分钟（1800 秒）

#### 2.3.2 `page_viewed` 去重

| 字段 | 作用 |
|---|---|
| `link_id` | 区分不同分享链接 |
| `visitor_id` | 区分不同访问者 |
| `page_number` | 区分不同页面 |
| `created_at` | 首次事件发生时间戳（value） |

**Redis key**：`dedup:page_view:{link_id}:{visitor_id}:{page_number}`  
**TTL**：5 分钟（300 秒）

### 2.4 实现要点

1. **接口抽象**：定义 `DedupChecker` 接口，便于测试与切换实现。
2. **主实现 `RedisDedupChecker`**：
   - 使用 `SET key value NX EX ttl` 原子判断 + 设置。
   - key 不存在时返回 `false`（允许写入），并设置 TTL。
   - key 存在时返回 `true`（重复，跳过）。
3. **兜底实现 `DBDedupChecker`**：
   - 查询 `access_logs` / `page_views` 最近窗口内的事件。
   - 与 Redis 行为保持一致。
4. **组合实现 `FailoverDedupChecker`**：
   - Redis 可用时走 Redis。
   - Redis 故障时自动降级 DB，并记录 warning 日志。
5. **事件写入后标记**：
   - 仅在事件成功写入 DB 后，才调用 `MarkOpen` / `MarkPageView` 设置 Redis key。
   - 避免 Redis 写入成功但 DB 写入失败导致的“假去重”。
6. **配置化**：
   - `LINK_OPEN_DEDUP_WINDOW_MINUTES`（默认 30）
   - `PAGE_VIEW_DEDUP_WINDOW_MINUTES`（默认 5）
   - `DEDUP_REDIS_ENABLED`（默认 true）

### 2.5 风险与缓解

| 风险 | 缓解措施 |
|---|---|
| Redis 故障导致去重失效 | DB 兜底查询；兜底时允许性能下降但保证正确性 |
| Redis 与 DB 短暂不一致 | 事件写入成功后才标记 Redis；Redis TTL 短，不一致窗口有限 |
| 高频刷新仍产生大量 Redis 操作 | O(1) 操作成本低；必要时可批量或 pipeline |
| 多实例并发写入重复事件 | `SET NX EX` 原子操作保证唯一性 |
| Redis key 无限增长 | 必须设置 TTL；定期监控 key 数量 |
| 丢失首次事件时间戳 | Redis value 存储 ISO 8601 时间戳，便于调试 |

---

## 3. Key Page 关键词来源：推荐 document/page 标题 + 元数据，业务作用是识别高意向页面

### 3.1 推荐方案

**Phase 1**：使用 **document title + page 元数据（标题/章节）** 进行关键词匹配。  
**Phase 2**：当需要更高精度时，引入 **OCR 文本片段** 或 **chunk 文本** 作为补充。

### 3.2 业务作用

Key Page 识别的核心商业价值：

| 页面类型 | 关键词示例 | 商业意图 |
|---|---|---|
| **财务页** | financial, revenue, unit economics, run rate, ARR, MRR | 投资/采购决策进入深度评估 |
| **团队页** | team, founders, advisors, leadership | 关注执行团队背景 |
| **价格页** | pricing, plan, cost, ROI, budget | 购买意愿强烈，进入比价阶段 |
| **安全/合规页** | security, compliance, privacy, SOC2, GDPR | 企业采购的安全审查 |
| **产品/方案页** | product, solution, features, architecture | 了解产品能力 |

这些页面被查看的时间、频次、深度，直接反映接收方（投资人、客户、LP）的**决策阶段与兴趣焦点**。Heat Score 中 Key Page Views 权重高达 25%，因此识别准确性直接影响意图判断质量。

### 3.3 当前代码的偏差

当前后端把“停留 ≥3 秒”当作 key page view，导致：
- 用户在目录页、附录页停留 3 秒也被误算为高意图。
- 财务页只看了 1 秒可能被漏掉（如果按停留时长）。

### 3.4 推荐关键词来源优先级

| 来源 | 优先级 | 原因 |
|---|---|---|
| `pages.title` / `pages.metadata` | **P0** | 已解析的页面标题最准确、成本低 |
| `documents.title` + page_number | **P1** | 若页面无标题，可用文档标题推断 |
| `chunks.text`（前 500 字符） | **P2** | 精度高但查询成本高 |
| OCR 全文 | **P3** | 成本最高，留到后期 |

### 3.5 实现要点

1. 确认 `pages` 表是否有 `title` / `text` 字段（当前 migration 003/004 中有 `chunks` 表，但不确定 `pages` 表结构）。
2. 新增 `heat.KeyPageConfig` 结构，per-circle 配置关键词集合。
3. 在 `GetLinkPageViewMetrics` 中 JOIN `pages` 或 `documents` 获取标题文本。
4. 匹配规则：
   - 不区分大小写。
   - 子串匹配或简单 TF-IDF/关键词命中数。
   - 相似度阈值默认 0.3（与 TASK-FRONTEND-010 对齐）。
5. 保留 `engaged_page_views` 指标（停留 ≥3s）用于展示用户参与度，但 Heat Score 使用 `key_page_views`。

### 3.6 与 TASK-FRONTEND-010 的协调

- `TASK-FRONTEND-010` 修复前端 `topKeyPages` 展示逻辑。
- `TASK-SHARE-MID-001` 修复后端 `key_page_views` 统计逻辑。
- **建议**：
  - 由后端统一维护 `heat_score_configs` 或 `key_page_keywords` 配置表。
  - 前端只读取配置并展示，不参与评分逻辑。
  - 避免两边算法不一致导致 Dashboard 展示与后端评分脱节。

---

## 4. 实时推送协议：推荐 SSE（Server-Sent Events）作为 MVP

### 4.1 推荐方案

**MVP 阶段采用 SSE**，而非 WebSocket：

```
GET /api/workspaces/:slug/events/stream
Authorization: Bearer <jwt>
Content-Type: text/event-stream
```

服务端推送 JSON 事件：

```text
event: signal
data: {"type":"hot_signal","link_id":"...","priority":"high"}

event: notification
data: {"id":"...","subject":"..."}
```

### 4.2 推荐理由

| 维度 | SSE | WebSocket |
|---|---|---|
| **协议基础** | HTTP/1.1，兼容性好 | 独立协议，部分代理/防火墙限制 |
| **认证** | 天然支持 cookie/header | 需在连接建立后额外认证 |
| **调试** | 简单，可用 curl/browser devtools | 较复杂，需专用工具 |
| **重连** | 浏览器内置 `EventSource` 自动重连 | 需手动实现心跳与重连 |
| **负载均衡** | 友好，长连接可由 LB 分发 | 需 sticky session 或共享 pub/sub |
| **适用场景** | 服务端单向推送 | 高频双向通信 |
| **本场景匹配度** | **高**：只需服务端推事件/信号/通知 | 过度设计 |

### 4.3 实现要点

1. **后端**：
   - 新增 `internal/realtime/handler.go` 处理 SSE 连接。
   - 使用 `http.Flusher` 持续推送。
   - 集成 Redis Pub/Sub 支持多实例广播。
   - 按 `workspace_id` 订阅频道。
2. **前端**：
   - 使用原生 `EventSource` 或封装 `useEventSource` hook。
   - 实现指数退避重连。
   - 收到事件后更新 Zustand store（signalStore / dashboardStore）。
3. **事件类型**：
   - `link_opened`
   - `page_viewed`
   - `signal_created`
   - `notification_created`
4. **心跳**：每 30 秒发送一次 `event: ping` 保持连接。

### 4.4 何时迁移到 WebSocket

若未来出现以下需求，再考虑 WebSocket：
- 需要前端向后端实时发送大量数据（如协作光标、实时批注）。
- 每秒推送频率 >10 条/用户。
- 需要二进制数据传输。

### 4.5 风险与缓解

| 风险 | 缓解措施 |
|---|---|
| 大量长连接占用资源 | 限制单用户连接数；使用连接池；必要时关闭空闲连接 |
| 多实例消息不同步 | 必须使用 Redis Pub/Sub 广播 |
| 代理缓冲导致延迟 | SSE 响应头设置 `Cache-Control: no-cache`, `X-Accel-Buffering: no` |

---

## 5. AI 意图分析：优先外部 LLM，本地轻量模型兜底

### 5.1 推荐方案

**优先使用外部 LLM**（OpenAI / OpenRouter / 兼容服务）进行意图分析，原因如下：

1. **准确率高**：分类、主题提取、情感/紧迫度判断都需要语义理解，LLM 明显优于规则。
2. **开发速度快**：无需训练数据，通过 prompt 工程即可快速上线。
3. **可解释性强**：可要求 LLM 返回分类依据（如“因为问题中包含 pricing 和 budget”）。
4. **基础设施已存在**：项目已有 `apps/api/internal/llm/client.go`，可直接复用。

### 5.2 分析任务拆分

| 子任务 | 方法 | 输出 |
|---|---|---|
| **主题分类** | LLM zero-shot 分类 | `pricing`, `security`, `team`, `implementation`, `competition`, `other` |
| **重复问题检测** | Embedding 语义相似度 | 相似度 ≥0.85 视为重复 |
| **情感分析** | LLM 判断 | `positive`, `neutral`, `negative` |
| **紧迫度评分** | LLM + 规则 | 1-5 分 |
| **购买意向评分** | 规则 + LLM | 0-100 分 |

### 5.3 实现架构

```text
用户提问 → assistant.Service.Chat
                ↓
         异步 enqueue AI intent analysis job
                ↓
         AI Intent Worker
                ↓
         1. PII 脱敏
         2. Embedding 计算/缓存
         3. 相似问题检索
         4. LLM 分类/情感/紧迫度
         5. 写入 assistant_message_intents
                ↓
         若 buying_intent 高 → 生成 signal / action
```

### 5.4 实现要点

1. **异步处理**：
   - 不能阻塞 AI 回答返回。
   - 使用后台 worker 或 Redis 队列。
2. **PII 脱敏**：
   - 在发送给 LLM 前，移除邮箱、公司名、人名等敏感信息。
   - 可用简单正则替换，如 `user-{hash}@example.test`。
3. **Embedding 缓存**：
   - 复用 `llm.Client.Embed`。
   - 将问题 embedding 存入 `assistant_message_intents` 或单独表，避免重复计算。
4. **Prompt 设计**：
   - 要求输出结构化 JSON。
   - 提供分类枚举和评分标准。
   - 要求返回可解释理由。
5. **成本控制**：
   - 只对用户消息分析，不分析 assistant 回答。
   - 对短问题/无意义问题可跳过 LLM，直接规则分类为 `other`。
6. **开关与降级**：
   - 配置 `AI_INTENT_ANALYSIS_ENABLED`。
   - LLM 不可用时降级为规则分类（关键词匹配）。

### 5.5 示例 Prompt

```text
You are an intent analyzer for a document sharing platform.
Classify the user's question about a shared document.

Categories: pricing, security, team, implementation, competition, other.
Output JSON:
{
  "topic": "pricing",
  "sentiment": "positive",
  "urgency": 4,
  "buying_intent": 75,
  "reason": "User asks about budget and ROI, indicating purchase evaluation."
}

Question: {sanitized_question}
```

### 5.6 风险与缓解

| 风险 | 缓解措施 |
|---|---|
| LLM 成本过高 | 限流、缓存 embedding、跳过短问题 |
| PII 泄露 | 脱敏处理，禁止发送真实邮箱/公司名 |
| 分类不稳定 | 使用低 temperature（0.1~0.3），定义清晰枚举 |
| LLM 不可用 | 配置开关，降级规则分类 |

---

## 6. 决策总览表

| 决策 | 推荐方案 | 关键理由 | 主要风险 |
|---|---|---|---|
| 公共 AI 端点 | 新建独立 Public Handler | 认证/权限/审计边界清晰 | 需复用 RAG 核心并限制 document 范围 |
| 去重存储 | Redis 为主 + DB 兜底 | 高性能、原子并发控制、可扩展 | 需封装 Redis key 与故障降级逻辑 |
| Key Page 来源 | document/page 标题 + 元数据 | 成本低、覆盖大部分场景 | 需与前端 TASK-FRONTEND-010 协调 |
| 实时推送 | SSE | HTTP 友好、认证简单、适合单向推送 | 代理缓冲、长连接资源 |
| AI 意图分析 | 外部 LLM | 准确率高、开发快、可解释 | 成本、PII、稳定性 |

---

## 7. 对任务文件的修改建议

基于以上决策，建议在执行任务前更新对应任务文件：

| 任务文件 | 更新内容 |
|---|---|
| `TASK-SHARE-SHORT-001.md` | 明确“新建 `internal/assistant/public_handler.go` + `PublicChat` 方法” |
| `TASK-SHARE-SHORT-004.md` | 明确“Redis TTL key 去重为主，DB 查询兜底，窗口 30min/5min” |
| `TASK-SHARE-MID-001.md` | 明确“关键词来源优先级：pages.title > documents.title > chunks.text > OCR”，并强调与 TASK-FRONTEND-010 协调 |
| `TASK-SHARE-LONG-003.md` | 明确“优先 SSE，多实例用 Redis Pub/Sub” |
| `TASK-SHARE-LONG-002.md` | 明确“优先外部 LLM，异步 + PII 脱敏 + embedding 缓存 + 规则降级” |

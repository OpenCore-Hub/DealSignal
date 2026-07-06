# DealSignal 文档分享业务逻辑：设计初衷 vs 代码实现 缺口分析报告

**报告日期**：2026-07-05  
**分析范围**：`apps/api`（Go 后端）、`apps/web`（React 前端）、`docs/`（PRD/TDD/API/产品设计/算法文档）  
**核心目标**：面向“查看分享链接的用户”，端至端盘点当前已实现的业务逻辑与统计逻辑，识别当初设计与现有代码之间的缺口、缺陷与差异。

---

## 1. 执行摘要

DealSignal 的文档分享链路当前已经跑通一个**最小可行闭环**：

> 上传 → 解析 → 生成分享链接 → 公开/受控访问 → 页面级事件采集 → 热度评分 → 跟进建议/信号 → 邮件/Slack 通知 → Dashboard/Insights 展示

然而，与 `docs/backup/` 中 PRD/TDD/算法/产品设计文档所描述的完整愿景相比，**当前实现仍停留在“规则驱动的统计仪表盘”阶段**，而非“实时事件驱动、AI 增强的意图洞察与即时行动系统”。

### 1.1 关键结论一览

| 维度 | 设计初衷 | 当前实现 | 差距等级 |
|---|---|---|---|
| **实时性** | WebSocket/SSE 实时推送查看事件与通知 | 同步 HTTP 写入 Postgres；30s 轮询 Worker；前端页面加载时拉取 | 🔴 高 |
| **事件丰富度** | 18 种事件（滚动、转发、打印、复制等） | 仅 3 种：`link_opened`、`page_viewed`、`download_attempted` | 🔴 高 |
| **AI Copilot（公开链接）** | 共享查看器内置证据型 AI 问答，问题内容作为意图信号 | `ai_copilot_enabled` 字段已存储，但公共 viewer 未接入公共 AI 端点，问题也未被追踪分析 | 🔴 高 |
| **意图模型** | ML/AI 意图分类、时间衰减、A/B 权重校准 | 纯规则加权求和（Heat Score），无 ML，无衰减 | 🟡 中 |
| **关键页识别** | 按关键词识别财务/团队/价格/安全页并加权 | 后端把“停留 ≥3s”当作 key page view；前端仅展示 topKeyPages 但不影响分数 | 🟡 中 |
| **通知触达** | 首次打开、重复关键页、多人转发、异常访问等可配置规则，10 分钟合并 | 仅 `hot_signal`（热度 hot + opens≥2）触发邮件；Slack 可选；无合并/摘要/异常检测 | 🟡 中 |
| **CRM 集成** | HubSpot/Salesforce：写 timeline、更新 deal stage、创建 task | 仅同步联系人与 deal 记录，未推送热度/信号/活动 timeline | 🟡 中 |
| **去重与会话** | 30min 会话去重、5min page_view 去重 | 无显式去重；visitor_id 由邮箱/UA 派生，无会话模型 | 🟡 中 |
| **安全与合规** | 动态水印（邮箱+时间+IP 哈希）、签名 URL | 水印字段/开关存在但动态水印未完整实现；签名 URL 未实现 | 🟡 中 |
| **品牌化/自定义域名** | 企业自定义域名、品牌化分享页 | 未实现 | 🟢 低（明确 out of MVP） |

---

## 2. 当初设计意图梳理（基于文档）

### 2.1 产品定位

> “把每一份关键文档变成可控、可追踪、可推进成交的交易信号系统。”  
> —— `docs/backup/PRD + 产品设计的完整文档草案.md`

5 条设计原则（`PRODUCT-DESIGN-v2.1.1.md` §2）：

1. **Signal-First**：首屏是“交易雷达”，不是文件列表。
2. **Actionable Transparency**：分析必须导向行动。
3. **Friction by Choice**：默认低摩擦，高敏感材料才启用强验证。
4. **Trustworthy AI**：每个 AI 回答必须附带证据引用。
5. **Circle-Specific Language**：创始人看“投资人热度”、基金看“LP engagement”、销售看“deal intent”。

### 2.2 文档分享（Smart Link）设计意图

| 能力 | 来源 | 关键要求 |
|---|---|---|
| 多格式上传 | PRD FR-01 | PDF、PPT、DOC、XLS、图片、视频 |
| 智能链接 | PRD FR-02 | 每个文档可生成多个独立链接 |
| 5 级权限 | PRODUCT-DESIGN §3.3.2 | Open → Email → Password → Whitelist → NDA |
| 过期/次数限制 | PRD FR-08 | `expires_at`、`max_access_count` |
| 动态水印 | PRD FR-09 / TDD C-05 | 水印 = 邮箱 + 访问时间 + IP 哈希 |
| 链接撤回 | API-SPEC | `POST /links/{id}/revoke` |
| 品牌化分享页 | PRD §4.2 In Scope #9 | 自定义域名、品牌页配置 |
| 多文档 Bundle | `docs/product/link-bundle-pipeline-redesign.md` | 选文档 → 安全设置 → 发布；同一 bundle 共享安全策略 |

### 2.3 查看器追踪（Viewer Tracking）设计意图

PRD §8.2.2 FR-05 与 EVT-01 ~ EVT-18 定义了 **18 种追踪事件**：

- `document_uploaded`、`link_created`、`link_opened`、`page_viewed`
- `duration_recorded`、`scroll_depth_recorded`、`download_attempted`
- `forward_signal`（转发/新访客）、`return_visit`、`key_page_viewed`
- `ai_question_asked`、`ai_answer_viewed`、`ai_evidence_clicked`
- `security_gate_passed`、`security_gate_failed`、`expired_link_accessed`
- `max_access_reached`、`sensitive_download`

去重规则（`HEAT-SCORE-ALGORITHM-v2.1.1.md` §3.1）：

- **30min 会话去重**：同一 visitor 30 分钟内多次打开只算一次 open。
- **5min page_view 去重**：同一页 5 分钟内重复查看不计新 view。

### 2.4 AI Copilot 设计意图

不是聊天机器人，而是**基于证据的文档问答助手**：

- Hybrid Search：exact + full-text + vector，RRF 融合。
- Evidence 引用：每个回答附带 quote / page / bbox。
- 自动跳页高亮：点击 evidence → Canvas 跳页 → overlay 高亮框 pulse。
- 内部/外部会话：`POST /assistant/chat` 支持 `document_id` 和 `session_id`。

### 2.5 Deal Radar / Dashboard 设计意图

登录首屏回答三个问题：

1. **谁今天表现出高意图？**
2. **什么材料在被评估？**
3. **我应该下一步做什么？**

核心能力：

- 信号流 + 热度地图 + 行动队列（布局 40% / 35% / 25%）。
- 0-100 热度评分，三圈分层：founder≥75、investor_ir≥70、sales≥72。
- 关键页识别：财务页/团队页/价格页/安全页。
- 跟进建议：例如“红杉合伙人重复查看财务页，建议发送 financial model”。
- 异常访问提醒：≥5 地区 1h 内访问、敏感下载、过期后访问。
- 行动队列：postpone / dismiss / act 三种处理。

### 2.6 通知与跟进设计意图

| 能力 | 来源 | 关键要求 |
|---|---|---|
| 邮件通知 | PRD FR-14 | 首次打开、重复关键页、多人转发、异常访问 |
| 事件合并 | PRD §8.2.7 | 默认 10 分钟合并；每日摘要开关；安全通知不可退订 |
| Slack 集成 | PRD FR-16 | 高意图事件推送到频道 |
| CRM 同步 | PRD FR-15 | HubSpot/Salesforce：写 timeline、更新 deal stage、创建 task |
| 跟进邮件草稿 | PRD FR-11 | 自动生成个性化 follow-up 文案 |
| 通知 bell UI | INTERACTION-SPEC §2.2 | 顶部导航通知图标与未读数 |

---

## 3. 当前代码实现盘点

### 3.1 文档分享实现

**核心包**：`apps/api/internal/link`

| 文件 | 职责 |
|---|---|
| `service.go` | 链接 CRUD、安全门校验、访问码生成与发送 |
| `handler.go` | 工作区路由 + 公开路由（Access / Events / NDA / 邮箱验证码） |
| `session.go` | HMAC 签名公开访客会话，15 分钟滑动过期 |

**数据库表**：

- `links`：核心链接表，含 `public_token`、`expires_at`、`max_access_count`、`access_count`、安全标志位、`ai_copilot_enabled`。
- `link_documents`：多文档 bundle 关联表（migration 029）。
- `link_contacts`：按联系人邮箱的验证码（migration 027）。
- `link_nda_agreements`：NDA 接受记录（migration 020）。
- `access_logs`：`link_opened`、`download_attempted`。
- `page_views`：`page_number`、`duration_seconds`、`scroll_depth`。

**已实现能力**：

- ✅ 创建/列表/获取/更新/删除（软删）分享链接。
- ✅ 多文档 bundle。
- ✅ 安全门：公开、邮箱验证码、密码（bcrypt）、邮箱/域名白名单、NDA。
- ✅ 过期时间与最大访问次数（请求时检查，无后台任务）。
- ✅ 下载开关与水印开关字段。
- ✅ AI Copilot 开关字段。
- ✅ 基于 HMAC 会话的公开访问复用。
- ✅ Redis 限流：验证码/密码尝试、邮件重发。

### 3.2 查看事件追踪实现

**事件采集**：

| 事件 | 触发位置 | 后端写入 |
|---|---|---|
| `link_opened` | 后端 `Handler.Access()` 成功访问后 | `analytics.RecordLinkOpened()` → `access_logs` + `links.access_count` 原子递增 |
| `page_viewed` | 前端 `useViewerDocument.ts` 页面切换/unmount | `analytics.RecordPageView()` → `page_views` |
| `download_attempted` | 前端 `CanvasViewer.tsx` 点击下载 | `analytics.RecordDownload()` → `access_logs` |

**前端实现**：

- 公开 viewer：`useViewerDocument.ts` 记录 `pageStartRef`，页面切换时计算 `Date.now() - pageStartRef`。
- 认证 viewer：2 秒停留后才触发 `page_viewed`。

**后端实现**：

- 同步 HTTP 写入 Postgres，无 WebSocket/SSE，无批量/队列。
- `RecordLinkOpened` 用 CTE 原子判断 `max_access_count`。
- `visitor_id` 由 lowercased 邮箱（或 UA）hash 生成 16 位 hex。

### 3.3 AI Copilot 实现

**后端**：

- `apps/api/internal/assistant/service.go`：会话管理、RAG 回答、证据引用。
- `apps/api/internal/search/service.go`：hybrid retrieval（vector + full-text + trigram + RRF）。
- `apps/api/internal/llm/client.go`：OpenAI-compatible embeddings/chat。

**前端**：

- `AIAssistant.tsx`（全局悬浮助手）
- `AIChat.tsx`（认证 viewer 文档页浮动聊天）
- `SidebarAIChat.tsx`（公共 viewer 侧边栏 AI）
- `aiStore.ts`：Zustand 状态管理。

**已实现能力**：

- ✅ 认证工作区内的 RAG 问答，带证据引用。
- ✅ Hybrid search 与 RRF 融合。
- ✅ `assistant_sessions` / `assistant_messages` 存储对话历史。
- ❌ **公共链接 viewer 未接入公共 AI 端点**。
- ❌ `assistant_sessions.link_id` / `document_id` 字段存在但**从未写入**。
- ❌ AI 问题内容未被分析为意图信号。

### 3.4 热度评分与信号实现

**核心文件**：

| 文件 | 职责 |
|---|---|
| `apps/api/internal/heat/score.go` | 0-100 热度评分算法 |
| `apps/api/internal/suggestions/service.go` | 基于热度生成跟进建议 |
| `apps/api/internal/signal/service.go` | 建议同步为信号 + 行动项 |
| `apps/api/internal/contact/service.go` | 联系人级聚合与评分 |
| `apps/api/internal/analytics/service.go` | 事件写入、Dashboard/Insights 聚合 |

**Heat Score 算法**：

7 个因子加权求和（founder 圈）：

| 因子 | 权重 |
|---|---|
| Opens | 3 |
| Revisits | 18 |
| Avg Duration Minutes | 12 |
| Key Page Views | 25 |
| Forward Signals（unique visitors） | 15 |
| Downloads | 8 |
| Bounce Penalty | -10 |

阈值：Hot ≥75、Warm ≥40、Cold <40。

**建议规则**（`suggestions/service.go`）：

| 条件 | 类型 | 优先级 | 行动 |
|---|---|---|---|
| `hot && opens >= 2` | `hot_signal` | high | `call` |
| `downloads > 0` | `follow_up` | low | `email` |
| `revisits > 0` | `follow_up` | low | `email` |
| `bounces > 0 && avgDuration < 0.5` | `risk_alert` | medium | `review` |

### 3.5 通知与 Deal Radar 实现

**通知**：

- 渠道：Email（SMTP/Resend）、Slack（incoming webhook）。
- 触发：仅 `hot_signal` 建议生成时调用 `Notifier.Enqueue(..., "email", ...)`。
- Worker：`notification/worker.go` 每 30 秒轮询 `notifications` 表。
- 收件人：当前实现发送到 `SMTP_USER`，代码注释 TODO“实际应查用户邮箱”。

**Deal Radar**：

- Dashboard：`DashboardPage.tsx` 展示 hot signals、pending actions、风险提醒、信号流、最近文档、行动项、热度图。
- Insights：`overview.tsx` / `suggestions.tsx` / `pages.tsx`。
- Signals API：`GET /api/workspaces/:slug/signals`。
- Activity Timeline：联系人详情页与链接详情页的活动时间线。

---

## 4. 缺口 / 缺陷 / 差异详细分析

### 4.1 事件追踪：从“18 种事件”到“3 种事件”

#### 4.1.1 已实现 vs 缺失事件

| 设计事件 | 是否实现 | 代码位置 / 说明 |
|---|---|---|
| `document_uploaded` | ⚠️ 部分 | 上传流程有，但未作为 analytics event 写入事件表 |
| `link_created` | ⚠️ 部分 | 创建 API 有，但未作为 analytics event 写入 |
| `link_opened` | ✅ 已实现 | `Handler.Access()` + `RecordLinkOpened` |
| `page_viewed` | ✅ 已实现 | `useViewerDocument` + `RecordPageView` |
| `duration_recorded` | ❌ 未实现 | 被合并进 `page_viewed.duration_seconds` |
| `scroll_depth_recorded` | ⚠️ 字段有 | `page_views.scroll_depth` 存储，但无聚合/使用 |
| `download_attempted` | ✅ 已实现 | `CanvasViewer` + `RecordDownload` |
| `forward_signal` | ❌ 未实现 | 设计定义：新访客来自转发。当前用 unique visitors 近似，无显式转发事件 |
| `return_visit` | ❌ 未实现 | 设计定义：老访客 30min 后回访。当前 `revisits` 由 heat 算法基于 visitor_id 统计 |
| `key_page_viewed` | ❌ 未实现 | 当前后端把“停留 ≥3s”当 key page view |
| `ai_question_asked` | ❌ 未实现 | AI 问题未追踪 |
| `ai_answer_viewed` | ❌ 未实现 | 证据点击未追踪 |
| `ai_evidence_clicked` | ❌ 未实现 | 高亮跳转未追踪 |
| `security_gate_passed` / `failed` | ❌ 未实现 | 安全门通过/失败未写入事件 |
| `expired_link_accessed` | ❌ 未实现 | 仅返回 410，未记录 |
| `max_access_reached` | ❌ 未实现 | 仅返回 403，未记录 |
| `sensitive_download` | ❌ 未实现 | 下载未区分敏感级别 |

#### 4.1.2 关键缺陷

1. **滚动深度废弃**：`scroll_depth` 字段存在且前端上报，但没有任何 SQL 聚合或业务逻辑使用它。
2. **缺少安全审计事件**：无法分析“谁在尝试暴力破解密码/验证码/过期链接”。
3. **缺少 AI 交互事件**：AI Copilot 的提问、回答、证据点击是极强的意图信号，但完全未进入事件流。

### 4.2 实时性缺口：设计是“实时”，实现是“准实时/拉取”

| 设计点 | 当前实现 | 影响 |
|---|---|---|
| WebSocket/SSE 推送事件与通知 | 无 WebSocket/SSE/EventSource |  sharer 无法即时感知高意图行为，Dashboard 需刷新才更新 |
| 10 分钟事件合并 | 无合并逻辑；每个事件独立处理 | hot_signal 可能重复生成（虽有 24h suggestion 去重） |
| 实时热度地图 | 热度评分按需 SQL 计算 | 大规模数据下查询延迟随事件量增长 |
| 通知即时触达 | Worker 30s 轮询 + SMTP 发送延迟 | 从“用户行为”到“通知到达”可能延迟数秒到数分钟 |

### 4.3 AI Copilot：公开链接viewer 处于“半吊子”状态

| 设计意图 | 当前实现 | 状态 |
|---|---|---|
| 公共 viewer 可启用 AI Copilot | `ai_copilot_enabled` 字段已存储并返回 | ✅ |
| 公共 viewer 根据 flag 显示/隐藏 AI 标签 | `RightSidebar.tsx` 始终显示 AI 标签，未读 flag | ❌ |
| 公共 viewer 调用公共 AI 端点 | 无公共 AI 端点；`SidebarAIChat` 调用的是认证 `/search` | ❌ |
| 公共 AI 会话按 link + visitor 隔离 | `assistant_sessions.link_id/document_id` 存在但从未写入 | ❌ |
| AI 问题内容作为意图信号 | 未分析 | ❌ |
| AI 证据点击追踪 | 未追踪 | ❌ |

**风险**：当前公共 viewer 的 AI 侧边栏对匿名用户展示，但实际上调用需要 workspace 认证的 API，会导致 401 或功能不可用，属于**前端与后端权限不一致的缺陷**。

### 4.4 热度评分算法：规则 vs ML；关键页识别偏差

#### 4.4.1 规则驱动的评分

- **设计**：Phase 1 规则评分 + Phase 2/3 引入时间衰减、A/B 权重校准、ML 校准。
- **实现**：仅 Phase 1 规则评分，无时间衰减、无 A/B、无 ML。

#### 4.4.2 Key Page Views 定义偏差（重要）

- **设计文档**：`HEAT-SCORE-ALGORITHM-v2.1.1.md` §5 定义关键页通过关键词规则识别（financials、team、pricing、security 等）。
- **代码实现**：`GetLinkPageViewMetrics` 中
  ```sql
  COUNT(*) FILTER (WHERE duration_seconds >= 3) AS key_page_views
  ```
  即**任意页面停留 ≥3 秒即视为 key page view**。
- **前端**：`apps/web/src/lib/heat/heatScore.ts` 用关键词匹配标题生成 `topKeyPages` 展示，但不参与分数计算。

**影响**：评分结果与“用户是否真正关注关键页”存在系统性偏差。

#### 4.4.3 去重规则缺失

- **设计**：30min 会话去重、5min page_view 去重。
- **实现**：无显式去重逻辑；`visitor_id` 由邮箱/UA hash 生成，同一用户快速刷新会重复计入 opens/page_views。
- **影响**：热度评分会被高频刷新/测试行为夸大。

#### 4.4.4 缺少会话模型

- 设计文档提到 `link_accesses` 表与 `visitor_id`、签名 URL。
- 当前无 `link_accesses` 表，也无会话实体；`access_logs` 和 `page_views` 是平铺事件表。

### 4.5 通知与消息通道：从“可配置规则引擎”到“单一路径硬编码”

| 设计意图 | 当前实现 | 差距 |
|---|---|---|
| 通知规则引擎（`notification_rules` 表） | 表未创建/未使用 | 无法配置“首次打开提醒”等规则 |
| 多事件触发通知（首次打开、重复关键页、转发、异常访问） | 仅 `hot_signal` 触发邮件 | 大量高价值场景丢失 |
| 10 分钟事件合并 + 每日摘要 | 无合并逻辑；每条通知独立发送 | 通知疲劳风险 |
| Slack 推送高意图事件 | Slack webhook 已实现，但仅 hot_signal 会触发 | 规则单一 |
| 通知 bell UI 与未读数 | UI bell 存在但 disabled（coming soon） | 无未读/已读状态管理 |
| CRM timeline / task / deal stage 更新 | 仅同步 contact/deal 记录 | 无法把热度/信号写回 CRM |
| 自动生成 follow-up 邮件草稿 | 未实现 | 行动项仍需用户手动撰写 |

**关键缺陷**：

- 通知收件人当前是 `SMTP_USER`（环境变量），而非 link creator 的真实邮箱，代码中有 TODO 注释未修复。
- `notification_settings.email_enabled` 存在但前端 `IntegrationStatus` 未暴露，用户无法关闭邮件通知。

### 4.6 Deal Radar / Dashboard：展示有余，实时与行动不足

| 设计意图 | 当前实现 | 差距 |
|---|---|---|
| 实时信号流 | 页面加载时拉取；无 WebSocket/SSE | 非实时 |
| 异常访问提醒（多地区/敏感下载/过期访问） | 未实现 | 安全风险与意图信号丢失 |
| 行动队列 postpone/dismiss/act | Action status 有 pending/done/snoozed/ignored | 基本可用，但缺少 postpone 时间选择 |
| 热度地图 | Insights 有最近链接热度 | 未实现“地图”式可视化 |
| 自动生成 follow-up 文案 | 未实现 | 行动项只有类型（call/email/review），无内容 |

### 4.7 安全与合规实现缺口

| 设计意图 | 当前实现 | 差距 |
|---|---|---|
| 动态水印 = 邮箱 + 访问时间 + IP 哈希 | `watermark_enabled` 字段存在，但公共 viewer 水印逻辑不完整 | 水印未真正覆盖 viewer canvas |
| 签名 URL（Cloudflare URL Signing） | 未实现 | 页面/下载 URL 可被转发/盗链 |
| 安全审计日志 | `access_logs` 只记录成功事件 | 密码/验证码失败、过期访问、越权访问未记录 |
| NDA 文本存储 | `link_nda_agreements` 只记录同意 checkbox | 实际 NDA 文本未版本化存储 |

### 4.8 数据模型与后台任务缺口

| 设计/文档 | 当前实现 | 差距 |
|---|---|---|
| `analytics_jobs` 表（`score`/`aggregate`/`report`） | 表未使用；所有聚合按需计算 | 无计划任务/物化视图 |
| 过期链接自动清理/归档 | 无后台 cron | 过期/撤销链接长期保留 |
| 物化聚合表 / rollup | 无 | Dashboard/Insights 查询随数据量增长变慢 |

---

## 5. 关键代码证据摘录

### 5.1 Key Page View 定义偏差

```sql
-- apps/api/internal/db/queries.sql:431-437
SELECT
    COALESCE(AVG(duration_seconds), 0) AS avg_duration_seconds,
    COUNT(*) AS total_page_views,
    COUNT(*) FILTER (WHERE duration_seconds >= 3) AS key_page_views,
    COUNT(DISTINCT visitor_id) AS unique_viewers
FROM page_views
WHERE link_id = $1;
```

### 5.2 通知仅 hot_signal 触发

```go
// apps/api/internal/suggestions/service.go:105-111
if best.Type == "hot_signal" {
    if err := s.notifier.Enqueue(ctx, workspaceID, linkCreator, "email",
        localizer.Subject(best.Type, lang),
        localizer.Body(best.Type, best, lang)); err != nil {
        slog.Warn("...")
    }
}
```

### 5.3 公共 viewer 未处理 AI flag

```tsx
// apps/web/src/components/viewer/RightSidebar.tsx
// 始终渲染 Documents / AI 两个 tab，未读取 aiCopilotEnabled
<Tabs defaultValue="documents">
  <TabsList>
    <TabsTrigger value="documents">Documents</TabsTrigger>
    <TabsTrigger value="ai">AI</TabsTrigger>
  </TabsList>
```

### 5.4 会话 link_id/document_id 未写入

```go
// apps/api/internal/assistant/service.go resolveSession
// 仅设置 workspace_id, user_id, title；未使用 link_id / document_id
```

### 5.5 通知收件人 TODO

```go
// apps/api/internal/notification/service.go
// TODO: In production this would lookup the user's email.
```

---

## 6. 风险与影响评估

| 风险项 | 影响 | 等级 |
|---|---|---|
| 公共 viewer AI 标签可用但 API 需认证 | 功能不可用或安全漏洞（若绕过） | 🔴 高 |
| 热度评分无去重，可被刷分 | 错误的高意图信号，误导销售行动 | 🔴 高 |
| Key Page Views 定义偏离设计 | 评分不反映真实关键页兴趣 | 🟡 中 |
| 通知收件人固定为 SMTP_USER | 真实 sharer 收不到通知 | 🟡 中 |
| 缺少安全审计事件 | 无法检测暴力破解/数据泄露尝试 | 🟡 中 |
| 无实时推送 | 错过即时跟进窗口 | 🟡 中 |
| 无 ML/时间衰减 | 评分无法随时间演进，长期准确性下降 | 🟢 低 |
| 缺少 CRM 信号同步 | 销售流程断裂，无法与现有工作流结合 | 🟡 中 |

---

## 7. 修复建议与路线图

### 7.1 短期（1-2 周）：修复功能可用性与安全缺陷

1. **修复公共 viewer AI 权限**
   - 在 `RightSidebar.tsx` 读取 `aiCopilotEnabled` 并条件渲染 AI tab。
   - 新增公共 AI 端点（或验证会话的 `/public/assistant/chat`），强制传入 `public_token` 与 `visitor_id`。
   - 写入 `assistant_sessions.link_id` / `document_id`。

2. **修复通知收件人**
   - 根据 `link.created_by` 查询 `users.email` 发送通知。
   - 前端暴露 `email_enabled` 开关。

3. **补齐安全审计事件**
   - 记录 `security_gate_failed`、`expired_link_accessed`、`max_access_reached`。
   - 增加异常访问告警（如 1h 内多地区、多失败尝试）。

4. **实现基础去重**
   - 在 `RecordLinkOpened` / `RecordPageView` 中按 `visitor_id` + 时间窗口去重。
   - 或引入 `link_accesses` 会话表。

### 7.2 中期（2-6 周）：补齐设计与实现的差异

1. **修正 Key Page Views 定义**
   - 后端引入 page title / OCR 文本关键词匹配（financials/team/pricing/security）。
   - 或至少提供 per-circle 的关键页配置表。

2. **扩展事件体系**
   - 补齐 `forward_signal`、`return_visit`、`scroll_depth_recorded`、`ai_question_asked`、`ai_evidence_clicked`。
   - 前端统一事件上报 SDK。

3. **通知规则引擎 MVP**
   - 创建 `notification_rules` 表，支持“首次打开、重复关键页、多人转发、异常访问”规则。
   - 实现 10 分钟事件合并与每日摘要。

4. **动态水印与签名 URL**
   - 在 Canvas 渲染时叠加动态水印。
   - 对页面/下载 URL 实现签名（HMAC 或 Cloudflare URL Signing）。

### 7.3 长期（6-12 周）：向“AI 增强意图洞察”演进

1. **引入时间衰减与 A/B 权重校准**
   - Heat Score 加入时间衰减函数。
   - 通过实验数据校准各 circle 权重。

2. **AI 问题意图分析**
   - 对 `assistant_messages.content` 做主题分类、重复问题检测、情感/紧迫度分析。
   - 将 AI 交互转化为信号（如“频繁询问 pricing → 高购买意向”）。

3. **预测性 lead scoring**
   - 基于历史转化数据训练轻量模型，输出成交概率。

4. **实时化**
   - 引入 WebSocket/SSE 推送事件、通知、Dashboard 更新。
   - 或采用事件流（Kafka/Redis Stream）+ 物化视图。

5. **CRM 深度集成**
   - HubSpot/Salesforce 写入 timeline activity、更新 deal stage、创建 task。
   - 同步热度评分与信号。

---

## 8. 附录：关键文件索引

### 8.1 后端核心文件

| 路径 | 说明 |
|---|---|
| `apps/api/internal/link/service.go` | 链接业务逻辑 |
| `apps/api/internal/link/handler.go` | 链接 HTTP 处理器 |
| `apps/api/internal/link/session.go` | 公开会话管理 |
| `apps/api/internal/analytics/service.go` | 事件记录与聚合 |
| `apps/api/internal/analytics/handler.go` | Dashboard/Insights API |
| `apps/api/internal/heat/score.go` | 热度评分算法 |
| `apps/api/internal/suggestions/service.go` | 跟进建议生成 |
| `apps/api/internal/signal/service.go` | 信号与行动项 |
| `apps/api/internal/notification/service.go` | 通知入队与发送 |
| `apps/api/internal/notification/worker.go` | 通知轮询 Worker |
| `apps/api/internal/assistant/service.go` | AI 助手会话与 RAG |
| `apps/api/internal/search/service.go` | Hybrid search |
| `apps/api/internal/db/queries.sql` | 所有 sqlc 查询 |
| `apps/api/internal/db/migrations/*` | 数据库迁移 |

### 8.2 前端核心文件

| 路径 | 说明 |
|---|---|
| `apps/web/src/components/viewer/PublicViewerPage.tsx` | 公共查看器页面 |
| `apps/web/src/components/viewer/useViewerDocument.ts` | 页面 dwell time 与事件上报 |
| `apps/web/src/components/viewer/CanvasViewer.tsx` | Canvas 渲染与下载 |
| `apps/web/src/components/viewer/RightSidebar.tsx` | 公共 viewer 侧边栏 |
| `apps/web/src/components/viewer/SidebarAIChat.tsx` | 公共 viewer AI 聊天 |
| `apps/web/src/components/dashboard/DashboardPage.tsx` | Deal Radar 仪表盘 |
| `apps/web/src/stores/signalStore.ts` | 信号状态管理 |
| `apps/web/src/lib/heat/heatScore.ts` | 前端热度评分镜像 |

### 8.3 设计文档

| 路径 | 说明 |
|---|---|
| `docs/backup/PRD-v2.1.0.md` | 产品需求文档 |
| `docs/backup/TDD-v2.1.0.md` | 技术设计文档 |
| `docs/backup/API-SPEC-v2.1.0.md` | API 规范 |
| `docs/backup/PRODUCT-DESIGN-v2.1.1.md` | 产品设计 |
| `docs/backup/HEAT-SCORE-ALGORITHM-v2.1.1.md` | 热度评分算法 |
| `docs/backup/INTERACTION-SPEC-v2.1.1.md` | 交互规范 |
| `docs/backup/ARCHITECTURE-v2.1.0.md` | 架构文档 |
| `docs/product/link-bundle-pipeline-redesign.md` | Link Bundle 重设计 |
| `docs/reviews/PRD-TDD-API-PLAN-TASK-consistency-review-v2.1.2.md` | 一致性评审 |
| `docs/reviews/frontend-implementation-doc-alignment-v2.1.2.md` | 前端实现对齐评审 |

---

## 9. 总结

当前 DealSignal 的文档分享业务逻辑已经实现了**一个可用的 MVP 闭环**：受控链接、页面事件采集、热度评分、建议/信号、邮件通知、仪表盘展示。然而，与原始设计愿景相比，存在三类主要差距：

1. **功能可用性缺陷**：公共 viewer 的 AI Copilot 未真正可用、通知收件人错误、缺少安全审计事件。
2. **设计语义偏差**：Key Page Views 被简化为“停留 ≥3s”、事件体系从 18 种缩水为 3 种、去重与会话模型缺失。
3. **架构演进滞后**：实时推送、事件流、通知规则引擎、ML 意图分析、CRM 深度同步均未实现。

建议按“短期修复可用性 → 中期补齐设计差异 → 长期引入 AI/实时化”的三阶段路线推进。

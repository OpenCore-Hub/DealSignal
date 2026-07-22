# 访客沟通整合与 Ask Docs 知识库设计方案

> 版本：v1.3  
> 日期：2026-07-22  
> 状态：**已落地（V1）** — 发布门槛 + Phase 0–2 + V1.5 通道建议已实现；Future 项仍见 §八  
> 范围：链接「沟通」能力整合 · 数据室知识库 · Ask Docs 作用域红线 · 审计可信 · 防绕过访问控制

---

## 一、背景与目标

### 1.1 背景

分享链接高级配置中并存两个开关：

| 现状开关 | 字段 | 访客侧表现 |
|----------|------|------------|
| AI 助手 | `ai_copilot_enabled` | 基于授权文档的即时 RAG 问答 + 证据定位 |
| 启用问答对话 | `qa_enabled` | 向所有者异步留言，Management 回复 |

两端能力**后端仍是两套系统**，访客侧已有半统一面板 `UnifiedQAPanel`（「问 AI / 问所有者」），但：

1. **所有者配置心智分裂**：两个平级开关文案几乎无差异，无法表达「即时检索 vs 人工沟通」。
2. **Ask Docs 作用域实现未与 Access 完全同源**（数据室 folder allowlist），存在安全对齐债。
3. **RAG 就绪态黑盒**：上传后自动异步 ingestion/embedding，所有者无法显式「创建/重建知识库」，也无法承诺审计。
4. **文件请求易被误解**：文件请求是所有者发起的收集通道；访客缺资料应走「问发起方」留言，而非访客发起文件请求。

### 1.2 目标

1. 将 AI 助手与访客问答整合为同一产品能力 **「沟通」**，下设两通道。
2. Ask Docs **仅作用于当前链接授权文档集合**；隐藏 RAG 实现，**作用域安全是红线**。
3. 数据室文档页新增 **创建知识库 / 重建知识库**（显式勾选范围），管理 Ask Docs 语料就绪态。
4. 访客 Ask Docs **可审计**，增强所有者信任。
5. 后端两字段可继续并存，避免强制 schema 大迁移。
6. **防止绕过访问控制**直接调用 Ask Docs / Ask Host API（与看文档同构门禁 + 身份重算 + 限额 + 证据截断）。

### 1.3 非目标（本方案不做）

- 将文件请求并入访客「沟通」入口。
- 为每条链接单独物理建一份向量索引。
- 单输入框全自动意图路由且无显式通道（可作为后续增强，非 V1）。
- 用知识库扩大链接可见范围。
- 为**单文档分享链接**做显式知识库产品面（继续自动 embed；**V2.0 考虑废弃单文档链接**）。
- V1 上独立 append-only 审计表（先复用会话投影，预留升级）。

---

## 二、已拍板决策

### 2.1 产品开放决策（初稿）

| # | 议题 | 决定 |
|---|------|------|
| 1 | 对外名称 | 中文 **沟通**；英文 Tab **Ask** / 配置主标题 **Visitor Ask**；子通道 **问文档 / 问发起方**（Ask Docs / Ask Host） |
| 2 | 双开默认通道 | 默认 **问文档（Ask Docs）** |
| 3 | 高级启用计数 | 「沟通」算 **1**；摘要写 `沟通 · 问文档` / `沟通 · 问发起方` / `沟通 · 问文档 + 问发起方` |
| 4 | 文件请求边界 | **文件请求 = 所有者发起收集**；访客缺资料 → **问发起方留言**；沟通引导不链到文件请求 |
| 5 | 存量沟通开关 | `ai_copilot_enabled OR qa_enabled` → 主开关 ON；子开关映射原字段 |
| 6 | Ask Docs 作用域 | 仅当前链接授权文档；`KB ∩ LinkAuthorizedDocuments` |
| 7 | 知识库 | **仅数据室**；文档页「创建知识库 / 重建知识库」 |
| 8 | 审计 | 访客 Ask Docs **必须可审计**，作为信任一等能力 |

### 2.2 Grilling 共识（Q1–Q19，2026-07-22）

| # | 议题 | 决定 |
|---|------|------|
| Q1 | KB 产品边界 | 显式 KB **仅数据室**；单文档链接不设创建/重建；V2.0 考虑废弃单文档链接 |
| Q2 | Embedding 触发 | 上传 **只**解析页/chunks（预览）；**仅**「创建/重建知识库」写/刷 embedding |
| Q3 | 变更后过期 | 室文档增删改 → KB `stale`；Ask Docs **仍可用**（软过期）；提示建议重建 |
| Q4 | 开「问文档」门控 | 数据室链接：KB ≠ `ready`/`stale`（即 `none`/`failed`/`building`）时 **保存拦截**「问文档」，引导建库 |
| Q5 | 存量 Ask Docs | 上线迁移：无 ready/stale KB 的室，链接 `ai_copilot_enabled` **一律置 false**；建库后重开 |
| Q6 | 审计存储 | **先复用** `assistant_sessions` / `assistant_messages`（补授权快照等）；预留独立审计表 |
| Q7 | 审计可见性 | **室成员 ∪ 工作区 admin**；默认可见完整问答原文 |
| Q8 | Signal | 审计 + Signal **双写**；UI 叙事分离（洞察 ≠ 审计） |
| Q9 | 重建中可用性 | 重建用 **旧索引**继续服务；新世代 ready 后 **原子切换**；失败回滚 |
| Q10 | KB 语料范围 | **显式勾选**文件夹/文档（非整室默认纳入） |
| Q11 | 勾选集演化 | 勾选**文件夹路径跟随**：路径下新增 → `stale`，重建才 embed；单文件勾选只跟 document_id |
| Q12 | 创建默认勾选 | 向导 **默认全不选**（安全默认） |
| Q13 | 授权 ⊄ KB | 链接保存时 **警告**缺口；不硬拦；访客不暴露「知识库」内部词 |
| Q14 | KB 写权限 | 仅 room **owner / admin** 可创建·重建·改勾选集 |
| Q15 | 无证据 | **必须拒答**（固定文案）；审计记 `no_evidence`；可引导问发起方 |
| Q16 | 检索默认范围 | 默认 **整链授权 ∩ KB**；UI 注明「基于本链接授权材料」 |
| Q17 | 审计入口 | **室级汇总 + 链接管理下钻**；V1 可先做链接侧 |
| Q18–19 | 留存 | 默认列表 **90 天**热数据；之后 **归档**（非删除）；归档仍可由 room/ws admin 检索 |

### 2.3 Grilling 共识 — 防绕过访问控制（Q20–Q25，2026-07-22）

| # | 议题 | 决定 |
|---|------|------|
| Q20 | 门禁同构 | Ask Docs / Ask Host **必须**与看文档走同一 `resolvePublicAccess`（会话 + security_version + 门禁；feature 关闭另 403） |
| Q21 | 身份重算 | 每次沟通用 session.email **重算 allow/block**；失败 → 403 并作废会话 |
| Q22 | 硬限额 | Ask Docs：**20 次/10 分钟、200 次/日**（per visitor+link）；Ask Host：**30 次/日**；超限 429 |
| Q23 | 证据截断 | 返回访客的 evidence quote **最长 320 字符**；完整阅读靠跳页 Viewer |
| Q24 | 路由绑 token | Ask Docs 改为 `/public/links/:publicToken/assistant/chat`（或等价）；`session.PublicToken` 必须匹配路径 token |
| Q25 | 安全事件 | 产品侧只记**高危**：block、scope_violation、超限等；普通 401 仅结构化日志 |

**现状缺口（实施必改）：** 现网 Host 已走 `verifyPublicAccess`；Ask Docs（`POST /assistant/chat`）仅验 session 签名 + `ResolvePublicLink` + `ai_copilot_enabled`，**未**做 security_version / 门禁同构 / allow-block 重算。

---

## 三、产品定位与 Jobs-to-be-done

### 3.1 能力分层

```text
数据室文档页（room admin）
  └─ 创建/重建知识库（显式勾选范围）   ← 语料就绪（室级）

链接 · 沟通（Visitor Ask）
  ├─ 问文档（Ask Docs）              ← 查询 = KB勾选就绪集 ∩ 本链接授权（安全红线）
  │     └─ 每次问答 → 可审计账本     ← 所有者信任红线（+ Signal 双写）
  └─ 问发起方（Ask Host）            ← 异步留言；缺资料走这里
```

### 3.2 Jobs-to-be-done

| 角色 | 任务 | 期望 |
|------|------|------|
| 访客 | 在授权文档里快速找事实、定位原文、看摘要 | 秒级、可溯源、不打断阅读 |
| 访客 | 材料缺失、商业判断、需承诺的问题 | 有人回应、可追踪状态 |
| 所有者 | 降低尽调往返成本，又不失控 | 一键开启「沟通」，细控 AI/人工边界 |
| 所有者 | 信任「AI 没越权」 | 可审计：谁问了什么、引用了哪些授权文档 |
| 所有者 | 控制何材料被 embed | 建库显式勾选；未勾选不得向量化 |
| 所有者 | 看到意图与阻塞点 | Ask Docs → 审计 + Signal；Ask Host → 待办收件箱 |

---

## 四、功能设计

### 4.1 所有者配置：访客沟通

**形态（Access「高级」）：**

```text
高级
└─ 沟通 / Visitor Ask  [主开关]
     ├─ 问文档（基于授权文档的即时问答与定位）  [子开关 → ai_copilot_enabled]
     └─ 问发起方（向发起方留言 / 缺资料反馈）  [子开关 → qa_enabled]
```

**规则：**

1. 主开关 OFF → 访客无「沟通」入口；两子开关禁用。
2. 主开关 ON → 至少开一个子能力；保存时校验。
3. 持久化：`ai_copilot_enabled` / `qa_enabled`；主开关 = OR。
4. 高级计数：沟通开启算 1，子开关不分别计数。
5. Tooltip / description 必须写清差异（替换现状模糊文案）。
6. **数据室链接：** 开启「问文档」时，室 KB 须为 `ready` 或 `stale`；否则 **保存拦截**并引导文档页建库（Q4）。
7. **授权 ⊄ KB 勾选集：** 允许保存，但显示 **警告**（哪些授权路径未纳入 KB）（Q13）。
8. 开「问文档」旁信任文案：访客文档问答可审计（含问题、回答、引用范围）。

**与文件请求：** 保持独立高级项；**不**在沟通空态引导访客去「提文件请求」。

**单文档链接（过渡）：** 无室级 KB 门控；继续上传自动 embed；不提供创建/重建按钮；V2.0 考虑废弃该类链接（Q1）。

### 4.2 访客：统一入口

沿用并产品化 `UnifiedQAPanel`，侧栏：

```text
文档 | 沟通 | 文件请求(可选，仅当所有者启用收集)
```

「沟通」仅在 Ask Docs ∪ Ask Host 任一开启时出现。

**默认通道：**

| 开启情况 | 默认 mode |
|----------|-----------|
| 仅 Docs | 问文档 |
| 仅 Host | 问发起方 |
| 两者都开 | 问文档 |
| 两者都开但 Ask Docs 运行时不可用 | 问发起方（拒答/降级场景） |

**输入区：** 双开时保留显式分段控件，文案「问文档 / 问发起方」，下方微文案说明即时 vs 异步。

**检索范围文案：** 「基于本链接授权材料」（整链授权 ∩ KB，Q16）。

**时间线：** 单时间线 + 来源徽章（AI / 发起方 / 我）；Host 未回复显示「待回复」；Docs 证据卡可跳页。

**无证据（Q15）：** 固定拒答文案（如「授权材料中未找到依据」）；若 Host 已开，引导切换「问发起方」。不生成无依据发挥性回答。

**智能建议路由（V1.5，非 V1 阻塞）：** 发送前对「缺少 / 能否提供」等意图建议切 Host，不强制自动投递。

### 4.3 问发起方（Ask Host）

- 后端继续 `link_visitor_questions`；访客 `questions/me`；所有者 Management 回复。
- 缺资料、需确认、商业条款 → Host。
- 与 Ask Docs 审计在链接管理侧可并列、按通道筛选。

### 4.4 问文档（Ask Docs）— 安全红线

**定义：** 检索与回答只能使用「当前链接 Access 暴露给访客的同一批 `document_id`」，且须落入 KB 勾选并已向量化的就绪集。

**禁止越权到：**

- 工作区其他文档
- 数据室未授权文件夹/文档
- 同房间其他链接的授权范围
- 「知识库已索引但本链接未授权」的文档
- 「链接已授权但未纳入 KB 勾选集」的文档（查询期自然排除；配置期警告）

**查询期硬公式：**

```text
检索集合 = KB勾选且已向量化(可检索) ∩ LinkAuthorizedDocuments(本链接)
```

默认对上述集合做整链检索（不默认收窄到当前打开的单文件）（Q16）。

**拒答：** 检索无 evidence → 不采用无依据 LLM 发挥；审计 `no_evidence`（Q15）。

**实现对齐债（P0）：**

| 环节 | 现状 | 要求 |
|------|------|------|
| 访客可见文档 | `documentsForAccessResponse`：数据室 + `folderPathInDealRoomScope` | 保持 |
| Ask Docs 检索 ID | `documentIDsForLink`：仅 `document_id` + `link_documents` | **必须与 Access 同源**，再 ∩ KB |
| 检索 API | `SearchInDocuments` + `document_id = ANY(...)` | 禁止回退 workspace 级 `Search()` |
| 空授权集合 | 通常无命中 | 安全失败文案，不搜全库 |
| 响应校验 | — | evidence.`document_id` 必须 ∈ 授权∩KB；越界丢弃并记安全事件 |

### 4.5 数据室知识库

#### 4.5.1 概念分离（Q2）

| 概念 | 职责 | 触发 |
|------|------|------|
| 文档 Ingestion | 解析页、chunks、预览；**不写** embedding（数据室路径） | 上传后异步 |
| 数据室知识库 | 勾选范围 + 向量化 + 就绪态 | room admin 显式「创建 / 重建」 |

预览不依赖 embedding；**Ask Docs 就绪靠 KB 状态 + 勾选集门控**。

单文档链接过渡期可继续上传时 embed，与室路径策略分离（Q1）。

#### 4.5.2 模型

- **归属：** Deal Room ↔ Knowledge Base（1:1）
- **勾选集：** 文件夹路径和/或 document_id；创建向导 **默认全不选**（Q10–Q12）
- **路径跟随（Q11）：** 已勾选文件夹下新增文档 → KB `stale`；**不**自动 embed；重建时纳入
- **状态：** `none` → `building` → `ready` | `failed` | `stale`
- **软过期（Q3）：** `stale` 时 Ask Docs **不停**；状态条提示重建
- **重建（Q9）：** 旧索引继续服务；新世代 ready 后原子切换；失败回滚并保留旧索引
- **写权限（Q14）：** 仅 room `owner` / `admin`
- **与链接：** 链接只消费 `KB就绪勾选集 ∩ 本链接授权`；不按链接物理复制索引

#### 4.5.3 文档页 UI

数据室 → **文档** 页工具区（room admin 可见）：

1. **创建知识库** — `none` / `failed`；向导勾选（默认全不选）→ 仅对勾选且 preview-ready 的文档写 embedding → 进度。
2. **重建知识库** — `ready` / `stale` / `failed`；可调整勾选集；二次确认（说明重建中访客仍可用旧索引）。

**状态条：**

- 未创建：知识库未创建 · 开启「问文档」前请先创建
- 构建中：构建中 12/40
- 就绪：知识库就绪 · 更新于 … · 已纳入 N 个文档 / M 个文件夹
- 过期：范围内文档有变更，建议重建（Ask Docs 仍可用）

#### 4.5.4 上线迁移（Q5）

对所有数据室：若 KB 不是 `ready`/`stale`，将该室下链接的 `ai_copilot_enabled` 置 `false`。发布说明引导：先建库 → 再开「问文档」。

### 4.6 Ask Docs 审计（信任红线）

Ask Docs 必须可审计；Signal **不能替代**审计账本，但 **继续双写**（Q8）。

#### 4.6.1 存储（Q6）

V1：以 public `assistant_sessions` / `assistant_messages` 为审计投影，补齐：

- 授权集合快照（document_id 列表或数量 + 哈希）
- 实际检索集合（授权 ∩ KB）
- 结果态：`success` / `no_evidence` / `kb_unavailable` / `scope_violation` / …

预留升级独立 append-only 审计表（B）。

#### 4.6.2 每次访客 Ask Docs 记录

| 字段 | 说明 |
|------|------|
| 时间、链接、数据室 | 会话上下文 |
| visitor_id / email | 与门禁一致 |
| 问题全文 | 访客输入 |
| 回答全文 | 模型输出或拒答文案 |
| 授权集合快照 | 本链接授权 document_id |
| 实际检索集合 | 授权 ∩ KB |
| 命中证据 | document_id、title、page、quote 摘要 |
| 会话 id | 串联多轮 |
| 结果态 | 见上 |

**越权：** 候选证据 ∉ 授权∩KB → 丢弃、记 `scope_violation`，不得返回访客、不得静默吞掉。

#### 4.6.3 所有者可见面（Q7、Q17）

- **可见角色：** 室成员 ∪ 工作区 admin；完整原文。
- **入口 C：**
  - 室级：数据室 → 活动/分析 → Ask Docs 时间线（可按链接筛）
  - 链接级：分享对话框 Management → 与问发起方并列下钻
  - V1 可先落地链接侧，室级紧随。
- 列表行示例：`访客 · 问文档 · 「估值假设？」 · 引用 2 处 · 2 分钟前`
- 详情：问答全文 + 证据列表。

#### 4.6.4 与问发起方分工

| | 问文档 | 问发起方 |
|--|--------|----------|
| 审计形态 | 只读对话账本 + 证据 | 待办收件箱 + 回复 |
| 所有者动作 | 复核 / 洞察（Signal） | 必须回复 |
| 信任点 | AI 只碰了授权∩KB | 人工沟通可跟进 |

#### 4.6.5 隐私与留存（Q18–Q19）

- 不对访客展示他人问答。
- **热数据：** 默认列表/详情展示 **90 天内**。
- **归档：** 90 天后移出默认列表，进入归档；**不删原文**；room/ws admin 仍可检索打开。
- 产品承诺：审计原文不用于模型训练。

### 4.7 防绕过访问控制（沟通 API）

原则：**过不了 Access / 已被移出允许或加入阻止的人，也打不通 Ask Docs / Ask Host。** UI 隐藏不是控件；一切以服务端为准。

#### 4.7.1 请求管道（每次 Ask Docs / Ask Host）

```text
1. publicToken（路径）与 X-Link-Session 绑定校验（Q24）
2. resolvePublicAccess 同构（Q20）
   — 会话签名/过期、security_version、链接状态/过期
   — NDA / 邮箱验证等门禁未满足 → 强制重新 Access
3. session.email 重算 allow/block（Q21）→ 失败作废会话
4. Feature 开关：ai_copilot_enabled / qa_enabled
5. Ask Docs 额外：KB 为 ready|stale；检索 ⊆ 授权∩KB；quote≤320；硬限额
6. Ask Host 额外：硬限额（更松）
7. 高危失败 → security_events（Q25）；其余 → 结构化日志
```

#### 4.7.2 限额默认值（Q22，可配置）

| 通道 | 窗口限额 | 日限额 | 维度 |
|------|----------|--------|------|
| Ask Docs | 20 / 10 分钟 | 200 / 日 | visitor_id + link_id |
| Ask Host | — | 30 / 日 | visitor_id + link_id |

超限：`429` + 审计/安全事件（高危）。

#### 4.7.3 证据防抽干（Q23）

- 返回访客的 `quote` 截断至 **320** 字符；保留 page / document_id / 跳转能力。
- 不得因「禁下载」关闭 Ask Docs，但不得用超长 quote 充当下载管道。

#### 4.7.4 API 形状

- Ask Host：保持 `/v1/public/links/:publicToken/questions`（已绑 token）。
- Ask Docs：从无 token 的 `/assistant/chat` **迁移**为带 `:publicToken` 的路径，并与 session 强一致（Q24）；前端与 e2e 同步改。

---

## 五、交互与 UI/UX

### 5.1 所有者 Access「高级」

- **不要：** 两个语义重叠的平行 Switch（现状问题）。
- **要：** 一张「沟通」能力卡 + 双子通道（compact rows / chips）。
- 图标语义与访客侧一致：问文档 = Robot；问发起方 = User。
- `?` Tooltip 各一句差异说明（Dialog 内需可用的 Tooltip）。
- KB 未就绪时「问文档」禁用或保存报错 + 链到文档页建库。
- 授权 ⊄ KB 时黄色警告列表（文件夹/文档名）。

### 5.2 访客「沟通」面板

```text
┌ 沟通 ─────────────────────────────────────┐
│ [问文档] [问发起方]   ← 仅双开时显示      │
│ 基于本链接授权材料                        │
│ ───────────────────────────────────────── │
│  (时间线 + 来源徽章 + 状态)               │
│ ───────────────────────────────────────── │
│ 占位符随通道变化                          │
│ [________输入________] [发送]             │
└───────────────────────────────────────────┘
```

- Tab：中文「沟通」/ 英文「Ask」。
- 空态（双开）：「总结授权材料要点」「材料好像缺了」（后者切 Host，不链文件请求）。
- 无证据拒答与 Host 引导同一视觉层级。
- 防截图模糊遮罩不得挡住侧栏操作。

### 5.3 文案原则

| 位置 | 忌 | 宜 |
|------|----|----|
| 所有者 AI | 「AI 助手」无说明 | 「问文档：基于授权内容即时回答并定位」 |
| 所有者 QA | 「启用问答对话」 | 「问发起方：访客可留言缺资料等问题」 |
| 访客 Tab | 「问答」 | 「沟通」 |
| 空态 | 「开始交流吧」 | 「先问文档找答案；找不到再问发起方」 |
| 访客侧 | 「知识库未创建」 | 「文档问答暂不可用」（隐藏 RAG 词） |

---

## 六、方案对比（归档）

| 方案 | 做法 | 结论 |
|------|------|------|
| **A. 配置统一 + 访客强化** | 主开关+双通道；两字段保留 | **采纳** |
| B. 强制合并为一个布尔 | 默认双开 | 拒：缺 AI 细控 |
| C. 访客再拆两个 Tab | AI / 问答分栏 | 拒：违背同入口 |
| D. 单输入自动路由无显式通道 | 全靠意图 | 拒作 V1：误投递风险 |

---

## 七、现状代码锚点（实施参考）

| 区域 | 路径 / 符号 |
|------|-------------|
| 访客统一面板 | `apps/web/src/components/viewer/UnifiedQAPanel.tsx` |
| 侧栏入口 | `RightSidebar` → `qaAvailable` / `UnifiedQAPanel` |
| 所有者开关 | `AccessTab` `ADVANCED_KEYS`：`aiCopilotEnabled` / `enableQaConversations` |
| 公共 AI（缺口） | 现 `POST .../assistant/chat`（无 publicToken）；目标绑 `:publicToken` + `resolvePublicAccess` |
| 公共 AI 服务 | `Service.PublicChat` · `SearchInDocuments` |
| 门禁同构参考 | `resolvePublicAccess` / `verifyPublicAccess` / `sessionSecurityGatesUnsatisfied` |
| 作用域债 | `documentIDsForLink` vs `documentsForAccessResponse` + `folderPathInDealRoomScope` |
| 访客问答（已较严） | `PublicCreateVisitorQuestion` → `verifyPublicAccess` |
| 信号 | `convertQuestionToSignalAsync` / `CreateQuestionSignal`（审计双写保留） |
| 上传 ingestion | `ingestion.Service.ProcessDocument`（室路径需改为不写 embedding） |
| 室角色 | `requireRoomAdmin`：owner / admin |

---

## 八、分阶段落地

| 阶段 | 内容 | 优先级 | V1 状态 |
|------|------|--------|---------|
| **Gate-0** | 沟通 API 门禁同构：Ask Docs 走 `resolvePublicAccess`、绑 publicToken、allow/block 重算、限额、quote 截断、高危 security_events | P0 | ✅ |
| **Sec-0** | Ask Docs documentIDs 与 Access scope 对齐 + 越权单测 + 响应二次校验 | P0 | ✅ |
| **Audit-1** | 会话投影补字段 + 链接侧审计 UI + 90 天/归档规则 + 无证据拒答 | P0 | ✅ |
| **Ingest-1** | 数据室上传路径停止自动 embedding（Q2）；单文档链接过渡策略隔离 | P0 | ✅ |
| **KB-1** | 室级 KB 模型、勾选向导、创建/重建、软过期、重建双世代、admin 权限 | P0/P1 | ✅ |
| **Mig-1** | Q5 迁移：无 KB 室关闭链接 Ask Docs + 发布说明 | P0（随 KB-1） | ✅ |
| Phase 0 | 文案 / Tab「沟通」；mode「问文档/问发起方」 | P1 | ✅ |
| Phase 1 | AccessTab「沟通」主卡 + Q4 门控 + Q13 警告 + 计数 | P1 | ✅ |
| Phase 2 | 室级审计汇总；访客空态/Host 待回复打磨 | P2 | ✅ |
| V1.5 | 发送前通道建议（非强制） | 可选 | ✅ |
| Future | 独立审计表；单文档链接废弃（V2.0） | 后续 | 未做 |

**发布门槛：** Gate-0 + Sec-0 + Audit-1 +（数据室）KB-1/Mig-1/Ingest-1 已满足；可对客户承诺「可审计且不可绕过门禁的室级 Ask Docs」（单文档链接仍按过渡策略，无室级 KB UI）。

**收工备注（2026-07-22）：** 访客冒烟 e2e（双开空态 → Ask Host → 待回复）与 SPEC #36（失焦模糊不挡 Ask）已覆盖；Out of Scope / Future 不在本 epic。

---

## 九、成功指标

| 指标 | 方向 |
|------|------|
| 双开链接中 Ask Docs 使用占比 | 显著高于 Host（默认正确） |
| Host 中「缺资料」类问题占比 | 可观测上升 |
| 访客首次提问时间 | 下降 |
| 问错通道导致的重复提问 | 下降 |
| 所有者配置困惑（两开关语义） | 下降 |
| 审计页打开率 / 证据复核点击 | 上升（信任） |
| `no_evidence` 后切 Host 比率 | 可观测（引导有效） |
| scope_violation 事件 | 应为 0（或仅测试触发） |
| 沟通 429 / block 事件 | 可观测（防抽干生效） |
| 无 session / 被 block 仍打通沟通 | 必须为 0 |

---

## 十、安全与信任验收清单

1. PublicChat 禁止无 document 过滤的 workspace `Search`。
2. `documentIDs` 与 Access 同源，再 ∩ KB 勾选就绪集；单测：allowlist 外 / 未勾选文档 chunk 不得出现在 evidence。
3. 响应后再校验 evidence.`document_id` ∈ 授权∩KB。
4. 空授权或空交集 → 安全失败/拒答，不搜全库。
5. 数据室上传不自动写 embedding；未勾选文档不得被 embed。
6. KB 重建不改变链接授权；重建中旧索引服务，原子切换。
7. 无 evidence 拒答；不返回无依据发挥内容。
8. 每次 Ask Docs 可审计；越权记 `scope_violation`。
9. 审计可见范围仅室成员 ∪ ws admin；90 天热数据 + 可检索归档。
10. Signal 与审计双写但入口文案不混淆。
11. Ask Docs / Host 均走 `resolvePublicAccess`；无有效会话不可用。
12. 每次沟通重算 allow/block；移出允许或加入阻止后旧会话立即失效。
13. Ask Docs 路径含 publicToken 且与 session 一致。
14. Ask Docs / Host 硬限额生效；超限 429。
15. 返回访客的 evidence quote ≤ 320 字符。
16. block / scope_violation / 超限写入所有者可见高危安全事件；普通 401 不刷爆事件表。

---

## 十一、修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-07-21 | 初稿：沟通整合、开放决策、KB、作用域红线、审计信任红线 |
| v1.1 | 2026-07-22 | Grilling Q1–Q19 共识写入：室级显式勾选 KB、embedding 门控、软过期、保存拦截、存量迁移、审计/Signal/留存/拒答/检索范围等 |
| v1.2 | 2026-07-22 | Grilling Q20–Q25：防绕过——门禁同构、allow/block 重算、硬限额、quote 截断、API 绑 publicToken、高危 security_events；新增 §4.7 与 Gate-0 |
| v1.3 | 2026-07-22 | V1 epic 收工：发布门槛与 Phase 0–2 / V1.5 标为已落地；Future 仍开放 |

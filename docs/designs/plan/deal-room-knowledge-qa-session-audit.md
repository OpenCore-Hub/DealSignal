# 数据室知识库 · 问答会话 / 审计记录

> 流式对话是载体；**可回放的本室问答**才是产品资产。  
> 本设计承接 [Grounded Chat 产品哲学](./deal-room-grounded-chat-philosophy.md)，把「单轮内存草稿」升级为「本室可核对审计时间线」。

状态：设计定稿 · **Phase A / A.1 / B / C 已落地**  
范围：数据室 Knowledge Tab（owner/editor）  
非范围：访客 AI Ask Docs 复辟、通用 Chat 线程、Perplexity 式来源默认收缩

---

## 1. 背景与决策

### 1.1 现状

| 能力 | 状态 |
|------|------|
| 语料同步 / 向量查询 | 已落地（`knowledge` + docling-rag） |
| 研究台 UI（答案 ∥ 证据轨） | 已落地；拒答隐藏证据 |
| Q&A 会话 / 审计 turns | **已落地**（`knowledge_qa_*`；刷新恢复最近 `active`；A.1 列表；Zustand 仅作舞台缓存） |
| 建议追问 V1 / 反馈 | **已落地**（前端模板追问；`knowledge_qa_feedback` upsert） |
| 旧 Ask Docs 审计栈 | 已删除（migration `106`）；模式可借鉴，表名/代码不复用 |

### 1.2 产品判断

- 整盘照搬 Perplexity **不优**：数据室优化的是**核对效率**，不是全网探索效率。
- 值得吸收的三点 + 审计底座：
  1. **本室问答会话 / 审计记录**（P0）
  2. **底栏继续提问 + 本室向建议追问**（P1）
  3. **轻量反馈**（出处有误 / 答非所问 / 有帮助）（P1/P2）

### 1.3 已拍板默认

| 议题 | 决策 |
|------|------|
| 会话开启 | **首次提问才创建 session**（避免空会话刷库） |
| 房间级历史 | **P0**：当前会话持久化 + 刷新恢复最近 `active`；**A.1**：session 列表 |
| 建议追问 | **V1 纯前端模板**；V2 再考虑模型生成（须锁「仅限本室」） |

---

## 2. 北极星与非目标

**一句话**  
在本室已同步语料内提问后，任何时候都能回看「问了什么、答了什么、依据哪页」，并在同一会话内继续深挖。

**成功判据**

1. 刷新页面后，最近研究会话与 turns 仍在。
2. 每轮可定位到文件 · 页码 · 摘录；拒答轮无假证据。
3. 从证据跳转 viewer 再返回，仍停留在同一 session。
4. 用户不会把界面理解成「通用聊天助手」。

**明确不做**

- ChatGPT 气泡墙 / 人格化 / 无出处闲聊
- 来源默认收缩为「来源 N >」侧栏
- 访客 AI Ask Docs 通道（可预留 `actor` 字段，本里程碑不做）
- SSE 流式（同一 turn 模型可后续挂接）
- 与 Ask Host（`link_visitor_questions`）混为同一面板

---

## 3. 体验隐喻

延续哲学文档「研究台」：

| 隐喻 | UI |
|------|-----|
| 研究会话 | Session：一次进入问答后的连续核对 |
| 批注回合 | Turn：问题 + 判定 + 证据快照 |
| 审计时间线 | 按时间回放 turns，非角色扮演线程 |
| 封条 | 语料就绪才允许开始提问（已有） |

---

## 4. 信息架构

### 4.1 两层表面（不变）

```
落地页：语义向量库 + AI文档问答
        └─ 开始提问（仅 corpus 真正就绪）
问答页：研究台
        ├─ 顶栏：返回向量库 | 仅限本室 | 出处可核
        │         | 本会话 · N 轮 | 新会话
        ├─ 主区：Turn 时间线（上卷）+ 答案 ∥ 证据轨
        ├─ 建议追问（Phase B）
        └─ 底栏：继续提问（固定）
```

### 4.2 Session / Turn

| 概念 | 含义 |
|------|------|
| **Session** | 本室研究会话；首次提问时创建；可「新会话」归档当前并清空舞台 |
| **Turn** | 一次提问 → 检索 → 回答/拒答/错误 → hits 快照 → 可选反馈 |

进入问答页**不**建 session；点「开始提问」只打开舞台。  
`POST …/query`（带 session 或「无 session 则懒创建」）时落库。

---

## 5. 分期

| Phase | 交付 | 依赖 |
|-------|------|------|
| **A · P0** | 表结构、写路径、刷新恢复、Turn 回放、打开原文 | 现有 `Query` |
| **A.1** | 房间级 session 列表（标题 / 预览 / 打开） | A |
| **B · P1** | 多轮时间线 UI、底栏绑定 session query、V1 建议追问 | A |
| **C · P1/P2** | 反馈表 + API + 每轮控件；审计展示 kind | A |

---

## 6. 数据模型

表前缀 `knowledge_qa_*`（与已删 `assistant_*` / `ask_docs_*` 隔离）。

### 6.1 `knowledge_qa_sessions`

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uuid PK | |
| `workspace_id` | uuid | |
| `room_id` | uuid | |
| `created_by` | uuid | 提问用户 |
| `title` | text | 可空；默认首问截断（如 80 runes） |
| `status` | text | `active` \| `closed` |
| `created_at` / `updated_at` | timestamptz | |
| `last_turn_at` | timestamptz | 列表排序 |

索引：`(room_id, last_turn_at DESC)`；`(room_id, status)`。

### 6.2 `knowledge_qa_turns`

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uuid PK | |
| `session_id` | uuid FK | |
| `room_id` / `workspace_id` | uuid | 冗余，便于房间级查询 |
| `sequence` | int | 会话内从 1 递增 |
| `question` | text | |
| `answer` | text | 可空 |
| `refused` | bool | **服务端**判定后入库 |
| `result_status` | text | `answered` \| `refused` \| `no_hits` \| `error` |
| `corpus_status_snapshot` | jsonb | 可选：synced/total、corpus status |
| `hits` | jsonb | 见下；拒答时必须 `[]` |
| `mode` | text | 查询模式快照 |
| `top_k` | int | |
| `error_summary` | text | `result_status=error` 时短摘要 |
| `created_at` | timestamptz | |
| `created_by` | uuid | |

索引：`(session_id, sequence)`；`(room_id, created_at DESC)`。

**`hits` 元素（对齐现网 `QueryHit`）**

```json
{
  "chunkId": "...",
  "documentId": "...",
  "text": "...",
  "score": 0.9,
  "sourceName": "...",
  "pages": [3, 4],
  "sheet": null,
  "viewerPage": 3
}
```

### 6.3 `knowledge_qa_feedback`（Phase C）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uuid PK | |
| `turn_id` | uuid FK | |
| `user_id` | uuid | |
| `kind` | text | `helpful` \| `wrong_citation` \| `not_answering` |
| `note` | text | 可选，限长（如 500） |
| `created_at` / `updated_at` | timestamptz | |

约束：`UNIQUE (turn_id, user_id)`（upsert）。

### 6.4 保留策略

- Recommended Default：**热数据 90 天**（可配置）。
- 超期 archive job：本里程碑不做；预留与旧 Ask Docs 冷热分层同思路。

---

## 7. API

权限：与 `GET/POST …/knowledge` 相同（房间成员可读可问）。  
路由挂在现有 knowledge handler 下。

### 7.1 Phase A

| Method | Path | 行为 |
|--------|------|------|
| `POST` | `/deal-rooms/:roomId/knowledge/sessions` | 显式建会话（「新会话」）；通常可省略，由 query 懒创建 |
| `GET` | `/deal-rooms/:roomId/knowledge/sessions` | 列表：`limit` + cursor；摘要含 title、last_turn_at、turn_count、question_preview |
| `GET` | `/deal-rooms/:roomId/knowledge/sessions/:sessionId` | session + turns（含 hits） |
| `POST` | `/deal-rooms/:roomId/knowledge/sessions/:sessionId/query` | 执行检索/生成，**原子追加 turn**，返回 `{ turn, …queryFields }` |
| `POST` | `/deal-rooms/:roomId/knowledge/query` | **保留**无会话探测；产品主路径走 session query |

**懒创建约定（推荐实现）**

```
POST /deal-rooms/:roomId/knowledge/sessions/query
body: { session_id?: string, query, answer?, top_k? }
```

- 无 `session_id` 或 session 已 `closed` → 创建新 `active` session 再提问  
- 有有效 `session_id` → 追加 turn  
- 响应带 `session_id`，前端写入 store  

（若希望 REST 更碎，可拆成「先 POST sessions 再 POST …/sessions/:id/query」；懒创建减少空会话。）

**写 turn 规则**

1. 每次调用无论 answered / refused / no_hits / error 均落库。  
2. `refused`：服务端与前端共用同一套启发式（或服务端权威）；拒答时 `hits = []`。  
3. 更新 session.`last_turn_at`、`title`（若空则用首问）。  
4. locked / folder-excluded 文档不得出现在 hits（沿用 Query 过滤）。

### 7.2 Phase C

| Method | Path | 行为 |
|--------|------|------|
| `PUT` | `/deal-rooms/:roomId/knowledge/turns/:turnId/feedback` | body `{ kind, note? }` upsert |

Get session 时附带当前用户的 `feedback?: { kind, note? }`。

---

## 8. 前端设计

### 8.1 状态

| 层 | 职责 |
|----|------|
| 服务端 sessions/turns | 真源 |
| `knowledgeQueryStore` | `activeSessionId`、`composer`、`activeCite`、可选 `turns` 缓存 |
| 落地 → 问答页 | `chatOpen=true`；**不**建 session |
| 首次提问 | 懒创建 session + 首 turn |
| 返回向量库 | 仅关舞台；不删 session |
| 刷新 | 恢复该房间最近 `active` session（若有 turns）并进入问答页或提供「继续上次」——**推荐：有 active session 且含 turns 时自动 `chatOpen=true` 并加载** |
| 新会话 | 将当前 session 标 `closed`（或仅前端换 id 并 POST 新 session）；舞台清空 |

### 8.2 问答页布局

```
[← 返回向量库] [仅限本室] [出处可核]     [本会话 · N 轮] [新会话]

┌─ Turn k ─────────────────────────────────────────────┐
│ 问题                                                  │
│ ┌ 判定 ─────────────┐  ┌ 证据轨（有 grounded 才显示）┐ │
│ │ answer / refused  │  │ 文件 · 页码 · 摘录 · 打开  │ │
│ └───────────────────┘  └────────────────────────────┘ │
│ [有帮助] [出处有误] [答非所问]          ← Phase C     │
└──────────────────────────────────────────────────────┘
（更早 turns 在上方，可滚动）

建议追问 · 基于本室文档          ← Phase B
  → …
  → …

┌ 继续提问 ────────────────────────────── [提问] ─────┐
└────────────────────────────────────────────────────┘
```

视觉约束：

- **非** user/assistant 气泡对；每轮是「研究卡片」。  
- 证据轨规则不变（哲学 P3/P4）。  
- 「返回向量库」与信任 chip 同款 pill（已落地）。

### 8.3 建议追问 V1（Phase B）

- 展示条件：存在至少一轮 turn。  
- 生成：前端模板，输入最近 turn 的 `sourceName` / `refused` / `result_status`。  
- 示例：
  - 有来源：`继续问《{sourceName}》里的责任条款？`
  - 拒答：`换个更具体的文件名或条款标题再问？`
- 标签文案：`基于本室文档`。  
- 禁止：行业常识、竞品对比、出室知识。

### 8.4 反馈（Phase C）

- 三选一（可切换）：`helpful` / `wrong_citation` / `not_answering`。  
- 不做社交化点赞条；可选极短 note（「哪条出处错了」）。  
- 写入审计，供后续评测/质检，非公开展示墙。

---

## 9. 权限与合规

- 读写范围 = 房间 knowledge 权限。  
- hits 摘录入库：可见性与 Query 一致。  
- 审计属工作区数据，随房间/工作区 ACL。  
- i18n：所有新文案走 `dealRooms`（en + zh-CN）。

---

## 10. 验收脚本

### Phase A

| ID | 步骤 | 期望 |
|----|------|------|
| A1 | 就绪语料下提问 → 硬刷新 | 恢复同一 session；可见 question / answer / 页码 |
| A2 | 触发拒答 | DB `refused=true` 且 `hits=[]`；UI 无证据轨 |
| A3 | 证据打开第 n 页 → 浏览器 Back | 仍在该 session 与 turn |
| A4 | 新会话后再问 | 旧 session 可在 A.1 列表回看；当前舞台为新 turns |
| A5 | corpus 未就绪 | 「开始提问」禁用（已有）；无法经 UI 开问 |

### Phase B

| ID | 步骤 | 期望 |
|----|------|------|
| B1 | 同 session 连问 2 轮 | 两条 turn，顺序正确；底栏始终可用 |
| B2 | 点建议追问 | 填入 composer 或直接发送；文案含本室线索 |

### Phase C

| ID | 步骤 | 期望 |
|----|------|------|
| C1 | 提交「出处有误」→ 刷新 | 仍显示；可改为「有帮助」 |
| C2 | 无 turn 时 | 不展示反馈控件 |

---

## 11. 实现切片

1. migration：`knowledge_qa_sessions` / `knowledge_qa_turns`（+ 后期 feedback）  
2. sqlc queries  
3. `knowledge` service：session CRUD、QueryAndAppendTurn、拒答判定入库  
4. handler routes + OpenAPI/适配层（若有）  
5. 前端 store 改造 + Tab 接入懒创建 session  
6. Turn 时间线 UI（替换「仅最新 result」）  
7. Phase B：建议追问模板 + 底栏  
8. Phase C：feedback API + 控件  
9. 测试：`go test ./internal/knowledge`；vitest Knowledge Tab；可选 e2e「刷新仍在」  
10. A.1：session 列表入口（向量库卡或问答顶栏）

---

## 12. 与哲学文档的关系

| 哲学原则 | 本设计落点 |
|----------|------------|
| P1 信任 > 流畅 | 审计可回放；不先炫多轮再补出处 |
| P2 范围即契约 | 建议追问锁本室；session 绑定 room |
| P3 证据一等公民 | hits 快照入库；拒答空 hits |
| P4 拒答是功能 | `refused` + UI 藏证据 |
| P5 审计连续 | 持久化替代纯内存；viewer Back 不丢 session |

多轮（哲学 §4.2）由 Phase B 兑现；流式仍按哲学 §5 后续挂接，**不阻塞**本里程碑。

---

## 13. 开放项（非阻塞）

| 项 | 说明 |
|----|------|
| 懒创建单路由 vs 两步 REST | 实现期二选一，契约以「首次提问建 session」为准 |
| 90 天清理 job | 后置 |
| 建议追问 V2（LLM） | 后置；须评测出室风险 |
| Visitor / Viewer 复用 | 组件白名单对齐哲学；本里程碑不做通道 |

---

## 14. 文档修订

| 日期 | 变更 |
|------|------|
| 2026-08-03 | 初稿定稿：Phase A/B/C + 已拍板三项默认 |
| 2026-08-03 | Phase A 实现：migration `111`、session query API、Turn 时间线 + 刷新恢复 |
| 2026-08-03 | A.1 / B / C：session 列表、建议追问、feedback `112`；§1.1 现状表同步 |

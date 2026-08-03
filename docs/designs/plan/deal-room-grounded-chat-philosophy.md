# 数据室知识库 · Grounded Chat 产品哲学

> DealSignal 不卖「会说话的助手」，卖「可核对的判断」。
> 流式对话是载体；出处、范围、语料完整性才是产品。

状态：设计定稿（实现可分阶段）  
范围：数据室 Knowledge Tab → 未来可复用到 Viewer 侧栏 / Visitor Ask（组件白名单一致）

---

## 1. 北极星

**一句话**  
在本数据室已同步语料内，用最短路径给出可核对答案，并让用户一键回到原文页码。

**成功判据（产品，非模型）**

1. 用户能说出「这句话来自哪份文件、哪一页」。
2. 用户从不误以为「全网知识」或「模型常识」在回答。
3. 拒答时界面不假装有证据。
4. 从答案跳到原文再后退，问答上下文仍在。

**反北极星（明确不做）**

- 不做通用 ChatGPT 皮肤（气泡堆砌、人格化、无出处闲聊）。
- 不做 A2UI 式任意 Generative UI 运行时（安全与品牌失控）。
- 不做「先炫流式、后补来源」的表演型 AI。

---

## 2. 五条不可妥协原则

### P1 · 信任先于流畅（Trust > Fluency）

流式打字可以有，但不得牺牲：

- 范围声明（仅限本室）
- 拒答诚实
- 证据与角标一致性

若检索未完成，宁可显示「正在检索本室语料…」，也不先吐无出处断言。

### P2 · 范围即契约（Scope is the Contract）

每一次回答都绑定：`workspace + dealRoom (+ link ACL if visitor)`。  
UI 必须持续可见「Scoped to this room / 仅限本室」。  
越权召回 = P0 缺陷，不是「偶发幻觉」。

### P3 · 证据是一等公民（Evidence is First-Class）

布局不是「聊天 + 可选 Sources」，而是：

```
答案（主） ∥ 证据轨（有 grounded hits 才存在）
```

无 grounded 来源 → **整块证据轨不渲染**（已落地）。  
相关分是次要；**文件 · 页码 · 摘录 · 打开原文** 才是主信息。

### P4 · 拒答是功能（Refusal is a Feature）

模型/管线拒答时：

- 展示拒答正文
- **隐藏**低分噪声 hits
- 不提供「也许相关」假安慰

数据室用户怕的是假阳性，不是「没搜到」。

### P5 · 审计连续（Audit Continuity）

- 引用跳转：同页 `navigate(/viewer/:id?page=n)`，不丢 workspace。
- Q&A 状态：room-scoped 内存恢复（viewer → Back）。
- 角标 `[n]` ↔ 证据卡双向高亮，形成可解释闭环。

---

## 3. 体验隐喻：研究台，不是聊天室

| 隐喻 | 含义 | UI 映射 |
|------|------|---------|
| 研究台 | 对着一摞已入库文件提问 | Ask-first 主舞台 |
| 批注 | 答案句旁脚注 | 引用角标 `[n]` |
| 书签 | 可回到原件页 | 打开第 n 页 |
| 封条 | 库是否完整、是否在同步 | 语料信任条 |
| 拒签 | 证据不足不盖章 | 拒答 + 无证据轨 |

禁止隐喻：伙伴、副驾驶、魔法棒。

---

## 4. 信息架构（单轮 → 多轮）

### 4.1 单轮（当前可演进）

1. **信任条**：状态 · 已同步比例 · 同步动作  
2. **提问台**：问题输入 · 提问/停止  
3. **判定区**：Grounded answer（流式）  
4. **证据轨**：有数据才出现  
5. **语料完整性**：文档行列表（次级，支撑信任，不抢问答）

### 4.2 多轮（下一阶段）

- Turn 列表上卷，输入固定底栏（或保留顶栏提问 + 历史）。
- 每轮独立：`query / phase / answer / sources / activeCite / refused`。
- 不把多轮做成「角色扮演上下文」；系统提示始终强调本室语料。

---

## 5. 流式时间哲学

**阶段顺序（推荐）**

1. `retrieving` — 诚实等待，不编造  
2. `sources`（可选先到）— 仅当后续不会被拒答清掉时展示；若管线会 refuse，可延后到 `done`  
3. `token*` — 答案生长；角标增量解析  
4. `done | refuse | error`

**闪烁禁忌**

- 禁止：先展示弱相关 sources → 再 refuse 清空（信任事故）。  
- 稳妥策略：refuse 判定前不挂证据轨；或 sources 事件仅在 `grounded: true` 时发出。

---

## 6. 组件白名单（A2UI 精神，非 A2UI 协议）

客户端只渲染固定目录：

| 组件 | 职责 |
|------|------|
| `TrustChip` | 仅限本室 / 出处可核 |
| `CorpusStatus` | 就绪·同步·失败 |
| `AnswerStream` | 流式文本 + 角标 |
| `CiteMarker` | `[n]` 交互 |
| `EvidenceRail` | 证据列表容器 |
| `EvidenceCard` | 单条出处 |
| `OpenPageAction` | 同页跳转 |
| `Composer` | 提问 / 停止 |

Agent/后端只许发：`phase | sources | token | done | error` 与上述 payload。  
禁止：任意 HTML、任意布局树、任意第三方 widget。

---

## 7. 品牌与视觉戒律

- 延续 DealSignal：slate、Geist、冷静 mono 页码。  
- 去模板：非对称答案/证据，少同质 Card 堆叠。  
- 禁止：紫系 AI 渐变、奶油衬线、emoji 状态、圆角彩虹 pill 集群。  
- 高级感来自：留白节奏、信任信息密度、出处可点——不是装饰光效。

---

## 8. 与现有实现对齐

已具备：

- Ask-first 布局、信任芯片、拒答隐藏证据、同页 viewer、store 恢复、角标解析。

待演进：

- 上游 docling **原生** token 流（仍为阻塞 JSON 时，服务端按已审计答案切分 `token*`）。  

已具备（相对初稿）：

- `KnowledgeStreamEvent` + reducer、`GroundedChatShell`、多轮 turns、`…/sessions/query/stream`（phase→sources?→token*→done）。

---

## 9. 度量（上线后）

| 指标 | 意图 |
|------|------|
| 出处点击率 / 问答次数 | 用户是否在核对 |
| 拒答后仍点「打开文档」比例 | 是否误导（应接近 0） |
| viewer 后退后继续追问率 | 审计连续是否成立 |
| 同步中提问后的负反馈 | 是否需阻断或强提示 |

**服务端底座（已落地 Prometheus）**

| 指标 | 用途 |
|------|------|
| `dealsignal_knowledge_qa_turns_total{result_status,transport}` | 问答量 / 拒答率 / JSON vs SSE |
| `dealsignal_knowledge_qa_turn_duration_seconds` | 检索+落库延迟 |
| `dealsignal_knowledge_qa_stream_errors_total{code}` | SSE 失败与客户端取消 |
| `dealsignal_knowledge_qa_feedback_total{kind}` | 反馈分布 |
| `dealsignal_knowledge_qa_retention_*` | 热数据 purge 健康度 |
| `dealsignal_knowledge_qa_cite_opens_total{turn_outcome}` | 出处/文档打开（`POST …/knowledge/events`） |
| `dealsignal_knowledge_qa_gate_rejects_total{transport,reason}` | 单飞 busy / RPM rate_limited |

出处点击率 ≈ cite_opens / turns；拒答后误导打开 ≈ cite_opens{refused} / turns{refused}。

---

## 10. 决策摘要

| 议题 | 决策 |
|------|------|
| 是否对标 A2UI 全协议 | 否；只吸收白名单声明式组件 |
| 是否对标通用 Chat UI | 否；对标研究助手「答案+证据」 |
| 无证据是否展示 Sources 空态 | 否；隐藏 |
| 跳转是否新标签 | 否；同页 navigate |
| 流式是否可牺牲拒答诚实 | 否 |

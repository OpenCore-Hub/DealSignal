# Visitor Ask / KB — V1 债务与补齐任务计划

> **关联**  
> - 设计：`docs/designs/plan/visitor-ask-knowledge-base.md`（v1.3）  
> - 规格：`docs/designs/plan/SPEC-visitor-ask-knowledge-base.md`  
> - 审计基准：2026-07-22 代码对照 SPEC（含工作区未合入改动）  
>
> **文档用途**：迭代实现、排期、验收与进度追踪。完成一项即改状态并记日期；发现新缺口追加行并 bump 修订记录。  
>
> **状态约定**

| 状态 | 含义 |
|------|------|
| `todo` | 未开始 |
| `doing` | 进行中 |
| `blocked` | 被依赖/环境挡住 |
| `review` | 实现完成，待评审/合入 |
| `done` | 已合入目标分支且验收通过 |
| `wontfix` | 明确不做（须写原因并考虑回写 SPEC） |
| `deferred` | 推迟到标注的里程碑 |

| 优先级 | 含义 |
|--------|------|
| P0 | 生产正确性 / 安全红线 / 假实现 |
| P1 | SPEC 字面缺口、可靠性 |
| P2 | 体验、命名、测试债、文档 |

---

## 0. 总览进度

| 批次 | 主题 | 项数 | done/review | 备注 |
|------|------|------|-------------|------|
| C | 生产正确性 hotfix | 3 | 3 review | C1–C3 已实现，待合入 |
| A | 假实现 / 空转 | 5 | 5 review | A1=C3；真双世代 + building Ask Docs + 无 chunk 校验 |
| B | 半真 / 缺口 | 11 | 9 review + 2 deferred | B1/B3–B7/B9–B11 review；B2/B8 deferred |
| D | 文档与追踪 | 2 | 2 review | 本文 + SPEC 对齐 |
| E | Out of Scope（仅登记） | — | — | 不计入完成率 |

**滚动完成率**：以合入 `done` 为准；当前工作区 P0 项均 `review`。

**当前建议下一刀**：合入部署 **C1–C3 + A2–A4 + B1 + B3–B7 + B9–B11**（含 migration `092`/`093`）；B2/B8 仍 deferred。

---

## 1. 批次 C — 已修待合入 / 部署

| ID | 优先级 | 状态 | 任务 | 验收 | 依赖 | 负责人 | 更新日期 |
|----|--------|------|------|------|------|--------|----------|
| C1 | P0 | `review` | KB upsert：`active_document_ids` / `building_document_ids` 禁止 Go nil→SQL NULL | 空选创建 KB 不再 23502；单测 `TestCreateKnowledgeBaseDefaultEmptySelection` / coalesce | — | | 2026-07-22 |
| C2 | P0 | `review` | Ask Docs 写 user 消息：`authorized_document_ids` / `retrieval_document_ids` 空数组 + SQL COALESCE | 访客提问不再 500「搜索失败」；有 scope 时走检索/拒答 | — | | 2026-07-22 |
| C3 | P0 | `review` | 生产接线 `KnowledgeBaseEmbedder`；无 provider/无 chunk fail-closed；`runEmbed` 禁止静默 no-op | 有 OPENAI 时 create/rebuild 真写 embedding；无 chunk 文档 → KB `failed` 带明确错误；相关单测绿 | C1 | | 2026-07-22 |

**合入清单（建议一次 PR）**

- [ ] `apps/api/internal/dealroom/knowledge_base.go` (+ tests)
- [ ] `apps/api/internal/assistant/service.go` (+ building ActiveDocumentIds test)
- [ ] `apps/api/internal/db/queries.sql` (+ sqlc) + migration `092_chunk_embedding_builds`
- [ ] `apps/api/internal/ingestion/document_embedder.go` (+ staging/promote/discard)
- [ ] `apps/api/internal/visitorask/limits.go` (Redis fail-closed)
- [ ] `apps/api/internal/server/routes.go`
- [ ] 部署后：对**有 text chunks** 的室文档 create/rebuild → `chunks.embedding IS NOT NULL`
- [ ] 部署后：rebuild 进行中 Ask Docs 仍命中旧 live 向量；promote 后切新世代
- [ ] 部署后：对**0 chunks** 文档 create → `400 no_searchable_chunks`，不进 building

---

## 2. 批次 A — 假实现 / 空转（P0）

| ID | 优先级 | 状态 | SPEC 依据 | 任务 | 验收 | 依赖 | 负责人 | 更新日期 |
|----|--------|------|-----------|------|------|------|--------|----------|
| A1 | P0 | `review` | KB-1 / US#7 | Embedder 生产接线（同 C3） | 见 C3 | C3 | | 2026-07-22 |
| A2 | P0 | `review` | US#8 | `documentIDsForLink`：status=`building` 时仍用 **当前 ActiveDocumentIds**（旧世代）做 Access∩KB | rebuild 进行中访客 Ask Docs 仍可命中旧索引；`TestPublicChatBuildingUsesActiveDocumentIds` | C3 | | 2026-07-22 |
| A3 | P0 | `review` | US#8 双世代 | **真双世代**：rebuild 写入 `chunk_embedding_builds`；成功后 Promote→live；失败 Discard；Ask Docs 全程读 live | 单测 stage/promote/discard；SPEC 与代码一致 | A2 | | 2026-07-22 |
| A4 | P0 | `review` | US#10 / 检索 | 无 text chunk 的 ready 文档：create/rebuild **进入 building 前**校验；`400 no_searchable_chunks` | `TestCreateKnowledgeBaseRejectsDocsWithoutChunks`；不出现假 ready | C3 | | 2026-07-22 |
| A5 | P2 | `review` | SPEC 状态 | SPEC 头与 §Phased delivery 与债务一致 | SPEC 链到本文；双世代/限额策略已回写 | D1 | | 2026-07-22 |

### A3 决策记录

| 选项 | 决定 | 日期 | 备注 |
|------|------|------|------|
| 真双世代向量 | ☑ | 2026-07-22 | `chunk_embedding_builds` + Promote/Discard |
| 降级 SPEC + A2 保底 | ☐ | | 未采用 |

---

## 3. 批次 B — 半真 / 缺口（P1–P2）

| ID | 优先级 | 状态 | SPEC 依据 | 任务 | 验收 | 依赖 | 负责人 | 更新日期 |
|----|--------|------|-----------|------|------|------|--------|----------|
| B1 | P1 | `review` | US#25 | Ask Docs/Host 限额：Redis 不可用时 **fail-closed（拒绝）** | 超限 → 429/`rate_limit_exceeded` + 安全事件；Redis 失败 → 503/`limiter_unavailable`（不写 rate_limit 事件）；单测覆盖 | — | | 2026-07-22 |
| B2 | P2 | `deferred` | US#28 | 物理/冷归档（V1 现为 90 天软过滤） | 里程碑：独立审计表或归档存储 | Future | | 2026-07-22 |
| B3 | P1 | `review` | US#32 | 所有者可见 Ask 高危安全事件（block / scope_violation / rate_limit）入口 | 链接/室分析页 `AskSecurityEventsPanel`；migration `093`；API `ask-security-events` | — | | 2026-07-22 |
| B4 | P2 | `review` | US#20 | 核对审计详情返回的 quote 是否也需 ≤320；与访客响应策略一致 | 持久化 + 审计投影均 ≤320；`TestGetAskDocsAudit_TruncatesLongQuotes` / stored evidence 断言 | — | | 2026-07-22 |
| B5 | P2 | `review` | Frontend 命名 | Bundle/Smart Link「AI Copilot」→ Visitor Ask / Ask Docs；清理遗留 `aiAgents` / `qaConversations` 文案 | Access + creator 无旧名；en/zh-CN 同步 | — | | 2026-07-22 |
| B6 | P1 | `review` | 测试债 | MSW：KB 预门控、coverage warning、public `assistant/chat`、（可选）KB/audit handlers | Playwright：`visitor-ask-docs` + `knowledge-base` + `ask-security-events` + `visitor-ask-naming` + `visitor-ask-kb-gate`（US#11/12/A4）；MSW handlers | C2 | | 2026-07-22 |
| B7 | P2 | `review` | US#31 | Management「Visitor questions」等与 Ask Host / 审计入口文案分离 | Engage：Ask Host activity/inbox 与 Ask Docs audit 分栏；中英文明确「审计≠信号≠问发起方」；e2e `visitor-ask-naming` | — | | 2026-07-22 |
| B8 | P2 | `deferred` | 架构 | 统一 Visitor Ask gate（限额+安全事件）减少 Docs/Host 双路径 glue | 非功能阻断；可随重构 | — | | 2026-07-22 |
| B9 | P1 | `review` | US#32 / US#24 | Owner 高危事件列表包含白名单撤销 `not_in_allow_list`（与 blocked_* 同属 block） | SQL 过滤 + en/zh-CN；MSW/e2e 可见「Removed from allowlist」 | B3 | | 2026-07-22 |
| B10 | P0 | `review` | 假实现 | 消灭 `DealRoomQATab` 假数据/`comingSoon`；室级 Ask Host 收件箱 + 真回复 | `GET …/deal-rooms/:id/visitor-questions`；owner/admin 或室成员鉴权；DTO snake_case；e2e `visitor-ask-host-inbox`；auth 单测 | — | | 2026-07-22 |
| B11 | P1 | `review` | US#25 / B1 UX | 访客侧区分超限 vs 限额器不可用（Ask Docs + Ask Host） | `rate_limit_exceeded` / `limiter_unavailable` 独立 i18n；e2e 断言文案；不把基础设施失败写成「请求过多」 | B1 | | 2026-07-22 |

### B1 决策记录

| 选项 | 决定 | 日期 | 备注 |
|------|------|------|------|
| Redis 失败 fail-closed | ☑ | 2026-07-22 | 生产硬限额；nil limiter 仅测试/未接线 |
| Redis 失败与访客超限分码 | ☑ | 2026-07-22 | 超限=`rate_limit_exceeded`/429；基础设施=`limiter_unavailable`/503，不记入高危 rate_limit 事件 |

### B4 决策记录

| 选项 | 决定 | 日期 | 备注 |
|------|------|------|------|
| 审计返回全文 quote（仅访客截断） | ☐ | | 与 US#20 防刮策略不一致，且历史行可能更长 |
| 持久化 + 审计投影均 ≤320；LLM 上下文用全文 | ☑ | 2026-07-22 | `complete` 写入前截断；`GetAskDocsAudit` 再截断旧数据 |

---

## 4. 批次 D — 文档与追踪

| ID | 优先级 | 状态 | 任务 | 验收 | 依赖 | 更新日期 |
|----|--------|------|------|------|------|----------|
| D1 | P2 | `review` | 本任务计划文档（本文） | 可勾选、可修订 | — | 2026-07-22 |
| D2 | P2 | `review` | SPEC / 设计状态与债务对齐；链到本文 | 读者不会误以为双世代仍未实现 | A5 | 2026-07-22 |

---

## 5. 已确认落地（对照基线，勿重复开工）

以下在审计时判定为**真实现**（部署含 C/A/B1 批后更稳）。回归时抽查即可，不单独立项除非回退。

- Gate-0：门禁同构、token 绑定、quote≤320、高危事件写入  
- Sec-0：Access ∩ KB、证据二次过滤、禁 public workspace-wide Search、空集 fail-closed  
- Ingest-1：室路径 `skip_embedding`  
- Mig-1：`090_disable_ask_docs_without_kb`  
- 保存侧 KB ready/stale 硬门控 + auth⊄KB soft warning  
- 审计投影 API + 链接/室 UI  
- Visitor Ask 主卡/计数、双空态、Host 待回复、无 file-request 深链、V1.5 通道提示、#36 blur、`no_evidence` 拒答  
- **A2/A3**：building 期 ActiveDocumentIds + `chunk_embedding_builds` 真双世代  
- **A4**：create/rebuild 前置无 chunk 校验  
- **B1**：Ask 限额 Redis 错误 fail-closed  
- **B3**：owner 可见 Ask 高危安全事件 API + 链接/室分析面板（migration `093`）  
- **B5**：Access / Bundle creator 用户文案统一为 Visitor Ask / Ask Docs / Ask Host；移除 aiAgents/qaConversations  
- **B4**：Ask Docs 证据 quote 持久化与审计投影均 ≤320（LLM 上下文仍可用全文）  
- **B6**：MSW Playwright Ask Docs / KB 覆盖  
- **B9**：owner Ask 高危事件含 `not_in_allow_list`（白名单撤销）
- **B10**：室级 Ask Host 收件箱（真 API + 鉴权，无假数据）
- **B11**：访客侧 `rate_limit_exceeded` / `limiter_unavailable` 独立 i18n（Ask Docs + Ask Host）

---

## 6. Out of Scope（仅登记，默认 `deferred` / 不排期）

| 项 | 来源 |
|----|------|
| 独立 append-only 审计表 | SPEC OOS / Future |
| 单文档链接正式 KB 产品 / V2 废弃 | SPEC OOS |
| 全自动意图路由（无显式通道） | SPEC OOS |
| File request 并入 Visitor Ask | SPEC OOS |
| Per-link 物理向量索引 | SPEC OOS |
| 扫描件 OCR（A4 可另开 epic） | 产品增量 |

---

## 7. 建议迭代节奏

| Sprint / 刀 | 内容 | 退出标准 |
|-------------|------|----------|
| 刀-0+1+2 | 合入部署 C1–C3 + A2–A4 + B1 | 有 chunk rebuild 出 embedding；building 可问旧集；无 chunk → 400；Redis 挂 → 503/`limiter_unavailable`（非 429） |
| 刀-3 | B3 + B6 | 事件可见；MSW 覆盖 Ask Docs（B3/B6 已实现待合入） |
| 刀-4 | B5/B7/B4/B9 | 命名、文案分离、quote、白名单撤销事件可见（均已实现待合入） |

---

## 8. 如何更新本文

1. 改对应行的 **状态 / 负责人 / 更新日期**。  
2. 完成时在「验收」列打勾或写 PR 链接（`PR #…`）。  
3. 新缺口：追加 ID（续号 `A6`/`B9`/`C4`…），并在 §0 总览改计数。  
4. 决策项（A3/B1）填决策表后再改任务状态，避免实现与 SPEC 漂移。  
5. 每次合入相关 PR，在下方修订记录加一行。

---

## 9. 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 0.1 | 2026-07-22 | 初稿：基于 SPEC 代码审计缺口清单（A/B/C/D）+ 建议迭代节奏 |
| 0.2 | 2026-07-22 | 落地 A2/A3/A4/B1：真双世代 staging、building Ask Docs、无 chunk 前置校验、Redis fail-closed；SPEC 回写 |
| 0.3 | 2026-07-22 | 落地 B3：migration `093` 扩展 event_type；owner API + `AskSecurityEventsPanel`（链接/室）；MSW handlers |
| 0.4 | 2026-07-22 | 落地 B5：Bundle/Access 文案 AI Copilot→Ask Docs/Visitor Ask；删除遗留 aiAgents/qaConversations 文案键 |
| 0.5 | 2026-07-22 | B5 评审跟进：zh-CN 统一「沟通/问发起方」；Bundle 主标题 Visitor Ask；MSW `visitor-ask-naming.spec.ts` |
| 0.6 | 2026-07-22 | 落地 B4：证据 quote 写入与审计投影统一 ≤320；SPEC 回写 |
| 0.7 | 2026-07-22 | 落地 B7：Ask Host inbox/activity 与 Ask Docs audit / Signal 文案分离；Engage 区顺序 Ask Host→审计→安全事件 |
| 0.8 | 2026-07-22 | B6 补齐：MSW access-rules + deal-room link create 门控；Playwright `visitor-ask-kb-gate`（US#11/12/A4） |
| 0.9 | 2026-07-22 | B9：Ask 高危事件列表纳入 `not_in_allow_list`（US#24/32 block） |
| 1.0 | 2026-07-22 | B10：消灭 DealRoomQATab 假实现；室级 visitor-questions API + owner 收件箱；Ask Host JSON DTO |
| 1.1 | 2026-07-22 | 评审跟进：Ask Host 室/链接鉴权对齐 audit；去掉 analytics comingSoon；zh-CN dashboard 复数键同步 |
| 1.2 | 2026-07-22 | B1 taxonomy：Redis fail-closed → 503/`limiter_unavailable`；访客超限仍 429/`rate_limit_exceeded` |
| 1.3 | 2026-07-22 | B11：访客 Ask Docs/Host 错误文案按 code 分流（超限 vs 限额器不可用） |

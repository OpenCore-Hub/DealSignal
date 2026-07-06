---
task_id: "TASK-SHARE-LONG-002"
parent_issue: "DS-SHARE-011"
agent_task_id: "AGENT-TASK-SHARE-011"
version: "v1.0.0"
priority: "P2"
status: "待执行"
type: "ai"
effort: "L"
branch: "feat/share-long-002-ai-intent-analysis"
estimated_files: "14"
max_lines: "800"
project_stack: "Go 1.25 + PostgreSQL + OpenAI-compatible LLM"
ai_red_flags:
  - "不得把用户 PII 发送给外部 LLM"
  - "主题分类必须可解释、可校准"
  - "分析结果必须异步生成，不能阻塞 AI 回答"
  - "重复问题检测必须考虑语义而不仅是字符串"
ai_confidence: "low"
pending_confirmation:
  - "是否使用外部 LLM 做分类，还是本地轻量模型？"
  - "意图分析是否实时写入 signal 表，还是仅作为 insights 展示？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-LONG-002 AI 问答意图分析

> **父 Issue**：`DS-SHARE-011`  
> **版本**：`v1.0.0`  
> **优先级**：`P2`  
> **类型**：`ai`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-long-002-ai-intent-analysis`

---

## 1. 目标

将 AI Copilot 的问答内容转化为意图信号：
- 对用户提问做主题分类（如 pricing、security、competition、implementation）。
- 检测重复/相似问题，识别用户持续关注点。
- 输出紧迫度/购买意向评分，并进入信号/行动流。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.3 / §4.4 |
| PRD | `docs/backup/PRD-v2.1.0.md` FR-06 / FR-11 |

### 2.1 已有代码

- `apps/api/internal/assistant/service.go` — 问答服务
- `apps/api/internal/db/migrations/003_search_assistant.up.sql` — `assistant_messages`
- `apps/api/internal/signal/service.go` — 信号生成

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 分析对象 | `assistant_messages.content`（user 消息） | 排除 assistant 回答 |
| 分类体系 | 可配置 | pricing / security / team / implementation / etc. |
| 相似度 | 语义相似度 ≥0.85 | 使用 embeddings 或 LLM 判断 |
| 异步 | 问答回答后立即入队分析 | 不阻塞用户 |
| 隐私 | 脱敏后分析 | 移除邮箱/公司名等 PII |

### 3.2 输出指标

| 指标 | 说明 |
|---|---|
| `topic` | 主题标签 |
| `sentiment` | 积极/中性/消极 |
| `urgency` | 1-5 |
| `buying_intent` | 0-100 |
| `repeat_count` | 同类问题重复次数 |
| `key_evidence_pages` | 用户关注的页码列表 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/assistant/analyzer.go` | 新增 | 问题意图分析服务 |
| `apps/api/internal/assistant/worker.go` | 新增 | 异步分析 worker |
| `apps/api/internal/db/migrations/0XX_ai_intent.up.sql` | 新增 | `assistant_message_intents` 表 |
| `apps/api/internal/db/queries.sql` | 新增 | 意图 CRUD 与聚合 |
| `apps/api/internal/signal/service.go` | 修改 | 根据 AI 意图生成信号 |
| `apps/api/internal/suggestions/service.go` | 修改 | 基于 AI 意图生成建议 |
| `apps/api/internal/llm/client.go` | 修改（可选） | 增加分类/分析 prompt |
| `apps/api/internal/server/server.go` | 修改 | 启动分析 worker |

### 4.2 行为定义

- 用户提问后，assistant 服务异步调用 analyzer。
- analyzer 输出意图标签与评分。
- 高 buying_intent 或重复关键主题时，生成 `ai_intent_signal`。
- 信号进入 Dashboard/Insights 与通知流。

---

## 5. 验收标准

- [ ] 用户问题可被分类到预定义主题。
- [ ] 相似问题检测准确率 ≥80%（基于测试集）。
- [ ] 高意向问题生成信号与行动项。
- [ ] 分析异步执行，不影响 AI 回答延迟。
- [ ] PII 在发送给 LLM 前已脱敏。

---

## 6. 实现步骤建议

1. 设计 `assistant_message_intents` 表。
2. 实现异步分析 worker 与队列（可用 Redis 或 DB 轮询）。
3. 实现 `Analyzer`：分类、情感、紧迫度、重复检测。
4. 集成到 `signal` / `suggestions` 服务。
5. 增加配置开关：是否启用 AI 意图分析。
6. 补测试与评估集。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/assistant/...
go test ./internal/signal/...
make lint
```

---

## 8. 约束与红线

- 不得把用户 PII 明文发送给外部 LLM。
- 分析结果必须有可解释性（保留分类依据）。
- 异步分析失败不得影响主流程。
- 主题分类体系必须可配置，避免硬编码。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-011`

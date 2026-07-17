# Signal & Risk 模块端到端重构方案

## 1. 背景与核心问题

「今日关注」里的 **高意向信号** 和 **风险** 是 DealSignal 的核心卖点：

- **高意向信号**：把文档访问行为变成「今天该联系谁」的可执行线索。
- **风险提醒**：发现泄密、异常下载、材料跳出等安全/效果问题。

但当前代码存在严重的类型/数据流断裂，导致这两个 tab 在生产环境几乎一定为空：

1. **后端 `analytics/handler.go` 过滤条件写错**：
   - `heatAlertList` 按 `"hot" / "warm"` 过滤，但 DB 实际值是 `"hot_signal"`。
   - `riskAlertList` 按 `"risk"` 过滤，但 DB 实际值是 `"risk_alert"`。
2. **前端 `SignalType` 枚举与后端不一致**：
   - 前端定义 `"hot" | "warm" | "cold" | "risk"`。
   - 后端返回 `"hot_signal" | "risk_alert" | "follow_up"`。
3. **DashboardStats 没有先同步 suggestions**：直接读 `signals` 表，可能漏掉最新建议。
4. **suggestion 未关联 contact**：`ContactID` 永远是空，导致信号无法跳转到联系人。
5. **关键页判断用 document title 而非 page title**：语义错误，影响 heat score 和建议生成。

## 2. 产品/商业价值

| 能力 | 价值 |
|---|---|
| 高意向信号 | 让创始人/AE/销售在 5 秒内知道今天该跟进谁，直接提升付费转化和 deal room upsell。 |
| 风险提醒 | 防止敏感材料泄露、发现异常访问、提升平台信任度，是合规审计的重要卖点。 |
| AI 增强建议 | 把「访问了 5 次」变成「投资人反复看财务页，建议今明两天发详细尽调包」，差异化竞争。 |

修复并增强这个模块，是 v2.1.x → v2.2 最重要的产品升级点之一。

## 3. 目标

1. **立即可用**：修复类型/API 契约，让两个 tab 真实展示数据。
2. **增强可解释性**：每个信号/风险都要展示来源行为、建议动作、一键跳转。
3. **建立 AI 扩展点**：把 LLM 生成的 reason/action、AI 提问意图纳入信号流。

## 4. 推荐方案：两阶段重构

### 阶段一：修复契约与数据流（必须，先落地）

**统一前后端类型契约**
- 以 DB 业务语义为准：前端 `SignalType` 改为 `"hot_signal" | "risk_alert" | "follow_up"`。
- 后端保持 DB 值输出，不在 API 层引入新的别名，避免两套命名。

**修复后端 dashboard stats 过滤**
- `internal/analytics/handler.go`:
  - `heatAlertList` 过滤 `s.Type == "hot_signal"`（如需包含 follow_up 则单独定义）。
  - `riskAlertList` 过滤 `s.Type == "risk_alert"`。

**修复前端渲染/排序**
- `AttentionZone.tsx`：`hotSignals = signals.filter(s => s.type === "hot_signal")`。
- `SignalCard.tsx`：`typeConfig` 增加 `hot_signal` / `risk_alert` / `follow_up` 配色与图标。
- `sortSignals.ts`：`typeOrder` 对齐新枚举。
- i18n：`signal.types.hot_signal` 等 key 已存在，确认中英文一致。

**统一信号来源**
- 让 `analytics.DashboardStats` 在列出信号前先 `syncFromSuggestions`（复用 `signal.Service` 的同步逻辑或提取公共函数）。
- 这样 `/dashboard/stats` 与 `/signals` 看到的信号一致，风险提醒也不会为空。

**修复 suggestion → signal 的上下文**
- `suggestions/service.go` 的 `Generate`：如果 link 有关联 contact，写入 `ContactID`。
- 关键页检测：从 `page_views` 取每页 title，而不是用 `document_title` 一次性判定全部页。

**修复测试与 Mock**
- `apps/web/src/lib/mocks/data.ts`：mockSignals 使用真实类型；mockRiskAlerts 从 `mockSignals.filter(s => s.type === "risk_alert")` 派生。
- 更新 `AttentionZone.test.tsx`、`DashboardPage.test.tsx` 的类型与断言。

### 阶段二：增强信号质量与 AI 扩展（产品亮点）

**语义化风险分类**
- 后端 `riskAlertList` 根据 signal title/description 推断 `type: download | expired | forward | anomaly`，与前端 `RiskAlert.type` 对齐。
- 前端 `RiskAlertList` 按类型展示不同图标/文案。

**信号上下文卡片**
- `SignalCard` 展示触发摘要："3 次打开，停留 4.2 分钟，查看了财务页"。
- 关联 contact 时显示联系人邮箱和跳转。

**AI 增强建议生成（可选开关）**
- 在 `suggestions/service.go` 中，当 `OPENAI_API_KEY` 存在时，把 metrics + document context 传给 LLM，生成更自然的 `Reason` 和 `Action`。
- 无 AI 或失败时回退到现有模板。

**AI 提问转信号（ roadmap LONG-002）**
- 当 public viewer 在 AI Copilot 提问时，`assistant/service.go` 分析问题意图，生成 `ai_intent_signal` suggestion，走 suggestions → signals 流水线。

**通知层增强**
- `hot_signal` 生成时，可选让 LLM 基于联系人、文档、行为起草 follow-up 邮件，推入 `notification` 队列。

**实时推送（未来）**
- 预留 WebSocket/SSE 接入点，让新信号和风险可以不刷新 Dashboard 即出现。

## 5. 主要文件变更清单

### 后端
- `apps/api/internal/analytics/handler.go`：修复 `heatAlertList`、`riskAlertList`；让 `DashboardStats` 先同步 suggestions。
- `apps/api/internal/analytics/service.go`：接入 suggestion sync 或调用 signal feed。
- `apps/api/internal/suggestions/service.go`：填充 `ContactID`、修复 key page 判断。
- `apps/api/internal/signal/service.go`：暴露同步函数供 analytics 调用（如需要）。
- `apps/api/internal/heat/keypages.go` / `score.go`：必要时配合 key page 语义调整。

### 前端
- `apps/web/src/types/index.ts`：`SignalType` 改为真实类型；`RiskAlert.type` 可扩展语义类型。
- `apps/web/src/components/dashboard/AttentionZone.tsx`：过滤条件与类型对齐。
- `apps/web/src/components/dashboard/SignalCard.tsx`：`typeConfig` 对齐。
- `apps/web/src/lib/sortSignals.ts`：排序对齐。
- `apps/web/src/lib/mocks/data.ts`：mock 数据对齐。
- 测试：`AttentionZone.test.tsx`、`DashboardPage.test.tsx`、可能新增 `SignalCard.test.tsx`。

## 6. 测试策略

- **后端**：
  - `go test ./internal/analytics`：覆盖 `heatAlertList`、`riskAlertList` 过滤。
  - `go test ./internal/suggestions`：覆盖 hot/risk/follow_up 生成条件。
  - `go test ./internal/signal`：覆盖 suggestion → signal → action_item 同步。
- **前端**：
  - `pnpm test --run` 全量通过。
  - 新增/更新：真实类型信号渲染在「高意向信号」tab；真实 `risk_alert` 渲染在「风险」tab；空状态；排序。
- **E2E**：
  - `./e2e-test.sh` 验证 dashboard 正常加载。
  - `./e2e-ai.sh` 验证 AI 链路不影响信号流。

## 7. 验收标准

- [ ] Dashboard「高意向信号」tab 显示来自真实访问数据的 `hot_signal`。
- [ ] Dashboard「风险」tab 显示来自真实访问数据的 `risk_alert`。
- [ ] 点击信号卡片能正确跳转到 document / link / contact。
- [ ] 信号按优先级/时间正确排序，空状态文案正常。
- [ ] 前端 `pnpm typecheck && pnpm lint && pnpm test --run` 通过。
- [ ] 后端 `go test ./...` 通过。
- [ ] MSW mock 与后端 API shape 一致。

## 8. 风险与回滚

- **类型改动影响面**：MSW、测试、Storybook（如有）需要全量更新；一次性改完比兼容两套命名更干净。
- **AI 增强失败**：保留模板 fallback，不会导致服务不可用。
- **关键页语义调整**：可能影响 heat score，需配合测试阈值调整。
- **回滚**：无需数据库 migration，回滚代码即可恢复。

## 9. 建议的落地顺序

1. 阶段一后端修复（类型过滤 + sync + contact + key page）。
2. 阶段一前端修复（类型、过滤、排序、mock、测试）。
3. 跑通前后端测试与 E2E。
4. 阶段二：风险语义分类 + 信号上下文卡片。
5. 阶段二：AI 增强建议与 AI 提问转信号（可拆分为独立里程碑）。

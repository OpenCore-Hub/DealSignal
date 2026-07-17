# Signal & Risk 模块深度 Code Review：洞察准确性

> Review 时间：2026-07-16  
> Review 范围：`apps/api/internal/suggestions/*`、`apps/api/internal/signal/*`、`apps/api/internal/heat/*`、`apps/api/internal/events/*` 及对应 SQL  
> 当前 commit：`f07843f feat(suggestions,signal): P2 production extensions`  
> 核心结论：**架构底座已具备（规则引擎、审计、特征缓存、异步流水线），但默认规则的阈值与时间窗口存在明显不一致，是当前“乱建议”的主要原因。**

---

## 1. 当前机制如何运行

数据流可以概括为：

```
访问事件（access_logs / page_views / security_events）
    ↓
HTTP handler 写入 suggestion_outbox
    ↓
Suggestion Worker 轮询 → Service.Generate(link)
    ├─ metrics()/behaviorFeatures() 聚合特征
    ├─ heat.Compute() 计算热度
    ├─ RuleEngine.Evaluate() 对 YAML 规则求值
    ├─ recentExists() 按 (link, type, subtype) 24h 去重
    ├─ CreateSuggestion 持久化
    └─ CreateSignalRuleRun 写审计
    ↓
发布 suggestion.generated 事件
    ↓
SignalConsumer → SyncWorkspace → signal / action_item 幂等同步
```

### 1.1 规则引擎

- `internal/suggestions/ruleengine.go` 使用 `expr-lang/expr` 对 YAML 中的 `expression_rules` 逐条求值。
- 每条规则可配 `enabled`、`bucket_percent`、`bucket_key`，通过 FNV-1a 哈希实现稳定的 A/B 分桶。
- `security_event_rules` 直接把 `security_events` 表的事件映射为 `risk_alert`。

### 1.2 特征来源

- 实时查询：`GetLinkAccessMetrics`、`GetLinkPageViewMetrics`、`GetLinkBounceCount`、`GetLinkKeyPageViewMetrics`、`CountRecentDistinctIPsByLink`、`CountRecentDownloadAttemptsByLink`。
- 缓存：`link_features` 由 `FeatureWorker` 每 5 分钟为最近 1 小时活跃的 link 预计算。
- `Service.metrics()` / `Service.behaviorFeatures()` **优先读 `link_features`，未命中再 fallback 实时查询。**

### 1.3 去重

`CountRecentSuggestionsByLinkTypeSubtype` 检查 `created_at > now() - interval '24 hours'` 且 `dismissed = false` 的 (link, type, subtype) 记录，避免 24h 内重复生成同类建议。

### 1.4 审计

每次生成成功后会写入 `signal_rule_run`，包含：
- `input_snapshot`：heat、metrics、behavior、security_events
- `matched_rule_ids`
- `generated_suggestion_ids`
- 运行耗时 `duration_ms`

---

## 2. 准确性风险点（按严重程度排序）

### 🔴 P0：规则使用的指标是“全量累积”，但文案/语义暗示“最近”

这是当前最可能导致误报的问题。

| 规则 | 当前条件 | 使用的字段 | 字段实际窗口 | 问题 |
|---|---|---|---|---|
| `follow_up_download` | `downloads > 0` | `Metrics.Downloads` | **lifetime**（`GetLinkAccessMetrics` 无时间过滤） | 30 天前的下载也会触发，且文案写“last 24 hours” |
| `follow_up_revisit` | `revisits > 0` | `Metrics.Revisits` = `opens - uniqueVisitors` | **lifetime** | 同上 |
| `risk_bounce` | `bounces > 0 && avgDurationMinutes / bounces < 0.5` | `Metrics.Bounces` | **lifetime** | 历史 bounce 累积后，极易长期误触发 |
| `hot_signal` | `heat.level == "hot" && opens >= 2` | `Metrics.Opens` | lifetime | 即使近期无活动，旧高分仍可能触发（见下文 decay 缺失） |

**证据：**

```sql
-- internal/db/queries.sql:435
SELECT
    COUNT(*) FILTER (WHERE event_type = 'link_opened') AS opens,
    COUNT(DISTINCT visitor_id) FILTER (WHERE event_type = 'link_opened') AS unique_visitors,
    COUNT(*) FILTER (WHERE event_type = 'download_attempted') AS downloads
FROM access_logs
WHERE link_id = $1;        -- 无 created_at 过滤
```

```yaml
# internal/suggestions/default_rules.yaml:31
reason_template: '{{.downloads}} download(s) in the last 24 hours'
```

**后果：** 用户看到“24 小时内有 1 次下载”的建议，实际可能是几个月前的行为；bounce risk 对老链接持续误报。

### 🔴 P0：`Suggestion` 的热度计算没有衰减（DecayDays = 0）

`heat.Input` 支持 `DecayDays`，但 `Service.Generate` 和 `Service.linkHeatResult` 都没有设置它：

```go
// internal/suggestions/service.go:562
func (m suggestionMetrics) heatInput() heat.Input {
    return heat.Input{
        Opens:              m.opens,
        Revisits:           m.revisits,
        AvgDurationMinutes: m.avgDurationMinutes,
        KeyPageViews:       m.keyPageViews,
        ForwardSignals:     m.uniqueVisitors,
        Downloads:          m.downloads,
        BouncePenalty:      m.bounces,
        // DecayDays 缺失！
    }
}
```

与此同时，`internal/analytics/service.go:362` 正确设置了 `DecayDays`。这导致：
- **信号模块与数据分析模块对同一 link 的热度打分不一致。**
- 一个曾经有过高活跃的 link 会长期保持 `hot`，持续触发 `hot_signal`。

### 🟠 P1：`risk_bounce` 阈值公式不合理

```yaml
condition: 'bounces > 0 && avgDurationMinutes / bounces < 0.5'
```

- `avgDurationMinutes` 是**每次 page_view 的平均停留**，`bounces` 是**没有 page_view 的访问次数**。两者量纲不同。
- 示例：10 次 bounce + 平均阅读 2 分钟 ⇒ `2/10 = 0.2 < 0.5`，触发风险。这很难说是“内容不匹配”。
- 更合理的条件应是 `bounceRate > threshold && avgDurationMinutes < threshold`。

### 🟠 P1：`link_features` 可能使用过期快照，且无 freshness 守卫

`Service.metrics()` 只要 `featureStore.GetForLink` 返回 `Found=true` 就直接使用，不检查 `updated_at`：

```go
// internal/suggestions/service.go:485
if s.featureStore != nil {
    if snap, err := s.featureStore.GetForLink(ctx, linkID); err == nil && snap.Found {
        return snap.toSuggestionMetrics(), nil
    }
}
```

`FeatureWorker` 只刷新“最近 1 小时有访问”的 link。若某 link 特征表里有 1 天前的快照，后续访问事件触发生成时，`link_features` 会被当作有效缓存使用，导致基于旧数据给出建议。

此外，`metrics()` 和 `behaviorFeatures()` 各查一次 `link_features`，两次可能读到不同窗口的快照。

### 🟡 P1：A/B 分桶当前不可解释

- `inBucket()` 在 `keyValue == ""` 时返回 `true`（`ruleengine.go:112`），对空 key 未做保护，可能让本不该进入桶的 link 被命中。
- `signal_rule_run` 没有记录：哪些规则因 bucket 被跳过、使用的 bucket seed/percent。
- 指标 `rulesEvaluatedTotal` 把“bucket 跳过”也计为 `matched=false`，无法单独分析 bucket 影响。
- 这会使得“为什么两个相似 link 一个有建议、一个没有”难以回溯。

### 🟡 P1：事件 Consumer 的“失败即停机”模式

`RedisBus.processMessage` 在 handler 返回 error 时不 ack，并且 `Subscribe` 会直接把 error 向上返回，导致 `ConsumerWorker.run` 退出：

```go
// internal/events/bus.go:117
if err := b.processMessage(ctx, msg, handler); err != nil {
    return err
}
```

`SignalConsumer.Handle` 在 5 次退避后返回 error。也就是说，**一个 workspace 的同步失败会让整个 signal consumer 停止消费**，影响所有 workspace。

### 🟡 P2：审计只记录成功，不记录失败

`CreateSignalRuleRun` 在 `Generate` 成功后才调用。所有错误（metrics 查询失败、规则求值 panic、数据库写入失败）都没有写入 `signal_rule_run.error`，也无法分析“哪些 link 一直无法生成建议”。

### 🟢 P2：Contact 选择策略过于简单

`Service.Generate` 取 `ListLinkContactsByLinkID` 的第一条 contact（无 ORDER BY），将其作为 suggestion 的关联 contact。多联系人场景下可能把 hot_signal 挂到未参与访问的联系人上。

### 🟢 P2：`avgDurationMinutes` 未在规则输入中做 heat 同款封顶

`heat.Compute` 会把 `AvgDurationMinutes` 封顶到 15，但传给规则的 `MetricsInput.AvgDurationMinutes` 是原始值。规则与热度打分看到的不一致。

---

## 3. 默认规则在当前数据下的典型误报场景

| 场景 | 触发规则 | 用户看到 | 实际含义 |
|---|---|---|---|
| Link 30 天前被下载过 1 次，之后再无活动 | `follow_up_download` | “24 小时内有 1 次下载，跟进下载内容” | 误报：不是最近行为 |
| Link 3 个月前有 5 次 bounce，现在偶尔打开 | `risk_bounce` | “5 次跳出，平均互动极短，检查内容是否匹配” | 误报：用历史 bounce 惩罚当前访问 |
| Link 上周热度很高，本周无人访问 | `hot_signal` | “高意向信号，立即联系” | 误报：未做时间衰减 |
| 同一办公室 3 个同事 1 小时内各自打开 | `risk_forward` | “3 个不同 IP 打开，检查未授权转发” | 可能误报：企业 NAT 下不同出口 IP |

---

## 4. 改进建议（分阶段）

### 4.1 短期（立即修复，预计 1-2 天）

1. **统一指标时间窗口**
   - 为 `GetLinkAccessMetrics`、`GetLinkPageViewMetrics`、`GetLinkBounceCount`、`GetLinkKeyPageViewMetrics` 增加 `window_days` 参数（建议默认 7 天或 30 天可配置）。
   - `follow_up_download` 改用 `downloads24h > 0`；`follow_up_revisit` 改用 24h/7d 的 `revisits`；`risk_bounce` 改用 24h/7d 的 bounces。
   - 修改文案，使其与数据窗口一致。

2. **修复热度衰减**
   - 在 `Service.Generate` 与 `Service.linkHeatResult` 中按 `analytics/service.go` 的方式传入 `DecayDays`。
   - 或把热度计算逻辑复用 `analytics.Service.getScoreForLink`。

3. **修复 `risk_bounce` 公式**
   - 改为 `bounceRate > 0.5 && avgDurationMinutes < 0.5` 之类基于“跳出率”的阈值；或直接 `bounces24h > 0 && avgDurationMinutes < 0.5`。

4. **`link_features` freshness 守卫**
   - `FeatureStore.GetForLink` 增加 staleness 阈值（如 15 分钟），过期视为未命中。
   - `Service.Generate` 内只读取一次 `FeatureSnapshot`，同时用于 `metrics()` 与 `behaviorFeatures()`。

### 4.2 中期（1-2 周）

5. **增强 A/B 可解释性**
   - `signal_rule_run` 新增 `bucket_skipped_rule_ids`、`bucket_percent`、`bucket_key` 字段。
   - 新增指标 `dealsignal_suggestion_rule_bucket_skips_total{rule_id, bucket_key}`。
   - Dashboard 展示按 rule/bucket 的命中率和用户 dismiss 率。

6. **事件 Consumer 可靠性**
   - 重试耗尽后 ack 并写入死信/延迟队列，而不是让 consumer 退出。
   - 增加 `dealsignal_signal_sync_errors_total` 与 consumer lag 监控。

7. **审计失败运行**
   - 在 `Generate` 的 defer 中写入 `signal_rule_run`，无论成功失败都记录 `error` 字段。

8. **用户反馈闭环**
   - 增加 `suggestion_feedback` 或扩展 `action_items` 状态，收集“dismissed without action / acted / spam”。
   - 用反馈数据计算每条规则的 precision / recall，驱动阈值调整。

### 4.3 长期（可选）

9. **规则影子模式（Shadow Mode）**
   - 新规则/阈值先以 `bucket_percent: 0` 或独立 `shadow_match` 指标运行，不生成 suggestion，只记录“本会命中”。
   - 与真实用户反馈对比后再上线。

10. **特征漂移监控**
    - 对比 `link_features` 与实时查询的差异分布，发现缓存偏差。
    - 监控 `avgDurationMinutes`、`downloads24h`、`distinctIPs1h` 的分布变化。

11. **从静态规则到轻量模型**
    - 当反馈数据足够后，可用 logistic regression / 简单树模型学习“用户是否采纳建议”，替代部分硬阈值规则。

---

## 5. 代码清单（需要重点修改的文件）

| 文件 | 修改点 |
|---|---|
| `internal/suggestions/default_rules.yaml` | 统一时间窗口、修正 `risk_bounce` 公式 |
| `internal/suggestions/service.go` | 传入 `DecayDays`、复用单次 FeatureSnapshot、失败审计 |
| `internal/suggestions/featurestore.go` | 增加 freshness 检查 |
| `internal/suggestions/ruleengine.go` | bucket 跳过审计、空 key 处理 |
| `internal/db/queries.sql` | 为 access/page_view metrics 增加时间窗口参数 |
| `internal/events/consumer.go` / `bus.go` | 失败不退出、引入死信/指标 |
| `internal/suggestions/metrics.go` | 增加 bucket skip / failure / feature drift 指标 |

---

## 6. 结论

当前 Signal & Risk 的**工程架构是合格的**：规则引擎可配置、生成异步化、去重维度细到 subtype、有审计表和事件总线。但**业务规则本身还不够准**：默认规则混用了“全量累积”和“最近 N 小时”指标，热度缺少衰减，缓存缺少 freshness 守卫。这些不是“算法不够智能”，而是**数据口径与规则语义不一致**。

建议优先做三件事：
1. 把所有规则指标统一到可配置的滚动窗口（24h/7d）。
2. 补上 `DecayDays`，让热度与数据分析模块一致。
3. 为 `link_features` 加 freshness 阈值，并在 `signal_rule_run` 里记录特征来源。

完成后再根据 `signal_rule_run` + 用户反馈数据做阈值 A/B 校准，逐步降低误报。

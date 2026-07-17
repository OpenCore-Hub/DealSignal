# Signal & Risk 模块 P0/P1/P2 完成进度

> 对应方案：`docs/tasks/phantom-stranger-catwoman-zatanna.md`  
> 最后更新：2026-07-17

---

## 总体状态

| 阶段 | 主题 | 状态 | 提交 |
|---|---|---|---|
| **P0** | 生产级修复（契约对齐、异步 suggestions、heat 修复） | ✅ 已完成 | `1e3a42e` |
| **P1** | 规则引擎替换硬编码候选生成 | ✅ 已完成 | `87ca2b3` |
| **P2** | 增量 signal 同步 + 可扩展配置 | ✅ 已完成 | `87ca2b3` |
| **P2 扩展** | 审计 / metrics / A/B 分桶 / 物化特征 / 事件总线 | ✅ 已完成 | 待提交 |

---

## P0 生产级修复 ✅

| 项 | 说明 | 状态 |
|---|---|---|
| `/signals` 字段补全 | `signal/presenter.go` 统一返回 `subtype`/`context`/`metadata` | ✅ |
| 异步 suggestion 生成 | `suggestion_outbox` + Worker（migration `070`），HTTP 不阻塞 | ✅ |
| 去重粒度细化 | 从 `(link, type)` 改为 `(link, type, subtype)` | ✅ |
| heat 修复 | `DecayDays` 以 `access_logs` 最后活动时间为准；`AvgDurationMinutes` 上限 15 | ✅ |
| 硬编码语言清理 | 消除 `signal.sync` 与 assistant 中的 `Lang: "en"` | ✅ |

---

## P1 规则引擎 ✅

| 项 | 说明 | 状态 |
|---|---|---|
| 引入 `expr-lang/expr` | `go.mod` 新增 `github.com/expr-lang/expr v1.17.8` | ✅ |
| `RuleEngine` | `internal/suggestions/ruleengine.go`：YAML 配置 + 表达式求值 | ✅ |
| 默认规则 | `internal/suggestions/default_rules.yaml`（内嵌 fallback） | ✅ |
| 外部规则配置 | `apps/api/config/signal_rules.yaml` 可由业务方调整 | ✅ |
| 启动加载与注入 | `server/routes.go` 加载并注入 `suggestions.Service` | ✅ |
| 移除旧硬编码规则 | 删除 `buildCandidates`/`buildSecurityCandidates`/`buildBehaviorRiskCandidates` | ✅ |
| 规则引擎测试 | `service_test.go` / `ruleengine_test.go` 覆盖 hot/bounce/forward/security/bucket | ✅ |

---

## P2 增量 signal 同步 ✅

| 项 | 说明 | 状态 |
|---|---|---|
| `suggestions.synced_at` | migration `071_suggestion_synced_at` | ✅ |
| 增量查询 | `ListUnsyncedSuggestionsByWorkspace` + 部分索引 | ✅ |
| 批量标记同步 | `MarkSuggestionsSynced` | ✅ |
| 幂等创建 | `GetSignalBySuggestion` + `CreateSignal` fallback | ✅ |
| `SIGNAL_RULES_PATH` | `config.go` 默认 `config/signal_rules.yaml` | ✅ |

---

## P2 扩展（生产上线标准）✅

### Metrics 埋点

| 项 | 说明 | 状态 |
|---|---|---|
| Suggestion metrics | `dealsignal_suggestion_rules_evaluated_total` / `dealsignal_suggestion_generation_duration_seconds` / `dealsignal_suggestions_generated_total` / `dealsignal_suggestion_generation_errors_total` | ✅ |
| Signal metrics | `dealsignal_signal_sync_duration_seconds` / `dealsignal_signals_synced_total` | ✅ |
| 挂载点 | 复用现有 `/metrics` endpoint | ✅ |

### `signal_rule_run` 审计表

| 项 | 说明 | 状态 |
|---|---|---|
| migration `072` | `signal_rule_run` 表 + 索引 | ✅ |
| sqlc 查询 | `CreateSignalRuleRun` | ✅ |
| 写入点 | `suggestions.Service.Generate` 完成后记录输入/命中规则/生成 suggestions/耗时 | ✅ |

### A/B 分桶 / kill switch

| 项 | 说明 | 状态 |
|---|---|---|
| 规则字段 | `enabled` / `bucket_percent` / `bucket_key`（link_id / workspace_id / tenant_id） | ✅ |
| 确定性分桶 | FNV-1a 哈希 `rule_id:key`，默认 100% | ✅ |
| 校验 | `RuleConfig.validate` | ✅ |
| 测试 | `ruleengine_test.go` 覆盖 disabled / bucket / workspace key | ✅ |

### `link_features` 物化特征表

| 项 | 说明 | 状态 |
|---|---|---|
| migration `073` | `link_features` 表 + 索引 | ✅ |
| sqlc 查询 | `UpsertLinkFeature` / `GetLinkFeature` / `ListStaleLinkFeatures` / `ListRecentlyActiveLinkIDs` / `GetLinkByID` | ✅ |
| FeatureStore | `internal/suggestions/featurestore.go`：聚合 + upsert + read + fallback | ✅ |
| FeatureWorker | 每 5 分钟刷新最近活跃 link 特征 | ✅ |
| Service 接入 | `metrics()` / `behaviorFeatures()` 优先读 `link_features` | ✅ |

### 事件总线

| 项 | 说明 | 状态 |
|---|---|---|
| Bus 抽象 | `internal/events/bus.go`：`Publisher` / `Subscriber` / `RedisBus` / `NoOpBus` | ✅ |
| 事件 | `suggestion.generated` | ✅ |
| Publisher | `suggestions.Worker.processJob` 成功后发布 | ✅ |
| Consumer | `events.SignalConsumer` 触发 `signal.Service.SyncWorkspace`，指数退避 | ✅ |
| Worker 包装 | `events.ConsumerWorker` 接入 server lifecycle | ✅ |
| 降级 | Redis 不可用时自动降级为 NoOpBus，保留轮询兜底 | ✅ |

---

## 配置项

| 配置项 | 默认值 | 说明 |
|---|---|---|
| `METRICS_ENABLED` | `true` | 已存在；新 metrics 自动挂载到 `/metrics` |
| `SIGNAL_RULES_PATH` | `config/signal_rules.yaml` | 已存在 |
| `FEATURE_WORKER_ENABLED` | `true` | 是否启动 link_features 刷新 worker |
| `FEATURE_WORKER_INTERVAL_MINUTES` | `5` | 刷新间隔 |
| `EVENTS_ENABLED` | `true` | 是否启用 Redis Streams 事件总线 |
| `EVENTS_STREAM_NAME` | `events:signal` | Redis stream key |
| `EVENTS_CONSUMER_GROUP` | `signal-sync` | consumer group |

---

## 验证结果

- `go test ./...` ✅ 全绿
- `go vet ./...` ✅ 通过
- `pnpm lint` ✅ 通过
- `pnpm typecheck` ✅ 通过
- `pnpm test` ✅ 461 passed

---

## 后续可选（非阻塞）

1. 为 `signal_rule_run` 增加只读 API / dashboard 调试面板。
2. 为 `link_features` 增加历史窗口（按小时/天）支持趋势分析。
3. 事件总线增加 dead-letter stream 与监控告警。
4. 规则配置支持按 workspace 覆盖。

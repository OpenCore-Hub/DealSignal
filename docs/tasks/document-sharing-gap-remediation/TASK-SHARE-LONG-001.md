---
task_id: "TASK-SHARE-LONG-001"
parent_issue: "DS-SHARE-010"
agent_task_id: "AGENT-TASK-SHARE-010"
version: "v1.0.0"
priority: "P2"
status: "已完成"
type: "backend"
effort: "M"
branch: "feat/share-long-001-heat-score-decay"
estimated_files: "10"
max_lines: "500"
project_stack: "Go 1.25 + PostgreSQL"
ai_red_flags:
  - "时间衰减不能导致历史高分突然归零"
  - "权重配置变更后需重新计算历史评分"
  - "A/B 实验必须可追踪分组与效果"
  - "不得破坏现有 Dashboard/Insights API 契约"
ai_confidence: "medium"
pending_confirmation:
  - "时间衰减函数：指数衰减还是阶梯衰减？"
  - "A/B 权重实验是否需要在 DB 中记录分组？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-LONG-001 Heat Score 时间衰减与权重校准

> **父 Issue**：`DS-SHARE-010`  
> **版本**：`v1.0.0`  
> **优先级**：`P2`  
> **类型**：`backend`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-long-001-heat-score-decay`

---

## 1. 目标

增强 Heat Score 算法：
- 引入时间衰减函数，使近期行为比旧行为权重更高。
- 支持 per-circle 权重配置，便于 A/B 实验与业务调优。
- 保持现有 API 响应格式不变。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.4 |
| 算法文档 | `docs/backup/HEAT-SCORE-ALGORITHM-v2.1.1.md` §5 / §8 |

### 2.1 已有代码

- `apps/api/internal/heat/score.go` — 规则评分
- `apps/web/src/lib/heat/heatScore.ts` — 前端镜像

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 衰减函数 | 指数衰减 `e^(-λt)` | t 为事件距今天数 |
| 半衰期 | 7 天 | 7 天前事件权重减半 |
| 权重配置 | per-circle DB 表 | founder/investor_ir/sales 可独立调整 |
| A/B 分组 | 按 workspace_id hash | 或按显式实验配置 |
| 兼容性 | API 不变 | 仅 score 值变化 |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 全部事件很旧 | 最近事件 90 天前 | score 显著降低但不归零 |
| 权重和不为 100 | 配置错误 | 归一化处理或返回配置错误 |
| 无事件 | 新 link 无访问 | score 为 0，level 为 cold |
| 实验分组未知 | workspace 不在实验范围 | 使用默认权重 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/0XX_heat_score_config.up.sql` | 新增 | `heat_score_configs` 表 |
| `apps/api/internal/heat/config.go` | 新增 | 权重配置加载与 A/B 分组 |
| `apps/api/internal/heat/score.go` | 修改 | 引入时间衰减与可配置权重 |
| `apps/api/internal/analytics/service.go` | 修改 | 计算 score 时按事件时间加权 |
| `apps/api/internal/db/queries.sql` | 修改 | 聚合查询返回带时间的事件指标 |
| `apps/api/internal/heat/score_test.go` | 修改 | 补衰减与权重测试 |

### 4.2 行为定义

- `heat.Compute` 接收按天分布的事件数据而非简单计数。
- 每个事件类型按时间衰减后求和。
- per-circle 权重从 `heat_score_configs` 读取，支持实验分组。

---

## 5. 验收标准

- [ ] Heat Score 引入时间衰减函数。
- [ ] per-circle 权重可配置。
- [ ] A/B 实验框架可切换不同权重配置。
- [ ] API 响应格式不变，仅 score 值按新算法计算。
- [ ] 单元测试覆盖衰减与权重边界。

---

## 6. 实现步骤建议

1. 创建 `heat_score_configs` 表与默认配置。
2. 修改聚合查询，返回事件时间分布。
3. 在 `heat` 包实现 `decayValue(value, days)`。
4. 修改 `Compute` 使用衰减后指标与可配置权重。
5. 增加 A/B 分组逻辑。
6. 补测试。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/heat/...
make lint
```

---

## 8. 约束与红线

- 不得改变 `GetScore` / `DashboardStats` / `InsightsOverview` 的响应字段。
- 衰减函数必须单调递减且收敛到 0。
- 权重配置必须验证非负且总和合理。
- A/B 分组必须可追踪、可回滚。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-010`

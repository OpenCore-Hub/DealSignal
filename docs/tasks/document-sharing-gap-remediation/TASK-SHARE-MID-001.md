---
task_id: "TASK-SHARE-MID-001"
parent_issue: "DS-SHARE-005"
agent_task_id: "AGENT-TASK-SHARE-005"
version: "v1.0.0"
priority: "P1"
status: "已完成"
type: "backend"
effort: "M"
branch: "feat/share-mid-001-key-page-views"
estimated_files: "10"
max_lines: "500"
project_stack: "Go 1.25 + Gin + PostgreSQL"
ai_red_flags:
  - "关键页关键词必须与算法文档一致"
  - "关键词匹配不得影响已有非关键页评分逻辑"
  - "新增配置必须可 per-circle 定制"
  - "避免在 SQL 中使用复杂正则影响性能"
ai_confidence: "medium"
pending_confirmation:
  - "关键页关键词来源：document.title、page 元数据、还是 OCR 文本？"
  - "是否保留‘停留 ≥3s’作为 key page view 的兜底？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-MID-001 后端 Key Page Views 语义修正

> **父 Issue**：`DS-SHARE-005`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **类型**：`backend`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-mid-001-key-page-views`

---

## 1. 目标

将后端的 `key_page_views` 统计从“任意页面停留 ≥3 秒”修正为“按关键词匹配的关键页”，与设计文档 `HEAT-SCORE-ALGORITHM-v2.1.1.md` §5 对齐。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.4 |
| 算法文档 | `docs/backup/HEAT-SCORE-ALGORITHM-v2.1.1.md` §5 |

### 2.1 已有代码

- `apps/api/internal/db/queries.sql` — `GetLinkPageViewMetrics`
- `apps/api/internal/heat/score.go` — Heat Score 算法
- `apps/web/src/lib/heat/heatScore.ts` — 前端 topKeyPages 展示

### 2.2 当前缺陷

```sql
COUNT(*) FILTER (WHERE duration_seconds >= 3) AS key_page_views
```

后端把任意 ≥3s 的页面都当作 key page view，未识别财务/团队/价格/安全等关键页。

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 关键词来源 | page 标题 / 文档元数据 | 首期可优先用 title |
| 关键词集合 | per-circle | founder / investor_ir / sales 可不同 |
| 匹配方式 | 不区分大小写，子串匹配 | 如 `financial`, `revenue`, `traction` |
| 兜底规则 | 停留 ≥3s | 可作为“感兴趣页”但不直接等同于 key page |

### 3.2 关键页关键词示例（founder 圈）

| 类别 | 关键词 |
|---|---|
| 财务 | `financial`, `revenue`, `unit economics`, `run rate` |
| 团队 | `team`, `founders`, `advisors` |
| 价格 | `pricing`, `plan`, `cost` |
| 安全 | `security`, `compliance`, `privacy` |

### 3.3 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 普通页停留 5s | 第 2 页标题 "Appendix" | 不计为 key page view |
| 财务页停留 1s | 第 5 页标题 "Financials" | 仍计为 key page view（关键词优先） |
| 标题为空 | page 无标题 | 不计为 key page view |
| 大小写不敏感 | 标题 "FINANCIALS" | 匹配成功 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/heat/keypages.go` | 新增 | 关键词匹配逻辑与 per-circle 配置 |
| `apps/api/internal/heat/score.go` | 修改 | `Input` 增加 `KeyPageViews` 字段 |
| `apps/api/internal/db/queries.sql` | 修改 | `GetLinkPageViewMetrics` 返回 `key_page_views` 基于关键词 |
| `apps/api/internal/analytics/service.go` | 修改 | 计算 key page views 时调用关键词匹配 |
| `apps/api/internal/config/config.go` | 修改 | 支持关键页关键词配置 |
| `apps/api/internal/heat/score_test.go` | 修改 | 补关键词匹配测试 |

### 4.2 行为定义

- `GetLinkPageViewMetrics` 返回两个指标：
  - `key_page_views`：按关键词匹配的关键页 view 数。
  - `engaged_page_views`：停留 ≥3s 的 page view 数（原 key_page_views 语义，保留用于兼容）。
- Heat Score 使用新的 `key_page_views`。

---

## 5. 验收标准

- [ ] 后端 `key_page_views` 按关键词匹配计算。
- [ ] per-circle 关键词可配置。
- [ ] Heat Score 使用修正后的 key page views。
- [ ] 原有 `duration_seconds >= 3` 指标保留为 `engaged_page_views`。
- [ ] 单元测试覆盖关键词匹配与 score 计算。

---

## 6. 实现步骤建议

1. 定义 per-circle 关键词配置结构。
2. 新增 `heat.IsKeyPage(title, circle)` 判断函数。
3. 修改查询：JOIN `documents` / `pages` 表获取 page title（或从现有元数据）。
4. 在 `analytics.Service` 中计算 key page views。
5. 修改 Heat Score 输入与计算。
6. 补测试。

---

## 7. 测试验证

```bash
cd apps/api
go test ./internal/heat/...
go test ./internal/analytics/...
make lint
```

---

## 8. 约束与红线

- 不得改变现有 `page_views` 表结构（除非必要）。
- 关键词匹配必须在后端完成，不能仅依赖前端。
- 新增指标命名避免与旧 `key_page_views` 混淆；如改名需同步前端。
- 性能：避免每行 page view 做复杂正则；优先考虑 title 索引或预计算标签。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-005`

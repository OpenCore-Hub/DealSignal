---
task_id: "TASK-SHARE-LONG-005"
parent_issue: "DS-SHARE-014"
agent_task_id: "AGENT-TASK-SHARE-014"
version: "v1.0.0"
priority: "P3"
status: "待执行"
type: "ai"
effort: "XL"
branch: "feat/share-long-005-predictive-lead-scoring"
estimated_files: "20"
max_lines: "1200"
project_stack: "Go 1.25 + PostgreSQL + Python/ML（可选）"
ai_red_flags:
  - "模型必须有可解释性，不能黑盒"
  - "训练数据必须排除测试/内部账号"
  - "模型预测必须有置信区间"
  - "必须防止数据泄漏（未来信息用于预测过去）"
ai_confidence: "low"
pending_confirmation:
  - "是否引入独立 Python ML 服务，还是 Go 内嵌轻量模型？"
  - "转化标签如何定义：注册？成交？回复邮件？"
available_tools:
  - "test"
  - "lint"
---

# TASK-SHARE-LONG-005 预测性 Lead Scoring

> **父 Issue**：`DS-SHARE-014`  
> **版本**：`v1.0.0`  
> **优先级**：`P3`  
> **类型**：`ai`  
> **预计工作量**：`XL`  
> **分支名**：`feat/share-long-005-predictive-lead-scoring`

---

## 1. 目标

基于历史转化数据训练预测模型，为每个 contact / link 输出成交概率（0-100），替代或补充规则驱动的 Heat Score。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.4 |
| PRD | `docs/backup/PRD-v2.1.0.md` FR-11 |

---

## 3. 输入

### 3.1 特征

| 特征 | 来源 |
|---|---|
| 打开次数、时间分布 | access_logs |
| 页面停留时长、关键页占比 | page_views |
| 下载、转发、回访 | access_logs + page_views |
| AI 问题主题与紧迫度 | assistant_message_intents |
| 联系人属性 | contacts |
| 历史转化标签 | 业务定义 |

### 3.2 标签

- `converted`： true/false，定义需业务确认（如回复邮件、进入谈判、签约）。

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/ml/` 或 `apps/api/internal/ml/` | 新增 | 模型训练与预测服务 |
| `apps/api/internal/db/migrations/0XX_predicted_scores.up.sql` | 新增 | `predicted_scores` 表 |
| `apps/api/internal/contact/service.go` | 修改 | 返回预测分数 |
| `apps/api/internal/analytics/service.go` | 修改 | Dashboard 展示预测分数 |
| `apps/web/src/components/dashboard/...` | 修改 | UI 展示预测分数与解释 |

### 4.2 行为定义

- 模型定期重训练（如每周）。
- 预测分数实时或近实时更新。
- Dashboard/Contact 详情展示分数与 top 影响因素。

---

## 5. 验收标准

- [ ] 模型 AUC-ROC ≥ 0.75（基于历史数据验证）。
- [ ] 预测分数与 top 特征解释返回给前端。
- [ ] 模型训练与预测流程自动化。
- [ ] 测试结果可复现。

---

## 6. 实现步骤建议

1. 定义转化标签与特征工程。
2. 选择模型（如 XGBoost / LightGBM）。
3. 构建训练 pipeline。
4. 部署预测服务。
5. 集成到 contact/analytics API。
6. 前端展示与解释。
7. 持续监控模型效果。

---

## 7. 测试验证

```bash
# 取决于 ML 服务实现
cd apps/ml
pytest

cd apps/api
go test ./internal/contact/...
make lint
```

---

## 8. 约束与红线

- 必须使用交叉验证，防止过拟合。
- 训练数据必须清洗测试账号与内部用户。
- 模型必须有可解释性（SHAP 或特征重要性）。
- 预测服务失败不得影响主业务 API。

---

## 9. Definition of Done

- [ ] 模型训练 pipeline 完成
- [ ] 预测服务集成完成
- [ ] 测试通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-014`

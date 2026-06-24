---
task_id: "TASK-DOCS-001"
parent_issue: "DS-039"
agent_task_id: "AGENT-TASK-039"
version: "v2.1.3"
priority: "P1"
status: "待执行"
type: "docs"
effort: "M"
branch: "feat/agent-task-039-docs-sync"
estimated_files: "8"
max_lines: "600"
project_stack: "Markdown / OpenAPI / dbdocs / README"
ai_red_flags:
  - "文档变更必须有代码侧对应修改或明确说明"
  - "不得凭空编造字段/枚举/路径"
  - "API-SPEC 示例必须与真实响应一致"
  - "敏感数据不得发送给 LLM"
ai_confidence: "high"
pending_confirmation:
  - "API-SPEC 是复用 v2.1.0 还是升级到 v2.1.1/v2.1.3"
  - "数据库模型文档是否随迁移脚本一并刷新"
available_tools:
  - "lint"
  - "browse"
---

# TASK-DOCS-001 v2.1.3 文档基线同步

> **父 Issue**：`DS-039`

---

## 1. 目标

同步 v2.1.3 代码现状与文档基线，确保 API-SPEC、database-model、ARCHITECTURE、README、PROJECT-PROGRESS、CHANGELOG 准确反映当前实现。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 前端审计计划 | `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md` |
| 一致性评审 | `docs/reviews/PRD-TDD-API-PLAN-TASK-consistency-review-v2.1.2.md` |
| 父 Issue | `DS-039` |
| 依赖 | 功能开发完成 |

### 2.1 已有代码/文档

- `docs/API-SPEC-v2.1.0.md`
- `docs/database-model-v2.1.0.md`
- `docs/ARCHITECTURE-v2.1.0.md`
- `docs/README.md`
- `docs/PROJECT-PROGRESS.md`
- `docs/CHANGELOG.md`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| API 路径 | 与代码一致 | `/api` 短期兼容，`/{ws}/api/v1/*` 长期 |
| 错误码 | 统一全大写 SNAKE_CASE | 与后端返回一致 |
| Schema | 与迁移脚本一致 | 新增 `allowed_domains`、`contacts` 等 |
| README | 使用 `pnpm` | 替换 npm 命令 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `docs/API-SPEC-v2.1.0.md` | 修改 | 补齐 Auth/Workspace/Contacts/缺失端点；统一错误码/响应格式/路径 |
| `docs/database-model-v2.1.0.md` | 修改 | 新增字段、中间件/基础模块相关表、枚举统一 |
| `docs/ARCHITECTURE-v2.1.0.md` | 修改 | 新增中间件、logger/mailer/redis 位置、ERD 更新 |
| `docs/README.md` | 修改 | `pnpm install`、补充 v2.1.3 文档链接 |
| `docs/PROJECT-PROGRESS.md` | 修改 | 随任务完成更新状态 |
| `docs/CHANGELOG.md` | 修改 | 补充 v2.1.3 unreleased 条目 |
| `docs/openapi-v2.1.0.yaml`（可选） | 新增/修改 | 若存在则同步 |

---

## 5. 验收标准

- [ ] API-SPEC 路径/响应/错误码/枚举与代码一致。
- [ ] database-model 与迁移脚本 013~016 一致。
- [ ] ARCHITECTURE 包含新中间件与基础模块。
- [ ] README 使用 pnpm，链接到 v2.1.3 计划。
- [ ] PROJECT-PROGRESS 中 v2.1.3 任务状态更新。
- [ ] CHANGELOG 有 v2.1.3 草稿条目。

---

## 6. 实现步骤建议

1. 整理代码侧变更清单（字段、路径、枚举、新增模块）。
2. 更新 API-SPEC。
3. 更新 database-model。
4. 更新 ARCHITECTURE。
5. 更新 README / PROJECT-PROGRESS / CHANGELOG。
6. 交叉检查前后端实现与文档是否一致。

---

## 7. 测试验证

```bash
# 文档无代码测试，主要做链接与一致性检查
cd /Users/mg/Workspace/DealSignal
# 检查死链（如有 markdown-link-check 可用）
# npx markdown-link-check docs/API-SPEC-v2.1.0.md
```

---

## 8. 约束与红线

- 不得编造未实现的接口或字段。
- 文档中的示例响应必须从真实 handler/service 中提取。
- 不要遗漏新增迁移脚本的字段说明。

---

## 9. Definition of Done

- [ ] 文档更新完成
- [ ] 与代码实现交叉检查通过
- [ ] PR 已关联父 Issue：`Closes #DS-039`

---

## 10. Agent 备注

- 可在功能开发分支合并后，按 diff 逐项更新文档，避免过早同步返工。
- 建议使用 `grep` 或 `git diff` 提取变更字段清单作为文档更新输入。

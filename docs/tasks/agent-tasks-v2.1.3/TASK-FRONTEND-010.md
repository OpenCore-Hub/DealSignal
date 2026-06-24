---
task_id: "TASK-FRONTEND-010"
parent_issue: "DS-032"
agent_task_id: "AGENT-TASK-032"
version: "v2.1.3"
priority: "P0"
status: "待执行"
type: "frontend"
effort: "S"
branch: "feat/agent-task-032-heatscore-topkeypages"
estimated_files: "5"
max_lines: "200"
project_stack: "React 19 + TypeScript + Vite 8"
ai_red_flags:
  - "不得改变 heatScore 对外接口签名"
  - "算法实现必须与 HEAT-SCORE-ALGORITHM 文档一致"
  - "新增/修改必须伴随单元测试"
  - "敏感数据不得发送给 LLM"
ai_confidence: "high"
pending_confirmation:
  - "topKeyPages 输出数量上限（默认 3 或 5）"
available_tools:
  - "test"
  - "lint"
---

# TASK-FRONTEND-010 heatScore topKeyPages 算法对齐

> **父 Issue**：`DS-032`

---

## 1. 目标

按 `docs/HEAT-SCORE-ALGORITHM-v2.1.1.md` 修复 `lib/heat/heatScore.ts` 的 `topKeyPages`：从「页码字符串匹配」改为基于「页面文本/标题关键词相似度」匹配。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 热度算法 | `docs/HEAT-SCORE-ALGORITHM-v2.1.1.md` §3.1 / §9.x |
| 前端审计计划 | `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md` §2.2 / §4.7 |
| 父 Issue | `DS-032` |

### 2.1 已有代码

- `apps/web/src/lib/heat/heatScore.ts:136-143`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 匹配依据 | 页面文本/标题 vs 关键词 | 不再使用 `pageNumber` 字符串 |
| 相似度阈值 | ≥ 0.3 | 与算法文档一致（默认 500 字符文本） |
| 输出上限 | 3 ~ 5 | 由调用方决定，默认 3 |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 关键词与页面标题高度相关 | `page.title` 含关键词 | 该页进入 topKeyPages |
| 关键词与页面内容相关 | `page.text` 含关键词 | 该页进入 topKeyPages |
| 关键词与页面无关 | 无匹配 | 不进入 topKeyPages |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/lib/heat/heatScore.ts` | 修改 | 修复 topKeyPages 匹配逻辑 |
| `src/lib/heat/heatScore.test.ts`（或新增） | 新增 | 覆盖 topKeyPages 正常/边界/异常 |

---

## 5. 验收标准

- [ ] `topKeyPages` 基于页面文本/标题关键词相似度，而非页码字符串。
- [ ] 新增单元测试覆盖匹配、阈值、上限。
- [ ] 不破坏 `computeHeatScore` 现有调用。
- [ ] `pnpm test heatScore` 全绿。

---

## 6. 实现步骤建议

1. 阅读 `HEAT-SCORE-ALGORITHM-v2.1.1.md` 关键词匹配章节。
2. 在 `heatScore.ts` 中实现文本相似度函数（可简化使用关键词出现次数/余弦相似度）。
3. 替换 `topKeyPages` 实现。
4. 补充测试。

---

## 7. 测试验证

```bash
cd apps/web
pnpm test heatScore
pnpm lint
```

---

## 8. 约束与红线

- 不得修改 `computeHeatScore` 的输入/输出接口。
- 不得引入外部 NLP 库；优先用轻量本地实现。
- 必须补测试。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-032`

---

## 10. Agent 备注

- 如果输入页面缺少 `text` 字段，可用 `title` 回退。
- 相似度可用简单关键词命中数 / 总词数，不必实现完整 TF-IDF。

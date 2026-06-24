---
task_id: "TASK-FRONTEND-013"
parent_issue: "DS-035"
agent_task_id: "AGENT-TASK-035"
version: "v2.1.3"
priority: "P1"
status: "待执行"
type: "test"
effort: "M"
branch: "feat/agent-task-035-frontend-tests"
estimated_files: "10"
max_lines: "500"
project_stack: "React 19 + TypeScript + Vite 8 + Vitest + React Testing Library + MSW"
ai_red_flags:
  - "测试数据必须使用 .test 域名或明确 fixture"
  - "不得在生产代码中为测试增加额外分支"
  - "不得测试实现细节，优先测试行为"
  - "敏感数据不得发送给 LLM"
ai_confidence: "high"
pending_confirmation:
  - "Vitest 覆盖率阈值目标（当前是否设为 70%）"
available_tools:
  - "test"
  - "lint"
---

# TASK-FRONTEND-013 前端单元与组件测试补强

> **父 Issue**：`DS-035`

---

## 1. 目标

为核心工具函数与关键组件补充单元/组件测试，降低 v2.1.3 改动回归风险。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 前端审计计划 | `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md` §2.3 / §3 Phase C |
| TDD | `docs/TDD-v2.1.0.md` §10 |
| 父 Issue | `DS-035` |
| 依赖 | `DS-031`（TASK-FRONTEND-009）、`DS-033`（TASK-FRONTEND-011） |

### 2.1 已有代码

- `apps/web/src/lib/apiClient.test.ts`（新增未落库）
- `apps/web/src/lib/heat/heatScore.ts`
- `apps/web/src/lib/formatters.ts`
- `apps/web/vitest.config.ts`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 测试框架 | Vitest + RTL | 与现有一致 |
| 覆盖率 | 核心函数 ≥ 80% | heatScore/formatters/calculations/apiClient |
| MSW | 测试用 server setup | 支持 API 契约测试 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/lib/heat/heatScore.test.ts` | 新增 | topKeyPages、趋势、边界 |
| `src/lib/formatters.test.ts` | 新增 | 日期、文件大小、事件标签 |
| `src/lib/calculations.test.ts` | 新增 | 日趋势聚合、热度分布 |
| `src/lib/apiClient.test.ts` | 修改 | 完善 FormData/BaseResponse/错误 |
| `src/hooks/useAsyncData.test.ts` | 新增 | hook 状态机 |
| `src/components/common/EmptyState.test.tsx` | 新增 | 空状态渲染 |
| `src/components/layout/TopNav.test.tsx` | 新增 | 账户菜单、铃铛提示 |
| `src/components/links/SmartLinkCreator.test.tsx` | 新增 | 复制反馈 |
| `vitest.config.ts` | 修改 | 调整 coverage 阈值 |

---

## 5. 验收标准

- [ ] `heatScore.ts`、`formatters.ts`、`calculations.ts` 单元测试覆盖 ≥ 80%。
- [ ] `apiClient.ts` 测试覆盖 FormData、BaseResponse、错误处理。
- [ ] 至少为 TopNav、SmartLinkCreator、EmptyState 补组件测试。
- [ ] 测试全绿：`pnpm test`。
- [ ] 无 `act()` 警告。

---

## 6. 实现步骤建议

1. 配置 Vitest MSW server setup。
2. 为工具函数写单元测试。
3. 为 `useAsyncData` 写 hook 测试。
4. 为关键组件写 RTL 测试。
5. 调整 coverage 阈值并验证。

---

## 7. 测试验证

```bash
cd apps/web
pnpm test --run
pnpm test --coverage
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 测试数据不得使用真实用户/域名。
- 不要测试私有函数；优先通过公开接口验证行为。
- 不要为无法稳定渲染的动画组件写脆弱断言。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过且 coverage 达标
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-035`

---

## 10. Agent 备注

- 可参考现有 `WorkspaceSwitcher.test.tsx` 的写法，但注意修复 `act()` 警告。
- 对 MSW handler 的修改应先写测试，再改实现。

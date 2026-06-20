---
task_id: "TASK-BACKEND-042"
parent_issue: "EXORG-031"
agent_task_id: "AGENT-042"
version: "v1.0.0"
priority: "P1"
status: "已完成"
type: "backend"
effort: "M"
branch: "feat/agent-042-task-analytics"
estimated_files: "5"
max_lines: "300"
project_stack: "TypeScript / Node.js / pnpm / React / PostgreSQL / Docker"
ai_red_flags:
  - "不得硬编码示例域名/邮箱/密码"
  - "不得修改范围外文件"
  - "不得破坏现有测试"
  - "敏感数据不得发送给 LLM"
ai_confidence: "high"
pending_confirmation:
  - "确认 task 分析查询是否支持按 organization 级联过滤"
---

> 本文件为示例，仅用于展示如何填写对应模板。实际项目中请替换为真实内容。

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-BACKEND-042` |
> | `parent_issue` | `EXORG-031` |
> | `agent_task_id` | `AGENT-042` |
> | **版本** | `v1.0.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P1` |
> | **状态** | `已完成` |
> | **类型** | `backend` |
> | **预计工作量** | `M` |
> | **分支名** | `feat/agent-042-task-analytics` |
> | **预计修改文件数上限** | `6` |
> | **建议最大变更行数** | `350` |
> | **项目技术栈约束** | `TypeScript / Node.js / pnpm / React / PostgreSQL / Docker` |

# TASK-BACKEND-042 实现 Task 任务分析查询 API

> **父 Issue**：`EXORG-031`  
> **版本**：`v1.0.0`  
> **模板版本**：`v2`  
> **优先级**：`P1`  
> **状态**：`已完成`  
> **类型**：`backend`  
> **预计工作量**：`M`  
> **分支名**：`feat/agent-042-task-analytics`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

实现 `GET /api/v1/tasks/:taskId/analytics` 接口，从聚合表读取 Task 的任务分析数据并返回，包含权限校验与单元测试。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v1.0.0.md FR-02` |
| TDD | `docs/TDD-v1.0.0.md 4.2` |
| API 契约 | `docs/openapi-v1.0.0.yaml` |
| 父 Issue | `#EXORG-031` |

---

## 3. 输入

### 3.1 已有代码

- `apps/api/src/routes/tasks.ts`
- `apps/api/src/services/task.ts`
- `packages/shared/types/task.ts`

### 3.2 数据模型

```typescript
interface TaskAnalytics {
  taskId: string;
  totalViews: number;
  uniqueVisitors: number;
  taskStats: PageStat[];
}

interface PageStat {
  pageId: string;
  avgDurationMs: number;
  views: number;
}
```

---

## 4. 输出

### 4.1 新增/修改文件

| 文件 | 说明 |
|------|------|
| `apps/api/src/routes/tasks.ts` | 新增 `GET /:taskId/analytics` handler |
| `apps/api/src/services/analytics.ts` | 新增查询聚合表逻辑 |
| `apps/api/src/db/migrations/024_task_analytics.sql` | 新增聚合表 |
| `apps/api/src/services/analytics.test.ts` | 单元测试 |

### 4.2 完成标准

- [ ] API 返回结构符合 OpenAPI 契约
- [ ] 非 organization 成员访问返回 403
- [ ] 单元测试覆盖率 ≥ 80%
- [ ] 数据库查询使用索引

---

## 5. 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 权限 | 仅 task 所属 organization 成员可访问 | 否则返回 403 |
| 数据不存在 | task 无分析数据 | 返回 200，字段为 0 |
| 分页 | 不分页，taskStats ≤ 100 条 | 超过则截断 |
| 响应时间 | P99 ≤ 200ms | 通过索引保证 |

---

## 6. 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 未认证请求 | 无 Authorization | 返回 401 unauthorized |
| 越权访问 | 非 organization 成员 token | 返回 403 forbidden |
| 资源不存在 | `taskId` 不存在 | 返回 404 task_not_found |


---

## 7. 测试验证

### 7.1 单元测试

```bash
pnpm test --filter=api -- task-analytics-service.test.ts
```

### 7.2 集成测试

```bash
pnpm test:integration -- task-analytics.integration.test.ts
```

### 7.3 手动验证

```bash
curl -X GET "https://api.example.com/api/v1/tasks/sl-001/analytics" \
  -H "Authorization: Bearer <token>"
```

---

## 8. 约束与红线

- 修改文件数不超过 5，变更行数建议不超过 300。
- 不得修改范围外文件；如必须修改，需在 Agent 备注中说明。
- 测试数据必须使用 `.test` 域名或明确标识为 fixture。
- 敏感数据不得发送给 LLM。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

| 检查项 | 自检要求 |
|--------|----------|
| 无硬编码示例域名/邮箱/密码 | 测试账号使用 `user-{n}@example.test` |
| 无未清理的 TODO / FIXME / placeholder | 实现完成后全局搜索并清除 |
| 未擅自实现范围外功能 | 严格按第 4 节文件列表实现 |

---

## 10. Definition of Done

- [ ] 代码实现完成
- [ ] 单元测试与集成测试通过
- [ ] lint / typecheck 通过
- [ ] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- PostgreSQL 查询注意时间范围索引。
- 热度评分规则后续可能 A/B 测试，建议将权重抽成配置。

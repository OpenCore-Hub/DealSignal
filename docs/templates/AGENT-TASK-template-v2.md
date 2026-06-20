---
task_id: "{TASK-BACKEND-001}"
parent_issue: "{项目前缀}-NNN"
agent_task_id: "{AGENT-TASK-NNN}"
version: "{v0.1.0}"
priority: "{P0}"
status: "{待执行}"
type: "{backend}"
effort: "{S}"
branch: "{feat/agent-task-NNN-short-name}"
estimated_files: "{8}"
max_lines: "{400}"
project_stack: "{TypeScript / Node.js / pnpm / React / PostgreSQL / Docker}"
ai_red_flags:
  - "{不得硬编码示例域名/邮箱/密码}"
  - "{不得修改范围外文件}"
  - "{不得破坏现有测试}"
  - "{敏感数据不得发送给 LLM}"
ai_confidence: "{high / medium / low}"
pending_confirmation:
  - "{需要人类确认的问题 1}"
  - "{需要人类确认的问题 2}"
available_tools:
  - "{test}"
  - "{lint}"
  - "{browse}"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `{TASK-BACKEND-001}` |
> | `parent_issue` | `{项目前缀}-NNN` |
> | `agent_task_id` | `{AGENT-TASK-NNN}` |
> | **版本** | `{v0.1.0}` |
> | **模板版本** | `v2` |
> | **优先级** | `{P0 / P1 / P2 / P3}` |
> | **状态** | `{待执行 / 执行中 / 已完成 / 阻塞}` |
> | **类型** | `{backend / frontend / fullstack / infra / ai / security / test}` |
> | **预计工作量** | `{XS / S / M / L}` |
> | **分支名** | `{feat/agent-task-NNN-short-name}` |
> | **预计修改文件数上限** | `{8}` |
> | **建议最大变更行数** | `{400}` |
> | **项目技术栈约束** | `{TypeScript / Node.js / pnpm / React / PostgreSQL / Docker}` |
> | **AI 置信度** | `{high / medium / low}` |
> | **待人工确认事项** | `{问题 1 / 问题 2 / -}` |
> | **可用工具/技能** | `{test / lint / browse / -}` |

# {TASK-BACKEND-001} {一句话目标}

> **父 Issue**：`{项目前缀}-NNN`  
> **版本**：`{v0.1.0}`  
> **模板版本**：`v2`  
> **优先级**：`{P0 / P1 / P2 / P3}`  
> **状态**：`{待执行 / 执行中 / 已完成 / 阻塞}`  
> **类型**：`{backend / frontend / fullstack / infra / ai / security / test}`  
> **预计工作量**：`{XS / S / M / L}`  
> **分支名**：`{feat/agent-task-NNN-short-name}`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

用 1-2 句话说明这个任务要交付什么。**必须是一个可独立合并、可测试、可验证的单元**。

示例：
> 实现 `POST /api/v1/organizations/{organizationId}/tasks` 接口，包括 DB migration、API handler、权限校验和单元测试。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `{docs/PRD-vX.Y.Z.md FR-07}` |
| TDD | `{docs/TDD-vX.Y.Z.md 7.2}` |
| ARCHITECTURE | `{docs/ARCHITECTURE-vX.Y.Z.md 章节}` |
| API 契约 | `{docs/API-SPEC-vX.Y.Z.md API-13 / openapi-vX.Y.Z.yaml}` |
| 测试用例 | `{TC-TASK-001 ~ TC-TASK-003}` |
| 埋点事件 | `{EVT-05 task_created}` |
| 父 Issue | `#{项目前缀}-NNN` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md 3.11` |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 建议预留 ≥ 30% token 给本任务输出；前置文档总 token 超过模型上限 60% 时必须分块。 |
| **必读顺序** | front matter → `ai_red_flags` → `pending_confirmation` → 目标 → 上下文 → 边界条件 → 输出。 |
| **分块策略** | 若 PRD + TDD 过大，先读摘要、范围、边界、相关接口；实现细节（如其他模块代码）按需拉取，不要一次性全量塞入 prompt。 |
| **关键约束优先** | `max_lines`、`estimated_files`、失败用例、安全红线必须在第一轮上下文就加载，避免后期返工。 |

### 3.1 已有代码/表（执行前必须阅读）

> **LLM 消费提示**：如果上下文窗口受限，先读取本模板的 `ai_red_flags`、`pending_confirmation` 与第 8 节「约束与红线」，再按 front matter → 目标 → 上下文 → 边界条件 → 输出的顺序加载剩余内容。

- `{apps/api/src/routes/organizations.ts}`
- `{apps/api/src/services/task.ts}`
- `{packages/shared/types/task.ts}`
- `{apps/web/src/pages/TaskCreatePage.tsx}`

### 3.2 数据模型/接口

```typescript
// {关键类型或 schema}
```

### 3.3 设计稿/示例

- `{Figma 链接或截图路径}`
- `{示例请求/响应}`

### 3.4 边界条件

必须量化，避免使用「较大」「较长」等模糊描述。

| 维度 | 约束 | 说明 |
|------|------|------|
| 最大值 | `{文件大小 100MB}` | 超过则返回 `413 payload_too_large` |
| 最小值 | `{标题长度 ≥ 1 字符}` | 空字符串返回 `400 title_required` |
| 长度限制 | `{code 最长 63 字符，仅小写字母/数字/连字符}` | 非法字符返回 `400 invalid_code` |
| 并发限制 | `{每用户每 organization 10 次/分钟}` | 超限返回 `429 rate_limited` |
| 超时 | `{上游服务调用 ≤ 5s，整体请求 ≤ 30s}` | 超时返回 `504 upstream_timeout` |
| 数量限制 | `{每个 organization 最多 1000 条记录}` | 超限返回 `403 resource_limit_exceeded` |
| 时区/语言/编码 | `{UTC 存储，响应 ISO 8601；UTF-8}` | — |

### 3.5 失败用例

至少 4 个典型失败场景，覆盖认证、授权、业务规则、系统异常。

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 未认证请求 | 无 Authorization | 返回 `401 unauthorized` |
| 越权访问 | 非 organization 成员 token | 返回 `403 forbidden` |
| 重复提交 | 相同 Idempotency-Key | 返回第一次结果，不重复创建（幂等） |
| 非法参数 | `status=invalid` | 返回 `400`，`code=invalid_status` |
| 资源不存在 | `organizationId=ws-not-found` | 返回 `404 organization_not_found` |
| 超出速率限制 | 1 分钟内同一用户调用 11 次 | 返回 `429 rate_limited`，含 `Retry-After` 头 |
| 数据库唯一性冲突 | 重复 code | 返回 `409 code_conflict` |
| 上游服务超时 | 文件解析服务 5s 无响应 | 返回 `504 upstream_timeout` |

### 3.6 测试数据 / Mock / Fixture

提供可直接复制到测试文件或 Mock 服务器的 JSON/YAML 样例，建议覆盖正常、边界、异常三种情况。

```json
{
  "organization": {
    "id": "ws-123",
    "code": "{组织标识}"
  },
  "sampleRequest": {
    "title": "Q3 Pitch",
    "status": "open"
  },
  "boundaryRequest": {
    "title": "A",
    "status": "open",
    "dueDate": "2099-12-31T23:59:59Z"
  },
  "invalidRequest": {
    "title": "",
    "status": "not_a_status"
  },
  "mockUser": {
    "id": "usr-001",
    "email": "agent-task-test@example.test",
    "role": "member"
  }
}
```

```yaml
# fixture/tasks.yml
fixtures:
  - id: task-001
    organization_id: ws-123
    title: Q3 Pitch
    status: open
    priority: high
    created_by: usr-001
    created_at: 2026-01-01T00:00:00Z
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

文件数不得超过 front matter 中 `estimated_files` 的值；变更总行数建议不超过 `max_lines`。

| 文件 | 操作 | 说明 |
|------|------|------|
| `{apps/api/src/routes/tasks.ts}` | 新增 | Task API 路由 |
| `{apps/api/src/services/task-service.ts}` | 新增 | 业务逻辑 |
| `{apps/api/src/db/migrations/00X_add_tasks.sql}` | 新增 | Migration |
| `{apps/api/src/routes/tasks.test.ts}` | 新增 | 单元/集成测试 |

### 4.2 行为定义

- `{接口成功时返回 201 和 task 对象}`
- `{失败时返回统一错误格式：code, message, details, request_id}`
- `{权限不足返回 403}`

---

## 5. 验收标准

- [ ] `{DB migration 可独立运行并回滚}`
- [ ] `{API 符合 OpenAPI 契约 API-13}`
- [ ] `{权限校验正确：非 organization 成员返回 403}`
- [ ] `{code 唯一且不可预测}`
- [ ] `{单元测试覆盖率 ≥ 80%}`
- [ ] `{集成测试通过}`
- [ ] `{无 TypeScript / lint 错误}`

---

## 6. 实现步骤建议

> 以下顺序供参考，Agent 可根据实际代码结构调整。

1. `{阅读父 issue 和上下文文档}`
2. `{创建 migration 并运行}`
3. `{实现 repository / service 层}`
4. `{实现 API route 和校验}`
5. `{补充单元测试}`
6. `{补充集成测试}`
7. `{运行测试和 lint}`
8. `{提交 PR}`

---

## 7. 测试验证

### 7.1 单元测试

```bash
# {示例命令}
pnpm test --filter=api -- task-service.test.ts
```

### 7.2 集成测试

```bash
# {示例命令}
pnpm test:integration -- tasks.integration.test.ts
```

### 7.3 手动验证

```bash
# {示例 curl}
curl -X POST https://api.example.com/api/v1/organizations/{组织标识}/tasks \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"title":"Q3 Pitch","status":"open"}'
```

### 7.4 回归测试命令

必须按项目技术栈选择对应命令；若项目未配置某命令，需在 Agent 备注中说明。

```bash
# 仅运行本任务相关测试
{pnpm test --filter=api -- tasks / npm test -- tasks / pytest tests/tasks}

# 项目专属 lint
cd {项目根目录} && {pnpm -r lint / npm run lint / make lint}

# 项目专属 typecheck
cd {项目根目录} && {pnpm -r typecheck / npm run typecheck}

# 全量检查（提交前必须执行）
cd {项目根目录} && {pnpm -r lint && pnpm -r typecheck && pnpm -r test / npm run lint && npm run typecheck && npm test}
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`；若超出，必须拆分为多个 AGENT-TASK。
- **范围锁定**：不得修改本任务范围外的文件；如必须修改，需在 Agent 备注中说明理由并征得审批。
- **保持现有测试通过**：运行全量回归测试命令全绿才能提交；如有预期内的失败，需在 PR 描述中明确列出并解释。
- **租户隔离**：所有数据库查询必须带 `tenant_id` / `organization_id`。
- **认证授权**：任何敏感操作必须通过 auth middleware。
- **不要提前实现**：本任务范围外的功能（如 NDA、审批流）不要碰。
- **禁止把敏感数据发送给 LLM**：Token、密码、密钥、PII、生产数据一律不得出现在 prompt、日志或测试数据中；测试账号/数据必须使用 `.test` 域名或明确标识为 fixture。
- **代码风格**：遵循项目已有目录和命名约定。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

本任务产出在提交 PR 前，应逐项对照 `docs/templates/CODE-REVIEW-template-v1.md` 第 3.11 节「AI 生成代码专项审查」自检：

| CODE-REVIEW 3.11 检查项 | 本任务自检要求 |
|------------------------|----------------|
| 无硬编码示例域名/邮箱/密码（如 example.com、admin/admin） | 测试数据与 mock 必须使用 `{project}.test`、`user-{n}@example.test` 等标识，禁止出现真实域名或默认密码。 |
| 无未清理的 TODO / FIXME / placeholder | 实现完成后全局搜索 `TODO`、`FIXME`、`placeholder`、`XXX`，确保无残留。 |
| 无幻觉常量（如 magic number、不存在的枚举值） | 所有常量应从已有配置或 shared 包引用；新增枚举必须同步到 `packages/shared`。 |
| 错误处理不过度 try-catch，不吞掉异常 | 异常必须记录并返回结构化错误；禁止空 catch 块或仅 `console.log`。 |
| 未引入未使用的依赖或代码 | 提交前运行 lint，删除无用 import/变量/依赖。 |
| 未擅自实现范围外功能 | 严格按第 4.1 节文件列表和第 1 节目标实现。 |
| 测试数据与生产数据隔离 | fixture、mock、测试数据库不得引用生产 schema 或真实用户数据。 |

---

## 10. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过（单元 + 集成）
- [ ] lint / typecheck 通过
- [ ] 代码审查通过
- [ ] 与父 Issue 的验收标准对齐
- [ ] PR 已关联父 Issue：`Closes #{项目前缀}-NNN` 或 `Relates to #{项目前缀}-NNN`
- [ ] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- `{已踩过的坑}`
- `{推荐使用的库/工具}`
- `{如果遇到困难先问的问题}`
- `{超出 max_lines / estimated_files 时的拆分建议}`

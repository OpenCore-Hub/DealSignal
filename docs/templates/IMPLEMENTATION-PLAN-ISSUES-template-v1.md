---
id: "IM-YYYY-NNN"
version: "{vX.Y.Z}"
status: "{草稿 / 评审中 / 已批准 / 已归档}"
owner: "{负责人}"
linked_docs:
  - "docs/PRD-vX.Y.Z.md"
  - "docs/TDD-vX.Y.Z.md"
---

# {产品名} 开发执行计划 issue 拆分清单

> **资源编号**：`IM-YYYY-NNN`  
> **版本**：`{vX.Y.Z}`  
> **模板版本**：`v1`  
> **状态**：`{草稿 / 评审中 / 已批准 / 已归档}`  
> **编写人/适用对象**：`{技术负责人 / 项目经理}`  
> **编写日期**：`{YYYY-MM-DD}`  
> **关联资源**：  
> - `docs/IMPLEMENTATION-PLAN-vX.Y.Z.md`  
> - `docs/PRD-vX.Y.Z.md`  
> - `docs/TDD-vX.Y.Z.md`  
> - `docs/ARCHITECTURE-vX.Y.Z.md`  
> - `docs/templates/CODE-REVIEW-template-v1.md`  
> **评审人**：`{技术负责人、项目经理、产品负责人}`  
> **执行状态（IMPLEMENTATION-PLAN 专用）**：`{未开始 / 执行中 / 已完成}`

---

## 1. 资源说明

本资源是 `IMPLEMENTATION-PLAN` 的下游拆分产物，用于把计划中的 `TASK-{模块}-{NNN}` 进一步拆成可进入 Sprint、可分配、可跟踪、可验收的 issues/tickets。

**核心目标**：
- 每个 issue 只负责一个可独立合并的交付单元。
- 每个 issue 都能追溯到 `TASK / PRD / TDD / API / 测试用例`。
- issue 的版本、优先级、状态、风险一目了然。

**与 IMPLEMENTATION-PLAN 的关系**：

```text
IMPLEMENTATION-PLAN（TASK 层）
        │
        ▼
ISSUE-MANIFEST（issue 层）
        │
        ▼
Sprint Board → 分支 → PR → 测试 → 上线
```

---

## 2. 字段规范

### 2.1 Issue 编号

格式：`{项目前缀}-{NNN}`

| 字段 | 示例 | 说明 |
|------|------|------|
| 项目前缀 | `DS` | 项目英文缩写 |
| 序号 | `001` | 自增，三位 |

示例：`{项目前缀}-001`

### 2.2 版本（Version）

按产品路线图划分，建议与里程碑对齐：

| 版本 | 目标 |
|------|------|
| `v0.1.0` | MVP 核心链路可用 |
| `v0.2.0` | 意图信号 / 热度评分 |
| `v0.3.0` | 协作空间 |
| `v0.4.0` | 工作流与洞察 |
| `v0.5.0` | 团队 GTM 工具栈 |
| `v0.6.0` | 企业信任 / 安全合规 |
| `v0.7.0+` | AI / 企业智能层 |

### 2.3 优先级（Priority）

| 优先级 | 说明 |
|--------|------|
| `P0` | 阻塞里程碑，必须完成 |
| `P1` | 重要，尽量在当前版本完成 |
| `P2` | 有价值，可延期到后续版本 |
| `P3` | 优化项，有空再做 |

### 2.4 状态（Status）

| 状态 | 说明 |
|------|------|
| `待创建` | 已规划，尚未在 issue tracker 创建 |
| `待开始` | 已创建，未进入开发 |
| `开发中` | 工程师正在实现 |
| `代码审查中` | PR 已提交，等待 review |
| `测试中` | QA 验证中 |
| `已验收` | 通过所有质量门禁 |
| `已上线` | 已合并发布 |
| `已关闭` | 因需求变更或重复等原因关闭 |
| `阻塞` | 因依赖/问题暂停 |

### 2.5 类型（Type）

| 类型 | 说明 |
|------|------|
| `backend` | 后端服务/API |
| `frontend` | Web/前端 |
| `fullstack` | 前后端联动 |
| `infra` | 基础设施/运维/部署 |
| `ai` | AI/LLM/解析相关 |
| `security` | 安全/合规 |
| `docs` | 资源/规范 |
| `test` | 测试工程 |

### 2.6 风险等级（Risk Class）

| 风险 | 说明 |
|------|------|
| `build_failure` | 高风险，可能导致构建/流水线失败 |
| `test_failure` | 中等风险，可能影响测试稳定性 |
| `unknown` | 风险未知，需要技术预研 |

---

## 3. Issue 清单

### 3.1 清单总表

| Issue ID | 标题 | 版本 | 优先级 | 状态 | 类型 | 风险 | 依赖 | 关联 TASK | PRD | TDD | API | 负责人 |
|----------|------|------|--------|------|------|------|------|-----------|-----|-----|-----|--------|
| `{项目前缀}-001` | `{Project scaffold and schema baseline}` | `v0.1.0` | `P0` | `待开始` | `infra` | `build_failure` | `-` | `TASK-INFRA-001` | `-` | `3.3` | `-` | `{姓名}` |
| `{项目前缀}-002` | `{Auth, sessions, and organization memberships}` | `v0.1.0` | `P0` | `待开始` | `backend` | `build_failure` | `{项目前缀}-001` | `TASK-AUTH-001` | `FR-01` | `4.2` | `API-01~04` | `{姓名}` |
| `{项目前缀}-003` | `{Resource upload API}` | `v0.1.0` | `P0` | `待开始` | `backend` | `build_failure` | `{项目前缀}-001,{项目前缀}-002` | `TASK-UPLOAD-001` | `FR-02` | `6.1` | `API-05` | `{姓名}` |
| `{项目前缀}-004` | `{文件元数据提取}` | `v0.1.0` | `P0` | `待开始` | `ai` | `unknown` | `{项目前缀}-003` | `TASK-INGEST-001` | `FR-02` | `6.2` | `-` | `{姓名}` |
| `{项目前缀}-005` | `{功能名称}` | `v0.1.0` | `P0` | `待开始` | `backend` | `test_failure` | `{项目前缀}-003` | `TASK-LINK-001` | `FR-07` | `7.2` | `API-13` | `{姓名}` |

### 3.2 按版本分组

#### v0.1.0 — {MVP 核心链路}

| Issue ID | 标题 | 优先级 | 状态 | 负责人 |
|----------|------|--------|------|--------|
| `{项目前缀}-001` | `{Project scaffold and schema baseline}` | `P0` | `待开始` | `{姓名}` |
| `{项目前缀}-002` | `{Auth, sessions, and organization memberships}` | `P0` | `待开始` | `{姓名}` |
| `{项目前缀}-003` | `{Resource upload API}` | `P0` | `待开始` | `{姓名}` |
| `{项目前缀}-004` | `{文件元数据提取}` | `P0` | `待开始` | `{姓名}` |
| `{项目前缀}-005` | `{功能名称}` | `P0` | `待开始` | `{姓名}` |

#### v0.2.0 — {意图信号}

| Issue ID | 标题 | 优先级 | 状态 | 负责人 |
|----------|------|--------|------|--------|
| `{项目前缀}-010` | `{Activity event taxonomy}` | `P0` | `待创建` | `{姓名}` |
| `{项目前缀}-011` | `{优先级评分规则 v1}` | `P0` | `待创建` | `{姓名}` |

---

## 4. Issue 详情模板

每个 issue 应包含以下信息。可保存为 `docs/tasks/issues-v2/issue-NNN-{issue-id}-{slug}.md`。

```markdown
# {项目前缀}-NNN {标题}

## 元数据

- **版本**: `v0.1.0`
- **优先级**: `P0`
- **状态**: `待开始`
- **类型**: `backend`
- **风险**: `build_failure`
- **依赖**: `{项目前缀}-001, {项目前缀}-002`
- **关联 TASK**: `TASK-UPLOAD-001`
- **PRD**: `FR-02`
- **TDD**: `6.1`
- **API**: `API-05`
- **测试**: `TC-UPLOAD-001 ~ TC-UPLOAD-004`
- **埋点**: `EVT-01 resource_created`
- **负责人**: `{姓名}`

## 背景

{这个 issue 要解决什么问题，引用 IMPLEMENTATION-PLAN 和上游资源。}

## 目标

1. {目标 1}
2. {目标 2}

## 验收标准

- [ ] {验收项 1}
- [ ] {验收项 2}
- [ ] {验收项 3}

## 实现提示

- {关键实现点}
- {可能的技术坑}

## 测试策略

- 单元测试：`{覆盖哪些函数}`
- 集成测试：`{覆盖哪些接口}`
- E2E：`{覆盖哪些用户路径}`

## 回滚/风险

- {变更失败如何回滚}
- {对现有功能的影响}
```

---

## 5. 从 Issue 到 Agent Task（最小可执行单元）

> `{项目前缀}-xxx` issue 是**功能级**单元；进入 AI 编码/开发前，应再拆成**一次 PR 可完成**的 `AGENT-TASK`。

### 5.1 拆分原则

- 一个 `AGENT-TASK` 只交付一个可独立合并的代码单元。
- 推荐粒度：一个 API endpoint、一张表的 migration、一个核心组件、一个 worker stage。
- 如果一个 issue 预计超过 3~5 天或 400 行代码，必须拆分。

### 5.2 Agent Task 模板

详见 `docs/templates/AGENT-TASK-template-v2.md`。

### 5.3 示例映射

| 父 Issue | Agent Task | task_id | parent_issue | 说明 |
|----------|------------|---------|--------------|------|
| `{项目前缀}-008 {功能名称}` | `AGENT-TASK-001` | `TASK-BACKEND-001` | `{项目前缀}-008` | `{resource}_table` 表 migration + repository |
| `{项目前缀}-008 {功能名称}` | `AGENT-TASK-002` | `TASK-BACKEND-002` | `{项目前缀}-008` | `POST /api/v1/{资源名}` 接口 |
| `{项目前缀}-008 {功能名称}` | `AGENT-TASK-003` | `TASK-BACKEND-003` | `{项目前缀}-008` | 状态机与访问模式实现 |
| `{项目前缀}-008 {功能名称}` | `AGENT-TASK-004` | `TASK-BACKEND-004` | `{项目前缀}-008` | 状态转换/撤销逻辑 + 单元测试 |

---

## 6. 自动化生成

### 6.1 JSON 清单（可选）

如需用脚本批量创建 GitHub issues，可维护一份 `issue-manifest.json`：

```json
{
  "issues": [
    {
      "local_id": "{项目前缀}-001",
      "title": "Project scaffold and schema baseline",
      "version": "v0.1.0",
      "type": "infra",
      "priority": "high",
      "risk_class": "build_failure",
      "dependencies": [],
      "local_path": "docs/tasks/issues-v2/issue-001-{项目前缀}-001-project-scaffold-and-schema-baseline.md"
    }
  ]
}
```

### 6.2 创建脚本

使用 GitHub CLI 批量创建：

```bash
python3 {scripts/create-issues-from-plan.py}
```

要求：
- 已安装 `gh` 并登录
- 有仓库写权限

---

## 7. 维护规则

1. **新增 issue**：必须补充 `版本、优先级、状态、类型、风险、依赖、关联 TASK`。
2. **状态流转**：只能按 `待创建 → 待开始 → 开发中 → 代码审查中 → 测试中 → 已验收 → 已上线` 顺序推进。
3. **依赖管理**：issue 开始前必须确认所有依赖已关闭或已解耦。
4. **同步回 IMPLEMENTATION-PLAN**：当某个 `TASK` 下的所有 issue 都已上线，更新该 `TASK` 状态为 `已验收`。
5. **归档**：版本发布完成后，本资源状态转为 `已归档`，新版本复制本模板重新生成。

---

## 8. 检查清单

- [ ] 所有 `TASK` 已拆解为具体 issue
- [ ] 每个 issue 都有 `版本、优先级、状态、类型、风险`
- [ ] 复杂 issue 已进一步拆分为 `AGENT-TASK`
- [ ] 依赖关系无环
- [ ] 每个 issue 已关联 PRD / TDD / API / 测试 / 埋点
- [ ] issue tracker（GitHub/Jira/Linear）已同步
- [ ] 负责人已分配

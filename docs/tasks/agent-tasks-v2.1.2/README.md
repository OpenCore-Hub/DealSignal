# DealSignal v2.1.2 可执行 AGENT-TASK 清单

本目录按 `docs/templates/AGENT-TASK-template-v2.md` 拆解了前端收尾与后端 MVP 的 17 个独立任务。

## 任务总览

### 前端任务（v2.1.2）

| 文件 | Task ID | 标题 | 优先级 | 类型 | 工作量 | Parent Issue | 依赖 |
|------|---------|------|--------|------|--------|--------------|------|
| [TASK-FRONTEND-001.md](./TASK-FRONTEND-001.md) | TASK-FRONTEND-001 | 前端质量收尾 | P1 | frontend | S | DS-026 | - |
| [TASK-FRONTEND-002.md](./TASK-FRONTEND-002.md) | TASK-FRONTEND-002 | Viewer 子组件拆分与 Canvas 体验增强 | P0 | frontend | M | DS-010 | TASK-FRONTEND-001 |
| [TASK-FRONTEND-003.md](./TASK-FRONTEND-003.md) | TASK-FRONTEND-003 | 前端-后端集成层 | P0 | frontend | M | DS-027 | TASK-BACKEND-002（契约）；完整验证依赖后端全链路 |
| [TASK-FRONTEND-004.md](./TASK-FRONTEND-004.md) | TASK-FRONTEND-004 | 悬浮 AI 助手前端 | P0 | frontend | M | DS-014 | TASK-FRONTEND-002, TASK-BACKEND-004 |
| [TASK-FRONTEND-005.md](./TASK-FRONTEND-005.md) | TASK-FRONTEND-005 | Dashboard 前端完善 | P1 | frontend | S | DS-016 | TASK-BACKEND-005 |

### 后端任务（v2.1.0）

| 文件 | Task ID | 标题 | 优先级 | 类型 | 工作量 | Parent Issue | 依赖 |
|------|---------|------|--------|------|--------|--------------|------|
| [TASK-BACKEND-001.md](./TASK-BACKEND-001.md) | TASK-BACKEND-001 | Go 后端脚手架 | P0 | backend | M | DS-001 | - |
| [TASK-BACKEND-002.md](./TASK-BACKEND-002.md) | TASK-BACKEND-002 | 认证与 Workspace/租户模块 | P0 | backend | L | DS-002 | TASK-BACKEND-001 |
| [TASK-BACKEND-003.md](./TASK-BACKEND-003.md) | TASK-BACKEND-003 | 文档上传、对象存储与 ingestion pipeline | P0 | backend | L | DS-003/005/006/007 | TASK-BACKEND-002 |
| [TASK-BACKEND-004.md](./TASK-BACKEND-004.md) | TASK-BACKEND-004 | ✅ Search、Evidence 与 Assistant 服务 | P0 | backend | L | DS-008/011/012/013 | TASK-BACKEND-003 |
| [TASK-BACKEND-005.md](./TASK-BACKEND-005.md) | TASK-BACKEND-005 | ✅ 智能链接、权限、Analytics 与热度评分 | P0 | backend | L | DS-009/015/017 | TASK-BACKEND-003 |
| [TASK-BACKEND-006.md](./TASK-BACKEND-006.md) | TASK-BACKEND-006 | ✅ 数据室模块 | P0 | backend | L | DS-019 | TASK-BACKEND-005 |
| [TASK-BACKEND-007.md](./TASK-BACKEND-007.md) | TASK-BACKEND-007 | 子域名/自定义域名与 SSL（P1 延后） | P1 | infra | L | DS-004 | TASK-BACKEND-001 |
| [TASK-BACKEND-008.md](./TASK-BACKEND-008.md) | TASK-BACKEND-008 | ✅ 行为提醒与跟进建议 | P1 | backend | M | DS-018 | TASK-BACKEND-005 |
| [TASK-BACKEND-009.md](./TASK-BACKEND-009.md) | TASK-BACKEND-009 | ✅ 邮件通知与 Slack/HubSpot/Salesforce 集成 | P1 | backend | L | DS-020/021/022 | TASK-BACKEND-005 |
| [TASK-BACKEND-010.md](./TASK-BACKEND-010.md) | TASK-BACKEND-010 | 安全扫描与修复 | P0 | security | M | DS-025 | TASK-BACKEND-006, TASK-BACKEND-009 |

### 测试任务

| 文件 | Task ID | 标题 | 优先级 | 类型 | 工作量 | Parent Issue | 依赖 |
|------|---------|------|--------|------|--------|--------------|------|
| [TASK-TEST-001.md](./TASK-TEST-001.md) | TASK-TEST-001 | 测试用例与自动化 | P0 | test | L | DS-023 | 功能开发完成 |
| [TASK-TEST-002.md](./TASK-TEST-002.md) | TASK-TEST-002 | 性能压测与优化 | P1 | test | M | DS-024 | 功能开发完成 |

## 执行顺序

```text
Sprint 1
├── TASK-FRONTEND-001
├── TASK-BACKEND-001
└── TASK-BACKEND-002

Sprint 2
├── TASK-BACKEND-003
└── TASK-FRONTEND-003（接入 auth/workspace 契约，完整验证延后）

Sprint 3
├── TASK-FRONTEND-002
├── TASK-BACKEND-004
└── TASK-BACKEND-005

Sprint 4
├── TASK-FRONTEND-004
├── TASK-FRONTEND-005
└── TASK-BACKEND-006

Sprint 5
├── TASK-BACKEND-008
├── TASK-BACKEND-009
└── TASK-TEST-001

Sprint 6
├── TASK-BACKEND-007（P1 延后）
├── TASK-BACKEND-010
└── TASK-TEST-002
```

## 关键路径

```text
TASK-BACKEND-001 → TASK-BACKEND-002 → TASK-BACKEND-003
                                          │
                    ┌─────────────────────┤
                    │                     ▼
                    │           TASK-BACKEND-004
                    │                     │
                    │                     ▼
                    │           TASK-BACKEND-005
                    │              │
                    │    ┌─────────┼─────────┐
                    │    ▼         ▼         ▼
                    │  TASK-BE-006  TASK-BE-008  TASK-BE-009
                    │    │                     │
                    │    └─────────┬───────────┘
                    │              ▼
                    │        TASK-BACKEND-010
                    │
TASK-FRONTEND-001 → TASK-FRONTEND-002 → TASK-FRONTEND-004
                    │
                    ▼
            TASK-FRONTEND-003（最终集成验证）
```

## 使用方式

1. 按依赖顺序选择一个任务。
2. 按任务文件 front matter 创建对应分支。
3. 严格遵循文件中的「约束与红线」和「验收标准」。
4. 完成后按第 10 节 Definition of Done 自检并提交 PR。

## 关联资源

- 实施计划：`docs/IMPLEMENTATION-PLAN-v2.1.2.md`
- Issue 清单：`docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.2.md`
- 计划文件：`/Users/mg/.kimi/plans/wolverine-bishop-steel.md`
- 一致性评审：`docs/reviews/PRD-TDD-API-PLAN-TASK-consistency-review-v2.1.2.md`
- 前端对齐评审：`docs/reviews/frontend-implementation-doc-alignment-v2.1.2.md`
- 任务模板：`docs/templates/AGENT-TASK-template-v2.md`

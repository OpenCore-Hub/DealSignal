# DealSignal v2.1.3 可执行 AGENT-TASK 清单

本目录按 `docs/templates/AGENT-TASK-template-v2.md` 拆解了 v2.1.3「前端审计优化 + 后端加固」的 12 个独立任务。

---

## 任务总览

### 前端任务

| 文件 | Task ID | 标题 | 优先级 | 类型 | 工作量 | Parent Issue | 依赖 |
|------|---------|------|--------|------|--------|--------------|------|
| [TASK-FRONTEND-006.md](./TASK-FRONTEND-006.md) | TASK-FRONTEND-006 | 前端阻塞按钮与即时反馈清零 | P0 | frontend | S | DS-028 | - |
| [TASK-FRONTEND-007.md](./TASK-FRONTEND-007.md) | TASK-FRONTEND-007 | 表单提交反馈、删除确认与账户菜单 | P0 | frontend | M | DS-029 | TASK-FRONTEND-006 |
| [TASK-FRONTEND-008.md](./TASK-FRONTEND-008.md) | TASK-FRONTEND-008 | 前端文案与中英混杂清理 | P1 | frontend | S | DS-030 | - |
| [TASK-FRONTEND-009.md](./TASK-FRONTEND-009.md) | TASK-FRONTEND-009 | API 请求层修复与真实后端适配 | P0 | frontend | M | DS-031 | - |
| [TASK-FRONTEND-010.md](./TASK-FRONTEND-010.md) | TASK-FRONTEND-010 | heatScore topKeyPages 算法对齐 | P0 | frontend | S | DS-032 | - |
| [TASK-FRONTEND-011.md](./TASK-FRONTEND-011.md) | TASK-FRONTEND-011 | 统一数据层与 oversized 组件拆分 | P1 | frontend | L | DS-033 | TASK-FRONTEND-009 |
| [TASK-FRONTEND-012.md](./TASK-FRONTEND-012.md) | TASK-FRONTEND-012 | UI/UX 细节打磨 | P1 | frontend | M | DS-034 | TASK-FRONTEND-011 |
| [TASK-FRONTEND-013.md](./TASK-FRONTEND-013.md) | TASK-FRONTEND-013 | 前端单元与组件测试补强 | P1 | test | M | DS-035 | TASK-FRONTEND-009 / 011 |

### 后端任务

| 文件 | Task ID | 标题 | 优先级 | 类型 | 工作量 | Parent Issue | 依赖 |
|------|---------|------|--------|------|--------|--------------|------|
| [TASK-BACKEND-011.md](./TASK-BACKEND-011.md) | TASK-BACKEND-011 | 后端未落库改动整理与接口稳定 | P0 | backend | L | DS-036 | - |
| [TASK-BACKEND-012.md](./TASK-BACKEND-012.md) | TASK-BACKEND-012 | 后端中间件与基础模块补全 | P0 | backend | M | DS-037 | TASK-BACKEND-011 |

### 测试与文档任务

| 文件 | Task ID | 标题 | 优先级 | 类型 | 工作量 | Parent Issue | 依赖 |
|------|---------|------|--------|------|--------|--------------|------|
| [TASK-TEST-003.md](./TASK-TEST-003.md) | TASK-TEST-003 | E2E 与契约测试 | P0 | test | M | DS-038 | TASK-FRONTEND-009 / TASK-BACKEND-011 |
| [TASK-DOCS-001.md](./TASK-DOCS-001.md) | TASK-DOCS-001 | v2.1.3 文档基线同步 | P1 | docs | M | DS-039 | 功能开发完成 |

---

## 执行顺序

```text
Sprint 1（阻塞项清零）
├── TASK-FRONTEND-006
├── TASK-FRONTEND-007
└── TASK-FRONTEND-008

Sprint 2（API/算法/后端稳定）
├── TASK-FRONTEND-009
├── TASK-FRONTEND-010
└── TASK-BACKEND-011

Sprint 3（架构与测试）
├── TASK-FRONTEND-011
├── TASK-FRONTEND-013
└── TASK-BACKEND-012

Sprint 4（体验、E2E、文档、发布）
├── TASK-FRONTEND-012
├── TASK-TEST-003
└── TASK-DOCS-001
```

## 关键路径

```text
TASK-FRONTEND-006 → TASK-FRONTEND-007
        │
        ▼
TASK-FRONTEND-009 ────────────────────→ TASK-FRONTEND-011 → TASK-FRONTEND-012
        │                                      │
        ▼                                      ▼
TASK-FRONTEND-010                    TASK-FRONTEND-013
        │
        ▼
TASK-BACKEND-011 → TASK-BACKEND-012
        │
        ▼
TASK-TEST-003 → TASK-DOCS-001
```

---

## 使用方式

1. 按依赖顺序选择一个任务。
2. 按任务文件 front matter 创建对应分支。
3. 严格遵循文件中的「约束与红线」和「验收标准」。
4. 完成后按第 10 节 Definition of Done 自检并提交 PR。

---

## 关联资源

- 实施计划：`docs/IMPLEMENTATION-PLAN-v2.1.3.md`
- Issue 清单：`docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.3.md`
- 前端审计计划：`docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md`
- 进度追踪：`docs/PROJECT-PROGRESS.md`
- 任务模板：`docs/templates/AGENT-TASK-template-v2.md`

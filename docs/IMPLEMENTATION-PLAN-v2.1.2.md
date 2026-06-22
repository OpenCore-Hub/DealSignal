---
id: "IP-2024-002"
version: "v2.1.2"
status: "已完成"
owner: "技术负责人 / 项目经理"
linked_docs:
  - "docs/PRD-v2.1.0.md"
  - "docs/TDD-v2.1.0.md"
  - "docs/API-SPEC-v2.1.0.md"
  - "docs/ARCHITECTURE-v2.1.0.md"
  - "docs/database-model-v2.1.0.md"
  - "docs/HEAT-SCORE-ALGORITHM-v2.1.1.md"
  - "docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.2.md"
  - "docs/tasks/agent-tasks-v2.1.2/*.md"
---

# DealSignal v2.1.2 实施计划

> **文档编号**：`IP-2024-002`  
> **版本**：`v2.1.2`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`技术负责人 / 项目经理`  
> **编写日期**：`2026-06-21`  
> **关联资源**：
> - `docs/PRD-v2.1.0.md`
> - `docs/TDD-v2.1.0.md`
> - `docs/API-SPEC-v2.1.0.md`
> - `docs/ARCHITECTURE-v2.1.0.md`
> - `docs/database-model-v2.1.0.md`
> - `docs/HEAT-SCORE-ALGORITHM-v2.1.1.md`
> - `docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.2.md`
> - `docs/reviews/PRD-TDD-API-PLAN-TASK-consistency-review-v2.1.2.md`
> - `docs/reviews/frontend-implementation-doc-alignment-v2.1.2.md`
> - `docs/templates/AGENT-TASK-template-v2.md`
> **评审人**：`CTO、技术负责人、产品经理、测试负责人`

---

## 1. 当前状态

- **前端 v2.1.2** 已合入 `main`：i18n（en/zh-CN）、Settings Language、Workspace Switcher、Theme Toggle、Viewer 子组件拆分、悬浮 AI 助手、Dashboard 信号排序、前端-后端集成层、MSW 开关、P0 E2E 覆盖。
- **后端 MVP** 已合入 `main`：Go 服务骨架、auth/workspace、upload/ingestion、search/evidence/assistant、links/analytics/rooms、notifications/integrations、tenant subdomain/custom domain/SSL、security scan 均已实现并通过测试。
- **测试与安全**：Vitest 覆盖率门禁、Go race-detector 测试、k6 压测脚本、Playwright P0 E2E、Trivy image/fs、gitleaks、govulncheck、`pnpm audit` 全部接入 CI。
- **文档**：PRD/TDD/API/DB/ARCHITECTURE/PLAN/TASK 随代码同步更新，v2.1.2 已具备可发布状态。

本计划状态：**已完成**。

---

## 2. 目标

1. **前端收尾**：清理 v2.1.1 遗留问题，完成 Viewer 子组件拆分，建立可切换真实后端的 API 集成层。
2. **后端 MVP**：搭建 Go 服务骨架，实现 auth/workspace、upload/ingestion、search/evidence/assistant、links/analytics/rooms、notifications/integrations/security 核心链路。
3. **文档对齐**：确保 PRD/TDD/API/DB/ARCHITECTURE/PLAN/TASK 在字段、枚举、路径、依赖上保持一致。
4. **可测试**：补齐测试自动化与性能压测，确保 P0 路径有基本门禁。

---

## 3. 范围

### 3.1 In Scope

- 前端质量收尾（act 警告、AI 关键词、workspace name、mock 响应结构清理）。
- Viewer 子组件拆分 + Canvas 体验增强。
- 前端-后端集成层（API base URL、版本、workspace 上下文、token、BaseResponse、错误处理、MSW 开关）。
- Go 后端脚手架、Auth/Workspace、对象存储、上传/ingestion、search/evidence/assistant、links/analytics、deal rooms、notifications/integrations、security scan。
- 测试自动化（单元/集成/E2E）与性能压测。

### 3.2 Out of Scope / Deferred

- 自定义域名/CNAME/SSL 自动签发（DS-004，P1，可延后到 Sprint 6）。
- Salesforce 集成（若资源不足，可先实现 HubSpot，Salesforce 标注二期）。
- 高级 AI agent 工具调用、多模态解析、OCR。
- 完整商业化计费、审计日志详情页。

---

## 4. 任务总览

### 4.1 前端任务

| Task ID | 标题 | 优先级 | 工作量 | Parent Issue | 依赖 |
|---|---|---|---|---|---|
| TASK-FRONTEND-001 | 前端质量收尾 | P1 | S | DS-026 | - |
| TASK-FRONTEND-002 | Viewer 子组件拆分与 Canvas 体验增强 | P0 | M | DS-010 | TASK-FRONTEND-001 |
| TASK-FRONTEND-003 | 前端-后端集成层 | P0 | M | DS-027 | TASK-BACKEND-002（契约） |
| TASK-FRONTEND-004 | 悬浮 AI 助手前端 | P0 | M | DS-014 | TASK-FRONTEND-002, TASK-BACKEND-004 |
| TASK-FRONTEND-005 | Dashboard 前端完善 | P1 | S | DS-016 | TASK-BACKEND-005 |

### 4.2 后端任务

| Task ID | 标题 | 优先级 | 工作量 | Parent Issue | 依赖 |
|---|---|---|---|---|---|
| TASK-BACKEND-001 | Go 后端脚手架 | P0 | M | DS-001 | - |
| TASK-BACKEND-002 | 认证与 Workspace/租户模块 | P0 | L | DS-002 | TASK-BACKEND-001 |
| TASK-BACKEND-003 | 文档上传、对象存储与 ingestion pipeline | P0 | L | DS-003/005/006/007 | TASK-BACKEND-002 |
| TASK-BACKEND-004 | Search、Evidence 与 Assistant 服务 | P0 | L | DS-008/011/012/013 | TASK-BACKEND-003 |
| TASK-BACKEND-005 | 智能链接、权限、Analytics 与热度评分 | P0 | L | DS-009/015/017 | TASK-BACKEND-003 |
| TASK-BACKEND-006 | 数据室模块 | P0 | L | DS-019 | TASK-BACKEND-005 |
| TASK-BACKEND-007 | 子域名/自定义域名与 SSL（P1 延后） | P1 | L | DS-004 | TASK-BACKEND-001 |
| TASK-BACKEND-008 | 行为提醒与跟进建议 | P1 | M | DS-018 | TASK-BACKEND-005 |
| TASK-BACKEND-009 | 邮件通知与 Slack/HubSpot 集成 | P1 | L | DS-020/021/022 | TASK-BACKEND-005 |
| TASK-BACKEND-010 | 安全扫描与修复 | P0 | M | DS-025 | TASK-BACKEND-006/009 |

### 4.3 测试任务

| Task ID | 标题 | 优先级 | 工作量 | Parent Issue | 依赖 |
|---|---|---|---|---|---|
| TASK-TEST-001 | 测试用例与自动化 | P0 | L | DS-023 | 功能开发完成 |
| TASK-TEST-002 | 性能压测与优化 | P1 | M | DS-024 | 功能开发完成 |

---

## 5. 执行顺序与依赖

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

### 关键路径

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

---

## 6. 验收标准

- [x] 前端 `pnpm lint && pnpm test && pnpm build` 全绿。
- [x] 后端 `make lint && make test && make build` 全绿；`docker compose up --build` 可启动。
- [x] 前端可通过 `VITE_API_BASE_URL` 切换真实后端；未配置时回退 MSW。
- [x] Auth/Workspace 端点可被前端调用完成登录/注册/邀请。
- [x] 文档上传、解析、签名 URL、Viewer 渲染可端到端跑通。
- [x] 智能链接创建、公开访问、事件上报、热度评分可端到端跑通。
- [x] AI 问答返回 evidence，可跳转高亮对应页面区域。
- [x] `make security` 无 HIGH/CRITICAL 漏洞。
- [x] 全量文档（PRD/TDD/API/DB/PLAN/TASK）在字段、枚举、路径、依赖上保持一致。

---

## 7. 风险与应对

| 风险 | 影响 | 应对 |
|---|---|---|
| 前端 `api.ts` 路径/响应格式改造影响大量组件 | 高 | 集中由 TASK-FRONTEND-003 完成，增加 mapper + BaseResponse 解析层；保持内部类型不变。 |
| 后端任务范围大，单 PR 可能超载 | 高 | 按逻辑域拆分（links/analytics/rooms/notify/security 独立任务），每个任务上限 12 文件/800 行。 |
| OnlyOffice / OpenAI 依赖不稳定 | 中 | 本地用 Docker 部署；开发环境提供 mock 降级开关。 |
| 热度评分算法前后端不一致 | 中 | 推荐后端计算分数与 factors；前端复用算法做本地 preview。 |
| 自定义域名/SSL 资源不足 | 中 | DS-004 / TASK-BACKEND-007 设为 P1，可延后。 |

---

## 8. 文档同步清单

- [x] `docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.2.md`
- [x] `docs/tasks/agent-tasks-v2.1.2/*.md`
- [ ] `docs/API-SPEC-v2.1.0.md`：补齐 Auth/Workspace/Contacts/缺失端点，统一错误码/响应格式。
- [ ] `docs/database-model-v2.1.0.md`：修正 `assistant_sessions.link_id`，新增 `allowed_domains`、`contacts` 等。
- [ ] `docs/HEAT-SCORE-ALGORITHM-v2.1.1.md`：补充 trend/key-page/opens/bounce caps 细节。
- [ ] `docs/ARCHITECTURE-v2.1.0.md`：更新 ERD 与 ingestion 数据流。

---

## 9. 实际落地备注（§10 预留）

> 本计划在执行过程中产生的偏差、范围变更、技术决策将记录于此。

---

> **模板版本**：v1  
> **实施计划版本**：v2.1.2  
> **状态**：已批准  
> **最后更新**：2026-06-21

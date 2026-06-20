# 通用企业文档模板库

本目录集中存放企业生产级文档模板，可直接复制到其他项目使用。所有新建项目文档应优先从本目录选择合适的模板开始编写。

## 模板清单

| 模板文件 | 用途 | 适用阶段 | 主要读者 | 成熟度 |
|----------|------|----------|----------|----------|
| `STRATEGY-template-v1.md` | 项目启动前置战略文档 | 立项前 | CEO、创始人、投资人 | 稳定 |
| `STRATEGY-LIGHT-template-v1.md` | 战略文档（轻量版） | 立项前 | CEO、创始人、产品 | 新引入 |
| `USER-RESEARCH-template-v1.md` | 用户研究报告 | 立项前/需求前 | 产品、设计、市场 | 稳定 |
| `USER-RESEARCH-LIGHT-template-v1.md` | 用户研究报告（轻量版） | 立项前/需求前 | 产品、设计、市场 | 新引入 |
| `COMPETITIVE-ANALYSIS-template-v1.md` | 竞品分析文档 | 立项前/需求前 | 产品、市场、战略 | 稳定 |
| `COMPETITIVE-ANALYSIS-LIGHT-template-v1.md` | 竞品分析文档（轻量版） | 立项前/需求前 | 产品、市场、战略 | 新引入 |
| `PRD-template-v2.md` | 产品需求文档（PRD） | 需求阶段 | 产品、设计、开发、测试、运营 | 稳定 |
| `PRD-LIGHT-template-v1.md` | 轻量需求文档 / User Story | 需求阶段（小功能/修复/优化） | 产品、Tech Lead、开发 | 新引入 |
| `PRD-REVIEW-CHECKLIST-template-v1.md` | PRD 评审检查清单 | 需求评审 | 产品、技术、设计、测试、运营、合规 | 稳定 |
| `UI-DESIGN-DELIVERABLE-template-v1.md` | UI/UX 设计交付文档 | 设计阶段 | 设计师、前端、产品、测试 | 稳定 |
| `UI-DESIGN-LIGHT-template-v1.md` | UI 设计交付物（轻量版） | 设计阶段 | 设计师、前端、产品 | 新引入 |
| `DESIGN-SYSTEM-template-v1.md` | 设计系统文档 | 设计阶段 | 设计、前端、产品 | 稳定 |
| `FRONTEND-COMPONENT-GUIDE-template-v1.md` | 前端组件规范 | 设计阶段 | 前端、设计、产品 | 稳定 |
| `TRACKING-REQUIREMENTS-template-v1.md` | 埋点需求文档（TRD） | 需求/设计阶段 | 产品、增长、数据、开发 | 稳定 |
| `ADR-template-v1.md` | 架构决策记录 | 技术设计阶段 | 架构师、Tech Lead | 稳定 |
| `TDD-template-v2.md` | 技术设计文档（TDD） | 技术设计阶段 | 架构师、开发、测试、运维 | 稳定 |
| `TECH-REVIEW-CHECKLIST-template-v1.md` | 技术方案评审检查清单 | 技术评审 | CTO、架构师、开发、运维、安全 | 稳定 |
| `ARCHITECTURE-DIAGRAMS-template-v1.md` | 架构与流程图统一文档 | 技术设计阶段 | 开发、测试、运维、产品 | 稳定 |
| `SLO-SERVICE-LEVEL-template-v1.md` | 服务等级目标（SLO/SLA） | 技术设计阶段 | SRE、架构师、产品 | 稳定 |
| `DISASTER-RECOVERY-PLAN-template-v1.md` | 灾难恢复方案 | 技术设计阶段 | SRE、架构师、运维 | 稳定 |
| `IMPLEMENTATION-PLAN-template-v1.md` | 开发执行计划 | 开发启动前 | 技术负责人、项目经理、开发 | 稳定 |
| `IMPLEMENTATION-PLAN-ISSUES-template-v1.md` | IMPLEMENTATION-PLAN issue 拆分清单 | 开发启动前 | 技术负责人、项目经理、开发 | 实验 |
| `AGENT-TASK-template-v2.md` | AI/Agent 最小可执行编码任务 | 开发阶段 | 开发、AI Agent、Tech Lead | 稳定 |
| `CODE-REVIEW-template-v1.md` | 代码审查模板 | 开发阶段 | 开发、Tech Lead | 稳定 |
| `DATABASE-MODEL-template-v1.md` | 数据库模型文档 | 技术设计阶段 | 后端、DBA、数据 | 稳定 |
| `API-SPEC-template-v1.md` | API 规范文档（Markdown） | 技术设计阶段 | 前后端、测试、集成方 | 稳定 |
| `API-DEPRECATION-POLICY-template-v1.md` | API 弃用策略 | 技术设计/运维阶段 | API 负责人、后端、产品 | 新引入 |
| `LLM-PROMPT-SPEC-template-v1.md` | LLM Prompt 规范 | 技术设计阶段 | AI 工程师、后端、产品 | 新引入 |
| `openapi-template-v1.yaml` | OpenAPI 规范文档（YAML） | 技术设计阶段 | 前后端、测试、SDK 生成 | 稳定 |
| `EVENT-TRACKING-template-v1.md` | 事件埋点技术规范 | 开发阶段 | 开发、数据、测试 | 稳定 |
| `FEATURE-FLAG-template-v1.md` | 功能开关规范 | 开发/运维阶段 | 开发、SRE、产品 | 新引入 |
| `ENVIRONMENT-CONFIG-template-v1.md` | 环境配置管理 | 开发/运维阶段 | DevOps、SRE、后端 | 新引入 |
| `QA-TEST-PLAN-template-v1.md` | QA 测试计划 | 开发/测试阶段 | QA、开发、产品、运维 | 稳定 |
| `QA-TEST-REPORT-template-v1.md` | QA 测试报告 | 测试完成后 | QA、开发、产品 | 新引入 |
| `PERFORMANCE-TEST-PLAN-template-v1.md` | 性能测试计划 | 开发/测试阶段 | QA、性能、开发、SRE | 新引入 |
| `SECURITY-TEST-PLAN-template-v1.md` | 安全测试计划 | 技术设计/测试阶段 | 安全、QA、开发 | 新引入 |
| `DATA-MIGRATION-template-v1.md` | 数据迁移方案 | 开发/上线前 | 后端、DBA、DevOps | 新引入 |
| `RELEASE-NOTES-template-v1.md` | 发布说明 | 发布前 | 产品、市场、客户成功、运维 | 稳定 |
| `CHANGELOG-template-v1.md` | 变更日志 | 发布后 | 开发、运维、用户 | 新引入 |
| `CHANGE-REQUEST-template-v1.md` | 变更请求 | 基线批准后 | 产品、技术、测试 | 新引入 |
| `DEPLOYMENT-LOG-template-v1.md` | 部署记录 | 部署时 | DevOps、SRE、发布负责人 | 新引入 |
| `ROLLBACK-PLAN-template-v1.md` | 回滚方案 | 上线前 | DevOps、SRE、技术负责人 | 新引入 |
| `INCIDENT-RESPONSE-template-v1.md` | 事件响应报告 | 生产事件后 | 运维、SRE、安全、管理层 | 稳定 |
| `RUNBOOK-template-v1.md` | 运维手册 | 上线前 | 运维、SRE、开发 | 稳定 |
| `THREAT-MODEL-template-v1.md` | 威胁建模文档 | 技术设计/安全评审 | 安全、架构师、开发 | 稳定 |
| `COMPLIANCE-template-v1.md` | 合规文档 | 设计/安全评审 | 合规、安全、法务、产品 | 新引入 |
| `ACCESSIBILITY-CONFORMANCE-template-v1.md` | 无障碍合规文档 | 设计/测试阶段 | 设计、前端、QA、合规 | 稳定 |
| `GTM-template-v1.md` | Go-to-Market 策略文档 | 上市前 | CEO、市场、销售、产品、运营 | 稳定 |
| `GTM-LIGHT-template-v1.md` | Go-to-Market 策略文档（轻量版） | 上市前 | 市场、产品、运营 | 新引入 |
| `USER-FEEDBACK-PLAN-template-v1.md` | 用户反馈计划 | 上市后/持续迭代 | 产品、用户研究、增长 | 新引入 |

## 使用原则

1. **单一职责**：每份文档只回答一类问题，避免文档过度膨胀。
2. **链接引用**：PRD、TDD 等主文档不直接嵌入大量图表，而是链接到 `ARCHITECTURE`、数据库模型等专题文档。
3. **文本优先**：架构图、流程图优先使用 Mermaid，便于 Git 版本控制与 diff。
4. **版本一致**：文档内部版本号、文件名版本号、引用路径版本号应保持一致。
5. **清理占位符**：文档状态转为"已批准"前，必须清除所有 `{占位符}`、`TODO`、`FIXME`。
6. **埋点文档分工**：业务埋点需求先写入 `TRACKING-REQUIREMENTS-template-v1.md`（TRD，回答“为什么埋、埋什么、什么时候埋、分析什么指标”），再由技术团队基于 TRD 输出 `EVENT-TRACKING-template-v1.md`（EVT Spec，回答“怎么埋、字段类型、采集链路、数据质量、保留策略”）。
7. **交叉引用 ID 化**：`templates-manifest.yaml` 为每份模板维护 `related_templates`（模板 ID 列表）。编写实际文档时，推荐在 front matter 的 `linked_docs` 中保留路径，同时在注释或清单中标注对应模板 ID，便于 Agent 和校验脚本解析依赖关系。

## 快速开始：复制到其他项目

### 一键复制全部模板

```bash
cp -R docs/templates /path/to/your-project/docs/templates
```

### 初始化项目文档

1. **复制并重命名**：从本目录选取所需模板，复制到目标项目的 `docs/` 目录，并按 `{TYPE}-vX.Y.Z.md` 格式重命名（例如 `PRD-v1.0.0.md`）。
2. **替换占位符**：全文替换所有 `{占位符}`，确保文档内容贴合实际项目。
3. **召开评审会**：
   - 需求阶段：PRD 评审（可参考 `PRD-REVIEW-CHECKLIST-template-v1.md`）。
   - 技术阶段：技术评审（可参考 `TECH-REVIEW-CHECKLIST-template-v1.md`）。
4. **基础校验**：运行 `python3 docs/templates/scripts/validate_templates.py` 检查模板完整性、版本一致性与残留标记。

### 最少文档集推荐

| 项目规模 | 推荐文档 |
|----------|----------|
| 初创项目 | `PRD` / `TDD` / `IMPLEMENTATION-PLAN` / `AGENT-TASK` / `QA-TEST-PLAN` |
| 企业级项目 | 在初创项目基础上，增加 `ARCHITECTURE` / `DATABASE-MODEL` / `ADR` / `THREAT-MODEL` / `COMPLIANCE` / `SLO` / `DISASTER-RECOVERY` |

## AI / LLM 消费指南

大 PRD / TDD 可能超出 LLM 上下文窗口，建议 Agent 按以下顺序分块消费，并优先读取约束与红线：

1. **先读元数据**：`templates-manifest.yaml`、当前文档的 YAML front matter、`ai_red_flags`、`pending_confirmation`、TDD 第 14 节「待确认事项」。
2. **再读战略与边界**：执行摘要 / 概述与目标 → 范围与边界 → 用户旅程 → 产品原则与设计约束。
3. **然后读实现细节**：功能需求 → 非功能需求 → 数据架构 / 数据库模型 → 接口契约（API-SPEC + OpenAPI）。
4. **最后读执行与风险**：实施计划 / AGENT-TASK → 验收标准 → 风险与缓解 → 合规安全。

**分块策略**：
- 如果文档超过模型窗口，按上述顺序切片，每次优先传递关键约束（最大文件数、最大变更行数、失败用例、边界条件）。
- 不要在单轮 prompt 中塞入整份 PRD + TDD + 多份示例；应通过 `linked_docs` 按需拉取。
- 遇到 `{占位符}` 或 `待确认事项` 时，Agent 必须标记为 `pending_confirmation`，不得擅自假设填充。

## 文档状态规范

所有文档模板统一使用以下状态机：

| 状态 | 说明 |
|------|------|
| 草稿 | 正在编写，未进入评审 |
| 评审中 | 已提交评审，收集反馈 |
| 已批准 | 评审通过，可作为开发基线 |
| 已归档 | 文档过期或被新版本替代 |

特殊模板：
- ADR：`提议 / 已批准 / 已弃用 / 已替代`
- Implementation Plan：`草稿 / 评审中 / 已批准 / 执行中 / 已完成`
- QA/Test Plan/Tracking/UI：可在"已批准"后增加执行态（已执行 / 已交付开发）

## 文档关系

```text
        战略 / 用户研究 / 竞品分析
                    │
                    ▼
            PRD（产品需求文档）
                    │
    ┌───────────────┼───────────────┐
    │               │               │
    ▼               ▼               ▼
  TDD            UI 设计        埋点需求 (TRD)
    │                               │
    ├─▶ ARCHITECTURE（架构与流程图）  │
    ├─▶ DATABASE-MODEL（数据库模型）  │
    ├─▶ API-SPEC / OpenAPI（接口契约）│
    └─▶ ADR（架构决策记录）           │
    │                               │
    └───────────────┬───────────────┘
                    ▼
      PRD + TDD + API + UI + 埋点
                    │
                    ▼
      IMPLEMENTATION-PLAN（开发执行计划）
                    │
    ┌───────────────┼───────────────┐
    │               │               │
    ▼               ▼               ▼
 QA-TEST-PLAN   CODE-REVIEW    EVENT-TRACKING
 （测试计划）      （代码审查）      （埋点实现）
    │               │               │
    └───────────────┬───────────────┘
                    ▼
        开发 / 测试 → 交付物
                    │
                    ▼
    RELEASE-NOTES + RUNBOOK（发布说明 / 运维手册）
                    │
                    ▼
            上线 → GTM / 分析
                    │
        ┌───────────┴───────────┐
        │                       │
        ▼                       ▼
  用户反馈 / 数据 ──────▶ PRD 更新    事故响应 ──────▶ TDD / ADR 更新
        │                       │
        └───────────┬───────────┘
                    │
                    ▼
              【持续迭代】
```

文档关系强调**闭环**：上线后的用户反馈、数据结论和生产事故应反向驱动 PRD、TDD 与 ADR 更新，而不是一次性的瀑布流程。

## 模板选择决策树

```text
项目启动前
├── 需要定义战略方向？ ──▶ STRATEGY-template-v1.md
├── 需要了解用户？ ──▶ USER-RESEARCH-template-v1.md
├── 需要分析竞品？ ──▶ COMPETITIVE-ANALYSIS-template-v1.md
│
需求阶段
├── 编写完整 PRD ──▶ PRD-template-v2.md
├── 小功能 / Bug / 优化用轻量 PRD ──▶ PRD-LIGHT-template-v1.md
├── PRD 评审 ──▶ PRD-REVIEW-CHECKLIST-template-v1.md
├── 定义埋点需求 ──▶ TRACKING-REQUIREMENTS-template-v1.md
│
设计阶段
├── UI/UX 设计交付 ──▶ UI-DESIGN-DELIVERABLE-template-v1.md
├── 设计系统 ──▶ DESIGN-SYSTEM-template-v1.md
├── 前端组件规范 ──▶ FRONTEND-COMPONENT-GUIDE-template-v1.md
│
技术设计阶段
├── 架构决策（单个） ──▶ ADR-template-v1.md
├── 完整技术设计 ──▶ TDD-template-v2.md
├── 定义 SLO/SLA ──▶ SLO-SERVICE-LEVEL-template-v1.md
├── 灾难恢复方案 ──▶ DISASTER-RECOVERY-PLAN-template-v1.md
├── 架构/流程图统一文档 ──▶ ARCHITECTURE-DIAGRAMS-template-v1.md
├── 数据库模型 ──▶ DATABASE-MODEL-template-v1.md
├── API 规范（Markdown） ──▶ API-SPEC-template-v1.md
├── API 规范（OpenAPI） ──▶ openapi-template-v1.yaml
├── 技术方案评审 ──▶ TECH-REVIEW-CHECKLIST-template-v1.md
│
开发启动前
├── 开发执行计划 ──▶ IMPLEMENTATION-PLAN-template-v1.md
├── 拆分为可执行 issues ──▶ IMPLEMENTATION-PLAN-ISSUES-template-v1.md
│
开发/AI 编码阶段
└── 最小可执行 agent task ──▶ AGENT-TASK-template-v2.md
│
开发/测试阶段
├── 埋点技术规范 ──▶ EVENT-TRACKING-template-v1.md
├── 功能开关规范 ──▶ FEATURE-FLAG-template-v1.md
├── 环境配置管理 ──▶ ENVIRONMENT-CONFIG-template-v1.md
├── 测试计划 ──▶ QA-TEST-PLAN-template-v1.md
├── 代码审查 ──▶ CODE-REVIEW-template-v1.md
├── 威胁建模 ──▶ THREAT-MODEL-template-v1.md
└── 无障碍合规 ──▶ ACCESSIBILITY-CONFORMANCE-template-v1.md

发布/上线阶段
├── 发布说明 ──▶ RELEASE-NOTES-template-v1.md
├── 运维手册 ──▶ RUNBOOK-template-v1.md
└── 上线验证 ──▶ QA-TEST-PLAN-template-v1.md

运营/上市后
├── 已批准基线后需要变更控制？ ──▶ CHANGE-REQUEST-template-v1.md
├── Go-to-Market ──▶ GTM-template-v1.md
└── 事件响应 ──▶ INCIDENT-RESPONSE-template-v1.md
```

## 命名规范

- Markdown 模板：`{TYPE}-template-v{N}.md`
- OpenAPI 模板：`openapi-template-v{N}.yaml`
- 实际文档：`{TYPE}-v{X.Y.Z}.md` / `openapi-v{X.Y.Z}.yaml`

### 版本号说明

- **模板版本**：文件名中的 `v{N}`，表示模板本身的迭代版本（如 `PRD-template-v2.md` 的模板版本为 `v2`）。
- **文档版本**：文档控制块中的 `vX.Y.Z`，表示使用该模板编写的具体文档实例版本（如某次 PRD 为 `v2.0.0`）。
- 两者不要混用：模板升级时改文件名版本；文档迭代时改文档控制块中的版本。

## 待补齐模板

以下模板被现有模板引用，建议后续补充：

## 机器可读清单与校验

- 模板元数据统一维护在 [`templates-manifest.yaml`](templates-manifest.yaml)，机器可读 Schema 见 [`templates-schema.json`](templates-schema.json)。
- 运行 `python3 docs/templates/scripts/validate_templates.py` 可校验：
  - 清单与 Schema 是否一致；
  - 清单中的模板文件是否存在；
  - 模板头部是否包含 `模板版本`；
  - 文件名版本与头部版本是否一致；
  - YAML front matter 是否包含所有 `required_frontmatter_fields`；
  - `related_templates` 是否引用有效的模板 ID；
  - 是否存在未清理的 `TODO` / `FIXME` 标记（排除检查清单文本中的正常引用）；
  - `examples/` 中的示例文档是否仍残留模板级 `{占位符}`。

## 示例

`examples/` 目录提供已填写的参考实例：

| 示例文件 | 对应模板 |
|----------|----------|
| `examples/PRD-example-v1.0.0.md` | `PRD-template-v2.md` |
| `examples/PRD-ecommerce-example-v1.0.0.md` | `PRD-template-v2.md`（电商物流领域） |
| `examples/PRD-crm-example-v1.0.0.md` | `PRD-template-v2.md`（CRM 销售管理领域） |
| `examples/PRD-saas-billing-example-v1.0.0.md` | `PRD-template-v2.md`（SaaS 订阅计费领域） |
| `examples/TDD-example-v1.0.0.md` | `TDD-template-v2.md` |
| `examples/TDD-crm-example-v1.0.0.md` | `TDD-template-v2.md`（CRM 销售管理领域） |
| `examples/TDD-saas-billing-example-v1.0.0.md` | `TDD-template-v2.md`（SaaS 订阅计费领域） |
| `examples/DATABASE-MODEL-example-v1.0.0.md` | `DATABASE-MODEL-template-v1.md` |
| `examples/DATABASE-MODEL-crm-example-v1.0.0.md` | `DATABASE-MODEL-template-v1.md`（CRM 销售管理领域） |
| `examples/DATABASE-MODEL-saas-billing-example-v1.0.0.md` | `DATABASE-MODEL-template-v1.md`（SaaS 订阅计费领域） |
| `examples/API-SPEC-example-v1.0.0.md` | `API-SPEC-template-v1.md` |
| `examples/API-SPEC-crm-example-v1.0.0.md` | `API-SPEC-template-v1.md`（CRM 销售管理领域） |
| `examples/API-SPEC-saas-billing-example-v1.0.0.md` | `API-SPEC-template-v1.md`（SaaS 订阅计费领域） |
| `examples/QA-TEST-PLAN-example-v1.0.0.md` | `QA-TEST-PLAN-template-v1.md` |
| `examples/QA-TEST-PLAN-crm-example-v1.0.0.md` | `QA-TEST-PLAN-template-v1.md`（CRM 销售管理领域） |
| `examples/QA-TEST-PLAN-saas-billing-example-v1.0.0.md` | `QA-TEST-PLAN-template-v1.md`（SaaS 订阅计费领域） |
| `examples/ADR-ecommerce-example-v1.0.0.md` | `ADR-template-v1.md`（电商数据库选型） |
| `examples/ADR-crm-example-v1.0.0.md` | `ADR-template-v1.md`（CRM 租户隔离方案） |
| `examples/ADR-saas-billing-example-v1.0.0.md` | `ADR-template-v1.md`（SaaS 计费冲正模型） |
| `examples/AGENT-TASK-example-v1.0.0.md` | `AGENT-TASK-template-v2.md` |
| `examples/IMPLEMENTATION-PLAN-example-v1.0.0.md` | `IMPLEMENTATION-PLAN-template-v1.md` |

## 维护

- 新增模板需更新本 README、文档关系图和 [`templates-manifest.yaml`](templates-manifest.yaml)。
- 废弃模板应及时移入 `archive/` 目录，并在 `archive/README.md` 中记录废弃原因和替代模板。

## 持续集成

建议将模板校验脚本接入 CI，确保任何对 `docs/templates/` 的修改都会自动触发基础检查。

### 推荐校验内容

- **模板版本一致性**：文件名版本、模板头部版本、清单版本保持一致。
- **TODO/FIXME 残留**：文档状态转为“已批准”前，不应残留待办标记。
- **必填字段缺失**：检查 YAML front matter 是否包含 `required_frontmatter_fields` 中声明的字段。
- **占位符残留**：`examples/` 中的示例应全部替换，模板本身允许保留 `{占位符}`。
- **交叉链接有效性**：`related_templates` 与 `linked_docs` 引用的模板/文档必须存在。

### GitHub Actions 示例

项目已配置 `.github/workflows/validate-templates.yaml`，会自动在 PR / push 时触发。

### 复制到其他项目

```bash
python3 docs/templates/scripts/copy-templates-to-project.py \
  --target /path/to/your-project/docs/templates \
  --product-name "Your Product" \
  --project-prefix YP \
  --module-name "Core Module" \
  --feature-name "Key Feature" \
  --project-name "Project Name" \
  --brand-name "Brand Name" \
  --system-name "System Name" \
  --company-name "ExampleOrg Inc." \
  --org-identifier "example-org"
```

该脚本会：
- 复制全部模板、示例、manifest 与 schema（`archive/` 不会被复制，历史版本仅在源仓库保留）；
- 把 `{产品名}`、`{模块名}`、`{系统名}`、`{品牌名}`、`{项目名称}`、`{功能名称}`、`{公司名}`、`{组织标识}`、`{项目前缀}` 等替换为目标项目值；
- 将 `validate_templates.py` 复制到目标项目的 `scripts/` 目录；
- 输出残留的 `{占位符}` 供人工复核。

额外选项：

```bash
# 不复制 examples/ 目录
python3 docs/templates/scripts/copy-templates-to-project.py \
  --target /path/to/your-project/docs/templates \
  --product-name "Your Product" \
  --project-prefix YP \
  --skip-examples

# 只复制指定领域的示例（可重复传入，generic 表示无领域后缀的通用示例）
python3 docs/templates/scripts/copy-templates-to-project.py \
  --target /path/to/your-project/docs/templates \
  --product-name "Your Product" \
  --project-prefix YP \
  --select-domain crm \
  --select-domain saas-billing

# 只复制通用示例
python3 docs/templates/scripts/copy-templates-to-project.py \
  --target /path/to/your-project/docs/templates \
  --product-name "Your Product" \
  --project-prefix YP \
  --select-domain generic
```

### 本地手动执行

```bash
# 安装校验依赖
pip install pyyaml jsonschema openapi-spec-validator

# 校验模板元数据、front matter、交叉引用、占位符
python3 docs/templates/scripts/validate_templates.py

# 校验 OpenAPI 语义
openapi-spec-validator docs/templates/openapi-template-v1.yaml
```

CI workflow 示例见仓库 `.github/workflows/validate-templates.yaml`；复制到目标项目后，请根据实际 CI 平台调整。核心校验步骤如下：

```yaml
- name: Validate templates
  run: python3 docs/templates/scripts/validate_templates.py
- name: Validate OpenAPI template
  run: openapi-spec-validator docs/templates/openapi-template-v1.yaml
- name: Validate copy-to-project script
  run: |
    python3 docs/templates/scripts/copy-templates-to-project.py \
      --target /tmp/template-copy-test/docs/templates \
      --product-name "Test Product" \
      --project-prefix TP
    python3 /tmp/template-copy-test/docs/templates/scripts/validate_templates.py
```

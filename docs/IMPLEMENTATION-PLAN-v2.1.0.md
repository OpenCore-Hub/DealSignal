---
id: "IP-2024-001"
version: "v2.1.0"
status: "已批准"
owner: "技术负责人 / 项目经理"
linked_docs:
  - "docs/PRD-v2.1.0.md"
  - "docs/TDD-v2.1.0.md"
  - "docs/ARCHITECTURE-v2.1.0.md"
  - "docs/database-model-v2.1.0.md"
---

# DealSignal 开发执行计划 v2.1.0

> **资源编号**：`IP-2024-001`  
> **版本**：`v2.1.0`  
> **模板版本**：`v1`  
> **状态**：`已批准`  
> **编写人/适用对象**：`技术负责人 / 项目经理 / Scrum Master`  
> **编写日期**：`2026-06-20`  
> **关联资源**：  
> - `docs/PRD-v2.1.0.md`  
> - `docs/TDD-v2.1.0.md`  
> - `docs/ARCHITECTURE-v2.1.0.md`  
> - `docs/database-model-v2.1.0.md`  
> - `docs/templates/PRD-REVIEW-CHECKLIST-template-v1.md`  
> - `docs/templates/TECH-REVIEW-CHECKLIST-template-v1.md`  
> - `docs/templates/QA-TEST-PLAN-template-v1.md`  
> - `docs/templates/CODE-REVIEW-template-v1.md`  
> **评审人**：`CTO、技术负责人、产品负责人、测试负责人、项目经理`  
> **执行状态（IMPLEMENTATION-PLAN 专用）**：`未开始`

---

## 0. 资源使用说明

本资源是 **DealSignal** 的**开发执行计划（Implementation Plan）**，用于将已批准的 PRD、TDD、架构图、数据库模型转化为工程师可逐条执行的任务清单。

**核心目标**：
1. **纵向贯通**：每个开发任务都能追溯到 PRD 功能需求、TDD 技术方案、API 契约、测试用例、埋点事件。
2. **横向拆解**：按模块/服务/端拆解任务，明确负责人、依赖、验收标准。
3. **拓扑排序**：任务按依赖关系编排，避免工程师被阻塞或做无效返工。
4. **质量内建**：每个任务自带 Definition of Done（DoD），确保交付即合格。

**本资源与其他资源的关系**：

```text
PRD（做什么）
       │
┌──────┼──────┐
│      │      │
▼      ▼      ▼
TDD   UI设计   埋点需求
 │
 ├─▶ ARCHITECTURE / DATABASE / API / ADR
 │
 ▼
IMPLEMENTATION PLAN（怎么执行）◀── 本资源
 │
 ├─▶ Sprint 任务板
 ├─▶ 代码仓库
 ├─▶ 测试计划
 └─▶ 埋点/监控/运维
```

**读者对象**：
- 技术负责人 / 架构师
- 前后端开发工程师
- 测试工程师 / QA
- 产品经理 / 项目经理
- DevOps / SRE

---

## 1. 资源控制信息

### 1.1 变更日志

| 版本 | 日期 | 修改人 | 修改内容 | 影响范围 |
|------|------|--------|----------|----------|
| v2.1.0 | 2026-06-20 | 技术负责人 / 项目经理 | 按 IMPLEMENTATION-PLAN-template-v1 创建 DealSignal v2.1.0 执行计划，继承 PRD 第 14 节任务拆分并补充 TDD 模块关联 | 全资源 |

### 1.2 关联任务板

| 工具 | 链接 | 说明 |
|------|------|------|
| 项目管理 | `{待配置}` | GitHub Projects / Linear / Jira |
| 代码仓库 | `{待配置}` | GitHub / GitLab |
| CI/CD | `{待配置}` | GitHub Actions / GitLab CI |
| 资源库 | `{待配置}` | Notion / Confluence |

---

## 2. 执行原则

### 2.1 任务编号规范

`TASK-{模块缩写}-{NNN}`

| 模块缩写 | 说明 |
|----------|------|
| `INFRA` | 基础设施 / 运维 |
| `AUTH` | 认证授权 / 租户 / Workspace |
| `UPLOAD` | 资源上传 |
| `INGEST` | 资源解析 / Ingestion |
| `PUBLIC` | Public 资源访问 / Canvas |
| `SEARCH` | 搜索 / Evidence |
| `AI` | 智能助手 |
| `LINK` | 链接与权限 |
| `ROOM` | 协作空间 / 数据室 |
| `ANALYTICS` | 分析 |
| `NOTIFY` | 通知 |
| `INTEG` | 第三方集成 |
| `WEB` | Web 前端 |
| `TEST` | 测试 |
| `SEC` | 安全 |

### 2.2 任务状态

| 状态 | 说明 |
|------|------|
| 待开始 | 已规划，未进入开发 |
| 开发中 | 工程师正在实现 |
| 代码审查中 | PR 已提交，等待 Review |
| 测试中 | QA 验证中 |
| 已验收 | 通过所有质量门禁 |
| 已上线 | 已合并发布 |
| 阻塞 | 因依赖/问题暂停 |

### 2.3 Definition of Done（DoD）

每个任务必须满足以下通用 DoD，再根据任务类型补充专项 DoD：

- [ ] 代码实现符合 TDD 设计
- [ ] 单元测试通过，核心逻辑覆盖率 ≥ 80%
- [ ] 代码审查通过（至少 1 名资深工程师 Approve）
- [ ] 与关联 API 契约一致
- [ ] 与关联 PRD 验收标准对齐
- [ ] 无 P0/P1 缺陷遗留
- [ ] 资源已更新（API 资源、CHANGELOG、ADR 等）

### 2.4 依赖管理

| 依赖类型 | 说明 | 示例 |
|----------|------|------|
| 强依赖 | 必须先完成，否则无法开始 | 必须先有 `tenants/workspaces` 才能开发资源上传 |
| 弱依赖 | 可并行，但需约定接口 | 前端 Public Viewer 与后端 Public API |
| 外部依赖 | 依赖第三方或基础设施 | Cloudflare 账号、域名备案、OSS bucket |

---

## 3. 里程碑与阶段

### 3.1 里程碑规划

| 里程碑 | 目标日期 | 核心交付 | 成功标准 |
|--------|----------|----------|----------|
| M0：PRD 评审通过 | 2026-06-25 | 已批准 PRD-v2.1.0 | 所有关键方签字 |
| M1：技术方案确认 | 2026-07-02 | TDD-v2.1.0 + ARCHITECTURE-v2.1.0 + database-model-v2.1.0 | 架构评审通过 |
| M2：设计稿确认 | 2026-07-09 | 高保真设计稿 + 交互原型 | 产品+设计确认 |
| M3：基础服务完成 | 2026-07-23 | 上传、解析、存储、签名 URL | 自测通过 |
| M4：核心链路完成 | 2026-08-13 | 上传 → 查看 → AI 问答可跑通 | 集成测试通过 |
| M5：功能开发完成 | 2026-08-27 | 所有 P0 功能代码合并 | 自测 + 接口测试通过 |
| M6：测试通过 | 2026-09-10 | 测试报告 | P0/P1 用例 100% 通过 |
| M7：内测上线 | 2026-09-17 | 20 个种子用户 | 核心指标无异常 |
| M8：灰度发布 | 2026-09-24 | 10% 流量 | 监控稳定 48h |
| M9：正式上线 | 2026-09-30 | 全量 | 监控稳定 24h |

### 3.2 阶段划分

#### Phase 0：Sprint 0（基础准备）

| 目标 | 说明 |
|------|------|
| 工程脚手架 | 后端项目结构、前端项目结构、共享包、工具链 |
| 基础设施 | 阿里云 ACK / AWS EKS、RDS PostgreSQL + pgvector、Redis、OSS/S3、Cloudflare、域名 |
| CI/CD | 构建、测试、lint、安全扫描流水线 |
| 基础模块 | 租户、用户、Workspace、认证授权 |
| 资源基线 | 所有前置资源冻结，开发执行计划批准 |

#### Phase 1：核心资源链路

| 目标 | 说明 |
|------|------|
| 上传服务 | 文件上传、校验、hash、OSS 写入 |
| Ingestion | PDF / Office 解析、索引构建、元数据提取 |
| 数据模型 | 文档、页面、chunks、boxes 表与索引 |
| Public | 签名 URL、Canvas 渲染、页面切换 |

#### Phase 2：协作与智能

| 目标 | 说明 |
|------|------|
| 链接与权限 | 公开链接、密码、邮箱白名单、过期、撤销 |
| 智能助手 | Hybrid search、Evidence、LLM 回答、高亮 |
| 数据室 | 多资源、成员邀请、权限管理、NDA |

#### Phase 3：分析与集成

| 目标 | 说明 |
|------|------|
| 行为分析 | 事件采集、热度评分、Dashboard |
| 通知 | 邮件、Slack |
| CRM 集成 | HubSpot/Salesforce 同步 |

---

## 4. 任务追踪矩阵

### 4.1 矩阵结构

| 任务编号 | 任务名称 | 模块 | 优先级 | 负责人 | 依赖 | PRD | TDD | API | 测试 | 埋点 | 状态 |
|----------|----------|------|--------|--------|------|-----|-----|-----|------|------|------|
| TASK-AUTH-001 | 用户认证、租户与 Workspace 模块 | AUTH | P0 | 后端 | - | FR-01 ~ FR-02 | 6.1 | API-01 ~ API-04 | TC-AUTH-001 ~ TC-AUTH-004 | EVT-01 ~ EVT-03 | 待开始 |
| TASK-INFRA-001 | 对象存储与后端签名 URL / Cloudflare URL Signing | INFRA | P0 | 后端/运维 | 云账号 | FR-03 | 3.2 / 7.3 / 7.4 | API-06 | TC-INFRA-001 | - | 待开始 |
| TASK-INFRA-002 | 子域名/自定义域名与 SSL 自动签发 | INFRA | P0 | 后端/运维 | 云账号 | FR-03 | 3.2 / 7.4 | API-15 | TC-INFRA-002 | - | 待开始 |
| TASK-UPLOAD-001 | 文档上传 API | UPLOAD | P0 | 后端 | TASK-AUTH-001、TASK-INFRA-001 | FR-02 | 6.1 | API-05 | TC-UPLOAD-001 ~ TC-UPLOAD-004 | EVT-04 | 待开始 |
| TASK-INGEST-001 | PDF Pipeline（bbox + webp） | INGEST | P0 | 后端 | TASK-UPLOAD-001 | FR-02 | 6.2 | - | TC-INGEST-001 ~ TC-INGEST-003 | EVT-05 | 待开始 |
| TASK-INGEST-002 | Office Pipeline（OnlyOffice 转 PDF） | INGEST | P0 | 后端 | TASK-INGEST-001 | FR-02 | 6.2 | - | TC-INGEST-004 ~ TC-INGEST-006 | EVT-05 | 待开始 |
| TASK-DATA-001 | 数据库与搜索索引 | DATA | P0 | 后端/DBA | TASK-INGEST-001 | FR-02 | 4.x / 8.x | - | TC-DATA-001 | - | 待开始 |
| TASK-PUBLIC-001 | 签名 URL 与权限校验 | PUBLIC | P0 | 后端 | TASK-INFRA-001、TASK-DATA-001 | FR-03 | 6.3 / 7.4 | API-06 ~ API-08 | TC-PUBLIC-001 ~ TC-PUBLIC-004 | EVT-06 | 待开始 |
| TASK-WEB-001 | Viewer Canvas 前端 | WEB | P0 | 前端 | TASK-PUBLIC-001 | FR-03 ~ FR-04 | 6.3 | API-06 ~ API-08 | TC-WEB-001 ~ TC-WEB-004 | EVT-07 | 待开始 |
| TASK-SEARCH-001 | Search Service | SEARCH | P0 | 后端 | TASK-DATA-001 | FR-05 | 6.4 | API-09 | TC-SEARCH-001 ~ TC-SEARCH-003 | EVT-08 | 待开始 |
| TASK-SEARCH-002 | Evidence Service | SEARCH | P0 | 后端 | TASK-SEARCH-001 | FR-05 | 6.5 | API-10 | TC-SEARCH-004 | EVT-08 | 待开始 |
| TASK-AI-001 | Assistant Service | AI | P0 | 后端 | TASK-SEARCH-002 | FR-05 ~ FR-06 | 6.6 | API-11 ~ API-12 | TC-AI-001 ~ TC-AI-004 | EVT-09 | 待开始 |
| TASK-WEB-002 | 悬浮 AI 助手前端 | WEB | P0 | 前端 | TASK-WEB-001、TASK-AI-001 | FR-05 ~ FR-06 | 6.3 / 6.6 | API-11 ~ API-12 | TC-WEB-005 ~ TC-WEB-007 | EVT-09 | 待开始 |
| TASK-LINK-001 | 智能链接与权限 | LINK | P0 | 后端 | TASK-PUBLIC-001 | FR-07 ~ FR-09 | 6.7 | API-13 ~ API-14 | TC-LINK-001 ~ TC-LINK-005 | EVT-10 | 待开始 |
| TASK-WEB-003 | Dashboard 前端 | WEB | P0 | 前端 | TASK-LINK-001 | FR-10 ~ FR-11 | 11.2 | API-16 ~ API-18 | TC-WEB-008 ~ TC-WEB-010 | EVT-11 | 待开始 |
| TASK-ANALYTICS-001 | 热度评分与 Analytics | ANALYTICS | P1 | 后端 | TASK-WEB-001 | FR-10 | 6.8 | API-16 | TC-ANALYTICS-001 ~ TC-ANALYTICS-003 | EVT-11 | 待开始 |
| TASK-ANALYTICS-002 | 行为提醒与跟进建议 | ANALYTICS | P1 | 后端 | TASK-ANALYTICS-001 | FR-11 | 6.8 | API-17 | TC-ANALYTICS-004 | EVT-12 | 待开始 |
| TASK-ROOM-001 | 数据室模块 | ROOM | P1 | 后端 + 前端 | TASK-DATA-001、TASK-LINK-001 | FR-12 ~ FR-13 | 6.10 | API-19 ~ API-22 | TC-ROOM-001 ~ TC-ROOM-006 | EVT-13 | 待开始 |
| TASK-NOTIFY-001 | 邮件通知系统 | NOTIFY | P1 | 后端 | 邮件服务 | FR-14 | 6.9 | API-23 | TC-NOTIFY-001 ~ TC-NOTIFY-003 | EVT-14 | 待开始 |
| TASK-INTEG-001 | CRM 集成（HubSpot/Salesforce） | INTEG | P2 | 后端 | TASK-ANALYTICS-001 | FR-15 | 6.11 | API-24 | TC-INTEG-001 | EVT-15 | 待开始 |
| TASK-INTEG-002 | Slack 集成 | INTEG | P2 | 后端 | TASK-ANALYTICS-001 | FR-16 | 6.11 | API-25 | TC-INTEG-002 | EVT-16 | 待开始 |
| TASK-TEST-001 | 测试用例与自动化 | TEST | P0 | 测试 | PRD 评审 | 第 12 节 AC | 第 10 节 | - | TC-ALL | - | 待开始 |
| TASK-TEST-002 | 性能压测与优化 | TEST | P1 | 测试/开发 | 功能开发完成 | 第 9 节 NFR | 第 8 节 | - | TC-PERF | - | 待开始 |
| TASK-SEC-001 | 安全扫描与修复 | SEC | P0 | 安全团队 | 开发完成 | 第 17 节 | 第 7 节 | - | TC-SEC | - | 待开始 |

### 4.2 矩阵说明

| 列 | 说明 |
|----|------|
| 任务编号 | 唯一标识，便于在任务板、PR、提交信息中引用 |
| 任务名称 | 一句话描述 |
| 模块 | 所属模块/服务 |
| 优先级 | P0/P1/P2 |
| 负责人 | 主负责工程师 |
| 依赖 | 前置任务编号 |
| PRD | 关联的功能需求编号 |
| TDD | 关联的技术设计章节 |
| API | 关联的接口编号 |
| 测试 | 关联的测试用例编号 |
| 埋点 | 关联的埋点事件编号 |
| 状态 | 当前任务状态 |

---

## 5. 详细任务清单

### 5.1 Phase 0：Sprint 0（基础设施与基础模块）

#### TASK-AUTH-001 用户认证、租户与 Workspace 模块

| 属性 | 内容 |
|------|------|
| 任务名称 | 用户认证、租户与 Workspace 模块 |
| 模块 | AUTH |
| 优先级 | P0 |
| 负责人 | `{后端负责人}` |
| 依赖 | - |
| PRD | FR-01 ~ FR-02，第 5、6、10 节 |
| TDD | 第 4.2 节、第 6.1 节、第 7.1 ~ 7.2 节 |
| API | API-01 ~ API-04 |
| 说明 | 实现 tenants / users / workspaces / workspace_members 表；JWT/Session 认证；Workspace 切换与上下文注入；子域名/自定义域名解析 |
| 验收标准 | 1. 用户可注册/登录/切换 Workspace<br>2. 所有 API 自动携带 tenant_id + workspace_id<br>3. 跨 Workspace 越权访问被拒绝 |
| DoD | [ ] 代码实现 [ ] 单元测试 ≥ 80% [ ] Code Review [ ] 接口契约一致 [ ] 与 AC 对齐 |
| 预计工时 | 6 天 |

#### TASK-INFRA-001 对象存储与后端签名 URL / Cloudflare URL Signing

| 属性 | 内容 |
|------|------|
| 任务名称 | 对象存储与后端签名 URL / Cloudflare URL Signing |
| 模块 | INFRA |
| 优先级 | P0 |
| 负责人 | `{后端/运维负责人}` |
| 依赖 | 云账号 |
| PRD | FR-03，第 6.3、10.2 节 |
| TDD | 第 3.2 节、第 7.3 ~ 7.4 节 |
| API | API-06 |
| 说明 | 创建私有 OSS/S3 bucket；实现后端签名 URL 签发；配置 Cloudflare URL Signing；密钥存储与轮转机制 |
| 验收标准 | 1. 私有 bucket 不可直接访问<br>2. 签名 URL 有效期可配置<br>3. 篡改 token 后访问被拒绝 |
| DoD | [ ] 基础设施代码化 [ ] 签名验证测试 [ ] 密钥管理文档 [ ] 成本估算 |
| 预计工时 | 4 天 |

#### TASK-INFRA-002 子域名/自定义域名与 SSL 自动签发

| 属性 | 内容 |
|------|------|
| 任务名称 | 子域名/自定义域名与 SSL 自动签发 |
| 模块 | INFRA |
| 优先级 | P0 |
| 负责人 | `{后端/运维负责人}` |
| 依赖 | 云账号、域名 |
| PRD | FR-03，第 6.3、10.2 节 |
| TDD | 第 3.2 节、第 7.4 节 |
| API | API-15 |
| 说明 | 实现子域名解析；自定义域名 CNAME 验证；Let's Encrypt SSL 自动签发与监控 |
| 验收标准 | 1. 新 Workspace 自动生成子域名<br>2. 自定义域名 CNAME 验证通过后可访问<br>3. 证书过期前自动续期 |
| DoD | [ ] DNS 自动化 [ ] SSL 监控告警 [ ] 文档更新 |
| 预计工时 | 3 天 |

---

### 5.2 Phase 1：核心资源链路

#### TASK-UPLOAD-001 文档上传 API

| 属性 | 内容 |
|------|------|
| 任务名称 | 文档上传 API |
| 模块 | UPLOAD |
| 优先级 | P0 |
| 负责人 | `{后端负责人}` |
| 依赖 | TASK-AUTH-001、TASK-INFRA-001 |
| PRD | FR-02，第 8.1 ~ 8.2 节 |
| TDD | 第 6.1 节 |
| API | API-05 |
| 说明 | 实现分片上传、文件校验（magic + 扩展名）、hash 去重、OSS 直传/后端代理、创建 ingestion_job |
| 验收标准 | 1. 支持 PDF / Office / 图片上传<br>2. 大文件分片上传成功<br>3. 重复文件触发秒传<br>4. 上传事件上报 |
| DoD | [ ] 代码实现 [ ] 单元测试 [ ] 集成测试 [ ] Code Review |
| 预计工时 | 4 天 |

#### TASK-INGEST-001 PDF Pipeline（bbox + webp）

| 属性 | 内容 |
|------|------|
| 任务名称 | PDF Pipeline（bbox + webp） |
| 模块 | INGEST |
| 优先级 | P0 |
| 负责人 | `{后端/AI 负责人}` |
| 依赖 | TASK-UPLOAD-001 |
| PRD | FR-02，第 8.1 节 |
| TDD | 第 6.2 节 |
| API | - |
| 说明 | PDF 解析、页面转 webp、文本块/图片块提取、bbox 计算、生成 document_chunks / chunk_boxes / document_blocks |
| 验收标准 | 1. 解析成功率 ≥ 95%<br>2. bbox 坐标采用 PAGE_IMAGE_NORMALIZED<br>3. webp 渲染与原文一致 |
| DoD | [ ] 代码实现 [ ] 单元测试 [ ] 失败样本库 [ ] Code Review |
| 预计工时 | 7 天 |

#### TASK-INGEST-002 Office Pipeline（OnlyOffice 转 PDF）

| 属性 | 内容 |
|------|------|
| 任务名称 | Office Pipeline（OnlyOffice 转 PDF） |
| 模块 | INGEST |
| 优先级 | P0 |
| 负责人 | `{后端负责人}` |
| 依赖 | TASK-INGEST-001 |
| PRD | FR-02，第 8.1 节 |
| TDD | 第 6.2 节 |
| API | - |
| 说明 | 调用 OnlyOffice 自托管集群将 Office 文档转为 PDF；失败时降级到 LibreOffice；转换缓存清理 |
| 验收标准 | 1. Word/Excel/PPT 转换成功率 ≥ 95%<br>2. 转换失败可重试/降级<br>3. 转换任务可监控 |
| DoD | [ ] OnlyOffice 集成 [ ] 降级方案 [ ] 监控 [ ] Code Review |
| 预计工时 | 5 天 |

#### TASK-DATA-001 数据库与搜索索引

| 属性 | 内容 |
|------|------|
| 任务名称 | 数据库与搜索索引 |
| 模块 | DATA |
| 优先级 | P0 |
| 负责人 | `{后端/DBA}` |
| 依赖 | TASK-INGEST-001 |
| PRD | FR-02，第 10.1 节 |
| TDD | 第 4.x 节、第 8.x 节 |
| API | - |
| 说明 | 创建所有业务表、索引、约束、分区表；初始化 pgvector 扩展；HNSW / GIN 索引；迁移脚本 |
| 验收标准 | 1. 所有表包含 tenant_id + workspace_id<br>2. 索引与 TDD/database-model 一致<br>3. 迁移脚本可回滚 |
| DoD | [ ] DDL [ ] 迁移脚本 [ ] 索引验证 [ ] DBA Review |
| 预计工时 | 5 天 |

#### TASK-PUBLIC-001 签名 URL 与权限校验

| 属性 | 内容 |
|------|------|
| 任务名称 | 签名 URL 与权限校验 |
| 模块 | PUBLIC |
| 优先级 | P0 |
| 负责人 | `{后端负责人}` |
| 依赖 | TASK-INFRA-001、TASK-DATA-001 |
| PRD | FR-03，第 8.3 ~ 8.4 节 |
| TDD | 第 6.3 节、第 7.4 节 |
| API | API-06 ~ API-08 |
| 说明 | Public API 实现签名 URL 签发与校验；visitor_id 生成；访问权限校验；水印元信息签名 |
| 验收标准 | 1. 公开链接携带 token 可访问<br>2. 无 token/过期 token 被拒绝<br>3. 权限不足返回 403 |
| DoD | [ ] 代码实现 [ ] 安全测试 [ ] Code Review |
| 预计工时 | 5 天 |

#### TASK-WEB-001 Viewer Canvas 前端

| 属性 | 内容 |
|------|------|
| 任务名称 | Viewer Canvas 前端 |
| 模块 | WEB |
| 优先级 | P0 |
| 负责人 | `{前端负责人}` |
| 依赖 | TASK-PUBLIC-001 |
| PRD | FR-03 ~ FR-04，第 11 节 |
| TDD | 第 6.3 节 |
| API | API-06 ~ API-08 |
| 说明 | Canvas 渲染 webp 页面；缩放/翻页/搜索高亮；动态水印绘制；行为事件上报 |
| 验收标准 | 1. 页面清晰渲染<br>2. 水印不可轻易去除<br>3. 阅读事件精确上报 |
| DoD | [ ] 代码实现 [ ] 前端测试 [ ] Code Review |
| 预计工时 | 8 天 |

---

### 5.3 Phase 2：协作与智能

#### TASK-SEARCH-001 Search Service

| 属性 | 内容 |
|------|------|
| 任务名称 | Search Service |
| 模块 | SEARCH |
| 优先级 | P0 |
| 负责人 | `{后端/AI 负责人}` |
| 依赖 | TASK-DATA-001 |
| PRD | FR-05，第 8.5 节 |
| TDD | 第 6.4 节 |
| API | API-09 |
| 说明 | Hybrid search（exact + full-text + vector）、RRF 融合、结果过滤 |
| 验收标准 | 1. 三路召回可配置权重<br>2. 结果按相关度排序<br>3. P95 延迟 ≤ 800ms |
| DoD | [ ] 代码实现 [ ] 单元测试 [ ] 性能测试 [ ] Code Review |
| 预计工时 | 6 天 |

#### TASK-SEARCH-002 Evidence Service

| 属性 | 内容 |
|------|------|
| 任务名称 | Evidence Service |
| 模块 | SEARCH |
| 优先级 | P0 |
| 负责人 | `{后端/AI 负责人}` |
| 依赖 | TASK-SEARCH-001 |
| PRD | FR-05，第 8.5 节 |
| TDD | 第 6.5 节 |
| API | API-10 |
| 说明 | 将搜索结果转换为带 bbox/page 的 evidence；坐标换算；高亮定位 |
| 验收标准 | 1. evidence 可定位到原文具体位置<br>2. bbox 坐标准确<br>3. 多 evidence 排序合理 |
| DoD | [ ] 代码实现 [ ] 单元测试 [ ] Code Review |
| 预计工时 | 4 天 |

#### TASK-AI-001 Assistant Service

| 属性 | 内容 |
|------|------|
| 任务名称 | Assistant Service |
| 模块 | AI |
| 优先级 | P0 |
| 负责人 | `{后端/AI 负责人}` |
| 依赖 | TASK-SEARCH-002 |
| PRD | FR-05 ~ FR-06，第 8.5 节 |
| TDD | 第 6.6 节 |
| API | API-11 ~ API-12 |
| 说明 | LLM 调用、上下文管理、evidence 引用、答案生成、moderation 过滤 |
| 验收标准 | 1. 所有回答附带 evidence<br>2. 禁止凭空生成<br>3. 触发敏感词时友好提示 |
| DoD | [ ] 代码实现 [ ] 单元测试 [ ] 安全 review [ ] Code Review |
| 预计工时 | 5 天 |

#### TASK-WEB-002 悬浮 AI 助手前端

| 属性 | 内容 |
|------|------|
| 任务名称 | 悬浮 AI 助手前端 |
| 模块 | WEB |
| 优先级 | P0 |
| 负责人 | `{前端负责人}` |
| 依赖 | TASK-WEB-001、TASK-AI-001 |
| PRD | FR-05 ~ FR-06，第 11 节 |
| TDD | 第 6.3、6.6 节 |
| API | API-11 ~ API-12 |
| 说明 | 悬浮助手 UI；提问/回答/evidence 高亮跳转；首次使用引导气泡；访客限制 |
| 验收标准 | 1. 默认收起，不遮挡文档<br>2. evidence 点击跳转并高亮<br>3. 访客有提问次数限制 |
| DoD | [ ] 代码实现 [ ] 前端测试 [ ] Code Review |
| 预计工时 | 7 天 |

#### TASK-LINK-001 智能链接与权限

| 属性 | 内容 |
|------|------|
| 任务名称 | 智能链接与权限 |
| 模块 | LINK |
| 优先级 | P0 |
| 负责人 | `{后端负责人}` |
| 依赖 | TASK-PUBLIC-001 |
| PRD | FR-07 ~ FR-09，第 8.6 ~ 8.8 节 |
| TDD | 第 6.7 节 |
| API | API-13 ~ API-14 |
| 说明 | 创建/编辑/撤回公开链接；权限策略组合（公开/邮箱验证/白名单/密码/NDA）；权限强度滑块 |
| 验收标准 | 1. 多种权限策略可配置<br>2. 密码安全存储<br>3. 链接过期/撤销生效<br>4. 水印防篡改 |
| DoD | [ ] 代码实现 [ ] 安全测试 [ ] Code Review |
| 预计工时 | 5 天 |

#### TASK-WEB-003 Dashboard 前端

| 属性 | 内容 |
|------|------|
| 任务名称 | Dashboard 前端 |
| 模块 | WEB |
| 优先级 | P0 |
| 负责人 | `{前端负责人}` |
| 依赖 | TASK-LINK-001 |
| PRD | FR-10 ~ FR-11，第 11.2 节 |
| TDD | 第 11.2 节 |
| API | API-16 ~ API-18 |
| 说明 | Dashboard / Documents / Links / Deal Rooms / Contacts / Insights / Settings 页面 |
| 验收标准 | 1. 信息架构完整<br>2. 页面状态规范覆盖<br>3. 关键操作有埋点 |
| DoD | [ ] 代码实现 [ ] 前端测试 [ ] Code Review |
| 预计工时 | 6 天 |

#### TASK-ROOM-001 数据室模块

| 属性 | 内容 |
|------|------|
| 任务名称 | 数据室模块 |
| 模块 | ROOM |
| 优先级 | P1 |
| 负责人 | `{后端 + 前端负责人}` |
| 依赖 | TASK-DATA-001、TASK-LINK-001 |
| PRD | FR-12 ~ FR-13，第 8.9 ~ 8.10 节 |
| TDD | 第 6.10 节 |
| API | API-19 ~ API-22 |
| 说明 | 数据室 CRUD、文件夹权限、成员邀请、NDA gating、访问申请审批 |
| 验收标准 | 1. 模板默认文件夹生成<br>2. folder 级权限生效<br>3. NDA 未签署前限制访问<br>4. 访问申请可审批 |
| DoD | [ ] 代码实现 [ ] 单元测试 [ ] Code Review |
| 预计工时 | 8 天 |

---

### 5.4 Phase 3：分析与集成

#### TASK-ANALYTICS-001 热度评分与 Analytics

| 属性 | 内容 |
|------|------|
| 任务名称 | 热度评分与 Analytics |
| 模块 | ANALYTICS |
| 优先级 | P1 |
| 负责人 | `{后端负责人}` |
| 依赖 | TASK-WEB-001 |
| PRD | FR-10，第 8.11 节 |
| TDD | 第 6.8 节 |
| API | API-16 |
| 说明 | 事件采集、页面级阅读分析、热度评分规则、Hot/Warm/Cold 分级 |
| 验收标准 | 1. 事件不丢失<br>2. 热度评分可解释<br>3. 阈值上线后 14 天可校准 |
| DoD | [ ] 代码实现 [ ] 单元测试 [ ] Code Review |
| 预计工时 | 5 天 |

#### TASK-ANALYTICS-002 行为提醒与跟进建议

| 属性 | 内容 |
|------|------|
| 任务名称 | 行为提醒与跟进建议 |
| 模块 | ANALYTICS |
| 优先级 | P1 |
| 负责人 | `{后端负责人}` |
| 依赖 | TASK-ANALYTICS-001 |
| PRD | FR-11，第 8.11 节 |
| TDD | 第 6.8 节 |
| API | API-17 |
| 说明 | 同类事件合并、提醒生成、跟进建议、每日摘要 |
| 验收标准 | 1. 默认 10 分钟合并窗口<br>2. 每日摘要可开关<br>3. 安全类通知不可退订 |
| DoD | [ ] 代码实现 [ ] 单元测试 [ ] Code Review |
| 预计工时 | 4 天 |

#### TASK-NOTIFY-001 邮件通知系统

| 属性 | 内容 |
|------|------|
| 任务名称 | 邮件通知系统 |
| 模块 | NOTIFY |
| 优先级 | P1 |
| 负责人 | `{后端负责人}` |
| 依赖 | 邮件服务 |
| PRD | FR-14，第 8.12 节 |
| TDD | 第 6.9 节 |
| API | API-23 |
| 说明 | 邮件模板、合并发送、退订、硬退回处理 |
| 验收标准 | 1. 邮件可正常发送<br>2. 同类事件合并<br>3. 退订合规 |
| DoD | [ ] 代码实现 [ ] 单元测试 [ ] Code Review |
| 预计工时 | 3 天 |

#### TASK-INTEG-001 CRM 集成（HubSpot/Salesforce）

| 属性 | 内容 |
|------|------|
| 任务名称 | CRM 集成（HubSpot/Salesforce） |
| 模块 | INTEG |
| 优先级 | P2 |
| 负责人 | `{后端负责人}` |
| 依赖 | TASK-ANALYTICS-001 |
| PRD | FR-15，第 8.13 节 |
| TDD | 第 6.11 节 |
| API | API-24 |
| 说明 | CRM 字段映射、联系人同步、活动记录推送 |
| 验收标准 | 1. 授权连接 CRM<br>2. 字段映射可配置<br>3. 同步失败可重试 |
| DoD | [ ] 代码实现 [ ] 集成测试 [ ] Code Review |
| 预计工时 | 4 天 |

#### TASK-INTEG-002 Slack 集成

| 属性 | 内容 |
|------|------|
| 任务名称 | Slack 集成 |
| 模块 | INTEG |
| 优先级 | P2 |
| 负责人 | `{后端负责人}` |
| 依赖 | TASK-ANALYTICS-001 |
| PRD | FR-16，第 8.13 节 |
| TDD | 第 6.11 节 |
| API | API-25 |
| 说明 | Slack OAuth、Webhook 推送、频道绑定 |
| 验收标准 | 1. 可绑定 Slack 工作区<br>2. 事件按规则推送<br>3. 失败可重试 |
| DoD | [ ] 代码实现 [ ] 集成测试 [ ] Code Review |
| 预计工时 | 2 天 |

---

### 5.5 Phase 4：测试与发布

#### TASK-TEST-001 测试用例与自动化

| 属性 | 内容 |
|------|------|
| 任务名称 | 测试用例与自动化 |
| 模块 | TEST |
| 优先级 | P0 |
| 负责人 | `{测试负责人}` |
| 依赖 | PRD 评审 |
| PRD | 第 12 节 AC-01 ~ AC-32 |
| TDD | 第 10 节测试策略 |
| 说明 | 编写单元/集成/接口/E2E/安全/性能测试用例；搭建自动化流水线 |
| 验收标准 | 1. P0 用例 100% 覆盖<br>2. 自动化流水线通过<br>3. 测试报告可生成 |
| DoD | [ ] 用例编写 [ ] 自动化配置 [ ] 测试报告 |
| 预计工时 | 10 天 |

#### TASK-TEST-002 性能压测与优化

| 属性 | 内容 |
|------|------|
| 任务名称 | 性能压测与优化 |
| 模块 | TEST |
| 优先级 | P1 |
| 负责人 | `{测试/开发负责人}` |
| 依赖 | 功能开发完成 |
| PRD | 第 9 节 NFR |
| TDD | 第 8 节性能设计 |
| 说明 | 使用 k6/Locust 对 P0 接口压测；优化慢查询/缓存/限流 |
| 验收标准 | 1. 上传 P99 ≤ 2s<br>2. AI 问答 P95 ≤ 3s<br>3. 核心查看链路可用性 ≥ 99.5% |
| DoD | [ ] 压测脚本 [ ] 优化记录 [ ] 报告 |
| 预计工时 | 5 天 |

#### TASK-SEC-001 安全扫描与修复

| 属性 | 内容 |
|------|------|
| 任务名称 | 安全扫描与修复 |
| 模块 | SEC |
| 优先级 | P0 |
| 负责人 | `{安全团队}` |
| 依赖 | 开发完成 |
| PRD | 第 17 节合规、安全与隐私 |
| TDD | 第 7 节安全设计 |
| 说明 | 越权测试、注入测试、签名篡改测试、依赖漏洞扫描、渗透测试 |
| 验收标准 | 1. 无高危漏洞<br>2. 越权用例全部通过<br>3. 渗透测试报告通过 |
| DoD | [ ] 扫描报告 [ ] 修复记录 [ ] 复测通过 |
| 预计工时 | 3 天 |

---

## 6. 依赖关系图

```text
PRD 评审通过
    ↓
技术方案确认（TDD + ARCHITECTURE + database-model）
    ↓
设计稿确认
    ↓
Sprint 0：工程脚手架 + 基础设施 + 认证授权（TASK-AUTH-001 / TASK-INFRA-001 / TASK-INFRA-002）
    ↓
    ├──────→ 文档上传 API（TASK-UPLOAD-001）
    │             ↓
    │       PDF / Office Pipeline（TASK-INGEST-001 / TASK-INGEST-002）
    │             ↓
    │       数据库 + 搜索索引（TASK-DATA-001）
    │             ↓
    │       签名 URL + 权限校验（TASK-PUBLIC-001）
    │             ↓
    │       Viewer Canvas（TASK-WEB-001）
    │             ↓
    │       Search / Evidence / Assistant（TASK-SEARCH-001 / TASK-SEARCH-002 / TASK-AI-001）
    │             ↓
    │       悬浮 AI 助手（TASK-WEB-002）
    │             ↓
    │       智能链接 + Dashboard（TASK-LINK-001 / TASK-WEB-003）
    │             ↓
    │       热度评分 + 行为提醒（TASK-ANALYTICS-001 / TASK-ANALYTICS-002）
    └──────→ 数据室 + 通知 + 集成（TASK-ROOM-001 / TASK-NOTIFY-001 / TASK-INTEG-001 / TASK-INTEG-002）
                  ↓
            测试 + 压测 + 安全扫描（TASK-TEST-001 / TASK-TEST-002 / TASK-SEC-001）
                  ↓
            内测 → 灰度 → 全量
```

---

## 7. 风险与依赖

### 7.1 继承自 PRD 的风险登记册

| 风险编号 | 风险描述 | 影响 | 等级 | 应对策略 | 负责人 |
|----------|----------|------|------|----------|--------|
| R-01 | 文档转换/解析失败率高 | 用户无法查看、AI 问答失效 | 高 | 接入成熟转换服务；准备降级方案；建立失败样本库 | 技术 |
| R-02 | AI 问答幻觉/准确度低 | 用户不信任 | 高 | 强制 evidence 引用；答案置信度展示；用户反馈闭环 | 产品 |
| R-03 | 对象存储/CDN 故障 | 文档无法查看 | 高 | 多可用区 + 版本控制；备用 CDN 方案 | 运维 |
| R-04 | Embedding/LLM 服务不可用或限流 | AI 问答失败 | 高 | 本地 embedding 降级；请求缓存；队列限流 | 技术 |
| R-05 | 权限控制漏洞导致材料泄露 | 安全事故 | 高 | 安全评审；渗透测试；签名 URL 短有效期 | 安全 |
| R-06 | 需求变更频繁 | 工期延误 | 中 | PRD 基线冻结；变更走正式流程 | 产品 |
| R-07 | 法务合规审核延迟 | 上线延期 | 中 | 提前准备隐私政策、DPA、数据删除流程 | 运营 |
| R-08 | 用户接受度低于预期 | 产品-市场契合未验证 | 中 | MVP 快速验证；设置退出标准 | 产品 |

### 7.2 执行层新增风险

| 风险编号 | 风险描述 | 影响 | 等级 | 应对策略 | 负责人 |
|----------|----------|------|------|----------|--------|
| R-IP-01 | OnlyOffice 自托管集群交付延迟 | Office 解析延期 | 中 | 提前采购/部署；本地 LibreOffice 兜底 | 运维 |
| R-IP-02 | 前端 Canvas 性能瓶颈 | 大文档渲染卡顿 | 中 | 分页加载、懒渲染、Web Worker | 前端 |
| R-IP-03 | 多 Workspace 上下文传递遗漏 | 数据泄露 | 高 | Repository 层强制注入；自动化越权测试 | 后端 |
| R-IP-04 | LLM API 成本超预算 | 运营成本失控 | 中 | 限流、缓存、模型降级 | 技术 |

### 7.3 外部依赖

| 依赖 | 用途 | 风险 | 备选 |
|------|------|------|------|
| Cloudflare | CDN + URL Signing | 中 | 阿里云 CDN + 后端签名 |
| OnlyOffice | Office 转 PDF | 中 | LibreOffice / 预转 PDF |
| OpenAI API | LLM + Embedding | 中 | Azure OpenAI / 自托管 bge + vLLM |
| SendGrid/SES | 邮件发送 | 低 | 备用邮件服务商 |
| HubSpot/Salesforce API | CRM 同步 | 低 | 手动导出/导入 |
| Slack API | Slack 推送 | 低 | - |

---

## 8. 决策记录

| 编号 | 决策 | 原因 | 影响任务 |
|------|------|------|----------|
| IP-D-01 | 按 Phase 0 → 1 → 2 → 3 → 4 分阶段执行 | 降低集成风险，先打通核心链路 | 全部 |
| IP-D-02 | TASK 编号采用 `{模块缩写}-{NNN}` | 与模板一致，便于任务板与 PR 引用 | 全部 |
| IP-D-03 | P1 功能（数据室、通知、CRM/Slack）在 P0 核心链路完成后启动 | 保证 MVP 里程碑不受影响 | TASK-ROOM-001 / TASK-NOTIFY-001 / TASK-INTEG-* |
| IP-D-04 | 测试/安全/压测任务与开发任务并行启动 | 避免最后阶段集中爆发 | TASK-TEST-001 / TASK-SEC-001 |

---

## 9. 附录

### 9.1 术语表

| 术语 | 说明 |
|------|------|
| TDD | Technical Design Document，技术设计文档 |
| AC | Acceptance Criteria，验收标准 |
| NFR | Non-Functional Requirements，非功能需求 |
| DoD | Definition of Done，完成定义 |
| HNSW | Hierarchical Navigable Small World，pgvector 近似最近邻索引 |
| RRF | Reciprocal Rank Fusion，多路搜索结果融合 |

### 9.2 参考文档

- `docs/PRD-v2.1.0.md`
- `docs/TDD-v2.1.0.md`
- `docs/ARCHITECTURE-v2.1.0.md`
- `docs/database-model-v2.1.0.md`
- `docs/templates/IMPLEMENTATION-PLAN-template-v1.md`

---

## 10. 检查清单

- [x] 所有任务可追溯到 PRD / TDD / API / 测试用例
- [x] 所有任务有优先级、负责人、依赖、预计工时
- [x] 里程碑与 PRD 第 14.2 节一致
- [x] 依赖关系图无循环依赖
- [x] 风险登记册已更新
- [x] 下游 issue 拆分清单已规划

---

> **模板版本**：v1  
> **IMPLEMENTATION-PLAN 版本**：v2.1.0  
> **状态**：已批准  
> **最后更新**：2026-06-20

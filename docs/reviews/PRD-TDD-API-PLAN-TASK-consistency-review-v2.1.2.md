# DealSignal v2.1.2 文档一致性深度评审报告

> **评审范围**：`PRD-v2.1.0.md`、`TDD-v2.1.0.md`、`API-SPEC-v2.1.0.md`、`ARCHITECTURE-v2.1.0.md`、`database-model-v2.1.0.md`、`HEAT-SCORE-ALGORITHM-v2.1.1.md`、`DESIGN-TOKENS-v2.1.1.md`、`INTERACTION-SPEC-v2.1.1-REFINED.md`、`IMPLEMENTATION-PLAN-v2.1.0.md`、`IMPLEMENTATION-PLAN-v2.1.1.md`、`IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md`、`docs/tasks/agent-tasks-v2.1.2/*.md`  
> **评审日期**：2026-06-21  
> **评审目标**：在启动 TASK 执行前，端到端识别 PRD/TDD/API/PLAN/TASK 之间的冲突、缺口与术语漂移，避免实现返工。

---

## 1. 执行摘要

本次评审采用「逐文件精读 + 交叉引用」方式，共发现 **49 项** 一致性问题：

| 严重级别 | 数量 | 说明 |
|----------|------|------|
| **Critical** | 4 | 会阻塞开发或导致实现与文档严重背离，必须在编码前解决。 |
| **High** | 15 | 会导致数据模型、API 契约或任务拆分失效，需在 sprint 启动前修复。 |
| **Medium** | 22 | 功能可用但会引发边界行为不一致、测试缺失或维护成本。 |
| **Low** | 8 | 术语、拼写、流程文档层面的 polish。 |

**核心结论**：
1. `API-SPEC-v2.1.0.md` 与 `TDD`/`PRD` 在响应格式、错误码、角色枚举、热度评分事件、水印字段等方面存在多处冲突，若直接按 API 实现会导致前端 mock 契约与真实后端不一致。
2. `database-model-v2.1.0.md` 与 `API-SPEC` 在 `allowed_domains`、`thumbnail_object_key`、`contacts`、`assistant_sessions.link_id` 等关键字段上失配。
3. `IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` 中的 `DS-004`、`DS-014`、`DS-018`、`DS-023`、`DS-024` 等 issue 未被新 AGENT-TASK 覆盖；前端任务引用了不存在的 `DS-FRONTEND-xxx` issue。
4. 后端任务 `TASK-BACKEND-005` 缺少对 `TASK-BACKEND-003` 的依赖；任务文件数/代码行上限与实际范围不符。

**建议**：在启动任何 TASK 编码前，先召开一次文档同步会，按本报告「推荐行动项」逐项修复；修复后应重新评审并更新 AGENT-TASK 文件。

---

## 2. 评审方法

1. **逐文件精读**：由探索代理完整阅读 12 份文档，按 PRD↔TDD↔API、PLAN↔ISSUES↔TASK、ARCHITECTURE↔DB↔Algorithm↔Design 三条线交叉核对。
2. **单元格级对比**：重点核对枚举值、字段名、错误码、路径、状态机、依赖关系。
3. **可追溯标注**：每条发现均标注文档位置（章节/行号），便于快速定位。

---

## 3. Critical 阻塞项（必须编码前解决）

| # | 领域 | 位置 | 不一致描述 | 推荐修复 |
|---|------|------|------------|----------|
| C1 | 规划 / 任务映射 | `TASK-FRONTEND-001.md`、`TASK-FRONTEND-003.md` front matter；`IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` | 前端任务 `parent_issue` 使用 `DS-FRONTEND-001`、`DS-FRONTEND-003`，但 issue 清单中只有 `DS-001`~`DS-025`，无此前缀。PR 无法关联到已批准 issue。 | 为 v2.1.2 前端工作创建真实 issue，或重映射：`TASK-FRONTEND-001`→`DS-016`（Dashboard/Settings 打磨），`TASK-FRONTEND-003`→`DS-014`+`DS-010`（前端集成）。 |
| C2 | 规划 / 依赖 | `README.md` 执行顺序图；`TASK-BACKEND-005.md` front matter | `TASK-BACKEND-005` 包含 `DS-009`（签名 URL + 权限校验），而 `DS-009` 依赖 `DS-003`（对象存储）和 `DS-008`（搜索索引/DB），这些都在 `TASK-BACKEND-003` 中。当前 `TASK-BACKEND-005` 仅依赖 `TASK-BACKEND-002`。 | 将 `TASK-BACKEND-003` 加入 `TASK-BACKEND-005` 的前置依赖，并重绘执行顺序图。 |
| C3 | API / 算法 | `API-SPEC-v2.1.0.md` §4.3 API-05（line 464）；`HEAT-SCORE-ALGORITHM-v2.1.1.md` §3.1（line 116-127） | API-05 仅接受 `link_opened`、`page_viewed`、`download_attempted` 三种事件，而热度评分算法需要 `open / revisit / page_view / key_page_view / forward_signal / download / bounce` 七种事件。没有文档说明如何从 3 种原始事件推导出 7 种算法事件。 | 方案 A：扩展 API-05 接受全部 7 种事件并明确定义；方案 B：在 API/TDD 中补充服务端事件推导规则。二者择一并在所有文档同步。 |
| C4 | Schema / API | `database-model-v2.1.0.md` §4.5.1 `assistant_sessions`（line 922）；`API-SPEC-v2.1.0.md` §4.4 API-07（line 567-570） | `assistant_sessions.link_id` 为 `NOT NULL`，但 API-07 的 workspace 内部 AI 问答仅需要 `document_id`（文档详情页 AI 洞察 tab），没有 public link。这会导致内部会话无法创建。 | 将 `assistant_sessions.link_id` 改为 `NULLABLE`，并保证 CHECK 约束至少存在 `link_id` 或有效 `document_id` 上下文之一。 |

---

## 4. High 风险项（Sprint 启动前修复）

| # | 领域 | 位置 | 不一致描述 | 推荐修复 |
|---|------|------|------------|----------|
| H1 | API / 响应格式 | `API-SPEC-v2.1.0.md` §2.5 vs §4 全部接口示例 | §2.5 规定成功响应必须包裹在 `BaseResponse`（含 `code/message/request_id/data`），但 API-01~API-16 的成功示例均为扁平对象（如 API-01 直接返回 `{id, title, ...}`）。 | 统一响应包装策略：建议所有成功响应使用 `BaseResponse`，并逐条更新 §4 示例；或修改 §2.5 为「扁平对象」。 |
| H2 | API / 错误码 | `API-SPEC-v2.1.0.md` §2.7 vs §4 错误码表 vs `TDD-v2.1.0.md` §5.2 | §2.7 通用错误码使用小写下划线（`unauthorized`、`file_too_large`），但 §4 各接口与 TDD 使用全大写（`UNAUTHORIZED`、`FILE_TOO_LARGE`）。 | 统一错误码规范，建议全大写 `SNAKE_CASE`，并修正 §2.7。 |
| H3 | API / Auth | `PRD-v2.1.0.md` §5.4 路径 D、§6.3；`TDD-v2.1.0.md` §7.1.1；`API-SPEC-v2.1.0.md` §3.2 / §5.1 | PRD/TDD 要求通过邀请链接完成注册、登录、自动加入 Workspace，但 API-SPEC 缺失账号端点：注册、登录、Token 刷新、邀请创建/接受、Workspace CRUD。 | 在 API-SPEC 中补充 Auth 与 Workspace 管理端点，如 `POST /auth/register`、`POST /auth/login`、`POST /workspaces/invitations` 等。 |
| H4 | API / 功能 | `PRD-v2.1.0.md` §8.2.4 FR-09；`TDD-v2.1.0.md` §6.3.2、§7.4；`API-SPEC-v2.1.0.md` API-04 / API-09 | 动态水印要求前端 Canvas 绘制访问者邮箱、时间、IP 哈希，但 API-04 签名 URL 响应、API-09 公开链接响应均未返回安全的水印 payload。 | 在 API-09（或 API-04）返回经后端签名的 `watermark_payload`（含邮箱/时间/IP 哈希），并明确签名与校验方式。 |
| H5 | API / 架构 | `PRD-v2.1.0.md` §10.2 API-01；`TDD-v2.1.0.md` §6.1.2、§6.1.3；`ARCHITECTURE-v2.1.0.md` §8.1 时序图 | API-01 描述为前端直接 `multipart/form-data` 上传到后端；TDD/ARCHITECTURE 描述为后端返回 `upload_url`，前端直传 OSS，再 PATCH 通知完成。两种架构矛盾。 | 确定最终方案：若采用直传 OSS，则 API-01 改为创建上传会话并返回 `upload_url`，状态机拆分为 `created → uploaded → processing`；若采用后端代理，则修正 ARCHITECTURE/TDD。 |
| H6 | Schema / API / 术语 | `database-model-v2.1.0.md` §4.1.5（line 314）；`API-SPEC-v2.1.0.md` §5.2（line 984-989） | DB 使用 `owner / admin / member / guest`，API-SPEC 使用 `OWNER / ADMIN / CONTRIBUTOR / VIEWER`，且 `CONTRIBUTOR/VIEWER` 与 DB CHECK 不匹配。 | 统一 workspace 角色：建议 API-SPEC 对齐 DB/PRD（`owner / admin / member / guest`），或显式定义 API 角色到 DB 角色的映射。 |
| H7 | API / 算法 | `HEAT-SCORE-ALGORITHM-v2.1.1.md` §9.1-9.2；`API-SPEC-v2.1.0.md` §4.6 API-10 | 算法维度：`opens, revisits, avgDurationMinutes, keyPageViews, forwardSignals, downloads, bouncePenalty`；API-10 factors 缺少 `revisits`、`downloads`、`bounce`；算法结果字段为 `level`，API 用 `tier`；算法返回 `topKeyPages`，API 没有。 | 使 API-10 返回与算法一致的 7 个维度，统一 `level/tier` 命名，补充 `topKeyPages`。 |
| H8 | Schema / API | `API-SPEC-v2.1.0.md` §4.5 API-08（line 637）；`database-model-v2.1.0.md` §4.3.1 `links`（line 703） | API-08 请求接受 `allowed_domains`，但 `links` 表只有 `allowed_emails` JSONB，无法持久化域名白名单。 | 在 `links` 表新增 `allowed_domains jsonb NOT NULL DEFAULT '[]'`，或统一使用带前缀的 `allowed_emails` 并文档化。 |
| H9 | Schema / API | `API-SPEC-v2.1.0.md` §4.2 API-03（line 373）；`database-model-v2.1.0.md` §4.2.2 / §4.2.3 | API-03 每页返回 `thumbnail_object_key`，但 `document_pages` 表只有 `image_object_key`；缩略图以 `document_files(file_role='THUMBNAIL')` 建模。 | 修改 API-03 返回 `image_object_key`（即页面 webp），或新增 `thumbnail_object_key` 到 `document_pages` 并明确归属。 |
| H10 | Schema / 设计 | `INTERACTION-SPEC-v2.1.1-REFINED.md` §6；`PRD-v2.1.0.md` §11.2；`database-model-v2.1.0.md` | Contacts 是一级 UX 模块（列表、详情、热度排序、趋势图、备注、写邮件），但数据库无 `contacts` 表，只能从 `link_accesses`/`room_members` 推断。 | 新增 `contacts` 表（或物化视图），key 为 `(tenant_id, workspace_id, email)`，支持 name/company/first_seen/last_seen/notes/heat_score。 |
| H11 | 规划 / 覆盖 | `IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` line 127；`README.md` 任务总览 | `DS-004`（子域名/自定义域名/SSL 自动签发）为 P0，但 9 个 AGENT-TASK 中无对应任务。 | 新增 `TASK-BACKEND-007`（parent `DS-004`），或明确将子域名/SSL 工作并入 `TASK-BACKEND-001/002` 并更新范围。 |
| H12 | 规划 / 覆盖 | `IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` line 137 | `DS-014`（悬浮 AI 助手前端）为 P0，但仅 `TASK-FRONTEND-002` 覆盖 `DS-010`（Viewer），悬浮助手缺失。 | 新增 `TASK-FRONTEND-004`（parent `DS-014`），依赖 `TASK-FRONTEND-002` 与 `TASK-BACKEND-004`。 |
| H13 | 规划 / 覆盖 | `IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` line 141 | `DS-018`（行为提醒与跟进建议）映射到 `TASK-ANALYTICS-002`，但 `TASK-BACKEND-005` 只包含 `DS-017` 未包含 `DS-018`。 | 将 `DS-018` 加入 `TASK-BACKEND-005` parent_issue，或新建独立任务。 |
| H14 | 规划 / 映射 | `TASK-BACKEND-005.md` front matter / §4 / §10 | `DS-016` 是前端 Dashboard issue（`TASK-WEB-003`），却被列在 backend 任务下。 | 从 `TASK-BACKEND-005` 移除 `DS-016`；将其分配给前端任务（如重映射后的 `TASK-FRONTEND-001`）。 |
| H15 | 规划 / 估算 | `TASK-BACKEND-002.md` ~ `TASK-BACKEND-006.md` front matter & §4.1 | 所有后端任务声明 `estimated_files: 8`、`max_lines: 400`，但 §4.1 文件列表通常 9~10 个；Auth+Workspace、Upload+Ingestion、Search+Evidence+Assistant、Links+Analytics+Rooms、Notify+Integrations+Security 不可能在 400 行内完成。 | 拆分过载任务或提高上限。建议：将 `TASK-BACKEND-005` 拆为 links / analytics / deal rooms；将 `TASK-BACKEND-006` 拆为 notifications / integrations / security。 |

---

## 5. Medium 缺口项

| # | 领域 | 位置 | 不一致描述 | 推荐修复 |
|---|------|------|------------|----------|
| M1 | API / 资源管理 | `PRD-v2.1.0.md` §11.2；`API-SPEC-v2.1.0.md` §3.2 | PRD 信息架构需要文档/链接/数据室列表及管理操作，API-SPEC 只定义了创建与单条获取，缺失分页列表、更新、删除、链接撤回/禁用、数据室成员邀请/审批等。 | 补充 CRUD 与管理端点：`GET /documents`、`GET /links`、`PATCH /links/{id}`、`POST /links/{id}/revoke`、`GET /deal-rooms`、`POST /deal-rooms/{id}/members`、`POST /deal-rooms/{id}/access-requests/{id}/approve` 等。 |
| M2 | Schema / 架构 | `ARCHITECTURE-v2.1.0.md` §9.2；`database-model-v2.1.0.md` §4.3.1 | 架构状态图定义链接状态为 ACTIVE / DISABLED / EXPIRED / REVOKED / DELETED，DB `status` CHECK 只有 `active / disabled / revoked`。 | 扩展 `links.status` 枚举覆盖全部状态，或标注 `EXPIRED`/`DELETED` 为虚拟状态（由 `expires_at`/`deleted_at` 计算）。 |
| M3 | Schema / 架构 | `ARCHITECTURE-v2.1.0.md` §9.1；`database-model-v2.1.0.md` §4.2.1 | 架构状态图包含 DELETED 状态，DB `status` CHECK 为 `uploaded / processing / ready / failed / archived`，并通过 `deleted_at` 软删除。 | 在状态图旁标注 `DELETED` 由 `deleted_at IS NOT NULL` 表示，而非 `status`。 |
| M4 | Schema / 架构 | `ARCHITECTURE-v2.1.0.md` §9.3；`database-model-v2.1.0.md` §4.6.5 | 数据室访问申请状态图含 CANCELLED / REVOKED，DB `status` CHECK 只有 `pending / approved / rejected`。 | 增加 `cancelled` / `revoked` 到 `room_access_requests.status`，或从状态图中移除 MVP 不支持的状态。 |
| M5 | Auth / Schema | `ARCHITECTURE-v2.1.0.md` §7.2；`database-model-v2.1.0.md` §4.1.6；`PRD-v2.1.0.md` §6.3 | 架构邀请流程显示角色 admin / member / guest，PRD 规定注册后默认 member；DB 支持 owner/admin/member/guest，但对 invite 角色是否允许 `owner` 未说明。 | 明确邀请时可选角色：建议 admin / member / guest，owner 通过转让获得。 |
| M6 | Auth / 术语 | `PRD-v2.1.0.md` §6.3；`TDD-v2.1.0.md` §4.2.1 `tenants` | PRD 提到 "tenant admin" 拥有创建 workspace 权限，但 `tenants` 表无 `owner_id` 或 tenant 级角色，创建 workspace 的权限实际落在 workspace ADMIN。 | 明确租户所有权：要么 `tenants` 增加 `owner_id` 并定义 tenant admin 权限，要么 PRD 统一使用 workspace OWNER/ADMIN 术语。 |
| M7 | API / PRD | `PRD-v2.1.0.md` §10.2；`API-SPEC-v2.1.0.md` §3.2 | PRD 中 API-05 路径为 `/events`、API-09 为自定义域名根路径 `/?tenant=...&workspace=...&token=...`、API-14 为 `/public/...`；API-SPEC 统一使用 `/{workspaceSlug}/api/v1/...` 与 `/api/v1/public/...`。 | 更新 PRD §10.2 使用 API-SPEC 的完整路径；对 API-09 可备注自定义域名根路径是兼容落地页。 |
| M8 | 功能 / API / Schema | `PRD-v2.1.0.md` §8.2.6 FR-12/FR-13；`TDD-v2.1.0.md` §4.2.4；`API-SPEC-v2.1.0.md` §4.7 | PRD 要求数据室支持 Q&A 与 folder 级权限隔离，TDD 定义了 `room_member_folder_permissions`，但 API-SPEC 无 Q&A 模型及 folder 权限端点。 | 补充数据室 Q&A 表与 folder 权限管理端点，或在 PRD 中明确 Q&A/folder 权限为 P1/二期。 |
| M9 | 功能 / API | `PRD-v2.1.0.md` §4.2 In Scope #9、D-16、D-21；`TDD-v2.1.0.md` §4.2.1 `tenant_domains`；`API-SPEC-v2.1.0.md` §2.2 | 品牌化分享页与企业自定义域名在 PRD 决策中支持，TDD 设计了 `tenant_domains`，但 API-SPEC 无自定义域名/CNAME 验证/品牌页配置端点。 | 补充 tenant 域名管理 API，或在 PRD/TDD 中标记为二期。 |
| M10 | 合规 / TDD | `PRD-v2.1.0.md` §9.8；`TDD-v2.1.0.md` §4.6 vs C-06 | 数据保留策略不一致：PRD 与 TDD §4.6 规定 events/AI 日志默认 1 年、审计日志永久；TDD C-06 却规定 events/page_views 12–24 个月、AI 日志 90 天、审计日志 7 年。 | 统一数据保留策略，以合规需求为准，同步 PRD/TDD 各处。 |
| M11 | 规划 / 覆盖 | `IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` line 146-147 | `DS-023`（测试用例与自动化）、`DS-024`（性能压测与优化）无对应 AGENT-TASK。 | 新增测试/性能任务，或在 README 中注明由 QA/SRE 负责、不在本批次。 |
| M12 | 规划 / 依赖 | `README.md` 依赖表；`TASK-FRONTEND-003.md` front matter | `TASK-FRONTEND-003` 仅依赖 `TASK-BACKEND-002`（auth/workspace 契约），但完整 API 集成验证需要 documents、search、links、assistant 等端点就绪。 | 保留 `TASK-BACKEND-002` 为契约依赖，并注明完整集成验证需等待 `TASK-BACKEND-003`~`006`；或将其移至后端链末端。 |
| M13 | 规划 / Sprint | `IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` §5；`README.md` 执行顺序 | Sprint 3 同时安排 `DS-009`~`DS-012`，但 `DS-009` 依赖 `TASK-BACKEND-003`，而 `DS-011/012` 依赖 `TASK-BACKEND-004`，当前依赖图无法让它们在同一个 sprint 内并行完成。 | 调整 Sprint 分组：将 `DS-009`/signed-URL 工作移到 `TASK-BACKEND-003` 之后，或拆分 `TASK-BACKEND-003` 让对象存储+DB schema 提前落地。 |
| M14 | 规划 / 优先级 | `TASK-BACKEND-006.md` front matter（priority P1）；`IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` line 148（`DS-025` P0） | `DS-025`（安全扫描与修复）是 P0，但 `TASK-BACKEND-006` 整体为 P1，会延迟 P0 门禁。 | 将 `TASK-BACKEND-006` 提升为 P0，或将 `DS-025` 提取为独立 P0 安全任务。 |
| M15 | 规划 / 范围 | `TASK-BACKEND-003.md` ~ `TASK-BACKEND-006.md` front matters | 每个后端任务覆盖 3~5 个 issue，与 issue 清单「每个 issue 只负责一个可独立合并的交付单元」原则冲突。 | 对齐 task 边界与 issue 边界，或至少保证每个 task 只含一个逻辑域。 |
| M16 | 任务 / 范围 | `TASK-FRONTEND-002.md` front matter `pending_confirmation` | Canvas 渲染策略未定：使用 div/图片占位还是真实 Canvas？该决策阻塞实现。 | 在执行前确定策略并写入任务文件。 |
| M17 | Schema / API | `API-SPEC-v2.1.0.md` API-01（line 280）；`database-model-v2.1.0.md` §4.2.1（line 398/432-433） | API 接受小写 `pdf / docx / pptx / xlsx`，DB CHECK 为大写 `PDF / DOCX / PPTX / XLSX`。 | 文档化 API 输入会归一化为 DB 枚举大小写，或使 DB CHECK 大小写不敏感。 |
| M18 | Schema / API | `API-SPEC-v2.1.0.md` API-12（line 813）；`database-model-v2.1.0.md` §4.6.1（line 970/996-997） | API 接受小写 `seed / series_a / lp_update / sales_proposal`，DB CHECK 为大写 `Seed / SeriesA / LP_Update / Sales_Proposal`。 | 同上：统一枚举或文档化归一化规则。 |
| M19 | API / Schema | `API-SPEC-v2.1.0.md` API-05（line 467）；`database-model-v2.1.0.md` §4.4.1（line 809）；`PRD-v2.1.0.md` §8.2.2 | API-05 的 `scroll_depth` 为整数百分比（0-100），DB 为 `decimal(5,2)`，PRD 描述为 0.00–1.00。 | 将 API-05 改为 `number`（0.00–1.00）或文档 `/100` 转换。 |
| M20 | 架构 / Schema | `ARCHITECTURE-v2.1.0.md` §10.1；`database-model-v2.1.0.md` §3、§4 | 核心 ERD 缺少 `document_blocks`、`ingestion_jobs`、`workspace_invitations`、`events`、`audit_logs`、`notification_jobs`、`integration_jobs`、`analytics_jobs`、`dead_letter_jobs` 等表。 | 扩展 ERD 或添加说明：ERD 为简化视图，完整 schema 见 `database-model-v2.1.0.md`。 |
| M21 | 架构 / Schema | `ARCHITECTURE-v2.1.0.md` §6.1；`database-model-v2.1.0.md` §4.2.4；`PRD-v2.1.0.md` §10.1 | Ingestion 数据流输出描述为 pages/chunks/boxes，但遗漏了 `document_blocks`。 | 更新架构 §6.1 步骤 5 与 §8.1 时序图，包含 `document_blocks`。 |
| M22 | Schema | `database-model-v2.1.0.md` §4.2.2 / §4.2.3 | 页面 webp 同时通过 `document_pages.image_object_key` 与 `document_files(file_role='PAGE_WEBP')` 引用，存在单一事实源歧义。 | 选择一种方案：推荐保留 `document_pages.image_object_key` 并移除 `PAGE_WEBP` 角色，或反之。 |
| M23 | Schema / 设计 | `INTERACTION-SPEC-v2.1.1-REFINED.md` §2.2；`database-model-v2.1.0.md` §4.7 | UI 有通知 bell 与未读数，但 DB 只有 outgoing `notification_jobs`，无 in-app notifications / read receipts。 | 新增 `user_notifications` / `in_app_notifications` 表，或在交互文档中说明 bell 聚合的是邮件/Slack 状态。 |
| M24 | 算法 / 术语 | `HEAT-SCORE-ALGORITHM-v2.1.1.md` §2.2；`PRD-v2.1.0.md` §4.2 / §10.4 | 热度评分配置使用 `investor_ir`，PRD 使用 `investor` / "LP Engagement"；用户角色为 `founder / investor / sales / admin`。 | 统一命名，例如 `founder`、`investor_ir`（或 `lp`）、`sales`，并文档化与 PRD 场景映射。 |
| M25 | API / Schema | `API-SPEC-v2.1.0.md` API-12（line 814-815）；`database-model-v2.1.0.md` §4.6.1（line 971） | API-12 请求使用顶层 `requires_nda`/`requires_approval` 布尔值，DB `deal_rooms.settings` 为 JSONB。 | 在 `deal_rooms` 增加显式列，或文档化 JSONB 路径映射。 |
| M26 | 术语 / Schema | `PRD-v2.1.0.md` §10.1 `tenant_domains.domain_type`；`database-model-v2.1.0.md` §4.2.1 | PRD 描述枚举为大写 `SUBDOMAIN / CUSTOM / PUBLIC_LINK`，DB DDL 为小写 `'subdomain', 'custom', 'public_link'`。 | 统一为小写并修正 PRD 文本。 |
| M27 | Schema / TDD | `TDD-v2.1.0.md` §4.5 sqlc 示例；`database-model-v2.1.0.md` 各表 DDL | TDD 查询示例使用 `deleted_at IS NULL`，但 DDL 中部分表无 `deleted_at`（如 `document_files`、`workspace_invitations`）。 | 明确是否全局软删除：若采用，补充 DDL；若不采用，删除查询中的 `deleted_at` 条件。 |

---

## 6. Low 完善项

| # | 领域 | 位置 | 不一致描述 | 推荐修复 |
|---|------|------|------------|----------|
| L1 | 术语 | `PRD-v2.1.0.md` §3.4 / §19.1（Intent Score）；`API-SPEC-v2.1.0.md` API-10 | PRD 使用 Intent Score（交易意图分），API-10 名称为 Get Link Heat Score。 | 统一命名：API-10 改为 `Get Link Intent Score`，或在 PRD 中接受 Heat Score 作为别名。 |
| L2 | 术语 / 埋点 | `PRD-v2.1.0.md` §10.3 EVT-01；`API-SPEC-v2.1.0.md` / `database-model-v2.1.0.md` | EVT-01 埋点属性使用 `file_type`，而数据表/接口使用 `source_type`。 | 统一 EVT-01 属性名为 `source_type`。 |
| L3 | API / 流程 | `API-SPEC-v2.1.0.md` §6 Webhook | API-SPEC 定义 `document.uploaded`、`link.hot` 等 webhook，但 PRD/TDD 未设计推送机制、订阅管理或签名验证。 | 若 webhook 为 v2.1.0 范围，补充 PRD/TDD 设计；若为远期规划，在 API-SPEC 标注「二期」。 |
| L4 | API / 算法 | `HEAT-SCORE-ALGORITHM-v2.1.1.md` §9.1；`API-SPEC-v2.1.0.md` API-10 | 算法 `breakdown` 为 `Record<string, number>`，API-10 `factors` 为数组 `{name, value, weight}`。 | 对齐表达形式，建议算法结果采用更丰富的数组结构。 |
| L5 | 架构 / 术语 | `ARCHITECTURE-v2.1.0.md` §7.2；`database-model-v2.1.0.md` §4.1.6 | 架构邀请流程只显示 admin/member/guest，DB 支持 owner。 | 补充说明 owner 通过 workspace 转让获得，不通过邀请。 |
| L6 | 流程 | `API-SPEC-v2.1.0.md` §7.2（line 1049）/ §8 | API-SPEC 声称 OpenAPI 已同步，但 `docs/openapi-v2.1.0.yaml` 状态为「待创建」。 | 创建该 YAML 或取消勾选同步项。 |
| L7 | 术语 / API | `API-SPEC-v2.1.0.md` API-06；`PRD-v2.1.0.md` §6.3、§8.2.3 | API-06 搜索模式拼写为 `fulltext`，PRD 使用 `full-text`。 | 统一拼写。 |
| L8 | 规划 / 映射 | `IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` line 124；`IMPLEMENTATION-PLAN-v2.1.0.md` §4.1 | `DS-001` 映射到 `TASK-AUTH-001`，但 `TASK-AUTH-001` 描述为 auth/workspace 而非脚手架。新的 `TASK-BACKEND-001` 更贴合。 | 更新 v2.1.0 计划，将 `DS-001` 映射到 `TASK-BACKEND-001`。 |
| L9 | 规划 / 范围 | `TASK-BACKEND-006.md` §1 / §4.1；`IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` line 144 | `DS-021` 覆盖 HubSpot/Salesforce，但 `TASK-BACKEND-006` 只提到 HubSpot。 | 明确 Salesforce 是否在本次范围；若否，在任务中标注 deferred。 |

---

## 7. 跨文档主题总结

| 主题 | 问题本质 | 影响 |
|------|----------|------|
| **枚举/术语漂移** | 角色、状态、事件、文件类型、域名类型等在同一概念上存在大小写或命名差异。 | 会导致 DB CHECK 失败、API 校验失败、前后端理解不一致。 |
| **API 契约与示例脱节** | `BaseResponse`、错误码命名、成功响应示例三者未统一。 | 后端实现无所适从，前端 mock 与真实 API 结构不同。 |
| **Schema 缺字段** | `allowed_domains`、`contacts`、`thumbnail_object_key`、`in_app_notifications` 等 UX/PRD 需求在 DB 中无对应。 | 功能无法持久化或需要临时 hack。 |
| **状态机与 DDL 不一致** | 文档/链接/数据室状态图比 DB CHECK 多状态。 | 实现时状态转换无法落库。 |
| **任务拆分与 issue 清单错位** | issue 遗漏、依赖错误、scope 超载、估算不可行。 | 进入开发后会出现阻塞、PR 过大、验收标准不清。 |
| **AI/热度评分事件链路断裂** | API 事件集合无法支撑算法事件集合。 | 热度评分结果会失真或完全无法计算。 |

---

## 8. 推荐行动项（按优先级）

### Phase 1：编码前必须完成（Critical + High）

1. **修复任务与 issue 的映射**
   - 创建 v2.1.2 前端 issue，或把 `TASK-FRONTEND-001/003` 重映射到现有 `DS-xxx`。
   - 新增/调整任务覆盖 `DS-004`、`DS-014`、`DS-018`。
   - 从 `TASK-BACKEND-005` 移除 `DS-016`。
2. **修正依赖图**
   - `TASK-BACKEND-005` 增加对 `TASK-BACKEND-003` 的依赖。
   - 重新评估 `TASK-FRONTEND-003` 的完整集成验证时机。
3. **统一 API 基础契约**
   - 确定响应包装策略并更新所有 §4 示例。
   - 统一错误码为全大写 `SNAKE_CASE`。
4. **补齐缺失的 Auth / Workspace / CRUD 端点**
   - 在 API-SPEC 中补充注册、登录、邀请、Workspace、列表/更新/删除/撤回等接口。
5. **确定上传与水印架构**
   - 选择直接上传 OSS 预签名还是后端代理，并同步 PRD/TDD/API/ARCHITECTURE。
   - 确定动态水印 payload 字段与签名方式。
6. **修复 Schema 关键字段**
   - `assistant_sessions.link_id` 改为 nullable。
   - `links` 增加 `allowed_domains`。
   - 决定 `document_pages` 与 `document_files(PAGE_WEBP)` 的单一事实源。
   - 明确 `thumbnail_object_key` 是否保留。
7. **统一角色/状态枚举**
   - Workspace 角色：`owner / admin / member / guest`。
   - 链接、文档、数据室状态图与 DDL CHECK 对齐。
   - 文件/template 类型大小写统一或文档化归一化。

### Phase 2：Sprint 启动前完成（Medium）

8. **对齐热度评分全链路**
   - 统一 API-05 事件类型与算法事件集。
   - 统一 API-10 返回字段与算法结果字段。
9. **补充数据模型**
   - `contacts` 表/视图。
   - in-app notifications 模型。
   - 数据室 Q&A 与 folder 权限（若保留在 MVP）。
10. **完善架构与 ERD**
    - 更新 ERD 包含 job/event/audit/invitation 表。
    - ingestion 数据流增加 `document_blocks`。
11. **处理规划层面的遗漏**
    - 为 `DS-023/024` 安排任务或明确负责人。
    - 将 `DS-025` 提升为 P0 或独立任务。
    - 拆分 `TASK-BACKEND-005/006` 以匹配 `estimated_files` / `max_lines`。
12. **解决待确认事项**
    - 确定 `TASK-FRONTEND-002` 的 Canvas 策略。

### Phase 3：文档 Polishing（Low）

13. 统一 Intent Score / Heat Score、scroll_depth、fulltext/full-text 等术语。
14. 修正 EVT-01 属性名、`domain_type` 大小写、OpenAPI 同步勾选状态。
15. 明确 webhook、Salesforce、自定义域名等是 MVP 还是二期。

---

## 9. 与 TASK 文件的联动

本次评审发现的问题需要在 AGENT-TASK 层面做以下调整：

| 受影响任务 | 需要做的调整 |
|------------|--------------|
| `TASK-FRONTEND-001.md` | 修正 `parent_issue` 为真实 issue ID；补充 i18n/AI 关键词范围说明。 |
| `TASK-FRONTEND-002.md` | 在启动前回答 `pending_confirmation` 中的 Canvas 策略。 |
| `TASK-FRONTEND-003.md` | 明确仅验证 auth/workspace 契约，完整集成验证延后；修正依赖描述。 |
| `TASK-BACKEND-001.md` | 评估是否纳入 `DS-004`（子域名/SSL）的初步接口占位。 |
| `TASK-BACKEND-002.md` | 按统一后的角色/状态枚举更新 migration；补充 API 缺失端点对应的路由占位意识。 |
| `TASK-BACKEND-003.md` | 按统一后的上传架构调整 scope；确认 `PAGE_WEBP` 与 `image_object_key` 事实源。 |
| `TASK-BACKEND-004.md` | 确认 API-07 支持 `document_id` 会话；按统一热度事件设计 assistant/evidence。 |
| `TASK-BACKEND-005.md` | 增加 `TASK-BACKEND-003` 依赖；移除 `DS-016`；考虑拆分为 links / analytics / deal rooms。 |
| `TASK-BACKEND-006.md` | 提升优先级或拆分安全扫描；补充 Salesforce 或明确排除。 |
| `README.md` | 更新任务清单、依赖图、版本映射。 |

---

## 10. 附录：评审文件清单

- `docs/PRD-v2.1.0.md`
- `docs/TDD-v2.1.0.md`
- `docs/API-SPEC-v2.1.0.md`
- `docs/ARCHITECTURE-v2.1.0.md`
- `docs/database-model-v2.1.0.md`
- `docs/HEAT-SCORE-ALGORITHM-v2.1.1.md`
- `docs/DESIGN-TOKENS-v2.1.1.md`
- `docs/INTERACTION-SPEC-v2.1.1-REFINED.md`
- `docs/IMPLEMENTATION-PLAN-v2.1.0.md`
- `docs/IMPLEMENTATION-PLAN-v2.1.1.md`
- `docs/IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md`
- `docs/tasks/agent-tasks-v2.1.2/*.md`

# DealSignal 前端实现 ↔ 文档端到端对齐评审

> **评审目标**：在启动 TASK 编码前，逐页/逐组件/逐接口精读前端实现，再与 PRD/TDD/API/ARCHITECTURE/DB/ALGORITHM/PLAN 逐项交叉，找出代码与文档之间的真实偏差，确保后续 AGENT-TASK 基于一致的事实源。
> **评审日期**：2026-06-21
> **前端范围**：`apps/web/src/**/*.{ts,tsx}`、`apps/web/src/i18n/locales/**/*.json`
> **文档范围**：`docs/PRD-v2.1.0.md`、`docs/TDD-v2.1.0.md`、`docs/API-SPEC-v2.1.0.md`、`docs/ARCHITECTURE-v2.1.0.md`、`docs/database-model-v2.1.0.md`、`docs/HEAT-SCORE-ALGORITHM-v2.1.1.md`、`docs/INTERACTION-SPEC-v2.1.1-REFINED.md`、`docs/IMPLEMENTATION-PLAN-v2.1.1.md`、`docs/tasks/agent-tasks-v2.1.2/*.md`

---

## 1. 执行摘要

### 1.1 总体结论

当前前端 `apps/web` 是一套 **UI 完整、但后端集成极不完整** 的 mock 实现。页面结构、导航、组件、i18n、主题、基础计算逻辑均已落地；但以下关键链路仍停留在前端本地模拟或占位状态：

- **无真实认证/Workspace 上下文传递**：`api.ts` 使用硬编码 `/api` 路径，不携带 `workspaceSlug`，也没有全局 token 注入。
- **上传、Viewer、AI、链接创建均为 mock/占位**：`Uploader` 不调用接口；`CanvasViewer` 不获取签名 URL/渲染真实页面；`aiStore` 用本地正则返回建议；`createLink` 忽略请求体。
- **热度评分算法与 UI 未连接**：`lib/heat/heatScore.ts` 存在但前端展示仍依赖 mock 的 `heatLevel`。
- **公开链接/访客访问流程缺失**：无 `/s/:token` 或公开 viewer 路由，权限门（email/password/NDA）没有 UI。

### 1.2 与文档的严重偏差

| 偏差类别 | 数量 | 说明 |
|----------|------|------|
| **API 路径/版本/上下文** | 高 | 前端调用 `/api/*`，文档要求 `/{workspaceSlug}/api/v1/*` 或 `/api/v1/public/*`。 |
| **请求/响应格式** | 高 | 前端 `request<T>` 不解析 `BaseResponse`，错误处理为简单 `Error`；mock 响应形状与 API-SPEC 不一致。 |
| **字段命名/枚举** | 高 | 代码大量 `camelCase`（`fileType`、`permissionType`），文档多为 `snake_case`（`source_type`、`permission_type`）；workspace 角色枚举不一致。 |
| **缺失端点** | 高 | 注册/登录/邀请、search、assistant/chat、public events、signed URL、public links、CRM/Slack sync 等前端均未调用。 |
| **算法实现差异** | 中 | `heatScore.ts` 的 trend 算法、key-page 匹配、opens/bounce caps 与 `HEAT-SCORE-ALGORITHM-v2.1.1.md` 不完全一致。 |
| **功能占位** | 中 | 大量 disabled/coming-soon 功能在 PRD 中属于 In Scope。 |

### 1.3 对 AGENT-TASK 的影响

- `TASK-FRONTEND-003`（API 集成层）必须显式包含 **路径重写、BaseResponse 解析、token 注入、workspace 上下文、MSW 开关**；当前任务描述偏简单。
- `TASK-BACKEND-002`（Auth/Workspace）必须优先输出可被前端消费的登录/注册/邀请端点，否则前端无法接入真实后端。
- `TASK-BACKEND-003`~`006` 的接口形状需要向前端当前 mock 兼容或明确约定迁移路径。
- `TASK-FRONTEND-001/002` 的范围应加入「清理 mock 数据语言一致性」和「Viewer 真实渲染」相关子项。

---

## 2. 评审方法

1. **代码通读**：4 个探索代理并行阅读路由、API/模拟/状态、组件、逻辑/i18n/测试。
2. **文档回查**：将代码中发现的字段、枚举、端点、状态机回查对应文档。
3. **逐项对齐**：按功能域（Workspace/Auth、Documents、Links、AI、Deal Rooms、Contacts、Insights、Settings）建立「代码实现—文档契约—偏差」矩阵。
4. **输出修正建议**：区分「应修改代码」「应修改文档」「应双方同步」三类。

---

## 3. 前端实现总览

### 3.1 路由与信息架构

| 路由 | 文件 | 状态 |
|------|------|------|
| `/` | `routes/workspaces.tsx` | 工作区选择器；单工作区自动跳转 |
| `/:workspaceSlug/dashboard` | `routes/dashboard.tsx` | ✅ 实现 |
| `/:workspaceSlug/documents` | `routes/documents.tsx` | ✅ 实现 |
| `/:workspaceSlug/documents/upload` | `routes/upload.tsx` | ⚠️ UI 完成，未真正上传 |
| `/:workspaceSlug/documents/:documentId` | `routes/documents/detail.tsx` | ⚠️ 详情页完成，AI/Viewer 为占位 |
| `/:workspaceSlug/links` | `routes/links.tsx` | ✅ 实现 |
| `/:workspaceSlug/links/new` | `routes/links/new.tsx` | ⚠️ UI 完成，创建为 mock |
| `/:workspaceSlug/links/:linkId` | `routes/links/detail.tsx` | ✅ 基本实现 |
| `/:workspaceSlug/deal-rooms` | `routes/deal-rooms.tsx` | ⚠️ 列表/创建完成，成员/审批/Q&A 缺失 |
| `/:workspaceSlug/deal-rooms/new` | `routes/deal-rooms/new.tsx` | ⚠️ 创建表单完成，后端未保存完整字段 |
| `/:workspaceSlug/deal-rooms/:roomId` | `routes/deal-rooms/detail.tsx` | ⚠️ 详情页完成，邀请/上传 disabled |
| `/:workspaceSlug/contacts` | `routes/contacts.tsx` | ✅ 实现 |
| `/:workspaceSlug/contacts/:contactId` | `routes/contacts/detail.tsx` | ⚠️ 详情页完成，Notes 只读 |
| `/:workspaceSlug/insights/*` | `routes/insights/*.tsx` | ⚠️ overview/pages/suggestions 实现，部分 PRD 子页缺失 |
| `/:workspaceSlug/settings/*` | `routes/settings/*.tsx` | ✅ general/language/brand/members/integrations/billing/security 实现 |
| `/viewer/:documentId` | `routes/viewer.tsx` | ⚠️ 占位 viewer，非公开链接入口 |

### 3.2 API 与 Mock 端点

当前 mock  handlers 覆盖 34 条 `/api/*` 路径，但：
- 无 `/api/auth/*`、`/api/v1/public/*`、search、assistant/chat、signed-url。
- 大量写操作忽略请求体（`POST /documents`、`POST /links`）。
- 响应形状不统一：有的包 `data`，有的直接返回对象，有的返回 `{ signals, actions }`。

### 3.3 状态与逻辑

- `uiStore`：sidebar、theme、currentWorkspace、uploadDialogOpen（持久化 theme/sidebar）。
- `signalStore`：signals/actions，调用 `/api/signals`。
- `aiStore`：纯本地正则回复，无 API。
- `lib/heat/heatScore.ts`：规则评分，但未与 UI 数据流连接。

### 3.4 i18n 与测试

- 11 个 namespace，支持 `en`/`zh-CN`，检测顺序合理。
- 7 个测试文件、53 个用例全通过；`WorkspaceSwitcher.test.tsx` 有 `act` 警告。

---

## 4. 逐项对齐矩阵

### 4.1 Workspace / Auth / Routing

| 文档要求 | 前端实现 | 偏差 | 建议处理方 |
|----------|----------|------|------------|
| `API-SPEC` §2.2：内部 API base 为 `/{workspaceSlug}/api/v1`，tenant 由 Host 解析 | `api.ts` 使用 `/api${path}`，无 `workspaceSlug`、无 `/v1` | 高 | 代码：在 `api.ts` 中注入 workspaceSlug 与版本段；或 backend 从 session 解析 |
| `API-SPEC` §2.3：Bearer Token 认证 | `api.ts` 仅在 `options.token` 存在时附加 header；全局无 token store | 高 | 代码：新增 auth store 与请求拦截器 |
| `PRD` §5.4 / §6.3：注册、登录、邀请链接、自动加入 Workspace | 无 `/login`、`/register`、邀请接受页面；无 auth provider | 高 | 代码+文档：补充登录/注册/邀请流程；API-SPEC 补 Auth 端点 |
| `API-SPEC` §5.2：workspace 角色 `OWNER/ADMIN/CONTRIBUTOR/VIEWER` | `types/index.ts` 使用 `owner/admin/member/guest` | 高 | 文档/代码：统一枚举；推荐 `owner/admin/member/guest` |
| `ARCHITECTURE` §7.2：邀请角色可选 admin/member/guest | `workspace_invitations` DDL 支持 owner/admin/member/guest；UI 未限制 | 中 | 文档：明确邀请时不允许选 owner |
| `PRD` §11.2：Settings → 权限 独立页面 | Settings 中无单独 Permissions；权限分散在 SmartLinkCreator 与 Members | 中 | 文档：若 v2.1.0 不单独设权限页，更新 IA |

### 4.2 Documents / Upload / Viewer

| 文档要求 | 前端实现 | 偏差 | 建议处理方 |
|----------|----------|------|------------|
| `API-SPEC` API-01：`POST /{ws}/api/v1/documents` 接收 `file` + `source_type` | `uploadDocument` 调用 `POST /api/documents` FormData；mock 忽略文件并返回 `mockDocuments[0]` | 高 | 代码：真实上传 + 轮询 ingestion 状态 |
| `API-SPEC` API-02：文档状态 `uploaded/processing/ready/failed` | `types/index.ts` 使用 `uploading/processing/ready/failed` | 中 | 代码：统一为 `uploaded`（与 API-SPEC/DB 一致） |
| `API-SPEC` API-03：`GET /documents/{id}/pages` 返回 `thumbnail_object_key` | 前端无该调用；DB `document_pages` 只有 `image_object_key` | 高 | 文档/代码：统一字段名（建议 `image_object_key`） |
| `API-SPEC` API-04：签名 URL `POST /documents/{id}/pages/signed-url` | 前端未实现 | 高 | 代码：补充签名 URL 调用 |
| `PRD` §8.2.4 / `TDD` §6.3.2：动态水印（邮箱/IP/时间） | `CanvasViewer` 水印硬编码 `viewer@dealsignal.com` | 高 | 代码+后端：API 返回安全 watermark payload |
| `PRD` §8.2.3：Viewer Canvas 渲染真实页面 | `CanvasViewer` 为占位 div，不获取页面图片 | 高 | 代码：接入签名 URL + webp 渲染 |
| `PRD` §8.2.1 FR-01/02：hash/size/type 校验、断点续传 | `Uploader` 仅做前端扩展名提示，无 hash/断点续传 | 中 | 代码+文档：明确 v2.1.0 是否支持断点续传 |
| `API-SPEC` §2.5：统一 `BaseResponse` | `request<T>` 直接 `response.json()`，不检查 `code` | 高 | 代码：增加 BaseResponse 解析层 |
| `TDD` §4.2.1：`source_type` 大写 `PDF/DOCX/...` | API-01 接受小写；类型使用 `fileType` camelCase | 中 | 文档/代码：统一并文档化归一化规则 |

### 4.3 Links / Permissions / Analytics

| 文档要求 | 前端实现 | 偏差 | 建议处理方 |
|----------|----------|------|------------|
| `API-SPEC` API-08：创建链接字段 `permission_type`、`allowed_emails`、`allowed_domains`、… | `SmartLinkCreator` 使用内部 `PermissionConfig`（`level`、`requireEmail`、`whitelistEnabled`、…），`api.createLink` 发送 `{ documentId, config }`；mock 忽略请求体 | 高 | 代码：前端 adapter 将 `PermissionConfig` 映射为 API 字段 |
| `API-SPEC` API-08：响应 `public_token`、`short_url`、`status` | mock 返回的 `Link` 有 `shortUrl`、`isActive`、`accessCount` | 中 | 代码：使用 API 字段名或增加 mapper |
| `API-SPEC` API-09：公开链接访问 `/api/v1/public/links/{publicToken}` | 前端无公开路由/页面 | 高 | 代码：新增公开落地页与权限门 |
| `PRD` §8.2.4：白名单支持邮箱/域名 | UI 有 whitelist 输入；DB 无 `allowed_domains` 字段 | 高 | 后端：新增 `allowed_domains` 列 |
| `API-SPEC` API-10：热度评分返回 factors（open_count、key_page_views 等） | 前端使用 mock `heatLevel`，未调用评分接口 | 高 | 代码+后端：连接 `lib/heat/heatScore.ts` 与 API-10 |
| `API-SPEC` API-05：事件上报 `link_opened/page_viewed/download_attempted` | 前端无事件上报 | 高 | 代码：在 viewer/link 中埋点并上报 |
| `HEAT-SCORE-ALGORITHM` §3.1：需要 7 种事件 | API-05 只有 3 种事件 | 高 | 文档/后端：扩展事件类型或推导规则 |
| `PRD` §8.2.6：链接撤回（revoke） | UI 只有 deactivate（`isActive: false`） | 中 | 文档：明确 deactivate 与 revoke 的映射关系 |
| `API-SPEC` API-10：热度响应字段 `tier`；算法使用 `level` | 代码 `HeatLevel` 使用 `hot/warm/cold` | 中 | 文档：统一命名 |

### 4.4 AI Assistant / Search

| 文档要求 | 前端实现 | 偏差 | 建议处理方 |
|----------|----------|------|------------|
| `API-SPEC` API-06：`POST /{ws}/api/v1/search` | 未实现 | 高 | 代码：新增 search 调用 |
| `API-SPEC` API-07：`POST /{ws}/api/v1/assistant/chat` | `aiStore` 本地正则回复，无 API 调用 | 高 | 代码：替换为真实 API |
| `API-SPEC` API-07：请求 `document_id`/`query`/`session_id` | `AIAssistant` 使用本地消息列表，无 `document_id` 概念 | 高 | 代码：根据上下文注入 `document_id` |
| `API-SPEC` API-07：evidence 字段 `chunk_id`、`quote`、`boxes`、`score` | 代码 `Evidence` 使用 `id`、`text`、`bbox` | 高 | 代码：对齐 API 字段 |
| `database-model` §4.5.1：`assistant_sessions.link_id` NOT NULL | 内部 AI 会话无 link | 高 | 后端：改为 nullable |
| `PRD` §8.2.5：evidence 点击跳转到页并高亮 bbox | `AIChat` evidence 点击仅 `alert()` | 高 | 代码：实现页面跳转 + bbox 高亮 |
| `aiStore.ts` 自动回复关键词含中文 | PRD 默认语言为 en | 低 | 代码：TASK-FRONTEND-001 已覆盖 |


### 4.5 Deal Rooms

| 文档要求 | 前端实现 | 偏差 | 建议处理方 |
|----------|----------|------|------------|
| `API-SPEC` API-12：创建数据室字段 `name`、`slug`、`template_type`、`requires_nda`、`requires_approval`、`documents` | `NewDealRoomPage` 发送 `name`、`description`、`templateId`、`ndaEnabled` | 高 | 代码+后端：统一字段；新增 `slug`、`requires_approval`、初始 `documents` |
| `API-SPEC` API-12：响应 `template_type`、`folders`、`members`、`access_requests` | mock 返回 `template`、`documentCount`、`memberCount`、`pendingApprovals`、`uploadedFiles` | 高 | 代码：对齐返回字段 |
| `PRD` §8.2.6：数据室 Q&A | 未实现 | 高 | 文档/代码：明确是否 MVP；若是，补充 model + UI |
| `PRD` §8.2.6：成员邀请、角色、审批流程 | UI 按钮 disabled | 高 | 代码：实现邀请/审批流 |
| `ARCHITECTURE` §9.3：访问申请状态 PENDING/APPROVED/REJECTED/CANCELLED/REVOKED | DB CHECK 只有 `pending/approved/rejected` | 中 | 后端：扩展状态枚举或标注虚拟状态 |
| `TDD` §4.2.4：`room_member_folder_permissions` 表 | 有表但无 API/UI 配置入口 | 中 | 代码：补充 folder 权限 UI 与端点 |

### 4.6 Contacts

| 文档要求 | 前端实现 | 偏差 | 建议处理方 |
|----------|----------|------|------------|
| `INTERACTION-SPEC` §6 / `PRD` §11.2：Contacts 为一级模块 | 页面已实现，但 `components/contacts/` 为空，逻辑全在 routes | 低 | 代码：可选提取复用组件 |
| `database-model`：无 `contacts` 表 | 联系人从 `link_accesses`/`room_members` 推断 | 高 | 后端：新增 `contacts` 表或物化视图 |
| `PRD` §8.2.5：Contacts 热度排序、趋势图、Notes、写邮件 | Notes 只读，写邮件 disabled | 中 | 代码+后端：逐步补全 |
| `API-SPEC`：Contact API 未定义 | 前端 mock 有 `/api/contacts/*` | 高 | 文档：在 API-SPEC 补充 Contact 端点 |

### 4.7 Insights / Heat Score

| 文档要求 | 前端实现 | 偏差 | 建议处理方 |
|----------|----------|------|------------|
| `HEAT-SCORE-ALGORITHM` §2.1：输入为 `ReaderEvent[]`，内部推导 opens/revisits/... | `computeHeatScore` 直接接收聚合后的 `HeatScoreInput` | 中 | 代码+后端：明确事件聚合由后端还是前端完成 |
| `HEAT-SCORE-ALGORITHM` §3.1：事件去重规则（30min 会话、5min page_view 去重等） | 前端无事件聚合实现 | 高 | 文档/代码：若由后端计算，前端无需实现；否则补充 |
| `HEAT-SCORE-ALGORITHM` §9.x：trend 由 `delta = current − previous` 决定 | `heatScore.ts` 使用当前输入启发式（revisits>0 & avgDuration>1 → rising） | 高 | 代码：修复为文档定义的 delta 算法 |
| `HEAT-SCORE-ALGORITHM`：key-page 匹配为 500 字符文本相似度 ≥0.3 | `heatScore.ts` 仅对 `title + pageNumber` 做子串匹配 | 中 | 代码：升级为文档算法或更新文档 |
| `HEAT-SCORE-ALGORITHM`：未文档化 `opens` cap 10、`bouncePenalty` cap 5 | `heatScore.ts` 实现了这些 cap | 低 | 文档：补充 cap 规则 |
| `PRD` §8.2.5 FR-10：热度评分 0-100 展示 | UI 直接展示 mock `heatLevel`/`score`，未调用 `computeHeatScore` | 高 | 代码：将算法接入 dashboard/links/contacts |
| `API-SPEC` API-10：热度响应 factors 字段不完整 | 前端未调用 | 高 | 后端：返回完整 7 维 factors；前端展示 |

### 4.8 Settings / i18n / Theme / Tests

| 文档要求 | 前端实现 | 偏差 | 建议处理方 |
|----------|----------|------|------------|
| `PRD` §11.2：Settings 包含 General/Language/Brand/Members/Integrations/Billing/Security | 全部实现 | ✅ | — |
| `PRD` §6.3：Workspace 名称/ slug/ brand color/ viewer domain 可配置 | 已实现；mock `workspaceSettings.name` 硬编码 "Acme Capital" | 低 | 代码：TASK-FRONTEND-001 已覆盖 |
| `PRD` §4.2 / `IMPLEMENTATION-PLAN-v2.1.1.md`：i18n en/zh-CN | 已实现 11 namespace | ✅ | — |
| `API-SPEC` §2.4：`Accept-Language` 头 | `api.ts` 未自动携带 | 中 | 代码：自动注入当前 i18n 语言 |
| `TDD` §10：前端测试覆盖率 ≥70% | 仅 7 个测试文件、53 用例；核心组件/上传/Viewer/AI 无测试 | 中 | 代码：逐步补测试 |
| `WorkspaceSwitcher.test.tsx` | 通过但有 `act(...)` 警告 | 低 | 代码：TASK-FRONTEND-001 已覆盖 |
| `INTERACTION-SPEC` §2.2：通知 bell 未读数 | UI 存在但 disabled；DB 无 in-app notifications 模型 | 中 | 后端+代码：新增模型或调整交互说明 |

---

## 5. 关键缺口汇总（按严重级别）

### 5.1 Critical（不解决则后端集成会失败）

1. **API 路径与版本**：前端使用 `/api/*`，后端需要 `/{workspaceSlug}/api/v1/*` 或 `/api/v1/public/*`。
2. **无全局认证/token 注入**：`api.ts` 不会自动附加 `Authorization`，真实后端所有请求都会 401。
3. **无 `BaseResponse` 解析层**：后端返回统一 envelope 后，前端类型与取值会全部错位。
4. **Workspace 上下文未传递**：`workspaceSlug` 未进入请求路径或 header，多工作区场景无法工作。
5. **关键端点缺失**：search、assistant/chat、signed-url、public links、auth、events 等前端均未实现调用。
6. **热度评分事件链路断裂**：API-05 只有 3 个事件，无法支撑算法 7 个维度。
7. **`assistant_sessions.link_id` NOT NULL**：与内部 AI 问答冲突。

### 5.2 High（会导致功能缩水或数据不一致）

8. 上传为 mock，不调用 `api.uploadDocument`。
9. Viewer 为占位，不获取签名 URL/渲染真实页面。
10. 动态水印硬编码，未从后端获取。
11. 智能链接创建忽略请求体，前端 `PermissionConfig` 与 API schema 不一致。
12. 公开链接/权限门（email/password/NDA）无前端页面。
13. `allowed_domains` 无 DB 字段。
14. `contacts` 无数据模型。
15. 热度评分算法与 UI 未连接，trend 算法不一致。
16. workspace 角色枚举 `CONTRIBUTOR/VIEWER` vs `member/guest` 不匹配。
17. 字段命名 `camelCase` vs `snake_case` 普遍存在。
18. 数据室创建字段与 API 不一致，成员/审批/Q&A 未实现。

### 5.3 Medium（影响体验或测试完整性）

19. 大量 disabled/coming-soon 功能在 PRD 中属于 In Scope。
20. Settings 权限页缺失、Insights 部分子页缺失。
21. `Accept-Language` 未自动注入。
22. 测试覆盖不足，上传/Viewer/AI/组件无测试。
23. Heat score key-page 匹配简化、opens/bounce caps 未文档化。
24. `document_pages.image_object_key` 与 `document_files(PAGE_WEBP)` 冗余。

---

## 6. 推荐修正方案

### 6.1 必须立即明确的架构决策

| 决策点 | 推荐方案 | 影响 |
|--------|----------|------|
| API 路径如何兼容前端 mock | 后端同时支持 `/api/*`（过渡）与 `/{workspaceSlug}/api/v1/*`；`TASK-FRONTEND-003` 负责逐步迁移到规范路径。 | 让后端可独立开发，前端逐步切换。 |
| Workspace 上下文传递 | 短期：后端从 JWT/session 解析当前 workspace；长期：URL path 显式传递。 | 避免前端大面积改动路由。 |
| 响应 envelope | 后端返回 `BaseResponse`，前端 `request<T>` 拆包 `data`。 | 统一契约。 |
| 字段命名 | 后端 API 使用 `snake_case`；前端在 `api.ts` 层使用 mapper 转换为内部 `camelCase` 类型。 | 保持前端类型习惯，同时满足 API 规范。 |
| 热度评分由谁计算 | **推荐后端计算**：前端仅上报原始事件，后端聚合事件并调用算法；API-10 返回分数与 factors。 | 避免前后端算法不一致，保护商业逻辑。 |
| AI 会话上下文 | `assistant_sessions.link_id` 改为 nullable；内部会话以 `document_id` + `user_id` 为 key。 | 解决 C4 阻塞项。 |

### 6.2 对 `api.ts` 的改造清单

1. 注入 `workspaceSlug`（从 URL params 或 store）到请求路径。
2. 注入 `/v1` 版本段（可配置开关）。
3. 从 auth store 自动附加 `Authorization: Bearer <token>`。
4. 自动附加 `Accept-Language`。
5. 自动附加 `X-Request-ID`。
6. 解析 `BaseResponse`：校验 `code === "ok"`，返回 `data`；错误时抛出结构化错误对象。
7. 统一错误处理：解析 `code/message/details/request_id`。
8. 为 `createLink`、`createDealRoom` 等增加 adapter，把前端 `camelCase` 结构映射为 API `snake_case`。

### 6.3 对 mock handlers 的改造清单

1. 统一所有 handler 返回 `BaseResponse` 结构。
2. 增加 `/api/auth/*`、`/api/v1/public/*`、search、assistant/chat、signed-url 等占位 handler。
3. 让 `POST /documents`、`POST /links` 真正读取请求体并创建资源。
4. 将 `workspaceSettings.name` 改为可配置默认值（TASK-FRONTEND-001）。
5. 增加 MSW server setup for tests/CI。

### 6.4 对文档的更新清单

1. `API-SPEC-v2.1.0.md`：
   - 补齐 Auth/Workspace/Invite 端点。
   - 补齐 Contacts、in-app notifications、folder permissions、Q&A 端点（或标注二期）。
   - 统一错误码大小写。
   - 统一响应 envelope 与示例。
   - 扩展 API-05 事件类型或说明推导规则。
   - 统一 API-10 字段与算法文档。
2. `database-model-v2.1.0.md`：
   - `assistant_sessions.link_id` nullable。
   - 新增 `links.allowed_domains`。
   - 新增 `contacts` 表。
   - 统一角色/状态/枚举大小写。
   - 解决 `document_pages.image_object_key` 与 `document_files(PAGE_WEBP)` 冗余。
3. `HEAT-SCORE-ALGORITHM-v2.1.1.md`：
   - 补充 opens/bounce caps、key-page 匹配细节、trend 计算精确公式。
   - 或按前端实现更新文档。
4. `IMPLEMENTATION-PLAN-ISSUES-v2.1.0.md` / AGENT-TASK：
   - 修正任务映射、依赖、范围、估算。

---

## 7. 对 AGENT-TASK 文件的具体修订建议

| 任务文件 | 当前问题 | 建议修订 |
|----------|----------|----------|
| `TASK-FRONTEND-001.md` | 仅关注 act 警告、AI 关键词、workspace name | 增加「统一 mock 响应为 `BaseResponse` 结构」「清理中英文混合 mock 文案」子项 |
| `TASK-FRONTEND-002.md` | 拆分 Viewer 子组件 | 明确 Canvas 策略：若使用 div/图片占位，则需接入真实 `image_object_key`；若用 Canvas，需处理 SSR。增加动态水印从后端 payload 获取。
| `TASK-FRONTEND-003.md` | 偏简单，只提 base URL 与错误处理 | 明确必须改造 `api.ts`：路径注入、版本段、token 注入、BaseResponse 拆包、结构化错误、Accept-Language；补充 MSW server setup。
| `TASK-BACKEND-001.md` | 脚手架 | 考虑是否纳入 `DS-004`（子域名/SSL）的接口占位。 |
| `TASK-BACKEND-002.md` | Auth/Workspace | 强调必须输出前端可消费的 login/register/invite 端点；统一角色枚举为 `owner/admin/member/guest`。 |
| `TASK-BACKEND-003.md` | Upload/Ingestion | 明确上传架构（直传 vs 代理）、页面图片事实源、`source_type` 大小写归一化。 |
| `TASK-BACKEND-004.md` | Search/Assistant | 扩展 API-05 事件类型或推导规则；`assistant_sessions.link_id` nullable；API-07 支持 `document_id` 内部会话。 |
| `TASK-BACKEND-005.md` | Links/Analytics/Rooms | 新增 `allowed_domains` 列；拆分或扩大文件数/行数上限；移除 `DS-016`；增加 `TASK-BACKEND-003` 依赖。 |
| `TASK-BACKEND-006.md` | Notify/Integrations/Security | 提升为 P0 或拆分 `DS-025`；明确 Salesforce 范围。 |
| `README.md` | 依赖图未反映真实关系 | 更新依赖图：`TASK-BACKEND-005` 依赖 `TASK-BACKEND-003`；`TASK-FRONTEND-003` 完整验证需后端全链路就绪。 |

---

## 8. 下一步建议

1. **召开文档同步会**：基于本报告与前一份 `PRD-TDD-API-PLAN-TASK-consistency-review-v2.1.2.md`，逐条确认架构决策（API 路径、响应 envelope、字段命名、热度评分计算位置、AI 会话上下文）。
2. **先修文档，再修代码**：
   - 第 1 轮：统一 API-SPEC / DB model / PRD 中的枚举、字段、错误码、响应格式。
   - 第 2 轮：更新 AGENT-TASK 文件（映射、依赖、范围、估算）。
   - 第 3 轮：启动 `TASK-FRONTEND-003` 与 `TASK-BACKEND-001/002` 的并行开发。
3. **建立 API 契约测试**：在 `TASK-FRONTEND-003` 中增加 contract tests，校验前端请求/响应与 API-SPEC 一致。

---

## 9. 附录：评审时代码中确认的关键事实

- `api.ts` 当前请求路径：`fetch(`/api${path}`)`（`apps/web/src/lib/api.ts:74`）。
- `api.ts` 当前错误处理：`throw new Error(`API error: ${response.status} ${response.statusText}`)`（`apps/web/src/lib/api.ts:79`）。
- `heatScore.ts` 当前 trend 逻辑：基于 `revisits > 0 && avgDurationMinutes > 1`（`apps/web/src/lib/heat/heatScore.ts:129-134`）。
- `Uploader` 未真实上传：进度条为 `setInterval` 模拟（`components/upload/Uploader.tsx`）。
- `CanvasViewer` 水印硬编码：`viewer@dealsignal.com`（`components/viewer/CanvasViewer.tsx`）。
- `aiStore` 本地正则：关键词含中文（`stores/aiStore.ts`）。
- `WorkspaceSwitcher.test.tsx`：`act` 警告（已确认）。

---

> **关联文档**：
> - 前序一致性评审：`docs/reviews/PRD-TDD-API-PLAN-TASK-consistency-review-v2.1.2.md`
> - 任务目录：`docs/tasks/agent-tasks-v2.1.2/`

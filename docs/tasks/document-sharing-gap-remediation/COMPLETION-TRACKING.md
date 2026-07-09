# DealSignal 文档分享业务：设计 vs 代码完成度追踪

**追踪版本**：v1.0.0  
**日期**：2026-07-08  
**设计依据**：`/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §5–10  
**代码范围**：`apps/api/internal/link/*`、`apps/api/internal/notification/*`、`apps/api/internal/analytics/*`、`apps/api/internal/heat/*`、`apps/web/src/components/links/share/*`、`apps/web/src/components/viewer/*`、`apps/web/src/components/deal-rooms/DealRoomShareDialog.tsx`  
**任务目录**：`docs/tasks/document-sharing-gap-remediation`

---

## 1. 执行摘要

| 维度 | 总数 | ✅ 已完成 | ⚠️ 部分完成 | ❌ 未开始 | 完成度 |
|---|---:|---:|---:|---:|---:|
| 数据模型 | 10 | 5 | 1 | 4 | 55% |
| 后端服务层 | 10 | 6 | 4 | 0 | 70% |
| API 路由 | 12 | 10 | 0 | 2 | 83% |
| 会话与安全失效 | 3 | 1 | 2 | 0 | 50% |
| 安全审计 | 4 | 1 | 2 | 1 | 38% |
| 邮件与通知 | 5 | 1 | 3 | 1 | 35% |
| 前端三 Tab 弹窗 | 14 | 9 | 4 | 1 | 68% |
| 公共 Viewer | 6 | 5 | 0 | 1 | 83% |
| Analytics / 生命周期 | 5 | 1 | 0 | 4 | 20% |
| **合计** | **69** | **39** | **16** | **14** | **63%** |

> **说明**：完成度按“已完成=1、部分完成=0.5、未开始=0”加权计算。大量“部分完成”项涉及安全、异步、合规等高风险缺口，因此虽然表面完成度 63%，**核心 P0 阻塞项几乎全部未闭合**。

---

## 2. 关键阻塞项（必须先行）

1. **占位开关误导用户**（SHORT-006 / SHORT-008 / SHORT-009 / MID-008）：前端可切换但后端无字段。
2. **`/r/:slug` 未重定向**、Link 归档/续期/过期提醒未实现（MID-007）。
3. **Analytics / 生命周期 / retention**（MID-007 / INFRA-003 / COMPLIANCE-001）。

> 已完成并解除的阻塞项：INFRA-001（迁移编号统一）、SHORT-005-A（邀请 token hash + security_version）、SHORT-005-B（访问请求闭环）、SHORT-002（通知收件人/邮件开关）、INFRA-002（邮件持久化 + worker）、SHORT-003 核心字段（tenant_id/workspace_id）。

---

## 3. 按子系统完成度明细

### 3.1 数据模型

| 设计需求 | 状态 | 证据 | 缺口 | 映射任务 |
|---|---|---|---|---|
| `links.deal_room_id`、`require_password`、`password_hash` | ✅ 完成 | `042_deal_room_sharing.up.sql:6–9` | — | — |
| `document_id` 与 `deal_room_id` 互斥约束 | ✅ 完成 | `042_deal_room_sharing.up.sql:16–24`；`046_links_document_id_nullable.up.sql:3–4` | — | — |
| `link_access_rules` 表 + 索引 | ✅ 完成 | `042_deal_room_sharing.up.sql:34–51` | — | — |
| `link_invitations` 表 + 索引 | ✅ 完成 | `042_deal_room_sharing.up.sql:52–71`；`047_invitation_token_hash_and_security_version.up.sql:14–17` | token 改为 hash 存储，新增 `token_hash` | SHORT-005-A |
| 规则变更通过 `links.security_version` 使 session 失效 | ✅ 完成 | `047_invitation_token_hash_and_security_version.up.sql:11–12`；`session.go:25–34`；`handler.go:749–752` | 改用 `security_version` 精确失效 | SHORT-005-A |
| 邀请 token hash 迁移 | ✅ 完成 | `047_invitation_token_hash_and_security_version.up.sql:14–17` | 新增 `token_hash`；历史 token 由应用 lazy backfill | SHORT-005-A |
| `links.security_version` 列 | ✅ 完成 | `047_invitation_token_hash_and_security_version.up.sql:11–12` | 用于精确 session 失效 | SHORT-005-A |
| `link_access_rule_revisions` 审计快照表 | ⚠️ 部分完成 | `048_link_access_rule_revisions.up.sql` | 表已建，服务层写入逻辑待补 | SHORT-005-A |
| `link_access_requests` 表 | ✅ 完成 | `049_link_access_requests.up.sql` | 支持 pending/approved/rejected | SHORT-005-B / SHORT-007 |
| Migration 编号统一编排 | ✅ 完成 | 新增迁移统一编号 047–055；现有 `046_links_document_id_nullable.up.sql` 保留，无冲突 | INFRA-001 已重新统一编号 | INFRA-001 |

### 3.2 后端服务层

| 设计需求 | 状态 | 证据 | 缺口 | 映射任务 |
|---|---|---|---|---|
| `CreateDealRoomLink` | ✅ 完成 | `apps/api/internal/link/service.go:640–658` | — | — |
| `EvaluateAccessRules`（fail-closed，block > allow，email > domain） | ✅ 完成 | `service.go:673–795` | — | — |
| `UpdateAccessRules` 全量替换 + 校验 + 撤销邀请联动 | ✅ 完成 | `service.go:826–918` | — | — |
| `InviteViewers` 创建 token / allow rule / 发送邮件 | ⚠️ 部分完成 | `service.go:950–1089` | Token 改为 hash 存储；always 自动加入 allow list，未遵循前端 `autoAddInvited` 开关 | SHORT-005-A / SHORT-006 |
| `ResolveInviteToken` | ✅ 完成 | `service.go:1092–1118` | 按 HMAC-SHA256 hash 查询 | SHORT-005-A |
| `RevokeInvitation` + 从 allow list 移除 | ✅ 完成 | `service.go:1120–1167` | — | — |
| `Access()` 集成规则评估、密码、OTP、NDA | ✅ 完成 | `service.go:1299–1447` | — | — |
| bcrypt 密码 hash 与常量时间比较 | ✅ 完成 | `service.go:1948–1974` | — | — |
| 访问通知邮件发送给创建者 | ⚠️ 部分完成 | `service.go:1234–1256`；`service.go:1440–1444` | 直接 goroutine 调用 mailer，未入 `notifications` 表；未检查 `email_enabled` | SHORT-002 / SHORT-007 / INFRA-002 |
| 安全事件记录 | ⚠️ 部分完成 | `apps/api/internal/analytics/service.go:135–145` | `security_events` 缺少 `tenant_id`/`workspace_id`；异常阈值硬编码 | SHORT-003 / INFRA-003 |

### 3.3 API 路由

| 设计路由 | 状态 | 证据 | 缺口 | 映射任务 |
|---|---|---|---|---|
| `POST /deal-rooms/:id/links` | ✅ 完成 | `apps/api/internal/link/handler.go:65–66` | — | — |
| `GET /deal-rooms/:id/links` | ✅ 完成 | `handler.go:67` | — | — |
| `POST /links/:id/access-rules` | ✅ 完成 | `handler.go:59` | — | — |
| `GET /links/:id/access-rules` | ✅ 完成 | `handler.go:58` | — | — |
| `POST /links/:id/invitations` | ✅ 完成 | `handler.go:61` | — | — |
| `GET /links/:id/invitations` | ✅ 完成 | `handler.go:60` | — | — |
| `POST /links/:id/invitations/:invitationId/revoke` | ✅ 完成 | `handler.go:62` | — | — |
| `GET /api/v1/public/links/:token` | ✅ 完成 | `handler.go:72` | — | — |
| `POST /api/v1/public/links/:token` | ✅ 完成 | `handler.go:73` | — | — |
| `POST /api/v1/public/links/:token/send-email-code` | ✅ 完成 | `handler.go:74` | — | — |
| `POST /api/v1/public/links/:token/access-requests` | ✅ 完成 | `handler.go`：`CreateAccessRequest` | 访客可提交请求访问 | SHORT-005-B / SHORT-007 |
| `POST /links/:id/access-requests/:requestId/approve` | ✅ 完成 | `handler.go`：`ApproveAccessRequest` | 创建者可审批并自动发邀请 | SHORT-007 |

### 3.4 会话与安全失效

| 设计需求 | 状态 | 证据 | 缺口 | 映射任务 |
|---|---|---|---|---|
| HMAC 签名 `LinkSession`，15 分钟滑动过期 | ✅ 完成 | `apps/api/internal/link/session.go:25–96` | — | — |
| `PasswordVerified` 字段 | ✅ 完成 | `session.go:30` | — | — |
| 规则/密码变更后旧 session 失效 | ✅ 完成 | `handler.go:749–752`；`session.go:25–34` | 使用 `links.security_version` 精确失效 | SHORT-005-A |
| 邀请 token 一次性或限时有效 | ✅ 完成 | `service.go:1092–1118`（检查 `expired`/`revoked`/`used_at`） | — | — |

### 3.5 安全审计

| 设计需求 | 状态 | 证据 | 缺口 | 映射任务 |
|---|---|---|---|---|
| 扩展 `security_events.event_type`（blocked_email / blocked_domain / invite_token_redeemed 等） | ✅ 完成 | `044_expand_complete_security_event_types.up.sql`；`045...` | — | — |
| `RecordSecurityEvent` 写入失败/异常访问 | ✅ 完成 | `analytics/service.go:135–145`；`link/service.go:1574–1592` | `tenant_id` / `workspace_id` 已随事件写入；`recordSecurityEvent` 同样填充 | SHORT-003 |
| 异常访问检测（abnormal_access_pattern） | ⚠️ 部分完成 | `handler.go:39–41`；`service.go:1395–1402` | 阈值硬编码 5 事件/5 分钟，无可配置参数 | SHORT-003 |
| 安全事件 retention / partitioning | ❌ 未开始 | 无 TTL 或分区策略 | 长期数据合规风险 | INFRA-003 / COMPLIANCE-001 |

### 3.6 邮件与通知

| 设计需求 | 状态 | 证据 | 缺口 | 映射任务 |
|---|---|---|---|---|
| 邀请邮件模板 + 发送 | ✅ 完成 | `apps/api/internal/mailer/template/templates.go:300–379`；`link/service.go:1463–1502` | 通过 `notification.Service.EnqueueEmailJob` 入队，由 notification worker 异步消费 | SHORT-007 / INFRA-002 |
| 访问通知邮件 | ✅ 完成 | `link/service.go:1484–1502` | 统一入 `notifications` 表；worker 发送时检查 `email_enabled` | SHORT-002 / SHORT-007 / INFRA-002 |
| `notification.Service.Enqueue("email")` | ✅ 完成 | `apps/api/internal/notification/service.go:62–158` | 仅写入 `notifications` 表，不再同步发送 | INFRA-002 |
| 邮件收件人兜底 `SMTP_USER` | ✅ 完成 | `apps/api/internal/notification/service.go:176–198` | 已移除 fallback；必须解析 `users.email` 且 `email_verified=true` | SHORT-002 |
| 持久化 email worker（重试/死信） | ✅ 完成 | `notification/service.go:200–293`；`notification/worker.go`；`057_notification_async_worker.up.sql` | `SELECT FOR UPDATE SKIP LOCKED` 锁定；指数退避重试；最终 `dead` 并记录 dead letter | INFRA-002 |

### 3.7 前端 Share / Invite / Access 三 Tab 弹窗

| 设计需求 | 状态 | 证据 | 缺口 | 映射任务 |
|---|---|---|---|---|
| `DealRoomShareDialog` / `LinkShareDialog` 存在 | ✅ 完成 | `apps/web/src/components/deal-rooms/DealRoomShareDialog.tsx`；`apps/web/src/components/links/LinkShareDialog.tsx` | — | — |
| Header：link name、short URL、复制、Active toggle | ✅ 完成 | `DealRoomShareDialog.tsx:307–341` | — | — |
| Footer 保存按钮随 Tab 变化文案 | ✅ 完成 | `DealRoomShareDialog.tsx:431–452` | — | — |
| 未保存离开提示 | ✅ 完成 | `DealRoomShareDialog.tsx:101–149` | — | — |
| Active toggle 禁用二次确认 | ✅ 完成 | `DealRoomShareDialog.tsx:207–234` | — | — |
| allow list 存在时显示受限提示 | ✅ 完成 | `DealRoomShareDialog.tsx:343–354` | — | — |
| Preset public / standard / confidential / custom | ⚠️ 部分完成 | `ShareTab.tsx:89–105`；`presets.ts`；`DealRoomShareDialog.tsx:380–394` | 自动填充缺少 200ms 字段高亮反馈；手动修改通过推断而非显式 Custom 状态 | SHORT-006 |
| Share Tab workspace domain 下拉 | ❌ 未开始 | `ShareTab.tsx:141–149` 为自由文本输入 | 需从 workspace 配置读取域名列表 | SHORT-006 |
| Access Tab 字段分层（Authentication / Allowed / Blocked / Additional / Advanced） | ✅ 完成 | `AccessTab.tsx:52–182` | — | — |
| Advanced 折叠 + enabled-count badge | ✅ 完成 | `AccessTab.tsx:161–182` | — | — |
| `EmailTagInput` chip 添加/删除 | ⚠️ 部分完成 | `EmailTagInput.tsx` | 缺少缩放/透明度微动画 | SHORT-006 |
| Revoke 邀请二次确认 | ✅ 完成 | `DealRoomShareDialog.tsx:271–291` | — | — |
| Resend tooltip | ⚠️ 部分完成 | `InviteTab.tsx` | 未确认已按设计实现 hover tooltip | SHORT-006 |
| 旧 `/r/:slug` 兼容提示 | ✅ 完成 | `ShareTab.tsx:181–185` | — | — |
| 占位开关（qa/fileRequests/index/screenshot） | ⚠️ 部分完成 | `AccessTab.tsx:17`；`buildDraft:44–48` | UI 可切换但后端无字段，保存后会被重置 | SHORT-006 / SHORT-008 / SHORT-009 / MID-008 |

### 3.8 公共 Viewer `/l/:token`

| 设计需求 | 状态 | 证据 | 缺口 | 映射任务 |
|---|---|---|---|---|
| 加载 link 元信息、门控、session 复用 | ✅ 完成 | `apps/web/src/components/viewer/PublicViewerPage.tsx` | — | — |
| inviteToken 锁定邮箱只读 | ✅ 完成 | `handler.go:728–732`；`service.go:1325–1338` | — | — |
| Email / OTP / Password / NDA 门控 | ✅ 完成 | `PublicViewerPage.tsx` | — | — |
| Blocked / not_allowed 错误页 | ✅ 完成 | `PublicViewerPage.tsx` | — | — |
| `deal_room_id` 渲染 Deal Room 视图 | ✅ 完成 | `PublicViewerPage.tsx` → `PublicDealRoomLinkViewer` | — | — |
| “Request access” 请求访问表单 | ✅ 完成 | `PublicViewerPage.tsx:235–323`；`api.ts:349–355`；`types/index.ts:127–135`；`documents.json:203–214` | 表单在 `blocked_email`/`blocked_domain`/`not_allowed` 错误页展示；调用公共端点创建访问请求 | SHORT-007 |

### 3.9 Analytics 与生命周期

| 设计需求 | 状态 | 证据 | 缺口 | 映射任务 |
|---|---|---|---|---|
| Analytics Tab 存在 | ✅ 完成 | `DealRoomShareDialog.tsx:419–423` | 已有 UI 入口 | — |
| 后端 `GetLinkAnalytics` | ❌ 未开始 | `analytics/service.go` 无 link 级聚合 | 最近访问者、停留时长、下载次数等未实现 | MID-007 |
| Link 续期 / 归档 | ❌ 未开始 | 无 `links.status` / archive API | 设计中的 `archived` 状态未落地 | MID-007 |
| 过期前 24h/7d 提醒 cron | ❌ 未开始 | 无 cron 任务 | 业务中断风险 | MID-007 |
| `/r/:slug` 重定向到默认 `/l/:token` 并保留归因 | ❌ 未开始 | `apps/web/src/routes/deal-rooms/public.tsx:26` 仍为旧流程 | 未重定向 | MID-007 |
| `key_page_views` 语义修正 | ❌ 未开始 | `apps/api/internal/heat/score.go` 按 document title 匹配；`queries.sql:414–423` 返回 `document_title` | 设计/任务要求按 page 级标题匹配 | MID-001 |

---

## 4. 任务级完成度映射

> 所有任务文件均位于当前目录。新增的 `INFRA-*` / `COMPLIANCE-001` 任务文件已创建，用于承载跨任务基础设施与合规工作。

| 任务文件 | 任务 ID | 标题 | 计划范围 | 实际完成度 | 剩余核心缺口 |
|---|---|---|---|---|---|
| [TASK-SHARE-SHORT-001.md](./TASK-SHARE-SHORT-001.md) | TASK-SHARE-SHORT-001 | 公共 Viewer AI Copilot 权限 | fullstack | ~90% | AI 问答记录按 link 过滤（可选） |
| [TASK-SHARE-SHORT-002.md](./TASK-SHARE-SHORT-002.md) | TASK-SHARE-SHORT-002 | 通知收件人与邮件开关 | backend | ~95% | 收件人解析、email_enabled 开关、前端集成设置均完成；仅待运营告警 metric（可选） |
| [TASK-SHARE-SHORT-003.md](./TASK-SHARE-SHORT-003.md) | TASK-SHARE-SHORT-003 | 安全审计事件 | backend | ~70% | `tenant_id`/`workspace_id` 已补齐；剩余阈值可配置化、retention/分区策略 |
| [TASK-SHARE-SHORT-004.md](./TASK-SHARE-SHORT-004.md) | TASK-SHARE-SHORT-004 | 访问与页面浏览去重 | backend | ~95% | 30min/5min 窗口已生效 |
| [TASK-SHARE-SHORT-005.md](./TASK-SHARE-SHORT-005.md) | TASK-SHARE-SHORT-005 | Deal Room / 文档链接分享后端核心 | backend | ~95% | token hash、security_version、access_requests、rule revisions 已落地；剩余 UI 请求访问表单 |
| [TASK-SHARE-SHORT-006.md](./TASK-SHARE-SHORT-006.md) | TASK-SHARE-SHORT-006 | 前端三 Tab 弹窗 | frontend | ~70% | Preset 高亮反馈、domain 下拉、占位开关处理、微动画 |
| [TASK-SHARE-SHORT-007.md](./TASK-SHARE-SHORT-007.md) | TASK-SHARE-SHORT-007 | 邀请邮件、访问通知、请求访问 | fullstack | ~98% | 后端访问请求闭环（公共端点 + 审批 + allow-rule + 邀请邮件）已落地并接入每 IP 每 link 5 次/小时限流；前端请求访问 UI 已补齐；前后端测试全绿；剩余 E2E 与 PR 关联 |
| [TASK-SHARE-SHORT-008.md](./TASK-SHARE-SHORT-008.md) | TASK-SHARE-SHORT-008 | AI Assistant + Visitor Q&A | fullstack | ~10% | `qa_enabled` 占位，无后端字段与 Q&A 面板 |
| [TASK-SHARE-SHORT-009.md](./TASK-SHARE-SHORT-009.md) | TASK-SHARE-SHORT-009 | 访客文件请求 MVP | fullstack | ~10% | `file_requests_enabled` 占位，无表无 API |
| [TASK-SHARE-MID-001.md](./TASK-SHARE-MID-001.md) | TASK-SHARE-MID-001 | Key Page Views 语义修正 | backend | ~40% | 当前按 document title 而非 page title |
| [TASK-SHARE-MID-002.md](./TASK-SHARE-MID-002.md) | TASK-SHARE-MID-002 | 扩展追踪事件体系 | fullstack | ~30% | forward/return/scroll/ai 等事件类型未完全加入 |
| [TASK-SHARE-MID-003.md](./TASK-SHARE-MID-003.md) | TASK-SHARE-MID-003 | 通知规则引擎 | backend | ~5% | 无 `notification_rules` 表与合并窗口 |
| [TASK-SHARE-MID-005.md](./TASK-SHARE-MID-005.md) | TASK-SHARE-MID-005 | 页面与下载签名 URL | backend | ~5% | 签名 URL 未实现 |
| [TASK-SHARE-MID-006.md](./TASK-SHARE-MID-006.md) | TASK-SHARE-MID-006 | 可信水印 | fullstack | ~40% | 服务端 `watermarkText` 与审计日志待完善 |
| [TASK-SHARE-MID-007.md](./TASK-SHARE-MID-007.md) | TASK-SHARE-MID-007 | Link Analytics 与生命周期 | fullstack | ~30% | Analytics Tab 已存在；生命周期/归档/续期/重定向未实现 |
| [TASK-SHARE-MID-008.md](./TASK-SHARE-MID-008.md) | TASK-SHARE-MID-008 | 索引文件自动生成 | fullstack | ~5% | `index_file_enabled` 占位，无后端 |
| [TASK-SHARE-MID-009.md](./TASK-SHARE-MID-009.md) | TASK-SHARE-MID-009 | 文件收集链接 | fullstack | ~0% | 建议改为 `link_type=file_request`；无表无 API |
| [TASK-SHARE-INFRA-001.md](./TASK-SHARE-INFRA-001.md) | INFRA-001 | Schema 统一编排 | infra | ~100% | 新增迁移统一编号 047–057；与 046 无冲突 |
| [TASK-SHARE-INFRA-002.md](./TASK-SHARE-INFRA-002.md) | INFRA-002 | 可靠异步通知 worker | infra | ~95% | email 入队、worker 锁定消费、重试/死信完成；剩余 metric/告警（可选） |
| [TASK-SHARE-INFRA-003.md](./TASK-SHARE-INFRA-003.md) | INFRA-003 | 事件 retention / partitioning | infra | ~0% | access_logs / page_views / security_events 无 TTL |
| [TASK-SHARE-COMPLIANCE-001.md](./TASK-SHARE-COMPLIANCE-001.md) | COMPLIANCE-001 | Sharing PII 最小化与 retention | compliance | ~0% | 未启动 |

---

## 5. 建议优先级排序

```text
P0 阻塞（本周必须启动）
├── SHORT-006    收尾 Preset 反馈、domain 下拉、占位开关处理
├── SHORT-007    请求访问 UI + 审批联动
└── SHORT-008 / SHORT-009 / MID-008    qa / file requests / index file

P1 紧后（下周启动，依赖 P0）
├── MID-007    Link 生命周期 + /r/:slug 重定向
├── MID-001    Key Page Views 语义修正
└── MID-003    通知规则引擎

P2 中期
├── INFRA-003    retention / partitioning
└── SHORT-003    异常访问阈值可配置

P3 长期/合规
└── COMPLIANCE-001   PII 最小化、导出删除流程
```

---

## 6. 更新记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0.0 | 2026-07-08 | 初版，基于细胞级代码审阅与任务计划 v1.4.0 |
| v1.1.0 | 2026-07-09 | 完成 INFRA-001/002、SHORT-002/003/005-A/005-B 核心实现；更新完成度与优先级 |

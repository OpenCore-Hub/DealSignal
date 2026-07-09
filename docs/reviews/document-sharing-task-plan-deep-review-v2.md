# DealSignal 文档分享业务缺口修复任务计划：代码级深度评审 v2

> 评审角色：资深架构师 + 高级产品经理  
> 评审方法：只读代码分析 + 任务计划对照  
> 评审范围：`apps/api/internal/link/*`、`apps/api/internal/notification/*`、`apps/api/internal/analytics/*`、`apps/api/internal/heat/*`、`apps/web/src/components/links/share/*`、`apps/web/src/components/viewer/*`  
> 评审日期：2026-07-08  
> 计划版本：v1.3.0 → 建议升级为 v1.4.0

---

## 0. 核心结论

经过对关键代码路径的逐文件分析，任务计划与当前实现的差距比表面看更大。**有 3 个任务（SHORT-003/SHORT-005/SHORT-007）的“已完成/部分完成”状态需要下调，有 2 个关键基础设施任务（异步通知、schema 统一编排）尚未进入计划。**

### 0.1 最关键的发现

| # | 发现 | 影响 |
|---|---|---|
| 1 | **邀请 token 仍明文存储**：`link_invitations.token` 为 `TEXT` 明文，`ResolveInviteToken` 按明文查询 | SHORT-005 安全红线未闭合 |
| 2 | **迁移编号 046 已被占用**：仓库已有 `046_links_document_id_nullable.up.sql`（untracked），与计划中的 `046_invitation_token_hash` 冲突 | 多个特性分支合并必冲突 |
| 3 | **通知系统并非真正异步**：`notification.Enqueue("email")` 同步调用 `sendEmail`；邮件绕过 `notifications` 表；worker 只处理 Slack | SHORT-007 “邮件必须异步”不成立 |
| 4 | **`SMTP_USER` 仍是收件人兜底**：`notification/service.go:145` 在未传 userID 时发到 `SMTP_USER` | SHORT-002 红线未闭合 |
| 5 | **`link_access_requests` 表完全缺失**：只有 `room_access_requests` | SHORT-005/007 核心闭环未实现 |
| 6 | **`permission_type` 当前枚举是 `('public','email_required','nda')`，不是计划里的 `whitelist/password`** | MID-009 扩展方案需重新审视 |
| 7 | **Analytics Tab 已存在**：MID-007 不应再“新建 Analytics Tab”，而应补齐生命周期/归档/续期/旧 slug | MID-007 范围需修正 |
| 8 | **Key Page 实现有 bug**：`heat.IsKeyPage` 已存在，但 `getScoreForLink` 只按 **document title** 判断，不是 page title；SQL 仍按 `duration_seconds >= 3` | MID-001 比计划描述更复杂 |
| 9 | **占位开关会误导用户**：`qaEnabled`、`fileRequestsEnabled`、`indexFileEnabled`、`screenshotProtectionEnabled` 在 UI 可切换但后端无字段，保存后会被重置 | 产品体验风险 |

---

## 1. 代码现状 vs 任务计划逐项对照

### 1.1 SHORT-001 公共 Viewer AI Copilot

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| 后端公共 AI 端点 | `POST /v1/public/assistant/chat` 存在，校验 `ai_copilot_enabled` 与 `X-Link-Session` | ✅ 已闭环 |
| 前端条件渲染 AI tab | `RightSidebar` 读取 `aiCopilotEnabled` | ✅ 已闭环 |
| 会话隔离 | `publicAssistantChat` 带 `X-Link-Session`；后端按 link+visitor 隔离 | ✅ 已闭环 |

**状态修正**：`已完成` 保持不变。

### 1.2 SHORT-002 通知收件人与邮件开关

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| 收件人来自 `link.created_by` → `users.email` | `notification.sendEmail` 优先查 `users.email` | ⚠️ 部分满足 |
| 不再用 `SMTP_USER` 作为收件人 | 当 `userID` 为空/无效时仍 fallback 到 `s.cfg.SMTPUser` | ❌ **红线未闭合** |
| `email_enabled` 开关 | 前后端均存在，`sendEmail` 会检查 | ✅ 满足 |
| 租户隔离查邮箱 | `GetUserByID` 未带 `tenant_id` | ❌ 未满足 |
| 未验证邮箱处理 | 未检查 `EmailVerified` | ❌ 未满足 |

**状态修正**：保持 `部分完成`，新增阻塞项：必须移除 `SMTP_USER` fallback、补充 tenant 隔离与邮箱验证检查。

### 1.3 SHORT-003 安全审计事件

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| 记录安全失败事件 | `RecordSecurityEvent` + `link/handler.go` 已调用 | ✅ 基本满足 |
| 异常访问告警 | `CheckAnomaly` 存在，但阈值在 `link/handler.go:38-41` 硬编码为 `5 次/5 分钟` | ⚠️ 不满足可配置 |
| ≥90 天 retention | 无 schema/code  enforcing | ❌ 未满足 |
| `security_events` 字段完整 | 缺少 `tenant_id`、`workspace_id` | ⚠️ 影响租户隔离分析 |

**状态修正**：从 `已完成` 下调为 `部分完成`。收尾工作：阈值配置化、补充 tenant/workspace 列（可选但建议）、明确 retention。

### 1.4 SHORT-004 访问与页面浏览基础去重

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| 30 min open / 5 min page 去重 | `FailoverDedupChecker` 已实现，Redis + DB 兜底 | ✅ 满足 |
| 原子操作 | `SETNX` 实现 | ✅ 满足 |
| 窗口可配置 | `LINK_OPEN_DEDUP_WINDOW_MINUTES`、`PAGE_VIEW_DEDUP_WINDOW_MINUTES` | ✅ 满足 |
| `access_count` 保护 | CTE 中仅当未去重时递增 | ✅ 满足 |

**状态修正**：`已完成` 保持不变。

### 1.5 SHORT-005 Deal Room / 文档链接后端核心

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| `deal_room_id` / `password_hash` | 已支持 | ✅ |
| `link_access_rules` CRUD + 规则引擎 | 已实现，fail-closed | ✅ |
| `link_invitations` CRUD | 已实现 | ✅ |
| 邀请 token **hash 存储** | `token TEXT NOT NULL UNIQUE` 明文；按明文查询 | ❌ **关键缺口** |
| `links.security_version` | 不存在；session 失效用 `updated_at` + 触发器 | ❌ 未满足 |
| `link_access_requests` 表 + API | 不存在 | ❌ 未满足 |
| 规则变更后旧 session 失效 | 依赖 `updated_at`；密码修改不触发失效 | ⚠️ 有漏洞 |

**状态修正**：保持 `部分完成`，但剩余工作量比 README 描述的“收尾”大得多。建议拆分为：
- **SHORT-005-A**：token hash + `security_version` 会话失效（硬安全）。
- **SHORT-005-B**：`link_access_requests` 表 + 公共提交 + owner 审批 API。

### 1.6 SHORT-006 前端 Share / Invite / Access 三 Tab 弹窗

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| 三 Tab 职责分离 | 已实现 | ✅ |
| Preset Custom 状态 | `inferPreset` 自动变 custom；但缺少字段高亮动画 | ⚠️ 部分 |
| allow/block 冲突 inline 校验 | `validateDraft` 已实现 | ✅ |
| Revoke 二次确认 | 已实现 | ✅ |
| 所有文案 i18n | 基本走 `t()`；`AccessTab.tsx:89` 密码显隐 `aria-label` 硬编码英文 | ⚠️ 小缺口 |
| 占位开关处理 | `qaEnabled`/`fileRequestsEnabled`/`indexFileEnabled`/`screenshotProtectionEnabled` 可切换但后端无字段 | ❌ **产品风险** |

**状态修正**：保持 `部分完成`。建议立即隐藏未就绪占位开关或加“即将上线”提示。

### 1.7 SHORT-007 邀请邮件、访问通知与请求访问闭环

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| 邀请邮件含 `inviteToken` | `link/service.go:1210` 已构造 | ✅ |
| 访问通知邮件 | `link/service.go:1234-1256` goroutine 发送 | ⚠️ 非持久化异步 |
| 邮件通过 `notifications` + worker 异步 | `notification.Enqueue("email")` 同步；worker 只处理 Slack | ❌ **不满足** |
| `link_access_requests` 创建/审批 | 不存在 | ❌ **不满足** |
| 审批后自动加入 allow list + 发邀请 | 不存在 | ❌ **不满足** |
| `email_enabled` 控制 link 邮件 | link 服务未检查该开关 | ❌ 不满足 |

**状态修正**：从 `部分完成` 下调为 `待执行/核心未开始`。前端 Request access 表单、后端 access_requests、持久化通知 worker 均未实现。

### 1.8 SHORT-008 AI Assistant + Visitor Q&A

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| 后端 `links.qa_enabled` 列 | 不存在 | ❌ |
| `link_visitor_questions` 表 | 不存在 | ❌ |
| 公共/owner API | 不存在 | ❌ |
| 统一 Q&A 面板 | `RightSidebar` 只有 Documents/AI | ❌ |

**状态修正**：`待执行` 保持不变。

### 1.9 SHORT-009 访客文件请求

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| `links.file_requests_enabled` 列 | 不存在 | ❌ |
| `link_file_requests` 表 | 不存在 | ❌ |
| Viewer Requests tab | 不存在 | ❌ |

**状态修正**：`待执行` 保持不变。

### 1.10 MID-001 后端 Key Page Views 语义修正

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| 按关键词匹配关键页 | `heat.IsKeyPage` 已存在 | ✅ 部分 |
| 按 **page title** 匹配 | `getScoreForLink` 只检查 **document title** | ❌ 语义错误 |
| SQL `key_page_views` 修正 | `queries.sql` 仍按 `duration_seconds >= 3` | ❌ 未修正 |
| 保留 `engaged_page_views` | SQL 中已有该别名 | ✅ |

**状态修正**：保持 `部分完成`，但实现范围需扩大：需要 page-level title 源，或改用 chunk 标题/文档元数据逐页匹配。

### 1.11 MID-002 扩展追踪事件体系

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| `forward_signal` / `return_visit` / `scroll_depth_recorded` / AI 事件 | 均不存在 | ❌ 未实现 |
| 前端统一事件 SDK | 不存在 | ❌ |

**状态修正**：`待执行` 保持不变。

### 1.12 MID-003 通知规则引擎

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| `notification_rules` 表 | 不存在 | ❌ |
| 规则引擎 + 合并窗口 | 不存在 | ❌ |
| `hot_signal` 走规则引擎 | `suggestions/service.go:110` 直接 `notifier.Enqueue` | ❌ |

**状态修正**：`待执行` 保持不变。建议先完成 INFRA-002（通知持久化）再启动 MID-003，否则规则引擎无数据可消费。

### 1.13 MID-005 页面与下载签名 URL

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| HMAC 签名 URL | 不存在 | ❌ |
| `URL_SIGNING_SECRET` | 不存在 | ❌ |

**状态修正**：`待执行` 保持不变。

### 1.14 MID-006 可信水印

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| 后端生成 `watermarkText` | 不存在；前端 `WatermarkOverlay` 本地构造文本 | ❌ |
| Print Screen / 右键拦截 | 不存在 | ❌ |

**状态修正**：`待执行` 保持不变。

### 1.15 MID-007 Link 级 Analytics 与生命周期管理

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| Link Analytics Tab | **已存在**，渲染 views/visitors/avg duration/last visit/access logs | ✅ |
| 过期前 24h/7d 提醒 cron | 不存在 | ❌ |
| `/r/:slug` 重定向 | 未找到路由 | ❌ |
| 归档/续期 | `links.status` 无 `archived`；无 archive/renew 端点 | ❌ |
| 归档后 session 失效 | 未实现 | ❌ |

**状态修正**：从 `待执行` 上调为 `部分完成`。任务描述应修正为“补齐生命周期管理 + 扩展 Analytics”，而不是“新建 Analytics Tab”。

### 1.16 MID-008 索引文件自动生成

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| `links.index_file_enabled` 列 | 不存在 | ❌ |
| `link_index_files` 表 | 不存在 | ❌ |
| Index tab | 不存在 | ❌ |

**状态修正**：`待执行` 保持不变。

### 1.17 MID-009 文件收集链接

| 计划要求 | 代码现状 | 结论 |
|---|---|---|
| `permission_type = 'file_request'` | 当前枚举 `('public','email_required','nda')`；无 `file_request` | ❌ |
| `links.target_folder_path` | 不存在 | ❌ |
| `link_uploaded_files` 表 | 不存在 | ❌ |
| 公共 upload 端点 | 不存在 | ❌ |

**状态修正**：`待执行` 保持不变。**强烈建议**用新增 `link_type` 字段替代扩展 `permission_type`。

---

## 2. 新增的基础设施与合规缺口

以下问题跨多个任务存在，但当前计划中没有独立任务负责：

### 2.1 INFRA-001：Schema 统一编排

多个任务都要改 `links` 表和新建表，且迁移编号已冲突。**必须在 Wave 1 新增一个 INFRA 任务**，统一产出：
- `links` 新增列：`qa_enabled`、`file_requests_enabled`、`index_file_enabled`、`target_folder_path`、`security_version`、`link_type`（建议新增）。
- 新表：`link_access_requests`、`link_visitor_questions`、`link_file_requests`、`link_index_files`、`link_uploaded_files`、`notification_rules`。
- 扩展/重构 `permission_type` 约束或新增 `link_type`。

### 2.2 INFRA-002：可靠异步通知

当前 `notification.Enqueue("email")` 同步发送，邮件不写入 `notifications` 表，worker 只处理 Slack。**必须新增任务**：
- 所有邮件统一写入 `notifications` 表。
- worker 消费邮件，支持重试、死信、幂等。
- 移除 `SMTP_USER` 收件人兜底。
- link 服务所有邮件调用迁移到 `notification.Service.Enqueue`。

### 2.3 INFRA-003：事件与 Analytics 数据 retention

`access_logs`、`page_views`、`security_events` 均无 partitioning/TTL。随着 MID-002 扩展事件，数据量会激增。

### 2.4 COMPLIANCE-001：Sharing 链路 PII 最小化

新增表（`link_visitor_questions`、`link_file_requests`、`link_uploaded_files`）都会存储 visitor email/IP/UA。需要统一：
- visitor_id / IP hash 规则。
- 保留期限。
- 导出/删除流程。

---

## 3. 风险矩阵（基于代码分析更新）

| 风险 | 等级 | 关键证据 | 影响任务 |
|---|---|---|---|
| 邀请 token 明文存储 | 🔴 严重 | `042_deal_room_sharing.up.sql:59`；`service.go:1035,1094` | SHORT-005 |
| 通知并非真正异步 + SMTP_USER 兜底 | 🔴 严重 | `notification/service.go:56-100,145` | SHORT-002/007/008/009 |
| 迁移编号 046 冲突 | 🟠 高 | `046_links_document_id_nullable.up.sql` 已存在 | SHORT-005 及后续 |
| `link_access_requests` 完全缺失 | 🟠 高 | 无表/无查询/无路由 | SHORT-005/007 |
| 占位开关误导用户 | 🟠 高 | `AccessTab.tsx` 中 4 个开关无后端字段 | SHORT-006/008/009/MID-008 |
| `permission_type` 维度污染 | 🟠 高 | 当前枚举无 `whitelist/password`，且 `nda` 已存在 | MID-009 |
| Key Page 实现语义错误 | 🟠 高 | `analytics/service.go:257-260` 按 document title 判断 | MID-001 |
| 邮件未检查 `email_enabled` | 🟠 中 | `link/service.go:1440-1443` | SHORT-007 |
| `security_events` 缺 tenant_id | 🟡 中 | `031_security_events.up.sql` | SHORT-003/MID-007 |
| 水印可轻易绕过 | 🟡 中 | `WatermarkOverlay.tsx` 纯 CSS 覆盖 | MID-006 |
| Analytics 数据无 retention | 🟡 中 | 无 TTL/partitioning | MID-002/007 |

---

## 4. 修正后的执行波次建议

按 **2 后端 + 2 前端** 配置，预计 **12 周**完成 active 任务。

| Wave | 任务 | 目标 | 产出物 |
|---|---|---|---|
| **Wave 1（2 周）** | INFRA-001、SHORT-005-A、SHORT-002、SHORT-003 收尾 | Schema 定型 + 邀请 token hash + security_version + 移除 SMTP_USER 兜底 + 安全事件可配置阈值 | 核心安全基础设施 |
| **Wave 2（2 周）** | INFRA-002、SHORT-005-B、SHORT-006、SHORT-007 | 异步通知 worker、请求访问后端+前端、Share 弹窗收尾 | 邮件/请求闭环 |
| **Wave 3（2 周）** | SHORT-008、SHORT-009 | Visitor Q&A、访客文件请求 | Viewer 侧边栏能力 |
| **Wave 4（2 周）** | MID-005、MID-006、MID-002 | 签名 URL、服务端水印、扩展事件体系 | 安全与数据 |
| **Wave 5（2 周）** | MID-007、MID-009、MID-001 | 生命周期管理、文件收集链接、Key Page 修正 | 分析与收集 |
| **Wave 6（2 周）** | MID-008、INFRA-003、COMPLIANCE-001 | Index File、事件 retention、PII 合规 | AI/合规收尾 |
| **Wave 7+** | LONG-001~004 | Heat decay、AI intent、realtime、CRM | 按需启动 |

---

## 5. 立即需要产品/架构决策的开放问题

1. **是否引入 `link_type` 字段替代在 `permission_type` 中加 `file_request`？** 强烈建议引入。
2. **是否接受把 `MID-007` 的 Analytics Tab 改为“Management Tab”**，统一管理 Q&A / File Requests / Uploads / Invitations？
3. **是否把 `MID-008` 降级为 P2 或与 AI Copilot 摘要复用？**
4. **水印 anti-circumvention 的边界**：是否只做到服务端生成文本 + 审计日志，弱化 Print Screen 拦截？
5. **签名 URL 方案**：逐 URL 15 分钟签名，还是 session-scoped signed cookie + 下载短签？
6. **资源投入**：是否能配置 2 后端 + 2 前端 + 1 QA？否则必须继续砍 scope。

---

## 6. 结论

代码级评审证实：任务计划在“需求筛选”层面已经收敛，但在**执行就绪度**上仍存在重大缺口。**SHORT-005、SHORT-007 的实际剩余工作量被低估，通知系统的异步假设不成立，schema 变更缺少统一编排。**

建议立即：
1. 将计划升级到 **v1.4.0**。
2. 新增 **INFRA-001、INFRA-002、INFRA-003、COMPLIANCE-001** 四个跨任务。
3. 拆分 **SHORT-005-A/B**。
4. 修正 **MID-007** 范围，下调/上调相关任务状态。
5. 在中央位置（README）明确新的迁移编号和依赖关系。

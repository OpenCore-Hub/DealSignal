# DealSignal 文档分享功能：产品 + 技术最终评审

**评审日期**：2026-07-08  
**评审角色**：高级产品总监 + 资深架构师  
**评审对象**：
- `/Users/mg/.kimi/plans/huntress-spectre-falcon.md`（Deal Room / 文档链接分享设计 v2.0）
- `docs/tasks/document-sharing-gap-remediation/`（文档分享缺口修复任务计划）
- 当前代码实现：`apps/api/internal/link`、`apps/web/src/components/deal-rooms`、`apps/web/src/components/links`、`apps/web/src/components/viewer`

**评审目标**：
1. 以批判性视角识别产品设计和技术架构层面的深层差异、缺口与新增项。
2. 判断当前方案是否满足 DealSignal 产品长期规划。
3. 输出可执行的任务计划修正建议，并同步更新到 `document-sharing-gap-remediation/`。

---

## 1. 总体结论

**当前状态：方向正确，但“任务计划”与“实际交付”严重脱节，且存在若干影响长期演进的关键缺口。**

- **产品设计**：三入口模型（Share / Invite / Access）符合数据室用户心智，但缺少面向高阶场景的能力（访问请求、链接分析、批量邀请模板、工作区级策略、过期前提醒等）。
- **技术架构**：复用 `links` 表作为分享唯一实体是合理取舍，但访问规则全量替换、会话失效机制、去重顺序、水印防绕过、签名 URL 缺失等问题会在规模扩大后暴露。
- **任务计划**：`document-sharing-gap-remediation/` 没有覆盖 huntress 核心功能（Share/Invite/Access Rules 本身），且 14 个任务状态全部停留在“待执行”，与代码现实不符。必须修正。
- **长期规划**：当前方案仍停留在“功能补齐”阶段，距离“企业级数据室安全分享平台”还差统一审计视图、策略引擎、合规（GDPR/CCPA）、自动化工作流、可观测性等一层。

**最终建议**：
- 短期：完成 huntress 核心功能的收尾 + 修复已发现的技术缺陷。
- 中期：补齐签名 URL、扩展事件体系、通知规则引擎、Key Page 语义修正。
- 长期：从“规则驱动”升级为“策略 + 智能 + 实时”的分享治理平台。

---

## 2. 产品设计评审

### 2.1 肯定

| 设计点 | 评价 |
|---|---|
| **Link 作为分享唯一实体** | 正确。避免 Deal Room 与文档链接各建一套体系，长期维护成本可控。 |
| **Share / Invite / Access 三入口** | 职责清晰，分别解决“分发、识别、授权”，降低用户认知负荷。 |
| **Access Rules fail-closed** | block 优先、allow 必须命中的设计符合高安全数据室要求。 |
| **Preset 安全预设** | public / standard / confidential 预设能把复杂权限打包成一句话，适合小白用户。 |
| **邀请自动加入 allow list** | 产品闭环：邀请即授权，避免用户手动维护两份列表。 |
| **规则变更使旧 session 失效** | 通过 `links.updated_at` 触发器刷新，满足“修改后立即生效”的预期。 |

### 2.2 关键问题与缺口

#### P0：缺少“访问请求（Request Access）”闭环

- **huntress 设计**：公共 Viewer 错误页有 `requestAccess` 按钮，但只停留在 UI 文案。
- **当前代码**：`documents.viewer.requestAccess` i18n 键已存在，后端没有对应的 `link_access_requests` 表或 API。
- **风险**：数据室场景下，被 block / not allowed 的访客往往需要向创建者申请访问；没有闭环，用户会被卡在错误页。
- **建议**：新增 `link_access_requests` 表 + 创建者审批通知 + 一键 approve（自动加入 allow list 并发送邀请邮件）。

#### P0：没有 Link 级分析视图

- **huntress 设计**：Links 表格只有 Views / Last Viewed，点击无法查看明细。
- **风险**：数据室用户（投资人、法务）需要知道“谁看了什么、看了多久、下载与否”。当前能力只够 dashboard 汇总，不足以支撑销售/尽调跟进。
- **建议**：在 `DealRoomShareDialog` 增加第 4 个 Tab “Analytics”，至少展示最近访问者、停留时长、关键页、下载次数、AI 问答记录。

#### P1：批量邀请与模板缺失

- **huntress 设计**：Invite Tab 是 textarea 手动输入邮箱。
- **风险**：LP 报告、投后材料等场景常需一次性邀请 20~50 人；手动输入易错且无复用。
- **建议**：支持从通讯录导入、CSV 上传、保存为邀请列表模板。

#### P1：没有工作区级分享策略

- **huntress 设计**：所有规则挂在单个 Link 上。
- **风险**：企业客户需要统一策略（如“所有 confidential 链接必须 OTP + 密码”）。没有 workspace/tenant 级默认策略，管理员需要逐条配置。
- **建议**：增加 `workspace_sharing_policies` 表，允许设置默认 preset、强制水印、强制过期时间等。

#### P1：缺少过期前提醒与生命周期管理

- **huntress 设计**：`expires_at` 字段存在，但没有提醒机制。
- **风险**：高价值尽调链接过期会导致业务中断，且创建者可能忘记续期。
- **建议**：增加 cron 任务，在链接过期前 24h/7d 发送提醒邮件；支持一键续期/归档。

#### P1：旧 `/r/:slug` 路径未做统一治理

- **huntress 设计**：保留 `/r/:slug` 兼容，但新分享建议使用 `/l/:token`。
- **风险**：两条路径的访问日志、通知、安全策略可能分裂；老客户书签可能绕过新规则。
- **建议**：长期应将 `/r/:slug` 重定向到 `/l/:token`（带 room slug 解析为默认 share link），并在 analytics 中合并归因。

#### P2：NDA 流程过于简化

- **huntress 设计**：NDA 是一个 switch + 勾选框。
- **风险**：真实法律场景需要 NDA 文本版本化、电子签名记录、审计日志。当前 `link_invitations` 没有记录 NDA 同意时间/版本。
- **建议**：增加 `nda_versions` 表，记录用户同意的 NDA 版本 + 时间 + IP。

#### P2：缺少“预览为访客”能力

- **huntress 设计**：Preview 按钮是占位。
- **建议**：提供“以访客身份打开 `/l/:token`”的快捷入口，让创建者验证访问体验。

---

## 3. 技术架构评审

### 3.1 肯定

| 技术点 | 评价 |
|---|---|
| **复用 `links` 表承载 Deal Room 分享** | 避免数据模型分裂，迁移 042 的 check 约束保证了 document/deal_room 互斥。 |
| **访问规则与邀请作为 Link 子资源** | 符合 RESTful 与领域驱动设计，便于权限控制。 |
| **FailoverDedupChecker（Redis + DB）** | 去重架构正确，Redis 原子操作解决并发问题，DB 兜底保证可用性。 |
| **公共 AI 端点隔离** | `internal/assistant/public_handler.go` 独立处理匿名请求，限制文档范围，避免泄露工作区内容。 |
| **安全事件追加写入** | `security_events` 不可更新删除，符合审计要求。 |
| **密码 bcrypt + 常量时间比较** | 基本安全要求已满足。 |

### 3.2 关键问题与风险

#### 🔴 P0：去重标记顺序错误，可能导致事件丢失

- **当前代码**：`analytics/service.go` 中 `RecordLinkOpened` / `RecordPageView` 先调用 `dedup.MarkOpen/MarkPageView`，再写入 DB。
- **风险**：如果 Redis 标记成功但 DB 写入失败（如连接断开、max_access 达到），Redis key 仍会阻止后续真实访问被记录，导致指标永久偏低。
- **正确做法**：先写 DB，DB 成功后再标记 Redis；或在 Redis 标记时同时生成幂等 ID，DB 写入使用同一 ID 去重。
- **影响**：这是数据质量缺陷，必须在进入生产前修复。

#### 🔴 P0：邀请 token 以明文存储

- **当前代码**：`link_invitations.token` 是 `TEXT NOT NULL UNIQUE`，未 hash。
- **风险**：DB 泄露即可伪造邀请链接；运维人员可直接读取 token。
- **正确做法**：
  - 生成随机 token，返回给客户端的是原始值（仅一次）。
  - DB 存储 `token_hash`（SHA-256 或 bcrypt）。
  - 解析时按 hash 查询。
- **迁移成本**：需要新增 `token_hash` 列，历史 token 可重新生成或一次性 hash 迁移。

#### 🔴 P0：水印可被前端绕过

- **当前代码**：`WatermarkOverlay.tsx` 用 HTML/CSS 在 Canvas 上方叠加半透明文字。
- **风险**：用户可通过浏览器 DevTools 删除 DOM、截图时隐藏水印、下载原始 PDF 无水印。
- **正确做法**：
  - **短期**：后端生成 `watermarkText`（邮箱 + 时间 + IP 哈希），前端渲染；同时禁用右键保存、屏蔽 Print Screen（CSS blur + 警告）。
  - **中期**：对页面图片做服务端动态水印（签名 URL 时叠加）。
  - **长期**：PDF 下载走服务端渲染带水印版本。

#### 🟡 P1：签名 URL 缺失

- **当前代码**：`internal/link/signature.go` 不存在，页面图片与下载 URL 无 HMAC 签名。
- **风险**：URL 可被转发、盗链、长期滥用；MinIO 预签名 URL 有效期不可控。
- **建议**：按 gap remediation MID-005 实现，密钥独立于 JWT secret，URL 含 `expires` + `sig`。

#### 🟡 P1：Access Rules 全量替换丢失历史

- **当前代码**：`UpdateAccessRules` 删除旧规则后全部插入。
- **风险**：无法审计“谁在什么时间把谁加入了 allow list”；无法做撤销/恢复。
- **建议**：
  - 短期：保留全量替换，但把旧规则快照写入 `link_access_rule_revisions` 表。
  - 长期：改为行级 CRUD + `updated_by`/`updated_at`，支持审计视图。

#### 🟡 P1：会话失效机制依赖 `updated_at`

- **当前代码**：`LinkSession` 验证时比较 `LinkUpdatedAt` 与 link 的 `updated_at`。
- **风险**：
  - 任何 link 字段更新（包括非安全字段）都会踢掉所有访客。
  - 没有显式 session 黑名单，无法单独 revoke 某个访客。
- **建议**：
  - 引入 `links.security_version` 字段，仅规则/密码/过期变更时递增。
  - 增加 `link_session_revocations` 表，支持按 visitor/session 撤销。

#### 🟡 P1：通知服务同步发送邮件

- **当前代码**：`notification.Service.Enqueue` 对 `channel == "email"` 直接调用 `sendEmail`。
- **风险**：邮件服务商延迟或失败会阻塞 API；无重试/死信。
- **建议**：所有邮件统一入队（`notifications` 表 + worker），与 Slack 一致。

#### 🟡 P1：Key Page Views 实现粗糙

- **当前代码**：`analytics.getScoreForLink` 用 `pageViews.DocumentTitle`（单一聚合标题）判断整个 link 的关键页。
- **风险**：一个 link 可能包含多份文档，仅用文档标题判断会漏掉/误杀大量关键页。
- **建议**：按每页 title 判断，或从 `pages` 表取每页元数据；`GetLinkPageViewMetrics` 应返回 `key_page_views` 与 `engaged_page_views` 两个指标。

#### 🟡 P1：缺少统一的公共事件 SDK

- **当前代码**：事件由 `link/handler.go` 接收，`useViewerDocument.ts` 直接调用 API。
- **风险**：新增事件类型时前后端埋点散落，易遗漏。
- **建议**：按 MID-002 设计统一前端 `lib/analytics.ts` SDK，支持防抖、批量上报、失败重试。

#### 🟢 P2：测试策略需要加强

- **当前代码**：单元/集成测试已覆盖主要路径，E2E 已通。
- **缺口**：
  - 缺少去重“Redis 成功但 DB 失败”的容错测试。
  - 缺少邀请 token hash 的安全测试。
  - 缺少水印、签名 URL、访问请求等 E2E。

---

## 4. 与 DealSignal 长期规划的对齐

DealSignal 的长期愿景是成为“数据室安全分享与买家意图洞察平台”。当前方案需要再补三层：

| 层次 | 当前状态 | 长期目标 | 缺口 |
|---|---|---|---|
| **安全层** | 密码、OTP、allow/block、水印（前端）、邀请 token | 零信任访问、签名 URL、服务端水印、设备指纹、异常行为自动封禁 | 签名 URL、服务端水印、token hash、访问请求 |
| **策略层** | 单 Link 规则 | Workspace / Tenant 级策略、合规策略模板、审批流 | 工作区策略、NDA 版本化、访问请求审批 |
| **洞察层** | Heat Score、基础事件 | 实时信号、AI 意图、预测评分、CRM 深度集成 | 扩展事件、规则引擎、AI intent、realtime、CRM timeline |
| **治理层** | 无 | 审计日志视图、数据保留策略、GDPR 删除、数据脱敏 | 审计视图、生命周期管理、合规 |

**结论**：当前 huntress + gap remediation 组合能支撑 MVP，但要在 6~12 个月后进入企业市场，必须在中期任务中补全安全层与策略层，在长期任务中补全洞察层与治理层。

---

## 5. 必须立即修正的项（P0）

1. **修正去重顺序**：`RecordLinkOpened` / `RecordPageView` 应先写 DB，再标记 Redis；或引入幂等 ID 机制。
2. **邀请 token 存储 hash**：新增 `token_hash` 列，不再明文存储 token。
3. **水印内容后端化**：后端 `Access` 响应返回 `watermarkText`，至少包含邮箱、访问时间、IP 哈希。
4. **移除 SMTP_USER 通知兜底**：`notification/service.go` 不得以 `SMTP_USER` 作为收件人；无邮箱时跳过并记录 warn。
5. **任务计划状态对齐**：把 14 个任务文件的 `status` 更新为与代码一致的状态。

---

## 6. 建议新增/调整的任务

### 6.1 新增任务（补 huntress 核心功能缺口）

| 任务 ID | 标题 | 阶段 | 优先级 | 说明 |
|---|---|---|---|---|
| TASK-SHARE-SHORT-005 | Deal Room / 文档链接分享后端核心 | 短期 | P0 | migration 042、link service 方法、Access() 改造、workspace API、邀请 token hash |
| TASK-SHARE-SHORT-006 | 前端 Share / Invite / Access 三 Tab 弹窗 | 短期 | P0 | `DealRoomShareDialog`、`LinkShareDialog`、i18n、动画、Preset Custom 状态 |
| TASK-SHARE-SHORT-007 | 邀请邮件、访问通知与请求访问闭环 | 短期 | P1 | 邀请邮件模板、`link_access_requests` 表、创建者审批通知 |
| TASK-SHARE-MID-006 | 服务端水印与防绕过 | 中期 | P1 | 后端 `watermarkText`、IP 哈希、打印/保存防护 |
| TASK-SHARE-MID-007 | Link 级 Analytics 与生命周期管理 | 中期 | P1 | Analytics Tab、过期前提醒、续期/归档、旧 `/r/:slug` 重定向 |

### 6.2 现有任务修正

| 任务 | 修正内容 |
|---|---|
| SHORT-002 | 明确验收标准：彻底移除 `SMTP_USER` 兜底；无邮箱时记录 warn 不 panic。 |
| SHORT-003 | 说明异常访问告警由 MID-003 实现；本任务仅负责事件记录。 |
| SHORT-004 | 修正实现顺序：DB 写入成功后再标记 Redis；补充 DB 失败场景测试。 |
| MID-001 | 明确按页匹配关键页；修正 SQL 中 `key_page_views` 仍用 `duration>=3` 的问题。 |
| MID-002 | 增加公共/认证 viewer 统一事件 SDK；与 SHORT-004 去重兼容。 |
| MID-003 | 安全相关通知不可退订；与 SHORT-003 安全事件共享异常检测逻辑。 |
| MID-004 | 增加后端返回 `watermarkText`；与新增 MID-006 合并或分工明确。 |
| MID-005 | 签名密钥独立；验证失败不得回退到无签名访问。 |

### 6.3 长期任务补充

| 任务 | 补充内容 |
|---|---|
| LONG-003 实时推送 | 明确先用 SSE MVP，多实例用 Redis Pub/Sub；先推送信号/通知，再推 dashboard 指标。 |
| LONG-004 CRM 深度集成 | 优先 HubSpot；明确 timeline activity 字段映射与失败重试/死信机制。 |
| LONG-005 预测评分 | 明确转化标签定义（如回复邮件、进入谈判、签约）和最小样本量。 |

---

## 7. 更新后的任务计划总览

> 详见同步更新后的 `README.md` 与各 `TASK-SHARE-*.md` 文件。

| 阶段 | 任务 | 状态 | 优先级 | 类型 |
|---|---|---|---|---|
| 短期 | TASK-SHARE-SHORT-005 Deal Room / 文档链接分享后端核心 | 已完成 | P0 | backend |
| 短期 | TASK-SHARE-SHORT-006 前端三 Tab 弹窗 | 部分完成 | P0 | frontend |
| 短期 | TASK-SHARE-SHORT-007 邀请邮件与请求访问闭环 | 部分完成 | P1 | fullstack |
| 短期 | TASK-SHARE-SHORT-001 公共 Viewer AI Copilot | 已完成 | P0 | fullstack |
| 短期 | TASK-SHARE-SHORT-002 通知收件人与邮件开关 | 部分完成 | P0 | backend |
| 短期 | TASK-SHARE-SHORT-003 安全审计事件记录 | 已完成 | P0 | backend |
| 短期 | TASK-SHARE-SHORT-004 访问与页面浏览基础去重 | 已完成 | P1 | backend |
| 中期 | TASK-SHARE-MID-001 后端 Key Page Views 语义修正 | 部分完成 | P1 | backend |
| 中期 | TASK-SHARE-MID-002 扩展追踪事件体系 | 待执行 | P1 | fullstack |
| 中期 | TASK-SHARE-MID-003 通知规则引擎与事件合并 | 待执行 | P1 | backend |
| 中期 | TASK-SHARE-MID-004 公共 Viewer 动态水印 | 部分完成 | P1 | frontend |
| 中期 | TASK-SHARE-MID-005 页面与下载签名 URL | 待执行 | P1 | backend |
| 中期 | TASK-SHARE-MID-006 服务端水印与防绕过 | 待执行 | P1 | fullstack |
| 中期 | TASK-SHARE-MID-007 Link 级 Analytics 与生命周期管理 | 待执行 | P1 | fullstack |
| 长期 | TASK-SHARE-LONG-001 ~ LONG-005 | 待执行 | P2/P3 | backend/ai/infra |

---

## 8. 执行优先级

```text
Wave 1（本周）：修正 P0 技术缺陷
├── 修正去重顺序（SHORT-004 范围）
├── 邀请 token 存储 hash（SHORT-005 范围）
├── 水印内容后端化（MID-006 范围）
└── 移除 SMTP_USER 兜底（SHORT-002 范围）

Wave 2（2 周内）：完成 huntress 前端收尾
├── Preset Custom 状态与二次确认
├── Access / Share Tab 职责彻底分离
├── Invite Tab resend / revoke 二次确认
└── 公共 Viewer 错误状态与 i18n 补全

Wave 3（1 个月内）：补齐中期增强
├── MID-005 签名 URL
├── MID-002 扩展事件体系
├── MID-003 通知规则引擎
└── MID-007 Link Analytics + 生命周期

Wave 4（2~3 个月）：长期演进
├── LONG-001 Heat Score 衰减
├── LONG-003 实时推送（SSE MVP）
├── LONG-004 CRM 深度集成
└── LONG-002 / LONG-005 AI 意图与预测评分
```

---

## 9. 风险监控清单

| 风险 | 等级 | 监控指标 |
|---|---|---|
| 去重顺序错误导致指标丢失 | 🔴 高 | `RecordLinkOpened` DB 失败率、Redis key 与 DB 事件数差异 |
| 邀请 token 明文泄露 | 🔴 高 | 安全审计中是否出现 token 明文日志、DB 访问权限 |
| 任务状态与代码脱节 | 🟡 中 | 任务文件 status 与代码实现的匹配度 |
| 前端水印可绕过 | 🟡 中 | 用户反馈截图无水印、下载原始文件 |
| 签名 URL 缺失导致盗链 | 🟡 中 | 异常来源访问日志 |
| 实时推送/AI/CRM 长期任务范围膨胀 | 🟢 低 | 每个长期任务启动前必须重审 ROI |

---

## 10. 评审 Checklist

- [x] 已完成 huntress 设计与 gap remediation 任务的细胞级对比
- [x] 已识别产品层面的关键缺口（访问请求、Link Analytics、批量邀请、工作区策略、生命周期）
- [x] 已识别技术架构层面的关键风险（去重顺序、token 明文、水印绕过、签名 URL、会话失效、Key Page 粗糙）
- [x] 已给出与 DealSignal 长期规划的对齐判断
- [x] 已输出可执行的任务计划修正建议
- [x] 已同步更新 `document-sharing-gap-remediation/` 下的任务文件

---

**评审人签名**：Kimi Code CLI（高级产品总监 + 资深架构师视角）  
**结论**：方案可继续推进，但必须在 Wave 1 内修正 4 个 P0 技术缺陷，并立即更新任务计划以反映真实实现状态。

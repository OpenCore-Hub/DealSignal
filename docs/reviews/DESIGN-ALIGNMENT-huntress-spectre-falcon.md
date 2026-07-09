# DealSignal 文档分享：设计文档对齐报告

**对齐日期**：2026-07-08  
**对比对象**：
- `/Users/mg/.kimi/plans/huntress-spectre-falcon.md`（Deal Room / 文档链接分享功能设计，v2.0）
- `docs/tasks/document-sharing-gap-remediation/`（文档分享缺口修复任务计划，14 个 TASK-SHARE-*）
- 当前代码库实际实现（`apps/api/internal/link`、`apps/web/src/components/deal-rooms`、`apps/web/src/components/links`、`apps/web/src/components/viewer`）

**对齐目标**：
1. 明确两份设计文档的粒度、范围与依赖关系。
2. 把 huntress 设计逐条映射到 gap remediation 任务，找出覆盖真空。
3. 澄清已实现代码与设计之间的细胞级差异。
4. 给出任务计划调整建议，使 backlog 与真实实现/设计保持一致。

---

## 1. 定位与范围差异

| 维度 | `huntress-spectre-falcon.md` | `document-sharing-gap-remediation/` |
|---|---|---|
| **文档类型** | 单一功能详细设计（已评审终版） | 领域级工程任务拆分（AGENT-TASK） |
| **核心问题** | Deal Room / 文档链接如何对外分享、识别、授权 | 文档分享链路当前实现 vs 设计初衷的缺口 |
| **主要产出** | 数据模型、API、前端三 Tab 弹窗、访问流程、安全审计、i18n | 14 个独立任务，覆盖 AI、通知、安全、去重、Key Page、水印、签名、CRM、实时化、预测评分 |
| **时间跨度** | 一个功能从 0 到 1（≈ Phase 1~3） | 短/中/长期路线图（1~12 周） |
| **关键缺口** | 未涉及去重、Key Page 语义、扩展事件、CRM、实时化 | **未把 huntress 的核心功能拆成独立任务**（Share/Invite/Access Rules、Deal Room sharing、密码、公共 Viewer） |

**一句话结论**：
- **huntress 是“做什么”的终版蓝图**；
- **gap remediation 是“补什么”的任务 backlog**，但它把 huntress 蓝图中的大量已实现/待实现功能当作“已存在”或“不需要任务化”，导致任务计划与实际工作严重脱节。

---

## 2. 覆盖度矩阵（细胞级映射）

### 2.1 huntress 核心功能 vs gap remediation 任务

| huntress 章节 | 设计内容 | gap remediation 对应任务 | 代码实际状态 | 覆盖结论 |
|---|---|---|---|---|
| §5.1 Migration `042_deal_room_sharing` | `links.deal_room_id`、`require_password`、`password_hash`、`link_access_rules`、`link_invitations` | **无对应任务** | ✅ 已实现（migration 042） | ❌ 任务缺失 |
| §6.1 领域对象 | `AccessRule`、`LinkInvitation`、`AccessEvaluation` | **无对应任务** | ✅ 已实现（`internal/link/service.go`） | ❌ 任务缺失 |
| §6.2 访问流程 / §6.4 `Access()` 改造 | 规则评估、invite token 解析、密码校验、OTP/NDA 门控 | **无对应任务** | ✅ 已实现并 E2E 通过 | ❌ 任务缺失 |
| §6.3 服务方法 | `CreateDealRoomLink`、`InviteViewers`、`ResolveInviteToken`、`UpdateAccessRules`、`verifyPassword` | **无对应任务** | ✅ 已实现 | ❌ 任务缺失 |
| §6.5 规范化依赖 | allow rules 必须 `require_email`；高安全强制 OTP | **无对应任务** | ✅ 已校验 | ❌ 任务缺失 |
| §6.6 密码处理 | bcrypt hash、更新保留旧 hash | **无对应任务** | ✅ 已实现 | ❌ 任务缺失 |
| §6.7 邮件发送 | 邀请邮件、访问通知邮件 | 间接：SHORT-002 | ⚠️ 邀请邮件已发，但邮件类型与模板变量未完全对齐设计 | ⚠️ 部分覆盖 |
| §7 API 设计 | `/deal-rooms/:id/links`、`/links/:id/access-rules`、invitations、public access | **无对应任务** | ✅ 路由已实现 | ❌ 任务缺失 |
| §8 前端设计 | `DealRoomShareDialog` / `LinkShareDialog` 三 Tab、`PublicViewerPage` 增强 | **无对应任务** | ⚠️ 三 Tab 已建，但缺少部分设计细节（preset Custom 状态、Analytics Tab、Invite resend、Link 禁用确认等） | ⚠️ 部分覆盖 |
| §8.5 i18n 键名 | `dealRooms.share.*`、`dealRooms.invite.*`、`dealRooms.accessRules.*`、`documents.viewer.*` | **无对应任务** | ⚠️ 基础键已补，部分设计键未完全对齐 | ⚠️ 部分覆盖 |
| §8.7 终版布局 v2.0 | Share/Invite/Access 职责分离、Footer 统一保存、Preset Custom 状态 | **无对应任务** | ⚠️ 已实现大部分，但 preset 自定义状态、Analytics Tab、旧 slug 提示等未做 | ⚠️ 部分覆盖 |
| §8.8 动画规范 | Dialog/Tab/Switch/Chip/Copy/Save 动画 | **无对应任务** | ⚠️ 已加部分动画（Dialog scale、Switch spring、EmailTagInput chip 动画、Save success 反馈） | ⚠️ 部分覆盖 |
| §9 安全审计 | `security_events` 扩展事件类型 | SHORT-003 | ✅ 已实现（migration 045 完整事件类型） | ✅ 完整覆盖 |
| §9.2 防爆破 | IP+token 限流 | 无单独任务 | ✅ 已有中间件限流 | ✅ 已覆盖 |
| §9.3 会话 | `LinkSession` 15min 滑动、`PasswordVerified`、rules/password 变更使 session 失效 | 无单独任务 | ✅ 已实现 | ✅ 已覆盖 |
| §10 Phase 1~3 | 实现阶段拆分 | 与 14 个任务不直接对应 | — | ❌ 需要重新对应 |

### 2.2 gap remediation 独有任务（huntress 未覆盖）

| gap remediation 任务 | 内容 | huntress 覆盖情况 | 代码实际状态 |
|---|---|---|---|
| SHORT-001 公共 Viewer AI Copilot | 公共 AI 端点、按 flag 渲染、会话隔离 | §8.3/§8.4 提及 AI Copilot 开关 | ✅ 已实现 |
| SHORT-002 通知收件人与邮件开关 | 收件人改为 `link.created_by`、前端 `email_enabled` | §6.7 提及访问通知 | ⚠️ 收件人逻辑已改，但 `SMTP_USER` 兜底未移除 |
| SHORT-004 访问与页面浏览基础去重 | Redis + DB 双层去重 | 未提及 | ✅ 已实现 |
| MID-001 后端 Key Page Views 语义修正 | 关键词匹配关键页 | 未提及 | ⚠️ `IsKeyPage` 已用，但 SQL 聚合仍用 `duration>=3` |
| MID-002 扩展追踪事件体系 | forward/return/scroll/ai 事件 | 未提及 | ❌ 未实现 |
| MID-003 通知规则引擎与事件合并 | 规则引擎、10min 合并 | 未提及 | ❌ 未实现 |
| MID-004 公共 Viewer 动态水印 | Canvas 动态水印 | §8.7 Access Tab 提及 watermark | ⚠️ 组件存在，但后端未返回 `watermarkText`，IP 哈希未加入 |
| MID-005 页面与下载签名 URL | HMAC 签名 URL | 未提及 | ❌ 未实现 |
| LONG-001 ~ LONG-005 | Heat decay、AI intent、realtime、CRM deep、predictive scoring | 未提及 | ❌ 未实现 |

---

## 3. 已实现代码 vs 任务状态不一致

当前 14 个任务文件的 YAML frontmatter 全部标记为 `status: "待执行"`，但代码实际状态如下：

| 任务 | 文件状态 | 实际代码状态 | 建议更新状态 |
|---|---|---|---|
| SHORT-001 | 待执行 | 已完成 | `已完成` |
| SHORT-002 | 待执行 | 部分完成（SMTP_USER 兜底仍在） | `部分完成` |
| SHORT-003 | 待执行 | 已完成 | `已完成` |
| SHORT-004 | 待执行 | 已完成 | `已完成` |
| MID-001 | 待执行 | 部分完成 | `部分完成` |
| MID-002 ~ MID-005 | 待执行 | 未开始 | `待执行` |
| LONG-001 ~ LONG-005 | 待执行 | 未开始 | `待执行` |

**风险**：任务状态与代码不一致会导致后续 Agent 重复工作、估算失真、依赖判断错误。

---

## 4. 需要澄清的细胞级差异

### 4.1 `AccessRule` 领域对象是否带 `ID`

- **huntress 设计**：`AccessRule` 带 `ID`。
- **当前代码**：`internal/link/service.go` 中的 `AccessRule` 只有 `RuleType`、`Value`、`Action`。
- **DB 层**：`link_access_rules` 表有 `id UUID PRIMARY KEY`。
- **原因**：`UpdateAccessRules` 采用“全量删除 + 批量插入”策略，服务层不需要 ID。
- **澄清结论**：服务层可继续无 ID，但设计文档应注明“DB 保留 ID，服务层通过全量替换实现幂等更新”。若未来需要行级编辑/审计，再引入 ID。

### 4.2 `LinkInvitation` 状态机

- **huntress 设计**：`pending → opened → verified → expired / revoked`。
- **当前代码**：`Access()` 中：
  - 首次访问：`pending → opened`
  - 后续访问：`opened → verified`
- **DB 约束**：`status IN ('pending','opened','verified','expired','revoked')` 与设计一致。
- **澄清结论**：状态机已对齐。设计文档可增加说明“verified 表示已完整通过所有门控（密码/OTP/NDA）”。

### 4.3 邀请是否自动加入 allow list

- **huntress 设计**：发送邀请时自动将邮箱加入 allow list。
- **当前代码**：`InviteViewers` 会同时创建 `link_access_rules` `action=allow` `rule_type=email`。
- **澄清结论**：已对齐。需补充设计文档说明“revoke 时同步移除 allow rule”——当前代码已实现。

### 4.4 Access rules 评估优先级

- **huntress 设计**：
  1. block 优先于 allow
  2. email 优先于 domain
- **当前代码**：`evaluateAccessRules` 先扫描所有 block，再扫描 allow；同 action 内 email 与 domain 未显式区分优先级，但通常同时存在时 block 优先已足够。
- **澄清结论**：建议设计文档明确“同 action 内 email > domain”，代码中补充测试覆盖。

### 4.5 `require_email` 强制开启规则

- **huntress 设计**：存在 allow 规则时，`require_email` 必须为 true。
- **当前代码**：`UpdateAccessRules` 和 `CreateLink`/`UpdateLink` 已校验；`InviteViewers` 也要求 `link.RequireEmail`。
- **澄清结论**：已对齐。

### 4.6 水印内容来源

- **huntress 设计**：后端 `Access` 响应返回 `watermarkText`（邮箱 + 访问时间 + IP 哈希）。
- **当前代码**：后端只返回 `watermarkEnabled: bool`；前端自行用 `visitorId` 或邮箱 + 本地时间拼接。
- **澄清结论**：**未对齐**。存在两个方案：
  - **方案 A（推荐）**：后端生成 `watermarkText` 并返回，前端只负责渲染；便于统一算法、未来加入 IP 哈希。
  - **方案 B**：保持前端生成，后端仅控制开关；简单但无法加入 IP 等后端信息。

### 4.7 签名 URL

- **huntress 设计**：未明确涉及签名 URL。
- **gap remediation MID-005**：要求 HMAC 签名页面图片与下载 URL。
- **澄清结论**：这是 gap remediation 比 huntress 更严格的安全要求，应作为独立任务保留。

### 4.8 公共 AI 端点

- **huntress 设计**：§8.3 提到 AI Copilot 开关，但未深入公共 AI 端点。
- **gap remediation SHORT-001**：完整定义公共 AI 端点、session 隔离、按 link+visitor 限制文档范围。
- **当前代码**：`internal/assistant/public_handler.go` + `Service.PublicChat` 已实现。
- **澄清结论**：gap remediation 在此处比 huntress 更详细，已落地。

### 4.9 通知收件人

- **huntress 设计**：访问通知入队给创建者/团队（§6.7）。
- **gap remediation SHORT-002**：收件人从 `SMTP_USER` 改为 `users.email`，前端加 `email_enabled` 开关。
- **当前代码**：已按 `link.CreatedBy` 查 `users.email`，但 `sendEmail` 仍以 `s.cfg.SMTPUser` 作为兜底收件人。
- **澄清结论**：**未完全对齐**。应移除 `SMTP_USER` 兜底，避免 huntress 设计中的“通知给创建者”被环境变量覆盖。

### 4.10 Preset 与 Custom 状态

- **huntress 设计**：Share Tab 提供 public/standard/confidential/custom 预设；一旦用户手动偏离预设，自动进入 custom；再次选择预设需二次确认。
- **当前代码**：前端 `AccessTab` 有 Security presets，但 custom 状态与二次确认逻辑不完整。
- **澄清结论**：**未完全对齐**。是前端 polish 阶段剩余工作。

---

## 5. 缺口与建议调整

### 5.1 必须新增的任务（来自 huntress 核心功能）

| 建议新增任务 ID | 标题 | 阶段 | 优先级 | 说明 |
|---|---|---|---|---|
| TASK-SHARE-SHORT-005 | Deal Room / 文档链接分享核心功能 | 短期 | P0 | 包含 migration 042、link service 方法、Access() 改造、workspace API、公共 viewer 门控 |
| TASK-SHARE-SHORT-006 | 邀请与访问规则前端三 Tab 弹窗 | 短期 | P0 | `DealRoomShareDialog`、`LinkShareDialog`、Share/Invite/Access Tab、i18n、动画 |
| TASK-SHARE-SHORT-007 | 邀请邮件与访问通知 | 短期 | P1 | 邮件模板、邀请链接、`EmailTypeLinkInvite`/`EmailTypeDealRoomInvite` |
| TASK-SHARE-MID-006 | 公共 Viewer 增强与水印文案后端化 | 中期 | P1 | 后端返回 `watermarkText`、IP 哈希、Preview 模式 |
| TASK-SHARE-MID-007 | Preset / Custom 状态与 Analytics Tab | 中期 | P2 | 预设二次确认、Link 级 Analytics 入口 |

> 注：如果认为 SHORT-005/006/007 的工作已经由 huntress 实现完成，则可以把它们标记为 `已完成`，并用于更新 gap remediation 的覆盖率；否则应把它们加入 backlog 以便收尾。

### 5.2 需要修改的现有任务

| 任务 | 建议修改 |
|---|---|
| SHORT-002 | 明确验收标准“彻底移除 `SMTP_USER` 作为收件人，无兜底”。 |
| SHORT-003 | 补充说明：异常模式告警（`abnormal_access_pattern`）可放到 MID-003 实现，本任务只负责事件记录。 |
| MID-001 | 明确验收标准“修正 SQL 聚合查询中的 `key_page_views` 定义，不再使用 `duration_seconds >= 3`”。 |
| MID-004 | 增加验收项“后端返回 `watermarkText`，前端不自行构造”。 |

### 5.3 建议废弃或合并的假设

- **huntress §8.6 `CreateLinkSheet` 过渡期策略**：当前代码已经直接实现了 `DealRoomShareDialog` 三 Tab，`CreateLinkSheet` 仍可保留为快捷新建入口，但不应再作为“必须改造”的 P0 阻塞项。
- **gap remediation 中“未覆盖次要缺口”**：自动生成 follow-up 草稿、NDA 版本化、过期链接清理等，可在对齐后决定是否补充任务。

---

## 6. 执行顺序建议（调整后）

```text
Step 0: 更新任务文件状态，使其与代码一致
        ├── SHORT-001 / SHORT-003 / SHORT-004 → 已完成
        ├── SHORT-002 / MID-001 / MID-004 → 部分完成
        └── 其余 → 待执行

Step 1: 收尾短期剩余缺口
        ├── SHORT-002：移除 SMTP_USER 兜底
        ├── SHORT-005/006/007：若尚未完全收尾，补齐 huntress 核心功能
        └── 补齐公共 viewer 错误状态、i18n、动画

Step 2: 进入中期增强
        ├── MID-001：修正 key_page_views SQL 与按页匹配
        ├── MID-004：后端返回 watermarkText
        ├── MID-005：签名 URL
        ├── MID-002：扩展事件体系
        └── MID-003：通知规则引擎

Step 3: 长期演进（保持规划）
        └── LONG-001 ~ LONG-005 按需启动
```

---

## 7. 关键待确认问题

在继续编码前，建议产品/架构师确认以下 3 项：

1. **水印内容方案**：前端生成还是后端返回 `watermarkText`？是否加入 IP 哈希？
2. **SMTP_USER 兜底**：是否彻底移除？若用户邮箱为空/未验证，是跳过邮件还是记录 warn？
3. **Preset Custom 状态**：是否需要完整实现“手动偏离 preset → custom → 二次确认覆盖”的闭环？

---

## 8. 结论

- **huntress-spectre-falcon.md 与 gap remediation 计划不是替代关系，而是“详细设计”与“任务 backlog”的关系**。
- **最大缺口**：gap remediation 没有把 huntress 的核心分享功能任务化，导致任务计划看起来“短期只剩 AI/通知/安全/去重”，却漏掉了 Share/Invite/Access Rules 本身。
- **最大风险**：任务文件状态全部“待执行”，与已实现的代码严重脱节；不更新会导致后续 Agent/开发者误判进度。
- **推荐行动**：
  1. 立即更新 14 个任务文件的状态字段，使其反映代码现实。
  2. 新增/补充 SHORT-005 ~ SHORT-007 与 MID-006 ~ MID-007，或在 README 中明确说明这些工作已由 huntress 实现完成。
  3. 澄清并收敛 4.6（水印）、4.9（SMTP_USER 兜底）、4.10（Preset Custom）三处差异。

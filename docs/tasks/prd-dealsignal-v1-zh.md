---
workflow_contract_version: 1
feature_slug: dealsignal-v1
target_surface: web
product_depth: deep
recommended_next_step: to-issues
p0_stories: [US-001, US-002, US-003, US-004, US-005, US-006, US-007, US-008]
issue_mapping_count: 46
hard_constraints_count: 5
known_unknowns_count: 6
acceptance_scripts_count: 5
generated_at: 2026-06-17
---

# DealSignal v1 — 统一产品需求文档

> 由 `PRD.md` 与 `PRD + 产品设计的完整文档草案.md` 合并而来。版本 1。旧文件保持原样不动。

## 0. 流程就绪卡（Flow Readiness Card）

- **产品：** DealSignal —— 面向融资创始人、投资机构和 B2B 销售团队的安全文档分享、数据室与意图分析平台。
- **核心用户闭环：** 上传文档 → 创建受控的智能链接 → 接收方在品牌化阅读器中打开 → 发送方看到页面级互动与热度评分 → 发送方在合适时机跟进。
- **目标界面：** Web 优先（桌面管理后台 + 移动端阅读器 + 移动端轻量管理）。v1 不做原生 App。
- **P0 成果：** 创始人可以上传一份 pitch deck，创建投资人专属智能链接，看到谁打开了文档、哪些页面被反复阅读，并在几分钟内收到高意图提醒。
- **硬性约束：** 工作空间级别的租户隔离；在内容加载前强制执行访问控制；分析事件只追加不可改；接收方仅在发送方策略要求时才创建账户；链接撤回应立即生效。
- **推荐默认：** PostgreSQL 15+；对象存储存放文件；默认访问模式为邮箱验证；以创始人作为首个切入点；创始人计划包含 10 个活跃链接。
- **创意空间：** 具体 UI 文案、空状态、动画细节、品牌默认色、通知频率，以及评分算法权重（在文档规定的区间内）。
- **v1 不要做：** 原生移动 App、完整电子签工作流、法律级 DRM 防截屏、企业 DLP、数据驻留、v1 的 AI 生成内容改写。
- **最锐利的产品决策：** 在增加控制力之前，先减少不确定性 —— 首个切入点是创始人，他们需要知道哪些投资人真正感兴趣，而不是需要审计级 VDR 的基金。
- **P0 验收脚本：** 创始人上传一份 12 页 PDF，创建带有邮箱验证 + 水印的智能链接，从另一邮箱打开，翻到第 8 页停留 45 秒，60 秒内在仪表板看到事件与意图评分更新。
- **最佳下一步：** `/to-issues`，因为 P0 闭环只发生在单一 Web 界面，且已有 issue 映射可直接使用。

## 1. 产品决策核心

### 1.1 定位

DealSignal 让创始人、投资人和销售团队通过受控、可追踪的链接发送敏感商业文档，并将接收方行为转化为可执行的成交信号 —— 既不像传统安全文档工具那样笨重，也不像普通文件分享那样跟进全凭猜测。

中文定位：

> 把每一份关键文档变成可控、可追踪、可推进成交的交易信号系统。

### 1.2 差异化与转换触发

| 现有替代方案 / 临时做法 | 结构性失败原因 | 产品差异 | 转换触发点 | 事实状态 |
|---|---|---|---|---|
| DocSend / 传统安全文档 | 阅读器强制注册或重重门槛，在投资人/客户工作流中造成接收方摩擦；分析只停留在“谁打开了”，不会告诉你下一步该做什么。 | 低摩擦阅读器，配合细分场景意图评分与推荐下一步动作。 | 创始人把 deck 发给 20 位投资人，只有打开数据却没有信号，错过那位把财务页读了 3 遍的投资人。 | 未验证 —— 基于产品定位，非审计后的定价数据。 |
| Google Drive / Dropbox / 邮件附件 | 没有页面级分析，发送后无法撤回，没有意图信号，版本控制靠手动。 | 受控链接支持过期、撤回、版本更新和单页互动追踪。 | 发送方转发了一份提案，事后发现买方内部传阅的是过期版本。 | 已验证 —— 通用云盘按设计不提供互动分析。 |
| 传统 VDR（Intralinks、Merrill） | 采购重、上线慢，为并购设计，不适合轻量融资或销售提案。 | 轻量数据室，带模板，分钟级搭建。 | 新兴基金或销售团队今天就需要一个房间，却面临 2 周 VDR 上线流程。 | 未验证 —— 基于品类定位。 |

### 1.3 用户细分

| 细分 | 核心目标 | 当前痛点 | 值得切换的时刻 | P0 相关性 |
|---|---|---|---|---|
| 融资创始人 | 在跟进前识别真正的投资人兴趣。 | 不知道哪些投资人认真；deck 会泄露或过时。 | 发送 deck 后，看到投资人把团队页和财务页反复读了三次。 | P0 主要目标。 |
| 投资机构（VC/PE/IR/并购） | 对敏感资本材料保持可控与可审计；识别 LP/买方互动。 | LP 更新发出去就消失；没有互动可见性；VDR 对小基金太重。 | LP 委员会在下一轮关闭前要求提供互动证明。 | 次要；通过接收方接触与创始人推荐进入。 |
| B2B 销售 / BD 团队 | 识别购买意图并把握跟进时机。 | 提案石沉大海；内部 champion 转发材料但卖家一无所知。 | 一天内三位新利益相关者查看了价格页。 | 次要；创始人开始销售后的自然扩展。 |

### 1.4 用户问题

- **当前痛点：** 发送重要材料后，用户失去可见性、控制力和时机。他们不知道谁真正感兴趣、何时跟进，或敏感内容是否被不当分享。
- **为什么现在：** 融资越来越远程和异步；买方决策委员会更大；基金需要比传统 VDR 更轻的 LP 沟通工具。
- **现有 workaround：** DocSend、Google Drive、Dropbox、邮件附件或传统 VDR。
- **为什么 workaround 失败：** 见 1.2 节。

### 1.5 成功定义

- **用户可见的成功：** 发送方在接收方活动发生后几分钟内就知道谁是高意图对象，并能在不猜测的情况下采取正确下一步动作。
- **业务 / 项目成功：** 创始人通过上传 deck 并创建至少一条投资人链接完成激活；50%+ 活跃用户首次收到打开后回流；30%+ 内测用户创建数据室。
- **工程成功：** P0 功能可作为单一 Web 应用、一个数据库和一个对象存储后端交付；端到端闭环可在本地环境验证。

### 1.6 假设与事实状态

| 事项 | 状态 | 重要性 |
|---|---|---|
| 创始人切入是获取早期用户最快路径。 | 假设 | 决定 P0 细分聚焦与首屏落地页文案。 |
| 投资人无需注册就会打开创始人 deck。 | 假设 | 决定默认访问模式与阅读器体验。 |
| 浏览器 PDF 阅读器可以准确测量页面级停留时长。 | 假设 | 决定分析事件设计与评分输入。 |
| HubSpot/Salesforce API 速率限制不会制约 MVP 使用。 | 未知 | 影响 P1 CRM 同步的批处理设计。 |
| 动态水印在 PDF 下载时无需昂贵渲染基础设施即可实现。 | 未知 | 影响 P0 与 P1 的水印范围。 |
| 创始人计划 10 个活跃链接是合理限制。 | 假设 | 影响定价与免费转付费流程。 |

## 2. 范围契约

### 2.1 硬性约束

| 约束 | 为何是硬性 | 下游影响 |
|---|---|---|
| 每个租户范围表必须按 `workspace_id` 过滤。 | 多租户 SaaS；工作空间间数据泄露是灾难性的。 | 所有查询、索引和 API 处理程序都必须包含工作空间范围；测试必须验证跨工作空间隔离。 |
| 访问规则必须在文档字节返回前强制执行。 | 安全承诺；泄露的文档无法“撤回”。 | 阅读器中间件必须在流式传输内容前解析并验证链接；不允许直接对象存储 URL。 |
| 分析事件记录只追加不可改。 | 可审计性与评分可复现性依赖不可变的原始事件。 | 事件只能插入，不能更新；派生值存放在独立表中。 |
| 接收方仅在发送方策略明确要求时才创建账户。 | 核心增长闭环；摩擦会扼杀交易速度。 | 公开和邮箱验证模式必须无需注册即可工作。 |
| 链接撤回应立即生效。 | 发送方信任；已撤销链接绝不能返回内容。 | 状态检查是权威的，缓存的撤销必须快速失效。 |

### 2.2 推荐默认

| 默认项 | 为何这样默认 | 可接受的替代 |
|---|---|---|
| PostgreSQL 15+ 配合现有 schema。 | Schema 已设计；关系模型适合租户 + 文档 + 事件。 | 云托管 Postgres（RDS、Supabase、Neon）使用相同 schema。 |
| 对象存储存放文件 blob。 | 解耦文件服务与应用服务器；独立扩展。 | S3、R2、MinIO 或 GCS 等兼容 API。 |
| 默认访问模式为邮箱验证。 | 在发送方信心与接收方摩擦间取得平衡。 | 极低摩擦活动可用公开模式；高敏感材料用白名单/密码。 |
| 创始人细分作为首个切入点。 | 购买周期最短；痛点急迫；自然病毒传播到投资人。 | 若内测数据显示销售细分激活更强，可切换。 |
| 创始人计划 10 个活跃链接。 | 形成转化压力，同时不阻塞真实融资。 | 根据免费转付费数据调整。 |

### 2.3 创意空间

| 领域 | 可优化之处 | 护栏 |
|---|---|---|
| 仪表板卡片布局与文案 | 空状态、提醒徽章、推荐动作措辞 | 仍必须展示高意图信号、最近打开与风险。 |
| 评分算法权重 | 页面重读、转发、时长等具体分值 | 分数区间（0-39/40-69/70-100）与标签必须保留。 |
| 阅读器 UI 外壳 | 顶栏、底栏、目录抽屉、加载骨架 | 不得遮挡文档内容或隐藏下载策略状态。 |
| 邮件提醒设计 | 主题行、发送时机、摘要 vs 即时 | 必须可靠投递首次打开和高分事件。 |
| 引导流程 | 步骤顺序、提示、模板推荐 | 必须让用户在几分钟内创建第一条智能链接。 |

### 2.4 非目标

| 非目标 | 为何排除 | 重新考虑触发条件 |
|---|---|---|
| 通用云盘 | 超出品类；会稀释“交易情报”定位。 | 用户研究持续显示通用存储需求。 |
| 法律级 DRM 防截屏 | 技术上无法保证；水印 + 审计是 MVP 立场。 | 企业客户要求并愿意付费。 |
| 原生邮件营销自动化 | 会与 Mailchimp/Apollo 竞争；非文档信号核心。 | 销售用户反复要求序列功能。 |
| 完整电子签工作流 | HelloSign/Docusign 已存在；签名是另一种工作。 | 融资交割工作流明确要求。 |
| AI 改写 deck/提案 | 输出泛化风险高；动作推荐更安全。 | 用户研究显示强烈需求与信任。 |
| v1 企业 DLP / 数据驻留 / SSO / SCIM | 重企业采购；创始人切入点不需要。 | Secure Room 计划起量。 |

## 3. 用户、工作与场景

### 3.1 主要用户

- **角色：** 融资创始人（Seed/Series A 的 CEO/CFO/运营者）。
- **待完成工作：** 在发送下一条跟进前，知道哪些投资人真正感兴趣。
- **当前触发点：** 启动融资、发送第一份 deck、投资人要求数据室。
- **成功证据：** 创建投资人专属链接，看到 Hot 评分，并与认真投资人安排更多会议。

### 3.2 次要用户或运营者

- **投资机构运营者 / IR / 合伙人：** 需要对 LP/交易材料进行可控与审计。
- **B2B 销售代表 / 经理：** 需要提案追踪与采购委员会可见性。
- **工作空间管理员：** 配置默认项、审批内容、管理成员。

### 3.3 关键场景

| 场景 | 入口点 | 期望结果 | 需要避免失败 |
|---|---|---|---|
| 创始人向投资人发送 deck | Documents 页面 → 创建智能链接 | 投资人低摩擦打开；创始人看到首次打开提醒与页面分析 | 投资人遇到注册墙，或误拿到过期链接。 |
| 投资人把 deck 转发给合伙人 | 接收方阅读器 → 新邮箱打开链接 | 创始人发现新阅读者/转发，评分上升 | 新阅读者因白名单过窄被拦。 |
| 创始人打开融资数据室 | Deal Rooms → 创建房间 → 邀请投资人 | 投资人浏览材料；创始人追踪房间互动 | 房间搭建太慢或权限令人困惑。 |
| 销售向 champion 发送提案 | 内容库或 Documents → 智能链接 | Champion 与委员会查看价格页；销售收到 Slack/邮件提醒 | 销售始终不知道提案已被内部转发。 |
| 基金 IR 发送季度 LP 更新 | Deal Rooms → LP Update Room 模板 | LP 访问品牌化门户；IR 识别高互动 LP | LP 更新看起来不专业或缺乏访问控制。 |

## 4. 用户故事

### US-001：上传文档

**描述：** 作为发送方，我想要上传文档，以便创建安全、可追踪的链接。  
**优先级：** P0  
**来源：** 关键场景 —— 创始人向投资人发送 deck。

**验收标准：**
- [ ] 用户可以上传 PDF、PPT、DOC、XLS、图片和视频文件。
- [ ] 上传进度可见。
- [ ] 上传失败显示具体错误信息。
- [ ] 上传后的文档带状态出现在 Documents 列表中。
- [ ] 类型检查、lint 和构建通过。

### US-002：创建智能链接

**描述：** 作为发送方，我想要生成带访问设置的链接，以便安全分享文档。  
**优先级：** P0  
**来源：** 关键场景 —— 创始人向投资人发送 deck。

**验收标准：**
- [ ] 用户可以从文档创建命名链接。
- [ ] 用户可以选择访问模式与安全预设。
- [ ] 用户可以启用或禁用下载。
- [ ] 用户可以设置过期时间。
- [ ] 用户可以复制已创建的链接。
- [ ] 创建前显示接收方摩擦等级。
- [ ] 在浏览器中验证。

### US-003：查看共享文档

**描述：** 作为接收方，我想要以最小摩擦打开共享文档，以便快速审阅。  
**优先级：** P0  
**来源：** 关键场景 —— 投资人打开 deck。

**验收标准：**
- [ ] 策略允许时，接收方无需创建账户即可打开有效链接。
- [ ] 接收方在桌面和移动端看到可读的文档阅读器。
- [ ] 接收方可在页面间移动并查看目录。
- [ ] 访问过期或被拒时显示清晰信息。
- [ ] 仅在启用时显示下载按钮。
- [ ] 验证桌面与移动端浏览器视图。

### US-004：追踪接收方活动

**描述：** 作为发送方，我想要看到接收方活动，以便理解兴趣。  
**优先级：** P0  
**来源：** 关键场景 —— 投资人把 deck 转发给合伙人。

**验收标准：**
- [ ] 系统记录首次打开与重复打开。
- [ ] 系统记录页面级浏览及停留时长。
- [ ] 下载启用时记录下载事件。
- [ ] 活动在 60 秒内出现在链接分析中。
- [ ] 正常条件下事件延迟低于 10 秒。

### US-005：生成意图评分

**描述：** 作为发送方，我想要 DealSignal 为互动打分，以便优先跟进。  
**优先级：** P0  
**来源：** 主要用户待完成工作。

**验收标准：**
- [ ] 系统为每个接收方生成 0-100 分。
- [ ] 分数映射为 Cold（0-39）、Warm（40-69）或 Hot（70-100）。
- [ ] 分数附带自然语言解释。
- [ ] 新活动发生时分数更新。
- [ ] 支持细分场景专属评分类型。

### US-006：创建数据室

**描述：** 作为发送方，我想要从模板创建房间，以便快速分享多份尽调材料。  
**优先级：** P0  
**来源：** 关键场景 —— 创始人打开融资数据室。

**验收标准：**
- [ ] 用户可以选择房间模板（Seed Fundraising、Series A、LP Update、M&A Diligence、Enterprise Sales、Partner Enablement）。
- [ ] 房间包含模板默认文件夹。
- [ ] 用户可向文件夹上传文件。
- [ ] 用户可以邀请接收方。
- [ ] 用户可以查看房间活动。
- [ ] 在浏览器中验证。

### US-007：应用动态水印

**描述：** 作为发送方，我想要用接收方信息给文档加水印，以便震慑泄露并便于追溯。  
**优先级：** P0  
**来源：** 硬性约束 —— 震慑与可追溯性。

**验收标准：**
- [ ] 用户可以为链接或房间启用水印。
- [ ] 阅读器显示带接收方邮箱和时间戳的水印。
- [ ] 下载启用且策略要求时，下载文件包含水印。
- [ ] 水印设置在链接/房间设置中可见。

### US-008：接收高意图提醒

**描述：** 作为发送方，我想要在接收方表现出强烈兴趣时收到提醒，以便在合适时机跟进。  
**优先级：** P0  
**来源：** 主要用户待完成工作。

**验收标准：**
- [ ] 用户可以配置邮件提醒。
- [ ] 发送首次打开提醒。
- [ ] 发送高分提醒。
- [ ] 提醒链接到相关分析页面。
- [ ] 提醒排队并在失败时重试。

### US-009：将活动同步到 CRM

**描述：** 作为销售用户，我想要文档活动同步到 CRM，以便交易记录保持最新。  
**优先级：** P1  
**来源：** 次要用户细分 —— B2B 销售。

**验收标准：**
- [ ] 用户可以连接 HubSpot 或 Salesforce。
- [ ] 用户可以将智能链接与 CRM 交易/联系人关联。
- [ ] 系统将打开和高意图事件写入 CRM 时间线。
- [ ] 启用时，为 Hot 评分事件创建跟进任务。

### US-010：管理已批准销售内容

**描述：** 作为销售经理，我想要共享库中的已批准内容，以便销售发送正确材料。  
**优先级：** P1  
**来源：** 次要用户细分 —— B2B 销售。

**验收标准：**
- [ ] 管理员可将文档标记为 Approved、In Review、Draft 或 Archived。
- [ ] 团队成员可按状态筛选。
- [ ] 启用时，管理员可限制仅已批准内容才能创建智能链接。
- [ ] 内容表现可追踪。

## 5. 功能需求

- **FR-1：** 系统必须允许用户上传支持的文档文件。
- **FR-2：** 系统必须为文档生成唯一的智能链接。
- **FR-3：** 系统必须允许一份文档拥有多个智能链接。
- **FR-4：** 系统必须允许用户设置链接过期时间。
- **FR-5：** 系统必须允许用户撤销链接。
- **FR-6：** 系统必须允许用户要求接收方邮箱验证。
- **FR-7：** 系统必须允许用户按邮箱白名单限制访问。
- **FR-8：** 系统必须允许用户启用密码保护。
- **FR-9：** 系统必须允许用户启用或禁用下载。
- **FR-10：** 系统必须允许用户启用动态水印。
- **FR-11：** 系统必须记录文档打开事件。
- **FR-12：** 系统必须记录页面级浏览事件。
- **FR-13：** 系统必须记录下载事件。
- **FR-14：** 系统必须展示接收方级分析。
- **FR-15：** 系统必须展示文档级分析。
- **FR-16：** 系统必须生成分细场景专属意图评分。
- **FR-17：** 系统必须解释意图评分为何变化。
- **FR-18：** 系统必须允许用户创建数据室。
- **FR-19：** 系统必须允许用户应用文件夹级房间权限。
- **FR-20：** 系统必须允许用户邀请接收方进入房间。
- **FR-21：** 系统必须提供房间活动日志。
- **FR-22：** 系统必须提供高意图通知。
- **FR-23：** 系统必须允许用户连接 Slack。
- **FR-24：** 系统必须允许用户连接 HubSpot 或 Salesforce。
- **FR-25：** 系统必须将选定活动事件同步到 CRM。
- **FR-26：** 系统必须提供内容库。
- **FR-27：** 系统必须支持文档版本历史。
- **FR-28：** 系统必须允许管理员归档文档。
- **FR-29：** 系统必须展示被阻止、过期和拒绝访问的页面。
- **FR-30：** 系统必须提供分析 CSV 导出。

## 6. 体验与状态契约

### 6.1 主流程

```
[发送方]          [系统]              [接收方]           [发送方]
   │                 │                       │                    │
   │ 上传文档        │                       │                    │
   │────────────────>│                       │                    │
   │                 │ 处理页面              │                    │
   │                 │──────┐                │                    │
   │                 │<─────┘                │                    │
   │                 │                       │                    │
   │ 创建智能链接                           │                    │
   │────────────────>│                       │                    │
   │                 │ 生成 slug + 设置                          │
   │                 │<──────┐               │                    │
   │ 复制链接        │       │               │                    │
   │<────────────────│       │               │                    │
   │                 │       │               │                    │
   │ 通过邮件/Slack 等发送链接              │                    │
   │───────────────────────────────────────>│                    │
   │                 │                       │                    │
   │                 │ 解析链接              │                    │
   │                 │<──────────────────────│                    │
   │                 │ 执行访问规则          │                    │
   │                 │──────────────────────>│ (通过 / 阻止)      │
   │                 │                       │                    │
   │                 │ 流式传输文档          │                    │
   │                 │──────────────────────>│                    │
   │                 │                       │                    │
   │                 │ 记录事件              │                    │
   │                 │──────┐                │                    │
   │                 │      │ 更新评分       │                    │
   │                 │<─────┘                │                    │
   │                 │                       │                    │
   │                 │ 发送提醒              │                    │
   │                 │──────────────────────>│                    │
   │                 │                       │                    │
   │ 查看分析        │                       │                    │
   │<────────────────│                       │                    │
   │                 │                       │                    │
   │ 执行跟进动作                                               │
   │─────────────────────────────────────────────────────────────>│
```

### 6.2 布局或界面模型

桌面 Web 管理后台：

```text
┌────────────────────────────────────────────────────────────────────┐
│ DealSignal    搜索    + 创建    提醒    工作空间  个人资料         │
├──────────────┬─────────────────────────────────────────────────────┤
│ 仪表板       │ 今日                                                 │
│ 文档         │ ┌────────────┐ ┌────────────┐ ┌────────────┐        │
│ 链接         │ │ 高意图信号 │ │ 打开次数   │ │ 风险       │        │
│ 数据室       │ │ 8          │ │ 34         │ │ 2          │        │
│ 联系人       │ └────────────┘ └────────────┘ └────────────┘        │
│ 洞察         │                                                      │
│ 内容库       │ 推荐跟进                                             │
│ 设置         │ ┌─────────────────────────────────────────────────┐ │
│              │ │ Sequoia 财务页看了 3 次      发送跟进           │ │
│              │ │ Acme 提案转发给 4 人         安排会议           │ │
│              │ │ LP A 重新查看 Q4 报告        通知 IR            │ │
│              │ └─────────────────────────────────────────────────┘ │
│              │ 最近活动                       活跃房间             │
└──────────────┴─────────────────────────────────────────────────────┘
```

移动端 Web 阅读器：

```text
┌─────────────────────────────┐
│ Acme Capital   Series A Deck│
├─────────────────────────────┤
│                             │
│       文档页面              │
│                             │
├─────────────────────────────┤
│  ‹     第 4 / 16 页    ›    │
├─────────────────────────────┤
│ 目录   下载   提问           │
└─────────────────────────────┘
```

### 6.3 状态

| 状态 | 触发 | 视觉标记 | 用户/系统看到 | 退出条件 |
|---|---|---|---|---|
| 默认 | 页面加载 / 初始状态 | 骨架卡片、空表 | 仪表板显示“上传第一份文档”CTA | 用户上传文档或创建链接 |
| 加载中 | 异步操作开始 | 200ms 后显示 spinner 或骨架 | 带 spinner 的模糊占位 | 操作完成或失败 |
| 空状态 | 无数据条件 | 空状态插画 + 文案 | “还没有文档。上传你的第一份 deck。” | 用户上传文档 |
| 活跃 | 用户交互 | 高亮行、打开面板 | 选中文档或链接及操作 | 用户离开或关闭面板 |
| 错误 | 失败条件 | 红色横幅/提示带图标 | 具体错误信息 + 重试 CTA | 用户重试或关闭 |
| 已撤销 | 发送方操作 | 红色“已撤销”徽章 | 链接状态变更，阅读器被阻止 | 发送方重新激活（v1 仅支持重新创建） |
| 已过期 | 时间流逝 | 灰色“已过期”徽章 | 阅读器显示“此链接已过期” | 发送方延长过期时间 |
| 高意图 | 评分 >= 70 | 火焰图标 + “Hot” 徽章 | 仪表板中接收方卡片高亮 | 评分降至 70 以下 |

### 6.4 失败路径

| 失败 | 原因 | 用户/系统响应 | 恢复 |
|---|---|---|---|
| 上传失败 | 网络超时或不支持的文件类型 | 提示：“上传失败：[原因]” | 重试上传或转换文件 |
| 链接过期 | `expires_at` 已过 | 阅读器显示“此链接已过期”并提供联系发送方入口 | 发送方在链接详情中延长过期时间 |
| 访问被拒 | 邮箱不在白名单或链接已撤销 | 阅读器显示“你没有访问权限”并提供申请访问表单 | 发送方批准申请或更新白名单 |
| 邮箱验证失败 | 错误/过期验证码 | 阅读器显示“验证失败，请重试” | 重新发送验证码 |
| 分析事件丢失 | 接收方网络故障 | 事件排队并重试；兜底心跳 | 系统在下一次阅读器 ping 时调和 |
| 评分计算滞后 | 工作队列积压 | 仪表板显示缓存分数及“最后更新于”时间戳 | 工作队列追上后刷新分数 |

### 6.5 模块体验契约

#### 模块 A：文档管理

**a) 形态与流程**
- 界面：桌面 Web 管理后台
- 交互：Documents 列表表格 + Document Detail 标签页（Overview / Pages / Links / Versions / Settings）
- 正常流程：
  1. 用户点击“上传”或拖拽文件。
  2. 系统上传到对象存储并创建 document 与 version 记录。
  3. 文档以状态（processing → ready）出现在列表中。
  4. 用户点击文档查看详情并创建智能链接。
- 失败路径：
  - 失败：文件类型不支持。
    - 响应：提示支持的格式。
    - 恢复：用户选择支持的文件。
  - 失败：处理超时。
    - 响应：状态徽章“Failed”带重试。
    - 恢复：用户重试处理或重新上传。

**b) 状态**

| 状态 | 触发 | 视觉标记 | 用户/系统看到 | 退出条件 |
|---|---|---|---|---|
| 上传中 | 文件拖入 | 进度条 | “正在上传 Series A Deck.pdf 45%” | 上传完成 |
| 处理中 | 上传完成 | Spinner + “Processing” 徽章 | “正在提取页面…” | 处理成功/失败 |
| 就绪 | 处理完成 | 绿色“Ready”徽章 | 文档带缩略图、链接数 | 归档或删除 |
| 失败 | 处理错误 | 红色“Failed”徽章 | 错误信息 + 重试 | 用户重试 |

**c) 数据依赖**
- 读取：documents、document_versions、document_pages、smart_links 计数、intent_scores 聚合。
- 写入：documents、document_versions、对象存储。
- 数据流：文档元数据拥有版本；版本拥有页面。当前版本在 documents 上反规范化。

**d) 产品决策**

| 决策 | 安全默认 | 何时会改变 |
|---|---|---|
| v1 是否支持视频文件？ | 支持，但仅存储 + 基础播放；无页面级分析。 | 用户研究显示视频是核心 pitch 格式。 |
| 是否自动提取文本供搜索？ | PDF/PPT 支持；v1 扫描图片不支持 OCR。 | OCR 服务成本与准确率数据。 |

**e) 边界情况**
- 空工作空间：显示引导 CTA，隐藏表格。
- 超大 PDF（>100 MB）：流式上传，显示进度，限制页面提取。
- 重复文件名：允许重复，显示版本指示器。
- 删除文档：软删除，异步清理存储。

#### 模块 B：智能链接创建与分享

**a) 形态与流程**
- 界面：桌面 Web 管理后台
- 交互：创建智能链接表单，含预设（快速分享 / 平衡 / 高安全）+ 自定义控件
- 正常流程：
  1. 用户选择文档。
  2. 命名链接并输入接收方邮箱。
  3. 选择预设或自定义访问模式。
  4. 设置下载、水印、过期时间。
  5. 查看接收方摩擦等级。
  6. 创建链接并复制 URL。
- 失败路径：
  - 失败：接收方邮箱域名被工作空间策略屏蔽。
    - 响应：内联警告并说明策略。
    - 恢复：用户选择允许域名或联系管理员。
  - 失败：过期时间设在过去。
    - 响应：表单校验错误。
    - 恢复：用户选择未来日期。

**b) 状态**

| 状态 | 触发 | 视觉标记 | 用户/系统看到 | 退出条件 |
|---|---|---|---|---|
| 草稿 | 表单打开 | 默认表单 | 预设选择 + 控件 | 用户提交 |
| 校验中 | 点击提交 | 创建按钮上 spinner | “正在创建链接…” | 校验通过/失败 |
| 已创建 | API 成功 | 弹窗/提示带复制按钮 | 链接 URL + 复制 CTA | 用户关闭/复制 |
| 已撤销 | 点击撤销 | 红色“已撤销”徽章 | 链接不再可访问 | v1 无 |

**c) 数据依赖**
- 读取：documents、document_versions、workspace default_security_preset。
- 写入：smart_links、smart_link_recipients、activity_events。
- 数据流：链接指向文档版本；recipient 记录追踪预期阅读者。

**d) 产品决策**

| 决策 | 安全默认 | 何时会改变 |
|---|---|---|
| 默认访问模式 | 邮箱验证 | 内测数据显示公开链接打开率更高且安全可接受。 |
| 是否允许一份文档多个链接？ | 允许 | 用户需要投资人专属追踪。 |

**e) 边界情况**
- 同一接收方在多个链接上：独立记录，独立评分。
- 已撤销链接被访问：清晰阻止页面，无内容泄露。
- 密码保护链接：服务器端验证哈希，绝不返回哈希。

#### 模块 C：接收方阅读器

**a) 形态与流程**
- 界面：移动端 + 桌面 Web 阅读器
- 交互：紧凑顶栏（发送方品牌 + 文档标题）、文档画布、底部导航、可选下载/提问按钮
- 正常流程：
  1. 接收方点击链接。
  2. 系统解析 slug 并检查状态。
  3. 执行访问模式（公开 / 邮箱验证 / 密码 / 白名单）。
  4. 阅读器加载文档页面。
  5. 接收方浏览页面；事件被记录。
  6. 接收方在允许时下载或提问。
- 失败路径：
  - 失败：链接已撤销或过期。
    - 响应：阻止页面显示原因和“联系发送方”按钮。
    - 恢复：接收方联系发送方；发送方重新激活或延长。
  - 失败：邮箱不被允许。
    - 响应：“此链接受限制”并提供申请访问表单。
    - 恢复：发送方批准申请。

**b) 状态**

| 状态 | 触发 | 视觉标记 | 用户/系统看到 | 退出条件 |
|---|---|---|---|---|
| 访问检查 | 打开链接 | Spinner | “正在检查访问权限…” | 允许 / 阻止 |
| 验证邮箱 | 需要邮箱验证 | 输入框 + 发送按钮 | “请输入邮件中的验证码” | 验证码通过 |
| 加载文档 | 访问通过 | 骨架页 | “正在加载文档…” | 文档渲染完成 |
| 阅读中 | 文档加载完成 | 页面渲染并显示页码 | 文档页面 + 导航 | 用户关闭或离开 |
| 被阻止 | 访问被拒 | 锁图标 + 红色文案 | 原因 + 联系发送方 / 申请访问 | 用户联系发送方 |

**c) 数据依赖**
- 读取：smart_links、smart_link_recipients、access_grants、document_versions、document_pages、deal_room_files、deal_room_access_rules。
- 写入：view_sessions、activity_events、page_view_events、download_events、access_grants。
- 数据流：阅读器读多写少；事件为只追加写入。

**d) 产品决策**

| 决策 | 安全默认 | 何时会改变 |
|---|---|---|
| 是否要求阅读者注册账户？ | 除非策略要求，否则不要 | 企业客户要求审计身份。 |
| 是否显示隐私声明？ | 是，简短页脚/提示 | 监管反馈或用户投诉。 |

**e) 边界情况**
- 移动端视口：底栏适合拇指操作，支持双指缩放。
- 离线：缓存当前页面，事件排队。
- 屏幕阅读器：页面 alt 文本，键盘导航。
- 超大页面图片：懒加载、降采样。

#### 模块 D：分析仪表板

**a) 形态与流程**
- 界面：桌面 Web 管理后台 + 移动端轻量管理
- 交互：仪表板卡片（高意图信号 / 打开次数 / 风险）、推荐跟进列表、最近活动流
- 正常流程：
  1. 发送方打开 Dashboard。
  2. 系统聚合最近事件与评分。
  3. 卡片与推荐渲染。
  4. 发送方点击推荐打开链接/联系人详情。
  5. 发送方执行跟进动作。
- 失败路径：
  - 失败：评分工作队列滞后。
    - 响应：显示“X 分钟前更新”并带刷新按钮。
    - 恢复：系统追上；用户刷新。
  - 失败：尚无数据。
    - 响应：空状态带上传 CTA。
    - 恢复：用户上传并分享第一份文档。

**b) 状态**

| 状态 | 触发 | 视觉标记 | 用户/系统看到 | 退出条件 |
|---|---|---|---|---|
| 空状态 | 无事件 | 空状态插画 + 上传 CTA | “上传你的第一份 deck 以查看信号” | 用户创建链接并获得打开 |
| 加载中 | 打开 Dashboard | 骨架卡片 | 闪烁占位 | 数据加载完成 |
| 高意图信号 | 最近高意图事件 | 火焰徽章 | 高意图接收方列表 | 事件过期 |
| 风险提醒 | 被阻止/过期/可疑 | 红色提醒卡片 | 风险摘要 + 查看 CTA | 风险解决 |

**c) 数据依赖**
- 读取：activity_events、page_view_events、view_sessions、intent_scores、smart_links、contacts、recommendations。
- 写入：recommendations（行动助手）、notifications。
- 数据流：原始事件 → 物化评分/推荐 → 仪表板读取。

**d) 产品决策**

| 决策 | 安全默认 | 何时会改变 |
|---|---|---|
| 实时 vs 批量提醒 | 首次打开和高分实时；日报摘要稍后可选 | 用户抱怨噪音。 |
| 评分刷新频率 | 每次重大事件后，防抖 30 秒 | 性能问题。 |

**e) 边界情况**
- 一分钟内大量事件：聚合，不要淹没 UI。
- 评分相同：按最近性排序。
- 可疑访问（异常地理位置）：显示风险卡片。

#### 模块 E：数据室

**a) 形态与流程**
- 界面：桌面 Web 管理后台 + 移动端 Web 阅读器
- 交互：房间列表 → 从模板创建 → 房间详情标签页（Overview / Files / Recipients / Activity / Q&A / Settings）
- 正常流程：
  1. 用户从模板创建房间。
  2. 系统创建文件夹。
  3. 用户上传/分配文档到文件夹。
  4. 用户邀请接收方。
  5. 接收方访问房间并查看文件。
  6. 用户追踪房间活动。
- 失败路径：
  - 失败：接收方尝试访问无权限文件夹。
    - 响应：文件夹隐藏或禁用并带提示。
    - 恢复：发送方更新访问规则。
  - 失败：房间邀请邮件退信。
    - 响应：接收方行显示退信状态。
    - 恢复：发送方更正邮箱并重新邀请。

**b) 状态**

| 状态 | 触发 | 视觉标记 | 用户/系统看到 | 退出条件 |
|---|---|---|---|---|
| 草稿 | 房间创建 | “Draft” 徽章 | 房间尚未发布 | 用户发布 |
| 活跃 | 房间发布 | “Active” 徽章 | 接收方可访问 | 归档 / 过期 |
| 已归档 | 发送方归档 | “Archived” 徽章 | 只读历史视图 | 恢复（未来） |
| 待审批 | 接收方申请访问 | 黄色“Pending”徽章 | 发送方在仪表板看到申请 | 批准 / 拒绝 |

**c) 数据依赖**
- 读取：deal_rooms、deal_room_folders、deal_room_files、deal_room_members、deal_room_access_rules、documents、document_versions。
- 写入：deal_rooms、deal_room_folders、deal_room_files、deal_room_members、deal_room_access_rules、activity_events。
- 数据流：房间拥有文件夹和成员；文件引用文档版本；按文件/文件夹评估访问规则。

**d) 产品决策**

| 决策 | 安全默认 | 何时会改变 |
|---|---|---|
| v1 是否做 Q&A？ | 做，简化版（提问 + 回答，无 threading） | 用户要求 threading 和分配。 |
| 文件夹级权限 | 支持 | 用户发现仅房间级权限太粗。 |

**e) 边界情况**
- 嵌套文件夹：v1 支持一级，更多后续支持。
- 空文件房间：显示空状态并带上传 CTA。
- 成员移除：立即撤销访问授权。

## 7. 数据与集成契约

### 7.1 核心数据对象

```jsonc
{
  "version": "1",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "Jane Doe",
    "avatar_url": "https://...",
    "created_at": "2026-01-01T00:00:00Z"
  },
  "workspace": {
    "id": "uuid",
    "name": "Acme Capital",
    "slug": "acme-capital",
    "mode": "founder",
    "default_security_preset": {},
    "created_at": "2026-01-01T00:00:00Z"
  },
  "document": {
    "id": "uuid",
    "workspace_id": "uuid",
    "name": "Series A Deck.pdf",
    "status": "ready",
    "current_version_id": "uuid",
    "metadata": {}
  },
  "document_version": {
    "id": "uuid",
    "document_id": "uuid",
    "version_number": 1,
    "storage_bucket": "dealsignal-files",
    "storage_key": "path/to/file.pdf",
    "mime_type": "application/pdf",
    "file_size_bytes": 1048576,
    "checksum_sha256": "abc123",
    "page_count": 12,
    "processing_status": "ready"
  },
  "smart_link": {
    "id": "uuid",
    "workspace_id": "uuid",
    "document_id": "uuid",
    "document_version_id": "uuid",
    "name": "Sequoia - Sarah Chen",
    "slug": "abc123xyz",
    "access_mode": "email_verification",
    "download_policy": "allowed",
    "watermark_enabled": true,
    "expires_at": "2026-02-01T00:00:00Z",
    "revoked_at": null,
    "status": "active"
  },
  "view_session": {
    "id": "uuid",
    "smart_link_id": "uuid",
    "contact_id": "uuid",
    "recipient_email": "sarah@sequoiacap.com",
    "ip_address": "1.2.3.4",
    "user_agent": "Mozilla/5.0",
    "started_at": "2026-01-01T12:00:00Z",
    "ended_at": null
  },
  "page_view_event": {
    "id": "uuid",
    "view_session_id": "uuid",
    "document_version_id": "uuid",
    "page_number": 8,
    "visible_started_at": "2026-01-01T12:05:00Z",
    "visible_ended_at": "2026-01-01T12:07:00Z",
    "duration_ms": 120000
  },
  "intent_score": {
    "id": "uuid",
    "score_type": "investor_intent",
    "score": 84,
    "label": "hot",
    "explanation": "Hot because this recipient viewed the pricing page 3 times...",
    "factors": {},
    "contact_id": "uuid",
    "calculated_at": "2026-01-01T12:10:00Z"
  }
}
```

### 7.2 外部接口

| 接口 | 方向 | 契约 | 失败模式 |
|---|---|---|---|
| 对象存储（S3/R2/MinIO） | 出站写入 + 读取 | 通过预签名 URL 或 SDK 直接上传；文件通过应用代理提供 | 上传重试；仅当签名且短时效时才回退到直接 URL |
| 邮件服务商（SendGrid/Resend/AWS SES） | 出站 | 通过 SMTP/API 发送验证码、提醒、邀请 | 排队重试；持续失败时提醒管理员 |
| HubSpot API | 出站 | OAuth 2.0；写入时间线事件和任务 | 指数退避重试；标记集成错误 |
| Salesforce API | 出站 | OAuth 2.0；写入 Task/Event 对象 | 指数退避重试；标记集成错误 |
| Slack Web API | 出站 | OAuth 2.0；向频道发消息 | 重试；记录失败 |
| GeoIP 服务 | 出站 | 将 IP 解析为国家/地区/城市/设备 | 失败时优雅降级为未知 |

### 7.3 数据保留、隐私与权限

- 接收方 IP 地址默认保留 90 天；工作空间管理员可在 P2 配置更短周期。
- 密码和集成令牌在应用层哈希或加密存储。
- 删除的文档在 DB 中软删除；对象存储清理异步进行。
- 阅读器中展示分析披露说明。
- 工作空间所有者拥有一方互动数据；DealSignal 不售卖接收方数据。
- P2 通过管理员操作支持 GDPR 删除请求。

### 7.4 架构、包大小与可替换性

分层架构：

```
+--------------------------------------------------+
| 表现层：React Web 应用、移动端阅读器、邮件        │
+--------------------------------------------------+
         |
+--------------------------------------------------+
| API 层：REST/JSON、认证中间件、租户过滤           │
+--------------------------------------------------+
         |
+--------------------------------------------------+
| 领域服务：上传、链接、阅读器、分析、评分、通知、  │
| 房间、集成                                        │
+--------------------------------------------------+
         |
+--------------------------------------------------+
| 数据层：PostgreSQL + 对象存储                     │
+--------------------------------------------------+
```

依赖表：

| 依赖 / 库 | 用途 | 为何优于替代 | 包大小 |
|---|---|---|---|
| PostgreSQL 15+ | 关系数据、租户隔离、JSONB | Schema 已设计；审计事件需要 ACID | N/A（服务） |
| 对象存储（S3/R2/MinIO） | 文件 blob | 解耦扩展 | N/A（服务） |
| React 18+ | Web UI | 团队熟悉、生态丰富 | ~40 kB 运行时 |
| pdf.js 或 react-pdf | 浏览器 PDF 渲染 | 标准、维护良好 | ~200 kB |
| Tailwind CSS | 样式 | 快速 UI 迭代 | ~0 kB 运行时（purge） |
| BullMQ / pg-boss | 后台任务 | Postgres 支撑的可靠性 | ~100 kB |
| SendGrid/Resend SDK | 邮件投递 | 可靠的 transactional 邮件 | ~50 kB |

除现有堆栈假设外，不强制新增运行时依赖。项目已有偏好的库可继续使用。

最大架构风险：高事件量下的实时意图评分。如果评分工作队列跟不上，仪表板数据会变旧，提醒会延迟。缓解：评分计算是幂等的，可批量；积极缓存。

可替换性：

| 决策 | 推荐默认 | 可接受替代 | 绝不能变的不变量 | 选错风险 |
|---|---|---|---|---|
| 对象存储提供商 | S3 兼容（R2/MinIO/S3） | 任何 S3 兼容 API | 文件通过 bucket+key 引用，绝不直接暴露 | 迁移成本与链接失效 |
| 邮件服务商 | Resend 或 SendGrid | AWS SES、Postmark | transactional 邮件队列 + 重试语义 | 丢失提醒与验证码 |
| PDF 渲染器 | pdf.js | 服务端图片瓦片 | 页面级分析事件必须仍能触发 | 分析或阅读器体验损坏 |

### 7.5 输出与交付契约

| 输出形式 | 描述 | 消费者 |
|---|---|---|
| 智能链接 URL | 可分享的受控链接 | 发送方分发给接收方 |
| 文档阅读器页面 | 品牌化接收方阅读体验 | 接收方 |
| 仪表板视图 | 高意图信号、最近活动、跟进建议 | 发送方 |
| 链接/文档/房间分析 | 时间线、页面表现、意图评分 | 发送方 |
| 邮件提醒 | 首次打开或高分通知 | 发送方 |
| CSV 导出 | 离线报告用的分析数据 | 发送方、管理员 |
| 数据室门户 | 带权限的分组材料 | 接收方、LP、买方 |

## 8. 风险、未知项与待决策

### 8.1 风险

| 风险 | 影响 | 缓解 | 负责人 |
|---|---|---|---|
| 用户觉得追踪“ creepy ” | 声誉受损；打开率下降 | 透明隐私披露；专业文案；尽可能提供退出 | 产品 |
| 安全承诺被过度解读 | 法律风险；客户流失 | 明确说明截屏无法完全阻止；将水印定位为震慑手段 | 法务 / 产品 |
| 创始人市场有季节性 | 融资间歇期流失激增 | 扩展到投资人更新和销售提案 | GTM |
| DocSend 拥有品类认知 | CAC 更高；自然增长更慢 | SEO 针对“DocSend 替代”；创始人专属落地页；更低摩擦阅读器 | 市场 |
| 实时评分跟不上 | 仪表板陈旧；提醒延迟 | 幂等批量评分；缓存；工作队列自动扩缩 | 工程 |
| 企业安全需求拖慢采用 | 销售周期延长 | 从创始人/新兴基金起步，再进入企业采购 | GTM |

### 8.2 已知未知项

| 未知项 | 为何重要 | 解决前的安全默认 |
|---|---|---|
| HubSpot/Salesforce API 速率限制 | 影响批处理与重试策略 | 批量写入，尊重 429，指数退避 |
| 下载时动态水印可行性 | 影响 P0 水印范围 | 先实现阅读器层水印；文件级嵌入延后到 P1 |
| 跨设备准确测量页面级时长 | 影响评分准确性 | 使用 Visibility API + 心跳；忽略后台标签页 |
| 验证码邮件送达率 | 影响阅读器转化 | 使用知名服务商；监控退信/垃圾邮件率 |
| 创始人计划链接限制最优值 | 影响转化 | 从 10 个活跃链接开始；A/B 测试 |
| 投资人是否愿意打开追踪链接 | 影响核心价值主张 | 透明披露；低摩擦默认；测量打开率 |

### 8.3 待决策

| 决策 | 选项 | 默认推荐 | 改变信号 |
|---|---|---|---|
| 首个切入市场 | 创始人 vs 销售 | 创始人 | 内测中销售团队激活更强 |
| 水印免费还是付费 | 免费（震慑）vs 付费转化功能 | 创始人计划中免费；高级模板付费 | 免费伤害转化，或付费伤害采用 |
| 接收方隐私披露 | 标准化 vs 可配置 | 标准化，显示工作空间名称 | 受监管客户要求自定义 |
| AI 辅助深度 | 动作推荐 vs 邮件草稿 | v1 只做推荐；P2 邮件草稿 | 用户不信任或强烈需要 AI 草稿 |
| 受监管工作空间是否允许公开免认证链接 | 允许 vs 策略屏蔽 | 工作空间级开关，默认关闭 | 合规反馈 |

### 8.4 产品指标与性能目标

| 目标 | 目标值 | 测量方法 | 降级阈值 | 负责人 |
|---|---:|---|---:|---|
| 阅读器首次有意义渲染 | < 2.0 s | Chrome DevTools Lighthouse / web-vitals，典型 10 页 PDF | > 3.0 s | 前端 |
| 加载后页面切换 | < 500 ms | 手动/浏览器自动化计时 | > 1.0 s | 前端 |
| 分析事件在仪表板可见 | < 60 s | 端到端测试：打开链接，等待事件 | > 2 min | 后端 |
| 关键事件后意图评分刷新 | < 60 s | 后端测试 / 仪表板观察 | > 2 min | 后端 |
| 链接撤销传播 | < 5 s | 撤销后立即打开已撤销链接 | > 10 s | 后端 |
| 上传成功率 | > 95% | 支持文件类型的后端日志 | < 90% | 后端 |
| 邮箱验证投递 | > 98% | 邮件服务商分析 | < 95% | 后端 |
| 内测激活（创建第一条链接） | > 70% 注册 | 产品分析 | < 50% | 产品 |
| 内测首次打开后回流 | > 50% | 产品分析 | < 30% | 产品 |

## 9. 验证矩阵

| ID | 需求 | 证据 | 检查方法 | 发布前必须？ |
|---|---|---|---|---|
| US-001 | 上传支持的文件 | 上传文件出现在 DB 和存储中 | 单元测试 + 手动浏览器上传 | 是 |
| US-001 | 上传进度与错误 | 录屏 / 提示可见 | 浏览器手动测试 | 是 |
| US-002 | 创建带设置智能链接 | DB 记录 + 可复制 URL | API 测试 + 浏览器测试 | 是 |
| US-002 | 显示接收方摩擦 | 表单截图 | 浏览器手动测试 | 是 |
| US-003 | 无需账户打开链接 | 阅读器加载 | 浏览器手动测试（隐身） | 是 |
| US-003 | 移动端阅读器可用 | 移动端视口截图 | 浏览器 DevTools + 真机 | 是 |
| US-003 | 过期/被拒信息 | 阻止页面截图 | 用过期链接手动测试 | 是 |
| US-004 | 记录首次打开 | activity_events 行 | DB 查询 / API 响应 | 是 |
| US-004 | 记录页面浏览时长 | page_view_events 行 | DB 查询 / API 响应 | 是 |
| US-004 | 事件延迟 < 60 s | 时间差测试 | 端到端测试 | 是 |
| US-005 | 0-100 分数带标签 | intent_scores 行 | DB 查询 + 仪表板截图 | 是 |
| US-005 | 活动后分数更新 | 模拟事件后分数变化 | 后端测试 | 是 |
| US-006 | 从模板创建数据室 | DB 记录 + UI | 浏览器手动测试 | 是 |
| US-007 | 水印可见 | 阅读器截图 | 浏览器手动测试 | 是 |
| US-008 | 首次打开邮件提醒 | 邮件收件箱 / 通知日志 | 手动测试 + 邮件服务商日志 | 是 |
| US-008 | 高分邮件提醒 | 高分事件后邮件收件箱 | 手动测试 | 是 |
| HC-1 | 工作空间隔离 | 跨工作空间请求返回 403 | 集成测试 | 是 |
| HC-2 | 内容前访问控制 | 已撤销链接绝不返回字节 | 手动测试 + 网络抓包 | 是 |
| HC-3 | 只追加事件 | 更新尝试失败或被阻止 | 单元/集成测试 | 是 |
| FR-30 | CSV 导出 | 下载的 CSV 含预期列 | 手动测试 | 是 |

## 10. 建议 Issue 映射

### Issue 1：项目脚手架与数据库 schema
- Source: HC-1, database-model.md, sql/schema.sql
- Type: infra
- Priority: high
- Dependencies: None
- Why this slice: 所有其他工作的基础。
- Acceptance Criteria:
  - [ ] PostgreSQL schema 创建所有 P0 表和索引。
  - [ ] 配置迁移工具。
  - [ ] Lint/build 通过。
- Validation:
  - [ ] DB 包含 users、workspaces、documents、smart_links 表。
- Loop-it notes:
  - Branch hint: feat/issue-1-scaffold
  - Risk class: build_failure

### Issue 2：认证与工作空间模型
- Source: HC-1
- Type: backend
- Priority: high
- Dependencies: Issue 1
- Why this slice: 租户隔离与用户身份所需。
- Acceptance Criteria:
  - [ ] 邮箱注册/登录。
  - [ ] 工作空间创建与切换。
  - [ ] 基于角色的成员关系。
- Validation:
  - [ ] 跨工作空间访问返回 403。
- Loop-it notes:
  - Branch hint: feat/issue-2-auth
  - Risk class: test_failure

### Issue 3：文档上传与版本后端
- Source: US-001, FR-1, FR-27
- Type: backend
- Priority: high
- Dependencies: Issue 1, Issue 2
- Why this slice: 所有分享功能的核心资产。
- Acceptance Criteria:
  - [ ] 上传支持的文件。
  - [ ] 存储元数据和对象存储 key。
  - [ ] 版本历史。
- Validation:
  - [ ] 上传后创建 DB 记录。
- Loop-it notes:
  - Branch hint: feat/issue-3-upload
  - Risk class: build_failure

### Issue 4：文档处理流水线
- Source: US-001, FR-12
- Type: backend
- Priority: high
- Dependencies: Issue 3
- Why this slice: 页面级分析的前提。
- Acceptance Criteria:
  - [ ] 从 PDF 提取页面。
  - [ ] 生成缩略图和文本摘要。
  - [ ] 状态机：uploaded/processing/ready/failed。
- Validation:
  - [ ] 10 页 PDF 产生 10 行 document_pages。
- Loop-it notes:
  - Branch hint: feat/issue-4-processing
  - Risk class: unknown

### Issue 5：Documents 列表与详情 UI
- Source: US-001, UI/page-prototypes.md Section 6-7
- Type: frontend
- Priority: high
- Dependencies: Issue 3, Issue 4
- Why this slice: 发送方主要的文档管理界面。
- Acceptance Criteria:
  - [ ] 列表展示文档状态、链接数、打开数。
  - [ ] 详情标签页：Overview、Pages、Links、Versions、Settings。
- Validation:
  - [ ] 浏览器测试显示已上传文档。
- Loop-it notes:
  - Branch hint: feat/issue-5-documents-ui
  - Risk class: test_failure

### Issue 6：Smart Link 创建与权限后端
- Source: US-002, FR-2~FR-10
- Type: backend
- Priority: high
- Dependencies: Issue 3
- Why this slice: 核心分享原语。
- Acceptance Criteria:
  - [ ] 创建唯一 slug 链接。
  - [ ] 访问模式：public、email_verification、allowlist、password。
  - [ ] 过期、撤销、下载策略、水印。
- Validation:
  - [ ] 创建 DB 记录；过期/撤销链接被阻止。
- Loop-it notes:
  - Branch hint: feat/issue-6-smart-links
  - Risk class: build_failure

### Issue 7：Smart Link 创建表单 UI
- Source: US-002, UI/page-prototypes.md Section 8
- Type: frontend
- Priority: high
- Dependencies: Issue 6
- Why this slice: 发送方面向的分享流程。
- Acceptance Criteria:
  - [ ] 安全预设与自定义控件。
  - [ ] 接收方摩擦指示器。
  - [ ] 创建后复制链接。
- Validation:
  - [ ] 浏览器测试创建链接并复制 URL。
- Loop-it notes:
  - Branch hint: feat/issue-7-link-form
  - Risk class: test_failure

### Issue 8：Link 详情与管理 UI
- Source: US-002, UI/page-prototypes.md Section 9
- Type: frontend
- Priority: high
- Dependencies: Issue 6
- Why this slice: 发送方操作已有链接。
- Acceptance Criteria:
  - [ ] 展示状态、安全设置、复制、撤销。
  - [ ] 活动摘要。
- Validation:
  - [ ] 撤销操作阻止阅读器访问。
- Loop-it notes:
  - Branch hint: feat/issue-8-link-detail
  - Risk class: test_failure

### Issue 9：阅读器访问控制与邮箱验证
- Source: US-003, FR-6, FR-29
- Type: fullstack
- Priority: high
- Dependencies: Issue 6
- Why this slice: 内容前的安全闸门。
- Acceptance Criteria:
  - [ ] 解析 slug、检查状态、执行访问模式。
  - [ ] 邮箱验证流程。
  - [ ] 清晰的阻止页面。
- Validation:
  - [ ] 已撤销/过期链接显示阻止页面，无内容。
- Loop-it notes:
  - Branch hint: feat/issue-9-viewer-access
  - Risk class: test_failure

### Issue 10：文档阅读器渲染与导航
- Source: US-003, UI/page-prototypes.md Section 10.2
- Type: frontend
- Priority: high
- Dependencies: Issue 4, Issue 9
- Why this slice: 接收方阅读体验。
- Acceptance Criteria:
  - [ ] 渲染 PDF 页面、导航、缩放。
  - [ ] 移动端响应式。
  - [ ] 仅允许时下载。
- Validation:
  - [ ] 浏览器与移动端视口测试。
- Loop-it notes:
  - Branch hint: feat/issue-10-viewer
  - Risk class: test_failure

### Issue 11：页面级分析事件后端
- Source: US-004, FR-11, FR-12
- Type: backend
- Priority: high
- Dependencies: Issue 9, Issue 10
- Why this slice: 核心信号数据。
- Acceptance Criteria:
  - [ ] view_sessions、page_view_events、activity_events。
  - [ ] 延迟 < 10s。
- Validation:
  - [ ] 浏览后记录事件。
- Loop-it notes:
  - Branch hint: feat/issue-11-events
  - Risk class: test_failure

### Issue 12：下载与被拒访问事件捕获
- Source: US-004, FR-13, FR-29
- Type: backend
- Priority: high
- Dependencies: Issue 9
- Why this slice: 审计与分析完整性。
- Acceptance Criteria:
  - [ ] 允许/阻止的 download_events。
  - [ ] access_denied 事件。
- Validation:
  - [ ] 被阻止的下载创建记录。
- Loop-it notes:
  - Branch hint: feat/issue-12-download-events
  - Risk class: test_failure

### Issue 13：接收方时间线与分析 UI
- Source: US-004, FR-14, FR-15
- Type: fullstack
- Priority: high
- Dependencies: Issue 11, Issue 12
- Why this slice: 发送方看到信号。
- Acceptance Criteria:
  - [ ] 每条链接/接收方的活动时间线。
  - [ ] 页面分析。
  - [ ] 转发/新阅读者检测。
- Validation:
  - [ ] 浏览器测试在事件后显示时间线。
- Loop-it notes:
  - Branch hint: feat/issue-13-analytics-ui
  - Risk class: test_failure

### Issue 14：意图评分计算与解释
- Source: US-005, FR-16, FR-17
- Type: backend
- Priority: high
- Dependencies: Issue 11, Issue 12
- Why this slice: 将数据转化为优先行动。
- Acceptance Criteria:
  - [ ] 0-100 分，Cold/Warm/Hot 标签。
  - [ ] 自然语言解释。
  - [ ] 细分场景专属类型。
- Validation:
  - [ ] 模拟活动改变评分。
- Loop-it notes:
  - Branch hint: feat/issue-14-scoring
  - Risk class: test_failure

### Issue 15：仪表板与高意图信号 UI
- Source: US-005, US-008, UI/page-prototypes.md Section 5
- Type: frontend
- Priority: high
- Dependencies: Issue 13, Issue 14
- Why this slice: 发送方每日操作界面。
- Acceptance Criteria:
  - [ ] 高意图信号、打开次数、风险卡片。
  - [ ] 推荐跟进。
  - [ ] 细分场景变体。
- Validation:
  - [ ] 高意图事件出现在仪表板。
- Loop-it notes:
  - Branch hint: feat/issue-15-dashboard
  - Risk class: test_failure

### Issue 16：基础动态水印
- Source: US-007, FR-10
- Type: backend
- Priority: medium
- Dependencies: Issue 10
- Why this slice: 震慑与可追溯性。
- Acceptance Criteria:
  - [ ] 阅读器层水印显示邮箱和时间戳。
  - [ ] 链接设置中可开关。
- Validation:
  - [ ] 截图显示水印。
- Loop-it notes:
  - Branch hint: feat/issue-16-watermark
  - Risk class: unknown

### Issue 17：邮件提醒系统
- Source: US-008, FR-22
- Type: backend
- Priority: medium
- Dependencies: Issue 14
- Why this slice: 及时通知发送方。
- Acceptance Criteria:
  - [ ] 首次打开和高分提醒。
  - [ ] 排队与重试。
- Validation:
  - [ ] 事件后收到邮件。
- Loop-it notes:
  - Branch hint: feat/issue-17-alerts
  - Risk class: test_failure

### Issue 18：基础 Deal Room 后端
- Source: US-006, FR-18~FR-21
- Type: backend
- Priority: medium
- Dependencies: Issue 3, Issue 2
- Why this slice: 多文档分享原语。
- Acceptance Criteria:
  - [ ] Rooms、folders、files、members、access rules。
  - [ ] 活动日志。
- Validation:
  - [ ] 创建 DB 记录；访问规则生效。
- Loop-it notes:
  - Branch hint: feat/issue-18-rooms
  - Risk class: build_failure

### Issue 19：Deal Room 创建与管理 UI
- Source: US-006, UI/page-prototypes.md Section 12-14
- Type: frontend
- Priority: medium
- Dependencies: Issue 18
- Why this slice: 发送方操作房间。
- Acceptance Criteria:
  - [ ] 房间列表、从模板创建、详情标签页。
  - [ ] Files、recipients、activity、Q&A。
- Validation:
  - [ ] 浏览器测试创建房间并邀请成员。
- Loop-it notes:
  - Branch hint: feat/issue-19-rooms-ui
  - Risk class: test_failure

### Issue 20：CSV 导出
- Source: FR-30
- Type: backend
- Priority: medium
- Dependencies: Issue 13
- Why this slice: 离线报告需求。
- Acceptance Criteria:
  - [ ] 导出链接/文档/房间分析为 CSV。
  - [ ] 数千行下性能可接受。
- Validation:
  - [ ] 下载的 CSV 含预期列。
- Loop-it notes:
  - Branch hint: feat/issue-20-csv
  - Risk class: test_failure

### Issue 21：高级水印模板
- Source: P1: Advanced watermark templates
- Type: backend
- Priority: medium
- Dependencies: Issue 16
- Why this slice: 扩展水印能力，支持自定义水印文本、位置、透明度、颜色，以及下载文件的水印嵌入。
- Acceptance Criteria:
  - [ ] 支持配置水印内容模板（邮箱、时间、IP、自定义文本）
  - [ ] 支持调整水印位置与样式
  - [ ] 下载 PDF 时可在文件上嵌入水印
  - [ ] 不同链接可使用不同水印模板
- Validation:
  - [ ] 配置自定义水印后 viewer 与下载文件均显示对应水印
- Loop-it notes:
  - Branch hint: feat/issue-21-高级水印模板
  - Risk class: unknown

### Issue 22：Slack 提醒集成
- Source: P1: Slack alerts, FR-23
- Type: backend
- Priority: medium
- Dependencies: Issue 17
- Why this slice: 连接 Slack workspace，将首次打开、Hot score、转发检测等事件发送到指定频道。
- Acceptance Criteria:
  - [ ] 用户可通过 OAuth 连接 Slack
  - [ ] 可配置提醒事件类型与目标频道
  - [ ] Hot score 事件触发 Slack 消息
  - [ ] 消息包含链接到 DealSignal 的按钮
- Validation:
  - [ ] 配置后模拟 Hot score 事件，Slack 频道收到消息
- Loop-it notes:
  - Branch hint: feat/issue-22-slack-提醒集成
  - Risk class: test_failure

### Issue 23：HubSpot / Salesforce 连接
- Source: US-009, FR-24
- Type: backend
- Priority: medium
- Dependencies: Issue 2
- Why this slice: 实现 CRM 集成连接，支持 OAuth 授权并存储 access token，建立 DealSignal 对象与 CRM 对象的映射。
- Acceptance Criteria:
  - [ ] 支持连接 HubSpot 与 Salesforce
  - [ ] 存储加密后的 integration credentials
  - [ ] 支持将 contact / account / smart_link / deal_room 映射到 CRM 对象
  - [ ] 连接状态可显示在设置页
- Validation:
  - [ ] 完成 OAuth 后 integrations 表生成 connected 记录
  - [ ] crm_mappings 可保存对象映射
- Loop-it notes:
  - Branch hint: feat/issue-23-hubspot---salesforce-连接
  - Risk class: test_failure

### Issue 24：CRM 活动同步
- Source: US-009, FR-25
- Type: backend
- Priority: medium
- Dependencies: Issue 23, Issue 11
- Why this slice: 将文档打开、高意图等事件写入 CRM timeline，并在启用时自动创建跟进任务。
- Acceptance Criteria:
  - [ ] Smart Link 可与 CRM deal / contact 关联
  - [ ] 文档打开事件写入关联 CRM 对象 timeline
  - [ ] Hot score 事件触发 CRM task 创建（可配置）
  - [ ] 失败同步进入重试队列
- Validation:
  - [ ] 模拟文档打开后，HubSpot/Salesforce timeline 出现对应事件
- Loop-it notes:
  - Branch hint: feat/issue-24-crm-活动同步
  - Risk class: test_failure

### Issue 25：数据室模板
- Source: P1: Deal Room templates
- Type: fullstack
- Priority: medium
- Dependencies: Issue 18
- Why this slice: 为 Seed Fundraising、Series A、LP Update、M&A Diligence、Enterprise Sales 等场景预置数据室模板与默认文件夹。
- Acceptance Criteria:
  - [ ] 创建 room 时可选择模板
  - [ ] 模板自动创建默认文件夹结构
  - [ ] 模板附带推荐的默认权限与安全设置
  - [ ] 模板可在设置中维护
- Validation:
  - [ ] 选择 Seed Fundraising 模板后自动创建 Pitch/Financials/Legal 等文件夹
- Loop-it notes:
  - Branch hint: feat/issue-25-数据室模板
  - Risk class: test_failure

### Issue 26：内容库后端
- Source: US-010, FR-26, FR-28
- Type: backend
- Priority: medium
- Dependencies: Issue 3
- Why this slice: 实现内容库的数据模型，支持文档状态 Draft / In Review / Approved / Archived、集合管理与使用统计。
- Acceptance Criteria:
  - [ ] library_collections 与 library_items 表可用
  - [ ] 文档状态可在 Draft / In Review / Approved / Archived 间切换
  - [ ] 支持将文档加入集合
  - [ ] 可追踪文档被使用的链接数与打开数
- Validation:
  - [ ] 标记文档为 Approved 后状态更新并记录审批人
- Loop-it notes:
  - Branch hint: feat/issue-26-内容库后端
  - Risk class: build_failure

### Issue 27：内容库 UI
- Source: US-010, UI/page-prototypes.md Section 17
- Type: frontend
- Priority: medium
- Dependencies: Issue 26
- Why this slice: 实现 Content Library 页面，支持按 Approved / Drafts / Archived / Templates 分类查看、审批、归档与使用统计。
- Acceptance Criteria:
  - [ ] 内容库页面展示文档状态与集合
  - [ ] Admin 可审批或归档文档
  - [ ] 可配置仅允许从 Approved 内容创建 Smart Link
  - [ ] 展示文档内容表现（链接数、打开数、转化率）
- Validation:
  - [ ] 在浏览器中打开 Content Library 可看到文档列表
  - [ ] 审批后该文档状态变为 Approved
- Loop-it notes:
  - Branch hint: feat/issue-27-内容库-ui
  - Risk class: test_failure

### Issue 28：行动助手推荐
- Source: P1: Action Assistant recommendations, PRD.md Section 7.6
- Type: backend
- Priority: medium
- Dependencies: Issue 14
- Why this slice: 基于意图评分与行为模式生成下一步行动建议（如跟进时机、推荐材料、建议会议），并展示在 Dashboard 与 Link Detail。
- Acceptance Criteria:
  - [ ] 检测高意图、停滞、异常访问等模式
  - [ ] 生成推荐标题、正文与建议动作
  - [ ] 推荐展示在 Dashboard 与 Link Detail
  - [ ] 用户可 dismiss 或 mark done
- Validation:
  - [ ] 模拟高意图行为后 Dashboard 出现跟进建议
  - [ ] 点击 mark done 后 recommendations 状态更新
- Loop-it notes:
  - Branch hint: feat/issue-28-行动助手推荐
  - Risk class: unknown

### Issue 29：品牌化阅读器
- Source: P1: Branded viewer
- Type: frontend
- Priority: low
- Dependencies: Issue 10
- Why this slice: 允许工作区在文档 viewer 中展示自定义 logo、品牌色与发送方信息，提升专业形象。
- Acceptance Criteria:
  - [ ] 工作区可上传 logo 与设置主色
  - [ ] viewer 顶部栏展示工作区品牌
  - [ ] 品牌设置不遮挡文档内容
  - [ ] 移动端 viewer 同步展示品牌
- Validation:
  - [ ] 配置品牌后 viewer 页面显示自定义 logo
- Loop-it notes:
  - Branch hint: feat/issue-29-品牌化阅读器
  - Risk class: test_failure

### Issue 30：AI 跟进邮件草稿
- Source: P2: AI follow-up drafts
- Type: backend
- Priority: low
- Dependencies: Issue 28
- Why this slice: 根据收件人行为自动生成个性化跟进邮件草稿，供发送方一键复制或编辑后发送。
- Acceptance Criteria:
  - [ ] 基于行为摘要生成邮件主题与正文
  - [ ] 支持创始人/基金/销售三种语气
  - [ ] 用户可在 Link Detail 查看并复制草稿
  - [ ] 草稿明确标注为 AI 生成，需人工审核后发送
- Validation:
  - [ ] 高意图收件人详情页展示可用的跟进邮件草稿
- Loop-it notes:
  - Branch hint: feat/issue-30-ai-跟进邮件草稿
  - Risk class: unknown

### Issue 31：LP 门户
- Source: P2: LP Portal
- Type: fullstack
- Priority: low
- Dependencies: Issue 18, Issue 29
- Why this slice: 为投资机构提供品牌化 LP 门户，LP 可登录查看 fund deck、季度报告、税务文件等聚合材料。
- Acceptance Criteria:
  - [ ] 可创建 LP Update Room 类型的门户
  - [ ] LP 按账户/联系人权限看到不同内容
  - [ ] 门户首页展示最新报告与未读内容
  - [ ] 支持通知 LP 新内容上线
- Validation:
  - [ ] LP 登录门户后可见被授权的报告列表
- Loop-it notes:
  - Branch hint: feat/issue-31-lp-门户
  - Risk class: unknown

### Issue 32：自定义域名
- Source: P2: Custom domain
- Type: infra
- Priority: low
- Dependencies: Issue 10, Issue 31
- Why this slice: 支持工作区绑定自定义域名（如 investor.fund.com），使阅读器与门户展示企业自有域名。
- Acceptance Criteria:
  - [ ] 工作区可配置自定义域名
  - [ ] 提供 DNS 验证指引
  - [ ] viewer 链接可通过自定义域名打开
  - [ ] HTTPS 证书自动申请或支持上传
- Validation:
  - [ ] 配置自定义域名后，Smart Link 可通过该域名访问
- Loop-it notes:
  - Branch hint: feat/issue-32-自定义域名
  - Risk class: unknown

### Issue 33：高级审计导出
- Source: P2: Advanced audit export
- Type: backend
- Priority: low
- Dependencies: Issue 20
- Why this slice: 提供合规级审计导出，包含完整访问日志、IP、设备、下载记录、权限变更等，支持 PDF/CSV。
- Acceptance Criteria:
  - [ ] 可按时间范围导出完整审计日志
  - [ ] 导出包含 IP、设备、邮箱、事件类型、结果
  - [ ] 支持 tamper-evident 摘要或签名（可选）
  - [ ] 导出文件包含工作区与生成时间元数据
- Validation:
  - [ ] 导出审计日志后文件包含所有事件类型
- Loop-it notes:
  - Branch hint: feat/issue-33-高级审计导出
  - Risk class: test_failure

### Issue 34：SSO 单点登录
- Source: P2: SSO
- Type: backend
- Priority: low
- Dependencies: Issue 2
- Why this slice: 支持 SAML / OIDC 单点登录，满足企业客户对工作区成员统一身份管理的需求。
- Acceptance Criteria:
  - [ ] 支持 SAML 2.0 与 OIDC 身份提供商
  - [ ] 管理员可配置 SSO 元数据
  - [ ] SSO 用户首次登录自动加入工作区
  - [ ] 支持强制 SSO 登录
- Validation:
  - [ ] 通过 SSO 登录后成功进入工作区
- Loop-it notes:
  - Branch hint: feat/issue-34-sso-单点登录
  - Risk class: unknown

### Issue 35：SCIM 用户同步
- Source: P2: SCIM
- Type: backend
- Priority: low
- Dependencies: Issue 34
- Why this slice: 提供 SCIM 2.0 接口，允许企业通过身份提供商自动同步用户、分配角色、禁用账户。
- Acceptance Criteria:
  - [ ] 实现 SCIM /Users 与 /Groups 端点
  - [ ] 支持创建、更新、停用用户
  - [ ] 支持通过 group 映射工作区角色
  - [ ] 同步事件记录审计日志
- Validation:
  - [ ] 从 IdP 推送用户后 DealSignal 工作区出现对应成员
- Loop-it notes:
  - Branch hint: feat/issue-35-scim-用户同步
  - Risk class: unknown

### Issue 36：数据保留策略
- Source: P2: Data retention policies
- Type: backend
- Priority: low
- Dependencies: Issue 11, Issue 12
- Why this slice: 允许企业工作区配置数据保留周期，自动清理过期事件、IP 地址、已删除文件等。
- Acceptance Criteria:
  - [ ] 管理员可设置文档、事件、IP 的保留期限
  - [ ] 系统按策略自动匿名化或删除过期数据
  - [ ] 保留策略变更前通知管理员
  - [ ] 支持 GDPR 删除请求工作流
- Validation:
  - [ ] 设置 30 天事件保留后，过期事件被清理
- Loop-it notes:
  - Branch hint: feat/issue-36-数据保留策略
  - Risk class: unknown

### Issue 37：高级工作流自动化
- Source: P3: Advanced workflow automation
- Type: backend
- Priority: low
- Dependencies: Issue 28
- Why this slice: 支持用户自定义触发器与动作，如特定页面访问后自动发送邮件、进入数据室后创建 CRM 任务等。
- Acceptance Criteria:
  - [ ] 可视化或配置化规则编辑器
  - [ ] 支持事件触发器：打开、Hot score、下载、进入 room
  - [ ] 支持动作：发送邮件、创建任务、邀请成员、更新 CRM
  - [ ] 规则执行记录可查询
- Validation:
  - [ ] 配置规则后触发事件自动执行对应动作
- Loop-it notes:
  - Branch hint: feat/issue-37-高级工作流自动化
  - Risk class: unknown

### Issue 38：数据驻留
- Source: P3: Data residency
- Type: infra
- Priority: low
- Dependencies: Issue 1
- Why this slice: 支持企业客户选择数据存储区域（如 US/EU/Asia），满足合规与本地化要求。
- Acceptance Criteria:
  - [ ] 企业工作区可选择数据驻留区域
  - [ ] 文档、事件、数据库按区域隔离
  - [ ] 跨区域访问遵循策略限制
- Validation:
  - [ ] 选择 EU 区域后，该工作区数据存储在 EU
- Loop-it notes:
  - Branch hint: feat/issue-38-数据驻留
  - Risk class: unknown

### Issue 39：深度 BI 报表
- Source: P3: Deep BI reporting
- Type: backend
- Priority: low
- Dependencies: Issue 13, Issue 14
- Why this slice: 提供多维度 BI 报表：内容转化漏斗、团队表现、账户级 engagement、 cohort 分析等，支持导出与嵌入。
- Acceptance Criteria:
  - [ ] 提供漏斗、趋势、对比等报表视图
  - [ ] 支持按时间、segment、内容类型筛选
  - [ ] 支持导出报表为 CSV/PDF
  - [ ] 性能可支持百万级事件
- Validation:
  - [ ] 生成月度内容表现报表并导出
- Loop-it notes:
  - Branch hint: feat/issue-39-深度-bi-报表
  - Risk class: unknown

### Issue 40：SOC 2 支持工作流
- Source: P3: SOC 2 support workflows
- Type: docs
- Priority: low
- Dependencies: Issue 33, Issue 36
- Why this slice: 整理并实施 SOC 2 合规所需的政策、控制、证据收集与审计导出模板。
- Acceptance Criteria:
  - [ ] 制定访问控制、变更管理、事件响应等政策文档
  - [ ] 实现审计日志不可篡改与导出
  - [ ] 建立定期访问复核工作流
  - [ ] 提供审计师只读导出接口
- Validation:
  - [ ] 可生成 SOC 2 所需的审计证据包
- Loop-it notes:
  - Branch hint: feat/issue-40-soc-2-支持工作流
  - Risk class: unknown

### Issue 41：企业 DLP 集成
- Source: P3: Enterprise DLP integrations
- Type: backend
- Priority: low
- Dependencies: Issue 36
- Why this slice: 与常见 DLP/CASB 方案集成，支持内容扫描、敏感数据检测、外发策略联动等企业安全需求。
- Acceptance Criteria:
  - [ ] 提供 API 或 webhook 供 DLP 系统查询/扫描内容
  - [ ] 支持上传前敏感信息扫描
  - [ ] 支持按 DLP 策略阻止下载或分享
  - [ ] 记录 DLP 相关审计事件
- Validation:
  - [ ] 上传含敏感信息文档时触发 DLP 策略
- Loop-it notes:
  - Branch hint: feat/issue-41-企业-dlp-集成
  - Risk class: unknown

### Issue 42：移动端轻量管理后台（Mobile Web Management Lite）
- Source: UI/page-prototypes.md Section 11
- Type: frontend
- Priority: medium
- Dependencies: Issue 15, Issue 17
- Why this slice: 实现发送方在移动设备上的轻量管理界面，包括底部导航、Activity Feed、Hot Signals、Link/Room Summary、Access Requests 和通知设置。复杂的数据室搭建和文档上传仍保留在桌面端。
- Acceptance Criteria:
  - [ ] 底部导航包含 Activity / Hot / Links / Rooms / Me
  - [ ] Activity Feed 展示 first open、repeat open、hot score、forward、access request 等事件
  - [ ] Hot Signals 卡片展示收件人、评分、解释、建议动作
  - [ ] Link Summary 支持复制链接、发送跟进、撤销、打开桌面分析
  - [ ] Room Summary 支持批准访问、查看活跃收件人、打开桌面房间
  - [ ] Access Requests 支持一键批准/拒绝/批准域名
  - [ ] 在 iOS Safari 和 Chrome Android 上验证可用
- Validation:
  - [ ] 在移动端浏览器打开管理后台，Hot Signals 列表正常显示
  - [ ] 点击 Approve 后 access_grant 状态更新为 approved
- Loop-it notes:
  - Branch hint: feat/issue-42-移动端轻量管理后台mobile-web-management-lite
  - Risk class: test_failure

### Issue 43：联系人管理（Contacts + Contact Detail）
- Source: UI/page-prototypes.md Section 15
- Type: frontend
- Priority: medium
- Dependencies: Issue 2, Issue 13
- Why this slice: 实现 Contacts 列表与 Contact Detail 页面，展示投资人/LP/客户/合伙人的互动历史、数据室访问记录、总体热度评分和推荐下一步动作。支持公司与账户级视图。
- Acceptance Criteria:
  - [ ] Contacts 列表展示姓名、邮箱、组织、细分标签、总体评分
  - [ ] 支持按 segment、组织、评分筛选
  - [ ] Contact Detail 展示个人资料、看过的文档、访问过的数据室、时间线
  - [ ] 展示 Overall engagement score 和 Recommended next action
  - [ ] Company/Account Detail 展示关联联系人、账户级评分、相关链接和房间
  - [ ] 支持与 CRM 映射状态联动（P1）
- Validation:
  - [ ] 在浏览器中打开 Contacts 页面可见联系人列表
  - [ ] 点击联系人进入 Detail 后时间线与评分加载正常
- Loop-it notes:
  - Branch hint: feat/issue-43-联系人管理contacts-+-contact-detail
  - Risk class: test_failure

### Issue 44：洞察分析中心（Insights）
- Source: UI/page-prototypes.md Section 16
- Type: frontend
- Priority: medium
- Dependencies: Issue 13, Issue 14
- Why this slice: 实现 Insights 页面，包含 Intent Analytics、Content Performance、Page Performance、Team Performance 和 Risk & Audit 视图，帮助用户优化内容并识别机会与风险。
- Acceptance Criteria:
  - [ ] Intent Analytics 展示 Hot / Warm / Cold 收件人、停滞收件人、活跃度上升的账户
  - [ ] Content Performance 展示 Top converting documents、drop-off pages、最高/最低互动页面
  - [ ] Page Performance 展示每页平均停留时间、重读率、跳出率
  - [ ] Team Performance 展示成员活跃度、发送链接数、产生的高意图信号
  - [ ] Risk and Audit 展示被阻止访问、异常地区、下载事件、撤销/过期链接
  - [ ] 支持按时间范围和 segment 筛选
- Validation:
  - [ ] 打开 Insights 页面可见 Intent Analytics 卡片
  - [ ] 筛选时间范围后图表与表格数据更新
- Loop-it notes:
  - Branch hint: feat/issue-44-洞察分析中心insights
  - Risk class: test_failure

### Issue 45：设置中心（Settings）
- Source: UI/page-prototypes.md Section 18
- Type: frontend
- Priority: medium
- Dependencies: Issue 2
- Why this slice: 实现 Settings 页面，支持工作区配置、成员管理、角色权限、品牌设置、安全默认值、集成连接、账单和数据隐私设置。
- Acceptance Criteria:
  - [ ] Workspace 设置：名称、slug、模式（founder/investment_firm/sales/mixed）
  - [ ] Members 设置：邀请成员、分配角色 owner/admin/member/viewer、移除成员
  - [ ] Branding 设置：上传 logo、设置主色、预览品牌化阅读器
  - [ ] Security defaults：默认访问模式、下载策略、水印策略
  - [ ] Integrations：连接/断开 Slack、HubSpot、Salesforce
  - [ ] Billing：展示当前计划与使用配额（可占位）
  - [ ] Data and privacy：数据保留、删除请求入口
- Validation:
  - [ ] 在浏览器中打开 Settings 可切换各子页面
  - [ ] 修改品牌设置后 viewer 顶部栏显示自定义 logo
- Loop-it notes:
  - Branch hint: feat/issue-45-设置中心settings
  - Risk class: test_failure

### Issue 46：品牌化 LP 门户 UI（LP Portal）
- Source: P2: LP Portal, UI/page-prototypes.md Section 10.3 Mobile Room Viewer
- Type: frontend
- Priority: low
- Dependencies: Issue 18, Issue 29
- Why this slice: 为投资机构实现品牌化 LP 门户界面，LP 登录后可见 fund deck、季度报告、税务文件等聚合材料，支持按 LP 权限展示不同内容。
- Acceptance Criteria:
  - [ ] 门户首页展示工作区品牌、最新报告、未读内容
  - [ ] 按 LP 账户/联系人权限过滤可见房间和文件
  - [ ] 支持文件夹导航和文件搜索
  - [ ] 展示通知和新内容上线提醒
  - [ ] 响应式布局支持桌面和移动端
- Validation:
  - [ ] LP 登录门户后可见被授权的报告列表
  - [ ] 不同 LP 账户看到的内容按权限区分
- Loop-it notes:
  - Branch hint: feat/issue-46-品牌化-lp-门户-uilp-portal
  - Risk class: unknown

## 11. 下游交接

### 11.1 给 /prd-to-spec
- 若后端架构、认证、多租户、对象存储或评分工作队列设计不清晰，先运行 /prd-to-spec。
- 需保留的架构决策：PostgreSQL 15+；blob 用对象存储；只追加事件；到处都要 `workspace_id` 过滤。
- 待解决的技术问题：PDF 渲染策略；评分任务队列；邮件服务商；对象存储服务商。

### 11.2 给 /to-issues
- 以第 10 节为主要来源。
- 保留 Source、Dependencies、Acceptance Criteria、Validation 和 Loop-it notes。
- 未经用户确认，不要从 Creative Space 创建 issue。
- 本地模式默认路径：`.autoresearch/issues`。

### 11.3 给 /loop-it 或 /goal
- 构建顺序：Issue 1 → 2 → 3 → 4 → 6 → 9 → 10 → 11 → 14 → 15，API 可用后穿插 UI issue。
- 不要重新解释这些硬性约束：工作空间隔离、内容前访问控制、只追加事件、不强制注册、即时撤销。
- 安全的实现自由度：UI 文案、动画、具体颜色、空状态插画。
- 出现以下情况需停下来询问：需求改变 P0 闭环；安全控制会阻止合法接收方访问；需要新增依赖。

### 11.4 给 /review-it
- 审查必须验证：所有硬性约束都有测试或手动证据；P0 验收标准通过；阅读器不会泄露已撤销/过期链接的内容；分析事件只追加。
- 以下发现应判定为范围蔓延：把 AI 草稿、SSO、自定义域名或 BI 报告加入 P0；把默认访问模式改为必须注册；移除水印或隐私披露。

### 11.5 给 /note-it 和 /ship-it
- 笔记应记录：与 PRD 的偏差及原因；所选评分算法权重；性能基准；安全决策。
- PR 正文必须包含：关闭哪些 issue；用户可见变更摘要；验证证据（截图、测试输出）；新增环境变量或迁移。

### 11.6 验收脚本

验收脚本 1：在本地开发环境注册工作空间，上传一份 10 页 PDF，创建带邮箱验证 + 水印的智能链接，在隐身浏览器中打开链接，验证邮箱，翻到第 5 页停留 30 秒，确认仪表板在 60 秒内显示页面浏览事件。

验收脚本 2：在 Link Detail 页面点击撤销，然后在另一浏览器打开同一智能链接，验证显示阻止页面且未加载文档字节（检查 Network 标签）。

验收脚本 3：从 Seed Fundraising 模板创建数据室，向 Pitch 文件夹上传 pitch deck，邀请外部邮箱，从该邮箱打开房间，验证文件夹和文件可见且记录了活动事件。

验收脚本 4：在 2 分钟内用同一邮箱模拟重复打开和第 5 页重读，验证意图评分从 Cold 更新到 Warm/Hot，且高分邮件提醒已排队或发送。

验收脚本 5：在移动端视口打开一条有效智能链接，验证阅读器加载，滑动浏览 3 页，确认底部导航和页码指示器可用且无横向滚动。

## 12. 超值交付机会

| 机会 | 工作量 | 价值 | 护栏 |
|---|---|---|---|
| 阅读器键盘快捷键（方向键、Esc 打开目录） | 低 | 重度用户导航更快 | 不得与屏幕阅读器或系统快捷键冲突 |
| Dashboard 导航项上的高意图信号角标 | 低 | 不打断也能吸引注意 | 查看后必须清除 |
| 仪表板一键“发送跟进”，复制建议文案 | 低 | 降低从信号到行动的摩擦 | 不得自动发送邮件，仅复制草稿 |
| 空状态文案解释为何还没有信号 | 低 | 减少新用户困惑 | 必须链接到上传/创建链接 CTA |
| Link Detail 分析的骨架加载状态 | 低 | 感知性能提升 | 不得阻塞真实内容 |
| 引导时预选偏好细分 | 中 | 让仪表板语言立刻相关 | 必须在设置中可更改 |

# PRD：DealSignal — 智能文档分享与交易信号平台 v2.0.7（已归档）

> **归档说明**：本文档为 PRD-v2.0.7 的历史版本，已被冻结。后续所有开发实施、验收测试、上线运维均以 `docs/PRD-v2.1.0.md` 为准。

> **文档编号**：`PRD-2024-001`  
> **版本**：`v2.0.7`  
> **状态**：已归档（已被 PRD-v2.1.0.md 替代）  
> **编写人**：产品团队  
> **编写日期**：`2024-01-15`  
> **最后更新**：`2026-06-18`  
> **评审人**：技术负责人、设计负责人、测试负责人、运营负责人  
> **关联架构图**：`docs/ARCHITECTURE-v1.0.0.md`

---

## 0. 文档使用说明

本文档为 DealSignal 的生产级 PRD，基于 `docs/templates/PRD-template-v2.md` 模板编制，整合：

- `docs/PRD + 产品设计的完整文档草案.md`（产品战略与市场定位）
- `docs/tasks/上传-查看-AI问答设计文.md`（上传、查看、AI 悬浮助手技术设计）

文档面向产品、设计、开发、测试、运营五方协作，所有 FR、AC、EVT、TASK、API、R 编号在本 PRD 内唯一且可追溯。原始草案与设计文档自本文档发布之日起冻结为参考材料。

---

## 1. 文档控制信息

### 1.1 变更日志

| 版本 | 日期 | 修改人 | 修改内容 | 影响范围 |
|------|------|--------|----------|----------|
| v1.0.0 | 2026-06-18 | 产品团队 | 基于原始草案重构为工程化 PRD v1 模板 | 全文档 |
| v2.0.0 | 2026-06-18 | 产品团队 | 按 PRD-template-v2 升级；整合上传/查看/AI 问答设计文档；补充数据架构、接口契约、测试策略、上线运维 | 全文档 |
| v2.0.1 | 2026-06-18 | 产品团队 | PRD 评审后调整：裁剪 CSV 导出；Markdown 文档支持放到三期；Office 转 PDF 选型 OnlyOffice；动态水印采用前端 overlay；多租户隔离采用行级隔离 | 第 4、6、8、10、13、14、18 章 |
| v2.0.2 | 2026-06-18 | 产品团队 | PRD 评审后调整：CSV 导出与 Markdown 完全移出范围；OnlyOffice 明确为自托管；水印截图时不移除；租户隔离改为"子域名 + Workspace"混合模式 | 第 4、6、8、10、13、17、18 章 |
| v2.0.3 | 2026-06-18 | 产品团队 | PRD 评审后调整：明确 slug 为子域名；支持企业自定义域名；Workspace 创建权限为 admin；用户通过 workspace 邀请注册；补充 OnlyOffice 自托管资源规格 | 第 4、6、10、13、18、19 章 |
| v2.0.4 | 2026-06-18 | 产品团队 | PRD 评审后调整：自定义域名 SSL 采用 Let's Encrypt 自动签发；Workspace 切换入口在侧边栏设置子菜单；邀请 token 有效期可配置；OnlyOffice 自托管部署在独立集群 | 第 4、6、10、11、13、17、18 章 |
| v2.0.5 | 2026-06-18 | 产品团队 | PRD 评审后调整：Let's Encrypt 无需速率限制预案；自定义域名采用 CNAME 验证；Workspace 切换后 URL 为 /{workspaceSlug}/...；邀请 token 最大有效期 30 天；OnlyOffice 独立集群通过 VPC 对等连接 | 第 4、6、10、11、13、17、18 章 |
| v2.0.6 | 2026-06-18 | 产品团队 | TDD 技术方案确认后同步：移除 CDN Worker（改为后端签名 URL）；签名 URL token 不再携带 userId；公开链接改为品牌方自定义域名 query 参数形式；消息队列明确为 Go channel（二期引入中间件）；room_members 字段对齐；补充 assistant_sessions / deal_room_documents / room_access_requests / room_member_folder_permissions / document_blocks；统一 Analytics API 路径；明确 API-14 为公开接口 | 第 8、10、12、13、14、19 章 |
| v2.0.7 | 2026-06-19 | 产品团队 | 架构图治理：系统边界图、数据流图迁移至独立 `docs/ARCHITECTURE-v1.0.0.md`，PRD 改为链接引用；避免 PRD 过度膨胀 | 第 1、7、13 章 |

### 1.2 分发与评审记录

| 评审轮次 | 日期 | 参与人 | 结论 | 待办 |
|----------|------|--------|------|------|
| 产品内审 | 2026-06-18 | 产品团队 | 通过 | 无 |
| 技术评审 | 2026-06-18 | 技术负责人 | 通过 | 确认异步任务选型 |
| 设计走查 | 2026-06-18 | 设计负责人 | 通过 | 无 |

### 1.3 关联文档

| 文档类型 | 名称 | 路径/链接 | 说明 |
|----------|------|-----------|------|
| 原始 PRD 草案 | 《PRD + 产品设计的完整文档草案》 | `docs/PRD + 产品设计的完整文档草案.md` | 已冻结 |
| 上传查看 AI 设计 | 《上传-查看-AI问答设计文》 | `docs/tasks/上传-查看-AI问答设计文.md` | 已冻结 |
| v1 PRD | 《PRD-v1.0.0》 | `docs/PRD-v1.0.0.md` | 历史版本 |
| 数据库模型 | 《database-model》 | `docs/database-model.md` | 补充参考 |
| 产品路线图 | 《roadmap-dealsignal-v2》 | `docs/roadmap-dealsignal-v2.md` | 后续迭代参考 |
| 市场策略 | 《go-to-market》 | `docs/go-to-market.md` | 市场参考 |

---

## 2. 执行摘要

**DealSignal** 是一个面向融资创始人、投资机构、B2B 销售团队的智能文档分享与交易信号平台。本次 v2.0.5 交付在 MVP 基础上重点补齐 **文档上传解析、安全查看、AI 悬浮助手问答与自动定位** 三大核心能力，形成"上传 → 解析 → 分享 → 阅读 → 问答 → 洞察 → 跟进"的完整闭环。

- **目标用户**：融资创始人、投资机构 IR、B2B 销售 AE
- **本次交付核心**：
  1. PDF / Office 文档上传与统一解析（Office 转 PDF 采用 OnlyOffice）
  2. 基于 Canvas + 签名 URL 的安全文档查看
  3. AI 悬浮助手问答与自动定位框选
  4. 页面级阅读分析与热度评分
  5. 基础数据室与权限控制
- **预期成果**：上线后 30 天内激活团队 ≥ 100，核心查看链路可用性 ≥ 99.5%，文档解析成功率 ≥ 95%
- **关键风险**：文档转换成功率、AI 回答准确度、第三方对象存储依赖

---

## 3. 背景与战略

### 3.1 市场机会

- 融资、投资、B2B 销售场景中，高价值交易材料（pitch deck、proposal、data room）的分发长期依赖邮件附件、网盘或 DocSend，存在三大痛点：
  1. **不可追踪**：发件人不知道接收方是否真正阅读、关注哪些页面。
  2. **不可控**：材料被随意转发、下载、版本混乱。
  3. **不可行动**：阅读数据无法直接转化为下一步跟进动作。
- AI 与大模型技术成熟后，"基于文档内容的智能问答 + 原文定位"成为新的差异化机会，能显著提升接收方体验与发件人信任感。

### 3.2 用户洞察

| 用户群 | 当前方式 | 痛点 | 机会点 |
|--------|----------|------|--------|
| 融资创始人 | 邮件发 PDF deck | 不知道投资人兴趣、怕泄露、尽调补资料低效 | 阅读热度评分 + 数据室 + AI 问答 |
| 投资机构 IR | 邮件发 LP report | 无法判断 LP 兴趣、合规审计困难 | Engagement dashboard + 审计日志 |
| B2B 销售 | 邮件发 proposal | 不知道客户是否看价格页、跟进时机靠猜 | Deal intent score + Slack/CRM 同步 |

### 3.3 产品假设

| 假设编号 | 假设内容 | 验证方式 | 成功标准 | 负责人 |
|----------|----------|----------|----------|--------|
| H-01 | 融资创始人愿意为"投资人兴趣评分"付费 | 观察 Founder Plan 付费转化率 | 付费转化率 ≥ 5% | 产品 |
| H-02 | AI 悬浮助手问答能提升接收方对文档的信任与理解 | 观察 AI 问答使用率和满意度 | 查看页 AI 打开率 ≥ 20% | 产品 |
| H-03 | 页面级阅读分析 + 热度评分能提高用户 7 日留存 | 对比有无评分功能用户留存 | 7 日留存提升 ≥ 10% | 增长 |
| H-04 | 基础数据室是用户从个人付费升级到团队/企业付费的关键 | 观察数据室创建率与付费升级率 | 数据室创建用户升级率 ≥ 15% | 产品 |

### 3.4 成功标准

| 目标类型 | 目标描述 | 指标 | 基线 | 目标值 | 观察周期 |
|----------|----------|------|------|--------|----------|
| 业务目标 | 验证产品-市场契合度 | 付费团队数 | 0 | ≥ 100 | 上线后 90 天 |
| 业务目标 | 提升 ARPU | MRR | 0 | ≥ $10K | 上线后 90 天 |
| 用户目标 | 发件人能识别高意向对象 | 周活跃查看分析用户数占比 | - | ≥ 70% | 上线后 30 天 |
| 产品目标 | 文档上传-查看-分析核心链路稳定 | 文档解析成功率 | - | ≥ 95% | 上线后 14 天 |
| 技术目标 | 核心查看链路高可用 | 可用性 | - | ≥ 99.5% | 持续 |
| 产品目标 | AI 问答功能被接受 | AI 问答打开率 | - | ≥ 20% | 上线后 30 天 |

---

## 4. 范围与边界

### 4.1 版本范围

- **版本号**：`v2.0.5`
- **迭代周期**：`2026-07-01` 至 `2026-09-30`
- **发布目标**：内测 → 灰度 → 全量

### 4.2 In Scope

1. PDF / Office 文档上传与统一解析（Office 转 PDF 采用 OnlyOffice）
2. 文档页渲染为 webp，基于 Canvas 的安全查看
3. AI 悬浮助手问答与自动定位框选
4. 智能链接生成与权限控制（邮箱验证、白名单、密码、过期、下载控制、水印、撤回）
5. 页面级阅读分析与访问者行为记录
6. 0-100 热度评分（Investor Intent / LP Engagement / Deal Intent）
7. 基础数据室（文件夹、权限、访问日志、NDA gating、Q&A）
8. 邮件通知与行为提醒
9. 品牌化分享页

### 4.3 Out of Scope

| 功能点 | 原因 | 计划处理方式 |
|--------|------|--------------|
| AI 自动改 deck / 生成完整材料 | 超出本次 MVP 范围 | 二期探索 |
| 完整电子签与复杂合同协作 | 需单独合规与法务评估 | 后续立项 |
| 原生视频会议 | 非核心能力 | 集成第三方 |
| 复杂 BI 报表与企业级 DLP | 数据量不足 | 二期 |
| 法律级 DRM 保护 | 成本与合规门槛高 | 企业版考虑 |
| 自建邮件群发系统 | 使用现有邮件服务 | 不建设 |
| CSV 导出 | 非核心功能，完全不做 | 以后均不支持 |
| Markdown 文档上传与解析 | 产品定位不支持 Markdown 源文件 | 以后均不支持，用户可转 PDF 上传 |
| 多语言、数据驻留、SSO/SCIM | 企业版需求 | 二期 |

### 4.4 假设与依赖

| 假设/依赖 | 类型 | 说明 | 风险等级 | 备选方案 |
|-----------|------|------|----------|----------|
| 用户愿意上传 pitch deck 等敏感材料 | 业务假设 | 基于访谈结论 | 中 | 提供沙箱试用与权限粒度控制 |
| OnlyOffice 自托管实例可用 | 内部依赖 | 用于 Office 转 PDF | 中 | 接入 LibreOffice 或自研降级 |
| Embedding 服务可用 | 外部依赖 | OpenAI / 自托管 embedding | 中 | 本地 embedding 模型降级 |
| 对象存储与 CDN 可用 | 外部依赖 | AWS S3 / CloudFront 或阿里云 | 低 | 多云备份 |
| 法务确认隐私政策 | 内部依赖 | 需隐私政策与数据处理协议 | 中 | 提前准备 DPA 模板 |
| 子域名解析与 SSL 证书管理可用 | 外部依赖 | 用于 `{slug}.dealsignal.com` 路由与 HTTPS；Let's Encrypt 自动签发 | 低 | 路径 slug 模式兜底 |
| 自定义域名 CNAME 验证可用 | 外部依赖 | 企业客户添加 CNAME 记录验证域名所有权 | 低 | 暂不支持自定义域名 |
| Workspace 切换状态保持 | 内部依赖 | 用户可在多个 Workspace 间切换；URL 路径为 `/{workspaceSlug}/...` | 低 | 通过 header 携带 workspace_id 兜底 |

### 4.5 非目标

- 不追求 100% 文档格式完美解析，允许复杂版式存在偏差并后续优化。
- 本次及以后均不支持 Markdown 文档上传，仅支持 PDF / Office。
- 本次及以后均不支持 CSV 导出。
- 不追求 AI 问答 100% 准确，优先保证可解释性与原文引用。
- 不追求企业级 SSO/SCIM，本次仅支持邮箱注册与团队邀请。

---

## 5. 用户与用户旅程

### 5.1 用户画像

| 角色 | 身份描述 | 核心目标 | 当前痛点 | 使用频率 |
|------|----------|----------|----------|----------|
| 融资创始人 | Seed / Series A 创始人、CEO/CFO/COO | 识别投资人兴趣、控制融资叙事 | 不知道谁真的在看、材料易泄露 | 每周 |
| 投资机构 IR | VC/PE/Fund IR、LP Relations | 安全分发资本材料、判断 LP 兴趣 | 无法追踪 LP engagement、审计难 | 每月 |
| B2B 销售 AE | AE / SDR / Sales Manager | 判断客户购买意图、把握跟进时机 | 不知道客户是否看价格页 | 每周 |

### 5.2 Jobs-to-be-Done

| 角色 | When I want to | So I can | 当前替代方案 |
|------|----------------|----------|--------------|
| 融资创始人 | 发送 pitch deck 并知道投资人兴趣 | 优先跟进高意向投资人 | 邮件 + 网盘 + 凭感觉 |
| 投资机构 IR | 给 LP 发季度报告并追踪阅读 | 安排精准跟进 | 邮件 + Excel 手动记录 |
| B2B 销售 AE | 发送 proposal 并识别成交信号 | 在合适时机推进交易 | 邮件 + CRM 手工更新 |

### 5.3 关键用户路径

#### 路径 A：创始人上传 deck 并追踪投资人兴趣

```text
登录 Dashboard
    ↓
上传 PDF pitch deck
    ↓
系统异步解析生成页面 webp 与 chunks
    ↓
创建智能链接（邮箱验证 + 动态水印 + 可下载）
    ↓
发送给投资人
    ↓
投资人打开链接 → Canvas 查看
    ↓
系统记录阅读行为与热度评分
    ↓
创始人收到"高意图"提醒与跟进建议
    ↓
发送 follow-up 邮件 / 邀请进入数据室
```

**对应验收**：AC-01 ~ AC-05, AC-09 ~ AC-12

#### 路径 B：接收方使用 AI 悬浮助手问答并定位原文

```text
用户打开文档查看页
    ↓
看到右下角悬浮 AI 图标
    ↓
点击图标展开 AI 对话框
    ↓
输入问题（如"付款期限是多少"）
    ↓
系统执行 hybrid search 获取 evidence
    ↓
AI 生成回答并返回引用
    ↓
默认高亮 top1 evidence
    ↓
Canvas 自动跳转到目标页并绘制高亮框
    ↓
用户可切换其他引用查看不同位置
```

**对应验收**：AC-13 ~ AC-17

#### 路径 C：销售创建 proposal 并识别成交信号

```text
从内容库选择 proposal 模板
    ↓
个性化客户名称与报价
    ↓
生成客户专属链接
    ↓
发送给 champion
    ↓
系统检测内部转发给多人
    ↓
多人查看价格页与安全页
    ↓
Slack 通知 AE，系统自动建议安排会议
    ↓
CRM 自动更新 deal stage
```

**对应验收**：AC-18 ~ AC-20

#### 路径 D：通过 Workspace 邀请注册

```text
admin 在 Workspace 中输入被邀请人邮箱
    ↓
系统发送邀请邮件，内含注册链接（携带 workspace_id 与邀请 token）
    ↓
被邀请人点击链接进入注册页
    ↓
填写邮箱、密码完成注册
    ↓
系统自动将该用户加入对应 Workspace，角色为 member
    ↓
用户进入 Workspace Dashboard
```

**对应验收**：AC-31

---

## 6. 产品原则与设计约束

### 6.1 产品原则

1. **先减少不确定性，再增加控制**：用户真正买的是判断对方兴趣，而不是复杂权限面板。
2. **安全不能以牺牲成交为代价**：如果权限设计让投资人、LP、客户打不开，安全功能反而破坏交易。
3. **分析必须导向行动**："谁看了"只是数据，"现在该跟进谁、说什么"才是价值。
4. **AI 回答必须有据可查**：每个 AI 回答都要能定位到原文，增强可信度。
5. **接收方体验是增长飞轮**：每个接收方都是潜在下一位发件人，Viewer 页面必须专业、顺滑、可信。
6. **权限默认低摩擦，高敏感才强验证**：不强制接收方注册，但高敏感资料可启用 NDA / 白名单 / 水印。

### 6.2 设计约束

- 必须使用统一设计系统，保持 Dashboard、Viewer、AI 助手视觉一致。
- 必须支持响应式布局，移动端可完成核心查看操作。
- 必须为每个页面提供加载态、空态、错误态、权限不足态。
- 关键操作（删除、撤回、权限变更）需二次确认。
- AI 助手对话框默认收起，不遮挡文档主要内容。

### 6.3 技术约束

- 文档解析统一走"转 PDF → 提取 bbox → 渲染 webp" pipeline，前端基于 Canvas 绘制。
- 对象存储中的 page webp 必须通过后端签名的 CDN URL + Cloudflare URL Signing 鉴权访问，禁止直接公开读取。
- 搜索支持 exact + full-text + vector 三种模式，最终 hybrid 合并。
- 异步任务（ingestion、embedding、邮件发送）必须可重试、可观测。
- 采用"子域名隔离为主、独立 Workspace 隔离为辅"的混合模式：
  - 对外：每个租户分配不可变内部 ID（UUID），同时分配唯一 slug 作为子域名，如 `acme.dealsignal.com`。子域名由租户自定义申请，全局唯一，申请成功后不会冲突。
  - 路由关系：子域名 slug 在网关层解析为 tenant UUID，后端所有请求均携带 `tenant_id`。
  - 自定义域名：企业客户可绑定自有域名（如 `investor.fund.com`），通过 CNAME 记录指向 CDN，网关根据域名查找对应 tenant。SSL 证书由 Let's Encrypt 自动签发并续期（速率限制不构成瓶颈，无需额外预案）。
  - 对内：同一用户可属于多个 Workspace，权限各自独立；切换 Workspace 后 URL 路径变为 `/{workspaceSlug}/...`，页面数据刷新为当前 Workspace。
- 所有业务表必须包含 `tenant_id` 与 `workspace_id`；所有查询必须同时带 `tenant_id` 与 `workspace_id` 过滤。
- Workspace 创建权限：tenant admin（tenant owner 默认拥有 admin 权限）。
- Workspace 切换入口：侧边栏 Settings 子菜单；切换后 URL 跳转到 `/{workspaceSlug}/...`。
- 用户注册：通过 workspace 邀请链接注册，邀请 token 有效期可配置（默认 7 天，最大 30 天）；注册后自动加入对应 workspace，角色为 member（可升级为 admin）。

### 6.4 合规约束

- 遵循 GDPR / CCPA / 数据安全法要求。
- 明确告知接收方数据追踪范围，并提供隐私说明入口。
- 不向第三方出售接收方数据。
- 支持用户数据导出与删除请求。

---

## 7. 解决方案概述

### 7.1 价值主张

**DealSignal 帮助融资创始人、投资机构 IR 和 B2B 销售通过安全可控的智能文档分享、页面级阅读分析与 AI 原文定位问答，把每一份关键文档变成可追踪、可推进成交的交易信号。**

### 7.2 核心能力

| 能力 | 描述 | 解决的问题 | 对应模块 |
|------|------|------------|----------|
| 多格式文档解析 | 支持 PDF / Office 统一转 PDF 后渲染 webp | 文档格式不统一、预览体验差 | 上传与解析 |
| 安全文档查看 | Canvas 加载签名 webp，支持权限、水印、下载控制 | 材料泄露风险 | 查看与渲染 |
| AI 悬浮助手问答 | 基于 hybrid search 与 evidence 回答并定位原文 | 接收方快速理解内容、发件人提升信任 | AI 助手 |
| 智能链接与权限 | 邮箱验证、白名单、密码、过期、水印、撤回 | 访问控制与合规 | 链接与权限 |
| 意图分析 | 页面级行为记录、热度评分、关键页识别 | 判断接收方兴趣 | 分析 |
| 数据室 | 多文件、多权限、访问日志、NDA、Q&A | 尽调与资料分发 | 数据室 |

### 7.3 系统边界

> 完整系统架构图与部署拓扑图已迁移至 `docs/ARCHITECTURE-v1.0.0.md` 第 2、3 章。
>
> 参见：[ARCHITECTURE-v1.0.0.md#2-系统架构图](../ARCHITECTURE-v1.0.0.md#2-系统架构图)

DealSignal 平台由以下前端入口、业务服务、AI/Worker 服务、共享基础设施与外部依赖组成：

### 7.4 模块边界

| 模块 | 职责 | 边界说明 | 优先级 |
|------|------|----------|--------|
| Upload Service | 文件上传、校验、hash、创建 ingestion job | 不处理文档内容解析 | P0 |
| Ingestion Worker | 调度 PDF / Office pipeline，生成 webp / chunks / boxes | 不处理用户权限与链接管理 | P0 |
| Viewer Frontend | Canvas 渲染页面、悬浮 AI、自动定位、高亮框 | 不直接访问私有存储，通过签名 URL | P0 |
| Search Service | exact / full-text / vector / hybrid search | 不生成自然语言回答 | P0 |
| Evidence Service | quote + page + bbox 聚合 | 不直接调用 LLM | P0 |
| Assistant Service | 基于 evidence 生成回答 | 不替代用户做投资决策 | P0 |
| Link & Permission | 链接创建、权限校验、访问日志 | 不处理文档解析 | P0 |
| Intent Analytics | 行为记录、热度评分、洞察 | 不直接修改文档内容 | P0 |
| Deal Room | 多文件数据室、权限、Q&A | 复用 Viewer 与 Search | P0 |
| CRM/Integration | 第三方同步 | 不处理核心阅读体验 | P1 |

---

## 8. 功能需求

### 8.1 功能需求总览

| 模块 | FR 编号 | 功能名称 | 优先级 | 状态 |
|------|---------|----------|--------|------|
| 上传与解析 | FR-01 | 文档上传与校验 | P0 | 待开发 |
| 上传与解析 | FR-02 | 异步文档解析 pipeline | P0 | 待开发 |
| 查看与渲染 | FR-03 | 签名 URL 与 Canvas 页面渲染 | P0 | 待开发 |
| 查看与渲染 | FR-04 | 阅读进度与行为记录 | P0 | 待开发 |
| AI 助手 | FR-05 | 悬浮 AI 助手问答 | P0 | 待开发 |
| AI 助手 | FR-06 | 搜索结果自动定位与高亮 | P0 | 待开发 |
| 链接与权限 | FR-07 | 智能链接创建 | P0 | 待开发 |
| 链接与权限 | FR-08 | 权限控制与访问校验 | P0 | 待开发 |
| 链接与权限 | FR-09 | 动态水印 | P1 | 待开发 |
| 意图分析 | FR-10 | 热度评分 | P0 | 待开发 |
| 意图分析 | FR-11 | 行为提醒与跟进建议 | P0 | 待开发 |
| 数据室 | FR-12 | 数据室创建与管理 | P0 | 待开发 |
| 数据室 | FR-13 | 数据室权限与访问审批 | P0 | 待开发 |
| 通知 | FR-14 | 邮件通知系统 | P0 | 待开发 |
| 集成 | FR-15 | CRM 同步 | P1 | 待开发 |
| 集成 | FR-16 | Slack 集成 | P1 | 待开发 |

### 8.2 功能需求详情

#### 8.2.1 模块：上传与解析

##### FR-01：文档上传与校验

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 系统允许用户上传 PDF、DOCX、PPTX、XLSX 文件。上传后校验文件类型、大小、hash，写入私有对象存储，并创建 documents 记录与 ingestion_job。 |
| **用户价值** | 发件人能快速上传并分享各类交易材料。 |
| **前置条件** | 用户已登录；文件类型在允许列表内；租户存储配额未超限。 |
| **后置条件** | documents 状态为 UPLOADED 或 PROCESSING；文件写入对象存储；创建异步 ingestion job。 |
| **业务规则** | 单个文件大小上限 100MB；支持批量上传；相同 source_hash 在同一租户下去重；非法类型/超大文件拒绝并提示原因。 |
| **输入/输出** | **输入**：文件二进制、文件名、`source_type`（可选）<br>**输出**：`document_id`、`status`、`created_at` |
| **异常处理** | 格式不支持/大小超限/网络错误时返回明确错误码；上传中断支持断点续传或重试。 |
| **性能要求** | 上传 API 响应 ≤ 2s（P99）；支持并发上传。 |
| **安全要求** | 上传需鉴权；文件写入私有 bucket；文件 hash 用于去重与完整性校验。 |
| **关联接口** | API-01 |
| **关联验收** | AC-01、AC-02 |
| **关联埋点** | EVT-01 |
| **依赖项** | 无 |

##### FR-02：异步文档解析 pipeline

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 系统异步处理上传的文档：PDF 直接提取语义与 bbox；Office 使用自托管 OnlyOffice 实例转 PDF，再提取语义与 bbox；最终渲染每页为 webp，生成 chunks、normalized_text、search_vector、embedding，写入 pages / blocks / chunks / boxes。 |
| **用户价值** | 支持多格式文档统一预览与 AI 搜索。 |
| **前置条件** | 文档已上传至对象存储；ingestion_job 已创建。 |
| **后置条件** | documents.status = READY；pages / chunks / boxes 数据完整；pgvector 索引更新。 |
| **业务规则** | PDF pipeline：提取结构语义 + PDF 文本层 bbox + webp 渲染；Office pipeline：原始 Office 语义提取 + 自托管 OnlyOffice 转 PDF + PDF 文本层 bbox + webp 渲染；语义块与视觉文本对齐后生成 chunks 与 boxes。 |
| **输入/输出** | **输入**：`document_id`、原始文件 object key<br>**输出**：`pages` 元数据、`chunks`、`boxes`、`document_files` 记录 |
| **异常处理** | 解析失败时 documents.status = FAILED，记录错误原因，通知上传者，支持重新触发解析。 |
| **性能要求** | 50 页 PDF 解析完成 ≤ 3 分钟（P95）；30 页 Office 解析完成 ≤ 5 分钟（P95）；队列延迟 ≤ 30 秒。 |
| **安全要求** | 解析过程在隔离 worker 中执行；原始文件与渲染文件均存私有存储。 |
| **关联接口** | API-02 |
| **关联验收** | AC-03、AC-04 |
| **关联埋点** | EVT-02、EVT-03 |
| **依赖项** | FR-01 |

#### 8.2.2 模块：查看与渲染

##### FR-03：签名 URL 与 Canvas 页面渲染

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 后端校验文档权限后返回 page 元数据 + 签名 CDN 临时 URL；前端 Canvas 加载 webp 并绘制；Cloudflare URL Signing 验证签名（含 tenantId / workspaceId / documentId / pageNumber / purpose / expiresAt）后从私有对象存储读取图片。 |
| **用户价值** | 接收方无需注册即可流畅查看文档，同时保障文件不被盗链。 |
| **前置条件** | 文档状态为 READY；访问者通过权限校验。 |
| **后置条件** | 返回签名 URL；Canvas 渲染页面；记录阅读进度。 |
| **业务规则** | token 有效期默认 15 分钟；Cloudflare 必须校验签名与过期时间；对象存储不直接对外暴露。 |
| **输入/输出** | **输入**：`documentId`、`pageNumber`、访问者 token<br>**输出**：page 元数据 + 签名 imageUrl |
| **异常处理** | token 过期/签名错误返回 403；页面未生成返回 404 并提示处理中；加载失败允许重试。 |
| **性能要求** | 签名 URL 生成 ≤ 200ms；页面首图加载 ≤ 1.5s（P95）。 |
| **安全要求** | token 签名不可篡改；URL 仅限单页访问；禁止长期有效 URL；签名 URL 不携带用户/访客身份。 |
| **关联接口** | API-03、API-04 |
| **关联验收** | AC-05、AC-06 |
| **关联埋点** | EVT-04 |
| **依赖项** | FR-02 |

##### FR-04：阅读进度与行为记录

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 系统记录访问者每次打开、页面切换、停留时长、滚动深度、下载、转发行为，并关联到访问者与链接。 |
| **用户价值** | 为热度评分与跟进建议提供数据基础。 |
| **前置条件** | 访问者已加载 Viewer 页面。 |
| **后置条件** | 行为事件写入事件流；更新分析聚合数据。 |
| **业务规则** | 事件延迟 < 10 秒；同一访问者连续刷新去重；最小有效停留时长 2 秒。 |
| **输入/输出** | **输入**：客户端事件 payload<br>**输出**：事件确认 ack |
| **异常处理** | 网络异常时本地缓存并恢复后补发；失败事件记录日志。 |
| **性能要求** | 事件上报接口 P99 ≤ 100ms。 |
| **安全要求** | 事件需关联合法访问会话；禁止伪造其他访问者事件。 |
| **关联接口** | API-05 |
| **关联验收** | AC-07、AC-08 |
| **关联埋点** | EVT-05、EVT-06 |
| **依赖项** | FR-03 |

#### 8.2.3 模块：AI 悬浮助手

##### FR-05：悬浮 AI 助手问答

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 接收方在 Viewer 页面右下角看到悬浮 AI 图标，点击后展开对话框；用户输入问题，系统调用 Search API 执行 hybrid search，Evidence Service 聚合 quote / page / bbox，Assistant Service 基于 evidence 生成回答并返回 answer + results + evidence。 |
| **用户价值** | 接收方能快速从文档中找到答案，提升对文档的信任与理解。 |
| **前置条件** | 文档已解析完成且有 chunks；Viewer 页面已加载。 |
| **后置条件** | 返回带引用的 AI 回答；记录问答会话。 |
| **业务规则** | 搜索支持 exact + full-text + vector，经 RRF 合并与可选 reranker；未找到相关内容时明确提示；每个回答必须附带 evidence（quote、page、bbox）。 |
| **输入/输出** | **输入**：`documentId`、`query`、会话上下文<br>**输出**：`answer`、`results[]`、`evidence[]` |
| **异常处理** | 搜索失败返回友好提示；LLM 不可用时返回"服务暂不可用"；无引用结果时拒绝编造。 |
| **性能要求** | 问答响应 ≤ 3s（P95）；搜索阶段 ≤ 800ms。 |
| **安全要求** | 只能搜索当前用户有权限的文档；租户数据隔离。 |
| **关联接口** | API-06、API-07 |
| **关联验收** | AC-13、AC-14 |
| **关联埋点** | EVT-07、EVT-08 |
| **依赖项** | FR-02 |

##### FR-06：搜索结果自动定位与高亮

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | AI 回答返回 evidence 后，系统默认选择 top1 evidence，读取 documentId / pageNumber / bbox，将 PAGE_IMAGE_NORMALIZED 坐标换算为 Canvas 像素坐标，自动跳转目标页并绘制高亮框与 pulse 动效；用户点击其他引用时切换高亮。 |
| **用户价值** | 用户能直观看到答案在原文中的位置，增强可信度。 |
| **前置条件** | AI 回答返回有效 evidence；Canvas 可获取当前渲染尺寸。 |
| **后置条件** | 页面自动跳转；高亮框 overlay 绘制；用户可点击其他引用切换。 |
| **业务规则** | bbox 使用 PAGE_IMAGE_NORMALIZED 坐标；换算公式：`left = rect.left + box.x * rect.width`；高亮框支持 pulse 动画；多个 evidence 在同一页时按顺序切换。 |
| **输入/输出** | **输入**：`evidence.pageNumber`、`evidence.boxes[]`、Canvas render rect<br>**输出**：Canvas overlay 高亮框 |
| **异常处理** | 目标页未加载时请求签名 URL 并切换；bbox 异常时忽略该引用并提示。 |
| **性能要求** | 页面切换 + 高亮绘制 ≤ 500ms。 |
| **安全要求** | 仅高亮当前文档中有权限查看的页面。 |
| **关联接口** | API-04 |
| **关联验收** | AC-15、AC-16 |
| **关联埋点** | EVT-09 |
| **依赖项** | FR-03、FR-05 |

#### 8.2.4 模块：智能链接与权限

##### FR-07：智能链接创建

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 用户可为每个文档创建一个或多个独立分享链接，配置链接名称、权限策略、过期时间、最大访问次数、下载开关、水印开关。 |
| **用户价值** | 发件人能按需控制文档分发方式。 |
| **前置条件** | 文档状态为 READY。 |
| **后置条件** | 生成唯一短链接；记录链接配置。 |
| **业务规则** | 同一文档可创建多个链接；每个链接独立配置；支持启用/禁用/删除。 |
| **输入/输出** | **输入**：`documentId`、权限配置、过期时间等<br>**输出**：`linkId`、短链接 URL |
| **异常处理** | 文档未 READY 时禁止创建；配置非法返回明确错误。 |
| **性能要求** | 创建链接 API P99 ≤ 300ms。 |
| **安全要求** | 链接创建者必须拥有文档权限；链接 ID 不可预测。 |
| **关联接口** | API-08 |
| **关联验收** | AC-17 |
| **关联埋点** | EVT-10 |
| **依赖项** | FR-02 |

##### FR-08：权限控制与访问校验

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 系统支持公开访问、邮箱验证、指定邮箱白名单、密码访问、过期时间、最大访问次数、禁止下载、一键撤回；访问时校验权限策略并记录访问日志。 |
| **用户价值** | 发件人能保护敏感材料，同时降低接收方 friction。 |
| **前置条件** | 链接已创建。 |
| **后置条件** | 访问者满足策略后查看内容；不满足时展示拦截页面；记录访问尝试。 |
| **业务规则** | 默认不强制登录；白名单支持邮箱/域名；过期/达到上限/撤回后拒绝访问；权限变更实时生效。 |
| **输入/输出** | **输入**：`linkId`、访问者邮箱/密码/token<br>**输出**：访问结果或拦截原因 |
| **异常处理** | 无权限/过期/达到上限/已撤回均返回明确状态码与文案；提供联系发件人入口。 |
| **性能要求** | 权限校验 P99 ≤ 50ms。 |
| **安全要求** | 密码哈希存储；访问日志不可篡改；防止暴力破解密码。 |
| **关联接口** | API-09 |
| **关联验收** | AC-18、AC-19、AC-20 |
| **关联埋点** | EVT-11 |
| **依赖项** | FR-07 |

##### FR-09：动态水印

| 字段 | 内容 |
|------|------|
| **优先级** | P1 |
| **需求描述** | 系统在 Canvas 查看页通过前端 overlay 叠加访问者邮箱、访问时间、IP 等动态水印。 |
| **前置条件** | 链接已启用水印；访问者身份已被识别；Canvas 页面已渲染。 |
| **后置条件** | 每页预览显示与该访问者关联的半透明水印 overlay。 |
| **业务规则** | 水印使用前端 Canvas 绘制，位置随机或固定；不影响核心内容阅读；截图时水印仍保留在画面中（因水印是 Canvas 绘制的一部分）；截图泄露时可追溯到人；水印信息随签名 URL 或页面元数据下发，前端不可伪造。 |
| **输入/输出** | **输入**：访问者邮箱、时间、IP（来自后端签名或页面接口）<br>**输出**：Canvas overlay 水印 |
| **异常处理** | 身份未识别时显示匿名标识或拒绝访问；Canvas 不支持时 fallback 为 DOM 水印层。 |
| **性能要求** | 水印绘制在首屏渲染后 100ms 内完成，不影响交互。 |
| **安全要求** | 水印元信息来自后端签权，前端通过 Canvas 绘制；禁止纯前端生成可篡改的水印文本。 |
| **关联接口** | API-04 |
| **关联验收** | AC-21 |
| **关联埋点** | - |
| **依赖项** | FR-03、FR-08 |

#### 8.2.5 模块：意图分析

##### FR-10：热度评分

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 系统基于阅读行为生成 0-100 的 intent score，并自动分类 Hot / Warm / Cold。三类用户分别对应 Investor Intent Score、LP / Buyer Engagement Score、Deal Intent Score。 |
| **用户价值** | 发件人快速识别高意向对象，优先跟进。 |
| **前置条件** | 已积累至少一次有效打开事件。 |
| **后置条件** | Dashboard 展示热度分层；触发高意图提醒。 |
| **业务规则** | 评分综合考虑打开次数、阅读时长、关键页停留、回访、转发、下载；不同角色使用不同权重与关键页定义。 |
| **输入/输出** | **输入**：访问行为数据<br>**输出**：`score`、`tier`、`factors[]` |
| **异常处理** | 数据不足时显示"数据采集中"；模型异常 fallback 到简单加权。 |
| **性能要求** | 评分更新延迟 ≤ 1 分钟。 |
| **安全要求** | 用户只能查看自己有权限的文档评分。 |
| **关联接口** | API-10 |
| **关联验收** | AC-22 |
| **关联埋点** | EVT-12 |
| **依赖项** | FR-04 |

##### FR-11：行为提醒与跟进建议

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 系统在首次打开、重复查看关键页、多人转发、异常访问等事件发生时通知发件人，并基于行为信号生成跟进建议与邮件草稿。 |
| **用户价值** | 发件人把数据转化为可执行动作。 |
| **前置条件** | 链接已启用提醒；事件已发生。 |
| **后置条件** | 发送邮件通知；Dashboard 展示待跟进对象；生成建议。 |
| **业务规则** | 用户可配置提醒频率与事件类型；同类事件合并；创始人场景关注财务页/团队页；销售场景关注价格页/安全页；基金场景关注 report 高频查看。 |
| **输入/输出** | **输入**：事件数据<br>**输出**：通知、建议文案、邮件草稿 |
| **异常处理** | 邮件发送失败重试；用户可关闭提醒。 |
| **性能要求** | 事件触发到通知发送 ≤ 5 分钟。 |
| **安全要求** | 通知内容不包含敏感文档正文。 |
| **关联接口** | API-11 |
| **关联验收** | AC-23、AC-24 |
| **关联埋点** | EVT-13 |
| **依赖项** | FR-10 |

#### 8.2.6 模块：数据室

##### FR-12：数据室创建与管理

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 用户可以创建多文件数据室，支持文件夹结构、批量上传、批量权限设置、访问审批、活动日志。提供 Seed / Series A / LP Update / Sales Proposal 等模板。 |
| **用户价值** | 支持尽调、LP 更新、销售提案等复杂资料分发。 |
| **前置条件** | 用户已登录；已上传或选择待加入数据室的文档。 |
| **后置条件** | 数据室可访问；按访问者/组织设置权限；记录所有访问与操作。 |
| **业务规则** | 支持 NDA gating；支持 Q&A 与请求资料清单；复用 Viewer 与 Search 能力。 |
| **输入/输出** | **输入**：模板类型、文档列表、权限配置<br>**输出**：`roomId`、数据室结构 |
| **异常处理** | 审批被拒绝时通知双方；权限变更后已打开会话按新权限生效。 |
| **性能要求** | 数据室创建 ≤ 2s；列表查询 ≤ 300ms。 |
| **安全要求** | 数据室访问权限严格隔离；活动日志不可删除。 |
| **关联接口** | API-12、API-13 |
| **关联验收** | AC-25 |
| **关联埋点** | EVT-14 |
| **依赖项** | FR-02、FR-07 |

##### FR-13：数据室权限与访问审批

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 数据室支持按投资人、基金、组织设置权限；访问者需申请并被审批后才能进入；支持 NDA 确认。 |
| **用户价值** | 保护敏感尽调资料。 |
| **前置条件** | 数据室已创建。 |
| **后置条件** | 审批通过后授予访问权限；记录审批链。 |
| **业务规则** | 角色包括 OWNER（数据室创建者）、ADMIN、CONTRIBUTOR、VIEWER；`pending` 为申请状态而非角色；NDA 未确认前限制访问；支持批量审批。 |
| **输入/输出** | **输入**：申请者邮箱、申请理由<br>**输出**：审批结果通知 |
| **异常处理** | 审批超时自动提醒 owner；拒绝时说明原因。 |
| **性能要求** | 审批通知发送 ≤ 1 分钟。 |
| **安全要求** | 审批操作记录审计日志；防止越权审批。 |
| **关联接口** | API-14 |
| **关联验收** | AC-26 |
| **关联埋点** | EVT-15 |
| **依赖项** | FR-12 |

#### 8.2.7 模块：通知

##### FR-14：邮件通知系统

| 字段 | 内容 |
|------|------|
| **优先级** | P0 |
| **需求描述** | 系统通过邮件发送首次打开、重复查看、多人转发、异常访问、数据室审批等通知。 |
| **用户价值** | 发件人及时获知关键事件。 |
| **前置条件** | 事件已发生；用户已开启对应提醒。 |
| **后置条件** | 邮件进入发送队列；发送状态可追踪。 |
| **业务规则** | 支持按事件类型配置；同类事件合并；提供退订入口。 |
| **输入/输出** | **输入**：事件类型、收件人、上下文<br>**输出**：发送状态 |
| **异常处理** | 发送失败重试 3 次；硬退回记录并告警。 |
| **性能要求** | 事件到邮件入队 ≤ 30s；邮件服务商发送延迟按第三方计。 |
| **安全要求** | 邮件内容不包含敏感正文；退订链接有效。 |
| **关联接口** | - |
| **关联验收** | AC-27 |
| **关联埋点** | EVT-16 |
| **依赖项** | 消息队列 |

#### 8.2.8 模块：集成

##### FR-15：CRM 同步

| 字段 | 内容 |
|------|------|
| **优先级** | P1 |
| **需求描述** | 系统支持将文档事件同步到 HubSpot 或 Salesforce，包括创建联系人、写入 timeline、更新 deal stage、创建 follow-up task。 |
| **用户价值** | 销售团队无需手动更新 CRM。 |
| **前置条件** | 用户已完成 CRM 授权与映射配置。 |
| **后置条件** | 事件写入 CRM；高意图事件创建 task。 |
| **业务规则** | 支持字段映射；失败进入重试队列。 |
| **输入/输出** | **输入**：事件 payload、映射配置<br>**输出**：同步状态 |
| **异常处理** | 授权失效通知重新授权；同步失败记录日志。 |
| **性能要求** | 同步延迟 ≤ 1 小时。 |
| **安全要求** | token 加密存储；仅同步授权范围内的数据。 |
| **关联接口** | API-15 |
| **关联验收** | AC-28 |
| **关联埋点** | EVT-17 |
| **依赖项** | FR-04 |

##### FR-16：Slack 集成

| 字段 | 内容 |
|------|------|
| **优先级** | P1 |
| **需求描述** | 系统支持高意图事件触发 Slack 提醒，并可配置推送规则与频道。 |
| **用户价值** | 团队实时感知关键交易信号。 |
| **前置条件** | 用户已绑定 Slack workspace。 |
| **后置条件** | 事件消息推送到指定频道。 |
| **业务规则** | 仅推送 Hot / 异常访问；支持按用户/团队配置。 |
| **输入/输出** | **输入**：事件类型、频道、消息模板<br>**输出**：发送状态 |
| **异常处理** | Slack 断开连接时通知管理员；可手动重发。 |
| **性能要求** | 事件到 Slack ≤ 1 分钟。 |
| **安全要求** | 消息不包含敏感文档正文。 |
| **关联接口** | API-16 |
| **关联验收** | AC-29 |
| **关联埋点** | EVT-18 |
| **依赖项** | FR-10 |

### 8.3 功能依赖矩阵

| 依赖方 | 被依赖方 | 依赖类型 | 说明 |
|--------|----------|----------|------|
| FR-02 | FR-01 | 数据依赖 | 解析依赖上传后的文件 |
| FR-03 | FR-02 | 数据依赖 | 查看依赖解析后的 webp |
| FR-04 | FR-03 | 数据依赖 | 行为记录依赖 Viewer 加载 |
| FR-05 | FR-02 | 数据依赖 | AI 问答依赖 chunks 与 boxes |
| FR-06 | FR-03、FR-05 | 数据/渲染依赖 | 定位依赖 Viewer 与 evidence |
| FR-07 | FR-02 | 数据依赖 | 链接依赖 READY 文档 |
| FR-08 | FR-07 | 数据依赖 | 权限校验依赖链接配置 |
| FR-09 | FR-03、FR-08 | 渲染/数据依赖 | 水印依赖查看与身份 |
| FR-10 | FR-04 | 数据依赖 | 评分依赖行为数据 |
| FR-11 | FR-10 | 数据依赖 | 建议依赖评分 |
| FR-12 | FR-02、FR-07 | 数据依赖 | 数据室依赖文档与链接能力 |
| FR-13 | FR-12 | 数据依赖 | 权限审批依赖数据室 |
| FR-15 | FR-04 | 数据依赖 | CRM 同步依赖行为事件 |
| FR-16 | FR-10 | 数据依赖 | Slack 提醒依赖评分 |

---

## 9. 非功能需求

### 9.1 性能

| 场景 | 指标 | 验收标准 | 测试方法 |
|------|------|----------|----------|
| 文档上传 API | 响应时间 P99 | ≤ 2s | k6 |
| 解析 50 页 PDF | 完成时间 P95 | ≤ 3 分钟 | 端到端测试 |
| 签名 URL 生成 | 响应时间 P99 | ≤ 200ms | k6 |
| 页面首图加载 | 时间 P95 | ≤ 1.5s | Lighthouse / 真机 |
| AI 问答响应 | 端到端 P95 | ≤ 3s | k6 |
| 搜索阶段 | 响应时间 P95 | ≤ 800ms | k6 |
| 事件上报 | 响应时间 P99 | ≤ 100ms | k6 |
| 列表页查询 | 响应时间 P99 | ≤ 300ms | k6 |

### 9.2 安全

| 场景 | 要求 | 验收标准 |
|------|------|----------|
| 传输安全 | TLS 1.2+ | 全站 HTTPS |
| 文件存储 | 私有 bucket，签名访问 | 对象存储 bucket policy 禁止公开读取 |
| 签名 URL | 含过期时间与签名 | 篡改 token 后 Cloudflare/后端拒绝访问；直接访问 OSS 返回 403 |
| 数据库 | 敏感字段加密 | 不存明文密码/密钥 |
| 访问控制 | 租户 + Workspace 双重隔离 | 租户 A 无法访问租户 B 数据；Workspace 1 用户无法访问 Workspace 2 数据 |
| 审计日志 | 关键操作可追溯 | 上传、删除、权限变更、访问均有日志 |
| 输入安全 | 防注入、防 XSS | 安全扫描通过 |

### 9.3 可用性

| 场景 | 指标 | 目标值 |
|------|------|--------|
| 核心查看链路 | 年度可用性 | ≥ 99.5% |
| 解析服务 | 可用性 | ≥ 99% |
| 计划外停机 | MTTR | ≤ 30 分钟 |
| 降级能力 | 对象存储故障 | 核心查看可切换备用存储 |

### 9.4 扩展性

- Upload / Ingestion / Search / Assistant 服务独立部署，支持水平扩展。
- 数据库按 `tenant_id` + `workspace_id` 分区，预留分库分表能力。
- 对象存储支持多云接入。
- 搜索索引支持独立扩容。

### 9.5 可维护性

- 核心业务逻辑单元测试覆盖率 ≥ 70%。
- 接口文档与代码同步更新。
- 异步任务有统一监控与告警。

### 9.6 可观测性

| 类型 | 要求 |
|------|------|
| 日志 | 结构化日志，含 trace_id、tenant_id、workspace_id、user_id、request_id |
| 指标 | QPS、延迟、错误率、饱和度覆盖所有 P0 接口 |
| 追踪 | 核心链路分布式追踪（上传 → 解析 → 查看 → AI 问答） |
| 告警 | P0 接口错误率 > 1% 或 P99 延迟 > 阈值时触发 |

---

## 10. 数据架构与埋点

### 10.1 数据模型

> 详见 `docs/database-model.md`。本章仅列出与本次 PRD 强相关的核心表。
>
> **多租户/Workspace 隔离原则**：所有业务表必须包含 `tenant_id` + `workspace_id`；所有查询必须同时带这两个字段过滤。用户切换 workspace 后，接口上下文通过子域名/slug 或 `X-Workspace-ID` header 传递，后端据此隔离数据。

| 表/集合 | 变更类型 | 说明 | 索引 | 备注 |
|----------|----------|------|------|------|
| `tenants` | 新增 | 租户信息 | `id`, `slug` | 每个租户对外有唯一 slug 和不可变 UUID |
| `tenant_domains` | 新增 | 租户子域名/自定义域名映射 | `tenant_id`, `domain`, `domain_type`, `is_primary` | domain_type: SUBDOMAIN / CUSTOM / PUBLIC_LINK；SSL 由 Let's Encrypt 自动签发并续期；PUBLIC_LINK 用于公开链接品牌域名 |
| `workspaces` | 新增 | 工作空间 | `id`, `tenant_id`, `slug` | slug 用于 URL 路径 `/{workspaceSlug}/...`，同一租户内唯一 |
| `workspace_members` | 新增 | 用户-工作空间关系 | `workspace_id`, `user_id`, `role` | role: owner/admin/member/guest |
| `workspace_invitations` | 新增 | Workspace 邀请记录 | `workspace_id`, `email`, `token`, `expires_at`, `status` | token 有效期可配置，默认 7 天 |
| `users` | 新增 | 用户基础信息 | `id`, `email` | 全局用户，通过 workspace_members 关联权限 |
| `documents` | 新增 | 文档主表 | `id`, `tenant_id`, `workspace_id`, `status`, `created_at` | 含 source_type、source_hash、status；按 tenant + workspace 行级隔离 |
| `document_files` | 新增 | 文件多角色存储 | `document_id`, `file_role` | ORIGINAL / SEMANTIC_JSON / PDF_CANONICAL / PAGE_WEBP / THUMBNAIL；按 tenant + workspace 行级隔离 |
| `document_pages` | 新增 | 页面元数据 | `document_id`, `page_number` | image_object_key / width / height；按 tenant + workspace 行级隔离 |
| `document_blocks` | 新增 | 语义块 | `document_id`, `page_number`, `block_index` | 结构语义与 bbox；按 tenant + workspace 行级隔离 |
| `document_chunks` | 新增 | 可检索 chunk | `tenant_id`, `workspace_id`, `document_id`, `chunk_index` | 含 normalized_text、search_vector、embedding；按 tenant + workspace 行级隔离 |
| `chunk_boxes` | 新增 | chunk 视觉定位 | `chunk_id`, `document_id`, `page_number` | PAGE_IMAGE_NORMALIZED 坐标 |
| `ingestion_jobs` | 新增 | 解析任务 | `document_id`, `status` | 异步 worker 消费 |
| `links` | 新增 | 分享链接 | `document_id`, `tenant_id`, `workspace_id`, `public_token` | 权限字段扁平化：permission_type / allowed_emails / password_hash / expires_at / max_access_count / download_enabled / watermark_enabled / public_token；按 tenant + workspace 行级隔离 |
| `link_accesses` | 新增 | 访问记录 | `link_id`, `visitor_email` | 每次访问一条 |
| `page_views` | 新增 | 页面级阅读 | `link_id`, `page_number` | 停留时长、滚动深度 |
| `deal_rooms` | 新增 | 数据室 | `tenant_id`, `workspace_id` | 模板、文件夹结构；按 tenant + workspace 行级隔离 |
| `room_members` | 新增 | 数据室成员 | `room_id`, `email`, `user_id` | email 作为准入标识（允许未注册用户），user_id 注册后关联；角色、NDA 确认、审批状态 |
| `assistant_sessions` | 新增 | AI 问答会话 | `link_id`, `visitor_id`, `user_id`, `document_id` | 上下文与历史；登录用户用 user_id，访客用 visitor_id |
| `deal_room_documents` | 新增 | 数据室文档关联 | `room_id`, `document_id`, `folder_path` | 数据室中文档与文件夹结构 |
| `room_member_folder_permissions` | 新增 | 数据室 folder 级权限 | `room_id`, `email`, `folder_path` | 受限 folder 仅对指定成员可见 |
| `room_access_requests` | 新增 | 数据室访问申请 | `room_id`, `email`, `reason`, `status` | 审批流程 |
| `events` | 新增 | 分析事件流 | `tenant_id`, `workspace_id`, `event_type`, `created_at` | 所有埋点事件 |

### 10.2 接口契约

> **Workspace 上下文传递**：业务接口通过子域名确定 tenant，通过 URL 路径 `/{workspaceSlug}/...` 确定 workspace，实际调用时完整路径为 `/{workspaceSlug}/api/v1/...`。API-09 为公开链接访问，使用品牌方自定义域名，通过 query 参数传递 tenant/workspace/token。API-14 为公开访问申请接口，无需 workspace 前缀与认证。

| 接口编号 | 方法 | 路径 | 说明 | 归属模块 |
|----------|------|------|------|----------|
| API-01 | POST | `/api/documents` | 上传文档 | Upload |
| API-02 | GET | `/api/documents/{documentId}` | 获取文档状态 | Upload |
| API-03 | GET | `/api/documents/{documentId}/pages` | 获取页面列表 | Viewer |
| API-04 | POST | `/api/documents/{documentId}/pages/signed-url` | 获取签名 URL | Viewer |
| API-05 | POST | `/api/events` | 上报阅读事件 | Analytics |
| API-06 | POST | `/api/search` | 文档内搜索 | Search |
| API-07 | POST | `/api/assistant/chat` | AI 问答 | Assistant |
| API-08 | POST | `/api/links` | 创建链接 | Link |
| API-09 | GET | `https://{publicDomain}?tenant={tenantSlug}&workspace={workspaceSlug}&token={linkToken}` | 访问公开链接 | Link |
| API-10 | GET | `/api/analytics/links/{linkId}/score` | 热度评分 | Analytics |
| API-11 | GET | `/api/analytics/links/{linkId}/suggestions` | 跟进建议 | Analytics |
| API-12 | POST | `/api/deal-rooms` | 创建数据室 | Deal Room |
| API-13 | GET | `/api/deal-rooms/{roomId}` | 获取数据室 | Deal Room |
| API-14 | POST | `/api/deal-rooms/{roomId}/access-requests` | 访问申请/审批 | Deal Room |
| API-15 | POST | `/api/integrations/crm/sync` | CRM 同步 | Integration |
| API-16 | POST | `/api/integrations/slack/notify` | Slack 通知 | Integration |

### 10.3 埋点事件规范

> 命名规则：`对象_动作`，属性 snake_case，所有事件携带 `tenant_id`、`workspace_id`、`user_id`（或 `visitor_id`）、`session_id`、`timestamp`。Workspace 切换后事件归属到新的 workspace_id。

| 事件编号 | 事件名 | 触发时机 | 属性字段 | 优先级 | 归属模块 |
|----------|--------|----------|----------|--------|----------|
| EVT-01 | `document_uploaded` | 文档上传成功 | `document_id`, `file_type`, `file_size` | P0 | Upload |
| EVT-02 | `ingestion_job_created` | 创建解析任务 | `document_id`, `source_type` | P0 | Upload |
| EVT-03 | `document_ingestion_completed` | 解析完成 | `document_id`, `duration_ms`, `page_count` | P0 | Upload |
| EVT-04 | `page_image_loaded` | Canvas 加载页面图 | `document_id`, `page_number`, `load_time_ms` | P0 | Viewer |
| EVT-05 | `page_viewed` | 页面被查看 | `link_id`, `page_number`, `duration_ms`, `scroll_depth` | P0 | Analytics |
| EVT-06 | `link_opened` | 链接被打开 | `link_id`, `visitor_email`, `device`, `region` | P0 | Link |
| EVT-07 | `ai_assistant_opened` | AI 助手展开 | `link_id`, `document_id` | P0 | Assistant |
| EVT-08 | `ai_question_submitted` | 提交问题 | `link_id`, `query_length` | P0 | Assistant |
| EVT-09 | `evidence_highlighted` | 引用被高亮 | `link_id`, `chunk_id`, `page_number` | P0 | Assistant |
| EVT-10 | `link_created` | 创建链接 | `document_id`, `permission_type` | P0 | Link |
| EVT-11 | `access_denied` | 访问被拒绝 | `link_id`, `reason` | P0 | Link |
| EVT-12 | `intent_score_updated` | 热度评分更新 | `link_id`, `score`, `tier` | P0 | Analytics |
| EVT-13 | `follow_up_suggestion_viewed` | 查看跟进建议 | `link_id`, `suggestion_type` | P1 | Analytics |
| EVT-14 | `deal_room_created` | 创建数据室 | `room_id`, `template_type` | P0 | Deal Room |
| EVT-15 | `deal_room_access_requested` | 访问申请 | `room_id`, `visitor_email` | P0 | Deal Room |
| EVT-16 | `email_notification_sent` | 邮件通知发送 | `notification_type`, `recipient` | P1 | Notification |
| EVT-17 | `crm_sync_event` | CRM 同步 | `integration_id`, `event_type` | P1 | Integration |
| EVT-18 | `slack_notification_sent` | Slack 通知 | `channel`, `event_type` | P1 | Integration |

### 10.4 用户属性

| 属性 | 类型 | 说明 |
|------|------|------|
| `tenant_id` | uuid | 租户唯一标识（对外组织） |
| `workspace_id` | uuid | 当前工作空间标识（对内数据隔离） |
| `user_id` | uuid | 用户唯一标识 |
| `visitor_id` | string | 匿名访问者标识（未登录接收方） |
| `role` | string | founder / investor / sales / admin |
| `workspace_role` | string | owner / admin / member / guest |
| `plan` | string | free / founder / pro / secure_room |

### 10.5 数据报表需求

| 报表 | 维度 | 指标 | 更新频率 | 使用人 |
|------|------|------|----------|--------|
| 文档解析成功率 | source_type / workspace | 成功率、失败原因分布 | 实时 | 技术/产品 |
| AI 问答效果 | query / answer | 打开率、引用点击率、满意度 | 每日 | 产品 |
| 热度分层分布 | tier | Hot / Warm / Cold 占比 | 每日 | 产品/销售 |
| 链接活跃度 | link / document | 打开率、回访率、下载率 | 每日 | 用户/增长 |
| 数据室活跃度 | room | 访问人数、Q&A 数、审批数 | 每日 | 用户 |
| 用户激活漏斗 | step | 注册 → 上传 → 发链接 → 收到访问 | 每日 | 增长 |

---

## 11. 交互与体验

### 11.1 设计系统

- **设计稿链接**：【Figma 链接】
- **组件库**：基于 shadcn/ui + Tailwind CSS 构建 DealSignal 设计系统
- **色彩/字体/间距**：详见设计系统 Token 文档

### 11.2 信息架构

```text
Dashboard（交易雷达）
├── 今日高意图事件
├── 最近活跃链接
├── 需要跟进的人
├── 高风险访问
├── 表现最好的内容
└── 创建新链接 / 创建数据室

Documents
├── 所有文档
├── 版本管理
├── 内容表现
└── 团队内容库

Links
├── 所有分享链接
├── 权限状态
├── 活跃度
└── 过期 / 撤回

Deal Rooms
├── 数据室列表
├── 模板
├── 访问者
└── Q&A

Contacts
├── 投资人 / LP / 客户
├── 公司 / 账户
└── Engagement history

Insights
├── 热度评分
├── 内容转化
├── 页面分析
└── 团队表现

Settings
├── 切换 Workspace
├── 品牌
├── 权限
├── 集成
├── 团队成员
└── 安全策略
```

### 11.3 关键页面清单

| 页面 | 设计稿链接 | 状态 | 备注 |
|------|------------|------|------|
| Dashboard | 【链接】 | 待设计 | 交易雷达首屏 |
| 文档上传页 | 【链接】 | 待设计 | 拖拽上传、进度、格式提示 |
| 文档列表页 | 【链接】 | 待设计 | 状态、搜索、筛选 |
| 链接权限设置 | 【链接】 | 待设计 | 安全强度 vs 摩擦提示 |
| Viewer 阅读页 | 【链接】 | 待设计 | Canvas + 悬浮 AI + 目录 |
| AI 问答对话框 | 【链接】 | 待设计 | 答案 + 引用 + 页码跳转 |
| 数据分析页 | 【链接】 | 待设计 | 热度、页面停留、访问者 |
| 数据室页 | 【链接】 | 待设计 | 文件夹、权限、Q&A |
| 设置页 | 【链接】 | 待设计 | 品牌、集成、成员 |

### 11.4 交互说明

| 交互元素 | 触发方式 | 反馈 | 异常处理 |
|----------|----------|------|----------|
| 上传文档 | 拖拽/点击选择 | 进度条 → 转换中 → 完成 | 失败提示原因，保留重试 |
| 创建链接 | 点击 | 弹出权限配置抽屉 → 生成短链 | 文档未 READY 时禁用 |
| 权限强度滑块 | 拖动 | 实时显示安全强度与接收方摩擦 | - |
| 打开 Viewer | 点击链接 | 校验权限 → 加载 Canvas | 无权限/过期展示拦截页 |
| AI 助手图标 | 点击 | 展开对话框 | 文档未 READY 时隐藏 |
| 提交问题 | 回车/点击发送 | loading → 显示答案 + 引用 | 无结果提示"未找到相关内容" |
| 点击引用 | 点击 | Canvas 跳转目标页 + 高亮框 pulse | 目标页未加载时请求签名 URL |
| 热度评分卡片 | hover | 展示评分因子 | - |
| 数据室访问申请 | 提交 | 待审批状态 + owner 通知 | 拒绝时说明原因 |
| Workspace 切换 | 点击侧边栏 Settings → 切换 Workspace | 展开 workspace 列表，切换后 URL 跳转 `/{workspaceSlug}/...` 并刷新页面数据 | 仅展示用户有权限的 workspace |

### 11.5 页面状态规范

| 状态 | 说明 | 设计处理 |
|------|------|----------|
| 加载中 | 上传/解析/页面加载 | Skeleton / 进度条 |
| 处理中 | 文档正在 ingestion | "正在解析文档，预计 X 秒" |
| 空态 | 无文档/无链接 | 引导上传/创建 |
| 错误态 | 解析失败/加载失败 | 错误提示 + 重试/联系支持 |
| 权限不足 | 无权限/过期/撤回 | 清晰原因 + 联系发件人入口 |
| 成功态 | 操作成功 | Toast 提示 |

### 11.6 文案规范

| 场景 | 文案 |
|------|------|
| 文档处理中 | "正在解析文档，预计还需要 X 秒" |
| 解析失败 | "文档解析失败，请重试或联系支持" |
| 链接已过期 | "该链接已过期，请联系发件人获取新链接" |
| 无权限 | "你没有权限查看此文档" |
| 下载关闭 | "发件人已关闭下载，请在线查看" |
| AI 无结果 | "未找到与此问题相关的内容" |
| 高意图提醒 | "3 位投资人今天重新查看了你的财务页" |
| 转发提醒 | "Acme 的 proposal 被转发给 4 人" |

---

## 12. 验收标准与测试策略

### 12.1 测试策略总览

| 测试类型 | 覆盖范围 | 负责人 | 工具/方法 | 通过标准 |
|----------|----------|--------|-----------|----------|
| 单元测试 | Upload、Ingestion、Search、Evidence、Analytics 核心业务逻辑 | 开发 | Jest / pytest | 覆盖率 ≥ 70% |
| 集成测试 | 模块间接口、数据库事务、消息队列 | 开发 | pytest / 自动化 | 核心用例 100% 通过 |
| 接口测试 | 所有 P0 API | 测试 | Postman / pytest | P0 接口 100% 通过 |
| UI 测试 | 上传 → 解析 → 查看 → AI 问答关键路径 | 测试 | Playwright | P0 路径 100% 通过 |
| 兼容性测试 | Chrome 90+、Safari 14+、Firefox 88+、Edge 90+；iOS 14+ / Android 10+ | 测试 | 真机/浏览器矩阵 | 通过测试矩阵 |
| 性能测试 | 上传、签名 URL、AI 问答、搜索 | 测试/开发 | k6 / Lighthouse | 达到 NFR 指标 |
| 安全测试 | 鉴权、租户/Workspace 越权、签名 URL 篡改、SQL 注入、XSS | 安全团队 | 扫描 + 手工 | 无高危漏洞 |
| 域名/SSL 测试 | 子域名解析、自定义域名 CNAME、SSL 自动签发 | 测试/运维 | 手工 + 自动化 | 所有测试域名 HTTPS 可访问 |
| 回归测试 | 全量 P0/P1 功能 | 测试 | 自动化 + 手工 | 无阻塞 Bug |

### 12.2 验收标准

#### AC-01：文档上传成功

- **Given** 用户已登录且选择 10MB 的 PDF 文件
- **When** 用户点击上传
- **Then** 系统显示上传进度，完成后返回 document_id，documents.status = UPLOADED

#### AC-02：文档上传失败（大小超限）

- **Given** 用户选择 150MB 的视频文件
- **When** 用户点击上传
- **Then** 系统提示"文件大小超过 100MB 限制，请压缩后重试"

#### AC-03：PDF 解析成功

- **Given** 用户上传 20 页 PDF
- **When** ingestion worker 完成处理
- **Then** documents.status = READY；生成 20 条 document_pages 记录；生成 chunks 与 boxes；pgvector 索引可用

#### AC-04：Office 文档解析成功

- **Given** 用户上传 DOCX 文件
- **When** ingestion worker 完成处理
- **Then** 生成 PDF_CANONICAL 文件；提取 bbox；渲染 webp；状态变为 READY

#### AC-05：签名 URL 生成与访问

- **Given** 文档 READY 且用户有权限
- **When** 请求 `/{workspaceSlug}/api/v1/documents/{id}/pages/signed-url`
- **Then** 返回含过期时间与签名的 URL；篡改签名后 Cloudflare/后端返回 403；直接访问 OSS 对象返回 403

#### AC-06：Canvas 渲染页面

- **Given** 用户已获得签名 URL
- **When** Canvas 加载 webp
- **Then** 2 秒内渲染页面，页面文字清晰可读

#### AC-07：阅读行为记录

- **Given** 访问者浏览第 3 页 15 秒
- **When** 访问者切换页面
- **Then** 系统在 10 秒内记录 `page_viewed` 事件，包含页码与停留时长

#### AC-08：阅读行为去重

- **Given** 访问者连续刷新页面 3 次
- **When** 系统处理事件
- **Then** 仅记录 1 次有效 page_viewed

#### AC-09：AI 问答返回引用

- **Given** 文档已 READY 且用户打开 AI 助手
- **When** 用户输入"付款期限是多少"
- **Then** 系统返回带 quote、pageNumber、bbox 的回答

#### AC-10：AI 问答无结果

- **Given** 文档中无相关内容
- **When** 用户输入无关问题
- **Then** 系统返回"未找到与此问题相关的内容"

#### AC-11：自动定位与高亮

- **Given** AI 回答返回 pageNumber=2 的 evidence
- **When** 系统自动选择 top1 evidence
- **Then** Canvas 跳转至第 2 页并绘制高亮框

#### AC-12：切换引用定位

- **Given** AI 回答返回多个 evidence
- **When** 用户点击第 2 个引用
- **Then** Canvas 跳转对应页面并重新绘制高亮框

#### AC-13：创建智能链接

- **Given** 文档状态为 READY
- **When** 用户配置邮箱白名单并创建链接
- **Then** 生成短链接，链接配置写入数据库

#### AC-14：邮箱白名单拦截

- **Given** 链接仅允许 `investor@vc.com` 访问
- **When** `other@vc.com` 访问链接
- **Then** 系统拒绝访问并提示"你没有权限查看此文档"

#### AC-15：链接过期拦截

- **Given** 链接已设置过期时间
- **When** 过期后访问链接
- **Then** 系统展示"链接已过期"页面并提供联系发件人入口

#### AC-16：链接撤回

- **Given** 用户已创建链接
- **When** 用户点击撤回
- **Then** 该链接立即失效，已打开页面后续请求被拦截

#### AC-17：动态水印

- **Given** 链接启用水印且访问者已通过邮箱验证
- **When** 访问者查看文档并截图
- **Then** 截图中每页仍显示访问者邮箱与访问时间水印

#### AC-18：热度评分 Hot

- **Given** 某链接 7 天内被同一访问者打开 3 次并查看财务页 2 次
- **When** 系统计算热度
- **Then** 评分 ≥ 70 并标记为 Hot

#### AC-19：行为提醒发送

- **Given** 链接已开启首次打开提醒
- **When** 访问者首次打开链接
- **Then** 发件人在 5 分钟内收到邮件通知

#### AC-20：跟进建议生成

- **Given** 某投资人 24 小时内 3 次查看财务页
- **When** 发件人查看 Dashboard
- **Then** 系统展示"该投资人重复查看财务页，建议发送 financial model"

#### AC-21：数据室创建

- **Given** 用户选择 Seed Fundraising 模板并上传 5 个文件
- **When** 创建数据室
- **Then** 自动生成标准文件夹结构；生成 room_id

#### AC-22：数据室权限隔离

- **Given** 数据室中某文件夹仅对投资人 A 可见
- **When** 投资人 B 访问数据室
- **Then** 投资人 B 看不到该文件夹及其中文件

#### AC-23：数据室访问审批

- **Given** 数据室开启访问审批
- **When** 访问者提交申请
- **Then** owner 收到审批通知；审批通过后访问者才能进入

#### AC-24：邮件通知可配置

- **Given** 用户在设置中关闭某类提醒
- **When** 对应事件触发
- **Then** 系统不再发送该类邮件通知

#### AC-25：CRM 同步

- **Given** 用户完成 HubSpot 授权
- **When** 某 Hot 事件发生
- **Then** 1 小时内在 HubSpot timeline 记录事件并创建 follow-up task

#### AC-26：Slack 通知

- **Given** 用户绑定 Slack 并配置推送规则
- **When** proposal 被转发给 4 人
- **Then** 指定频道收到"Acme proposal 被内部转发给 4 人"消息

#### AC-27：性能 — 页面加载

- **Given** 100 并发用户同时请求签名 URL
- **When** 压测持续 5 分钟
- **Then** P99 响应时间 ≤ 200ms，错误率 ≤ 0.1%

#### AC-28：性能 — AI 问答

- **Given** 50 并发用户同时提交 AI 问题
- **When** 压测持续 5 分钟
- **Then** P95 响应时间 ≤ 3s，错误率 ≤ 1%

#### AC-29：安全 — 租户越权访问

- **Given** 用户 A 属于租户 1
- **When** 用户 A 尝试访问租户 2 的文档
- **Then** 系统返回 403，记录审计日志

#### AC-30：安全 — Workspace 越权访问

- **Given** 用户 A 属于 Workspace 1
- **When** 用户 A 切换上下文到 Workspace 2 并尝试访问 Workspace 2 的文档
- **Then** 系统返回 403，记录审计日志

#### AC-31：通过 Workspace 邀请注册

- **Given** admin 在 Workspace 中邀请 `newuser@example.com`
- **When** 被邀请人点击邀请链接完成注册
- **Then** 该用户注册成功并自动加入对应 Workspace，角色为 member

#### AC-32：最大访问次数限制

- **Given** 链接已设置最大访问次数为 3 次
- **When** 同一访问者第 4 次访问该链接
- **Then** 系统拒绝访问并提示"链接已达到最大访问次数"

### 12.3 验收检查清单

- [ ] 所有 P0 功能都有正常路径 + 异常路径验收标准
- [ ] 所有涉及权限的功能都有越权验收
- [ ] 所有涉及文件处理的功能都有失败/边界验收
- [ ] 所有 AI 功能都有"有结果"和"无结果"验收
- [ ] 所有性能需求都有对应压测验收
- [ ] 所有安全需求都有对应渗透/越权验收

---

## 13. 集成与接口

### 13.1 内部接口

| 接口编号 | 提供方 | 消费方 | 说明 | 稳定性 |
|----------|--------|--------|------|--------|
| INT-01 | Upload Service | Ingestion Worker | 新文档事件触发解析 | 强依赖 |
| INT-02 | Ingestion Worker | Search Service | 解析完成后通知更新索引 | 强依赖 |
| INT-03 | Viewer Frontend | Search Service | AI 问答调用搜索 | 强依赖 |
| INT-04 | Search Service | Evidence Service | 搜索结果聚合 evidence | 强依赖 |
| INT-05 | Evidence Service | Assistant Service | evidence 生成回答 | 强依赖 |
| INT-06 | Analytics Service | Notification Service | 事件触发通知 | 弱依赖，可降级 |
| INT-07 | Analytics Service | Integration Service | 事件同步 CRM/Slack | 弱依赖，可降级 |

### 13.2 第三方集成

| 集成方 | 集成内容 | 用途 | 资源规格 | 风险等级 | 备选方案 |
|--------|----------|------|----------|----------|----------|
| AWS S3 / 阿里云 OSS | 对象存储 | 原文件、PDF、webp 存储 | 按存储量与流量计费 | 低 | 多云备份 |
| Cloudflare CDN | CDN + URL Signing | 后端签名 URL 缓存分发 | 按流量计费 | 低 | 阿里云 CDN + 后端鉴权 |
| SendGrid / AWS SES | 邮件发送 | 通知、验证码 | 按发送量计费 | 中 | 切换邮件服务商 |
| OnlyOffice（自托管，独立集群） | Office 转 PDF | Office 文档解析 | 独立 K8s 集群：4C8G × 2 实例，并发转换 10 个文档；对象存储挂载转换缓存；与主服务通过 VPC 对等连接 | 中 | 自托管 LibreOffice 或自研降级 |
| OpenAI-compatible API | Embedding + Chat | AI 问答 | 中 | Azure OpenAI / 自托管 bge + vLLM |
| HubSpot / Salesforce | CRM 同步 | 销售工作流 | 低 | 手动导入 |
| Slack | 消息推送 | 团队通知 | 低 | 邮件替代 |

### 13.3 数据流图

> 详细数据流图（含文档上传、文档查看、AI 问答）已迁移至 `docs/ARCHITECTURE-v1.0.0.md` 第 4 章。
>
> 参见：[ARCHITECTURE-v1.0.0.md#4-数据流图](ARCHITECTURE-v1.0.0.md)

核心数据链路简述：

- **文档上传**：Upload Service 校验并写入 OSS → 推送 ingestion job → Ingestion Worker 解析/渲染 → 写入 PostgreSQL + pgvector/Elasticsearch。
- **文档查看**：Gateway 透传 Host → Viewer API 解析 tenant/workspace 并校验权限 → 生成后端签名 URL → Canvas 加载 Cloudflare CDN 缓存的 webp。
- **AI 问答**：Search Service 执行 hybrid search → Evidence Service 聚合 quote/page/bbox → Assistant Service 调用 LLM → 前端 Canvas 高亮。

### 13.4 同步/异步策略

| 场景 | 同步/异步 | 说明 | 失败处理 |
|------|-----------|------|----------|
| 文档上传 | 同步 | 用户等待上传结果 | 事务回滚 |
| 文档解析 | 异步 | 用户不等待 | Go channel / 本地队列；死信 + 告警 + 可重试 |
| 签名 URL 生成 | 同步 | Viewer 等待加载 | 缓存 + 重试 |
| AI 问答 | 同步 | 用户等待回答 | 超时返回友好提示 |
| 邮件通知 | 异步 | 用户不等待 | Go channel；重试 3 次 |
| CRM 同步 | 异步 | 用户不等待 | Go channel；重试 3 次后告警 |
| 热度评分计算 | 异步 | 用户不等待 | Go channel；失败告警 |

**说明**：当前 MVP 使用应用内 Go channel / 本地队列降低运维复杂度；日事件量达到千万级后二期引入 RabbitMQ / SQS / Kafka。

---

## 14. 实施计划

### 14.1 任务拆分

| 任务编号 | 任务名称 | 模块 | 负责人 | 依赖 | 预计工时 | 状态 |
|----------|----------|------|--------|------|----------|------|
| TASK-01 | 用户认证、租户与 Workspace 模块 | 基础 | 后端 | 无 | 6 天 | 待开始 |
| TASK-02 | 对象存储与后端签名 URL / Cloudflare URL Signing | 基础 | 后端/运维 | 云账号 | 4 天 | 待开始 |
| TASK-02a | 子域名/自定义域名与 SSL 自动签发 | 基础 | 后端/运维 | 云账号 | 3 天 | 待开始 |
| TASK-03 | 文档上传 API | Upload | 后端 | TASK-01、TASK-02 | 4 天 | 待开始 |
| TASK-04 | PDF Pipeline（bbox + webp） | Ingestion | 后端 | TASK-03 | 7 天 | 待开始 |
| TASK-05 | Office Pipeline（OnlyOffice 转 PDF） | Ingestion | 后端 | TASK-04 | 5 天 | 待开始 |
| TASK-06 | 数据库与搜索索引 | Data | 后端 | TASK-04 | 5 天 | 待开始 |
| TASK-07 | 签名 URL 与权限校验 | Viewer | 后端 | TASK-02、TASK-06 | 5 天 | 待开始 |
| TASK-08 | Viewer Canvas 前端 | Viewer | 前端 | TASK-07 | 8 天 | 待开始 |
| TASK-09 | Search Service | AI | 后端 | TASK-06 | 6 天 | 待开始 |
| TASK-10 | Evidence Service | AI | 后端 | TASK-09 | 4 天 | 待开始 |
| TASK-11 | Assistant Service | AI | 后端 | TASK-10 | 5 天 | 待开始 |
| TASK-12 | 悬浮 AI 助手前端 | AI | 前端 | TASK-08、TASK-11 | 7 天 | 待开始 |
| TASK-13 | 智能链接与权限 | Link | 后端 | TASK-07 | 5 天 | 待开始 |
| TASK-14 | Dashboard 前端 | Web | 前端 | TASK-13 | 6 天 | 待开始 |
| TASK-15 | 热度评分与 Analytics | Analytics | 后端 | TASK-08 | 5 天 | 待开始 |
| TASK-16 | 行为提醒与跟进建议 | Analytics | 后端 | TASK-15 | 4 天 | 待开始 |
| TASK-17 | 数据室模块 | Deal Room | 后端 + 前端 | TASK-06、TASK-13 | 8 天 | 待开始 |
| TASK-18 | 邮件通知系统 | Notification | 后端 | 邮件服务 | 3 天 | 待开始 |
| TASK-19 | CRM 集成 | Integration | 后端 | TASK-15 | 4 天 | 待开始 |
| TASK-20 | Slack 集成 | Integration | 后端 | TASK-15 | 2 天 | 待开始 |
| TASK-21 | 测试用例与自动化 | 测试 | 测试 | PRD 评审 | 10 天 | 待开始 |
| TASK-22 | 性能压测与优化 | 测试/开发 | 测试/开发 | 功能开发完成 | 5 天 | 待开始 |
| TASK-23 | 安全扫描与修复 | 安全 | 安全团队 | 开发完成 | 3 天 | 待开始 |

### 14.2 里程碑

| 里程碑 | 日期 | 交付物 | 通过标准 |
|--------|------|--------|----------|
| PRD 评审通过 | 2026-06-25 | 已批准 PRD | 所有关键方签字 |
| 技术方案确认 | 2026-07-02 | TDD 文档 | 架构评审通过 |
| 设计稿确认 | 2026-07-09 | 高保真设计稿 + 交互原型 | 产品+设计确认 |
| 基础服务完成 | 2026-07-23 | 上传、解析、存储、签名 URL | 自测通过 |
| 核心链路完成 | 2026-08-13 | 上传 → 查看 → AI 问答可跑通 | 集成测试通过 |
| 功能开发完成 | 2026-08-27 | 所有 P0 功能代码合并 | 自测 + 接口测试通过 |
| 测试通过 | 2026-09-10 | 测试报告 | P0/P1 用例 100% 通过 |
| 内测上线 | 2026-09-17 | 20 个种子用户 | 核心指标无异常 |
| 灰度发布 | 2026-09-24 | 10% 流量 | 监控稳定 48h |
| 正式上线 | 2026-09-30 | 全量 | 监控稳定 24h |

### 14.3 依赖关系图

```text
PRD 评审通过
    ↓
技术方案确认
    ↓
设计稿确认
    ↓
基础服务（对象存储 + CDN + 认证）
    ↓
    ├──────→ 文档上传 API
    │             ↓
    │       PDF / Office Pipeline
    │             ↓
    │       数据库 + 搜索索引
    │             ↓
    │       签名 URL + 权限校验
    │             ↓
    │       Viewer Canvas
    │             ↓
    │       Search / Evidence / Assistant
    │             ↓
    │       悬浮 AI 助手
    │             ↓
    │       智能链接 + Dashboard
    │             ↓
    │       热度评分 + 行为提醒
    │             ↓
    └──────→ 数据室 + 通知 + 集成
                  ↓
            测试 + 压测 + 安全扫描
                  ↓
            内测 → 灰度 → 全量
```

---

## 15. 风险与缓解

### 15.1 风险登记册

| 风险编号 | 风险描述 | 影响 | 概率 | 等级 | 应对策略 | 触发条件 | 应急预案 | 负责人 |
|----------|----------|------|------|------|----------|----------|----------|--------|
| R-01 | 文档转换/解析失败率高 | 用户无法查看、AI 问答失效 | 中 | 高 | 接入成熟转换服务；准备降级方案；建立失败样本库 | 解析成功率 < 95% | 切换备用服务；允许下载原文件 | 技术 |
| R-02 | AI 问答幻觉/准确度低 | 用户不信任、损害品牌 | 中 | 高 | 强制 evidence 引用；答案置信度展示；用户反馈闭环 | 用户满意度 < 70% | 降低 LLM 温度；增加 reranker；人工审核 | 产品 |
| R-03 | 对象存储/CDN 故障 | 文档无法查看 | 低 | 高 | 阿里云 OSS 多可用区 + 版本控制；Cloudflare 故障时切换源站认证；备用 CDN 方案 | 核心查看错误率 > 1% | 切换备用云；使用原文件兜底 | 运维 |
| R-04 | Embedding/LLM 服务不可用或限流 | AI 问答失败 | 中 | 高 | 本地 embedding 模型降级；请求缓存；队列限流 | AI 接口错误率 > 5% | 关闭 AI 功能入口；仅保留搜索 | 技术 |
| R-05 | 权限控制漏洞导致材料泄露 | 安全事故 | 低 | 高 | 安全评审；渗透测试；签名 URL 短有效期 | 发现越权访问 | 立即撤回受影响链接；启动安全响应 | 安全 |
| R-05a | Workspace 切换导致跨空间数据泄露 | 安全事故 | 低 | 高 | 所有查询强制带 workspace_id；自动化越权测试 | 发现 workspace 越权 | 立即修复；通知受影响租户 | 安全 |
| R-05b | SSL 自动签发失败 | 自定义域名无法访问 | 低 | 中 | 监控 Let's Encrypt 签发状态；失败告警 | 域名证书过期或签发失败 | 切换备用证书；通知租户管理员 | 运维 |
| R-06 | 需求变更频繁 | 工期延误 | 高 | 中 | PRD 基线冻结；变更走正式流程 | 任何未评审变更 | 评估影响，调整排期或裁剪范围 | 产品 |
| R-07 | 法务合规审核延迟 | 上线延期 | 低 | 中 | 提前准备隐私政策、DPA、数据删除流程 | 法务未在上线前确认 | 延迟上线；启用最小合规版本 | 运营 |
| R-08 | 用户接受度低于预期 | 产品-市场契合未验证 | 中 | 中 | MVP 快速验证；设置退出标准 | 30 天激活率 < 20% | 启动用户深度访谈；调整价值主张 | 产品 |

### 15.2 风险等级定义

| 等级 | 定义 |
|------|------|
| **高** | 可能导致项目延期、重大事故、合规问题或用户信任危机 |
| **中** | 可能影响部分功能、指标或工期，但可控 |
| **低** | 影响轻微，可在日常迭代中处理 |

---

## 16. 上线与运维

### 16.1 发布策略

1. **内测**：邀请 20 个种子用户，持续 1 周，收集反馈并修复。
2. **灰度**：开放 10% 新注册流量，观察 48 小时。
3. **扩量**：逐步扩大至 50%、100%。
4. **全量**：灰度无异常后全量开放。

### 16.2 发布检查清单

- [ ] 代码已通过代码评审并合并到 release 分支
- [ ] 数据库迁移脚本已在 staging 执行并验证可回滚
- [ ] 配置项（对象存储、CDN、邮件、LLM、CRM、OnlyOffice）已在生产环境确认
- [ ] 子域名 `{slug}.dealsignal.com` 解析与 SSL 自动签发已验证
- [ ] 自定义域名 CNAME 验证与 SSL 签发流程已验证
- [ ] 监控大盘与告警已配置
- [ ] Feature Flag 已就绪
- [ ] 回滚方案已准备并演练
- [ ] 客服与运营团队已培训
- [ ] 隐私政策与数据处理协议已上线

### 16.3 监控大盘

| 监控项 | 指标 | 告警阈值 | 级别 |
|--------|------|----------|------|
| 文档上传成功率 | 成功率 | < 98% | P0 |
| 文档解析成功率 | 成功率 | < 95% | P0 |
| 核心查看接口错误率 | 错误率 | > 1% | P0 |
| 签名 URL 生成 P99 | 延迟 | > 500ms | P0 |
| 页面首图加载 P95 | 时间 | > 3s | P1 |
| AI 问答 P95 | 延迟 | > 5s | P0 |
| 搜索 P95 | 延迟 | > 1s | P0 |
| 数据库连接池 | 使用率 | > 80% | P1 |
| 队列堆积数 | 数量 | > 1000 | P1 |
| LLM/Embedding 错误率 | 错误率 | > 5% | P0 |
| Workspace 越权尝试 | 次数 | > 0 | P0 |
| SSL 证书有效期 | 天数 | < 14 天 | P1 |
| Let's Encrypt 签发失败 | 次数 | > 0 | P1 |

### 16.4 On-Call 安排

| 阶段 | 值班人 | 响应时间 | 备注 |
|------|--------|----------|------|
| 上线后 24h | 技术负责人 | 15 分钟 | 重点关注解析与查看链路 |
| 上线后 7 天 | 后端负责人 | 30 分钟 | 关注 AI 问答与热度评分 |
| 上线后 30 天 | 轮值 | 1 小时 | 常规值班 |

### 16.5 回滚条件

- 文档解析成功率 < 95% 持续 30 分钟
- 核心查看接口错误率 > 1% 持续 10 分钟
- AI 问答错误率 > 10% 持续 15 分钟
- 发现数据安全或权限漏洞
- 用户客诉量异常激增
- 核心第三方依赖（对象存储、LLM）大面积不可用

### 16.6 回滚步骤

1. 关闭新用户注册或灰度开关。
2. 回滚前端与后端代码至上一稳定版本。
3. 如已执行破坏性数据库变更，运行逆向迁移脚本。
4. 验证核心路径（上传 → 解析 → 查看 → AI 问答）恢复正常。
5. 通知用户与相关方。
6. 启动复盘会议，输出改进 Action。

### 16.7 运营支持

| 运营项 | 内容 | 负责人 | 完成时间 |
|--------|------|--------|----------|
| 用户引导 | 新用户 onboarding：上传 → 创建链接 → 查看分析 | 运营 | 上线前 |
| 帮助文档 | 上传指南、权限说明、AI 助手使用、数据室 FAQ | 运营 | 上线前 |
| 客服培训 | 核心功能、常见问题、升级路径、投诉处理 | 客服 | 上线前 |
| 反馈收集 | 应用内反馈、NPS 问卷、用户访谈 | 产品 | 上线后 3 天 |
| 种子用户运营 | 1v1 跟进、收集使用案例 | 产品/运营 | 内测期 |
| 数据复盘 | 每日数据简报、周度产品复盘 | 产品/数据 | 上线后持续 |

---

## 17. 合规、安全与隐私

### 17.1 合规要求

| 法规/标准 | 要求 | 落实方式 |
|-----------|------|----------|
| GDPR | 数据可导出、可删除、知情权 | 提供账号注销与数据导出功能 |
| CCPA | opt-out、知情权 | 隐私政策明确数据使用方式 |
| 数据安全法 | 数据分类分级、出境评估 | 数据存储在合规区域；评估出境场景 |
| SOC 2 | 安全控制与审计 | 按 SOC 2 路线逐步落地 |

### 17.2 隐私设计

- 默认最小化收集数据，仅收集服务必需信息。
- Viewer 页面明确展示追踪说明，并提供隐私政策入口。
- 不向第三方出售接收方数据。
- 支持租户配置数据保留周期（如 90 天 / 1 年 / 永久）。
- AI 问答上下文不用于模型训练。

### 17.3 安全基线

- 所有 API 必须鉴权，禁止未授权访问。
- 采用"子域名隔离为主、独立 Workspace 隔离为辅"的混合模式：
  - 对外通过子域名 `{slug}.dealsignal.com` 或自定义域名识别租户，网关将域名解析为 tenant UUID。
  - 自定义域名 SSL 证书由 Let's Encrypt 自动签发并续期，失败时通知租户管理员。
  - 对内通过 `workspace_id` 隔离数据，同一用户可属于多个 Workspace，切换后只能看到当前 Workspace 的文档与分析数据。
  - 所有业务查询必须同时带 `tenant_id` 与 `workspace_id` 过滤。
  - Workspace 创建权限仅限 tenant admin/owner。
- 敏感操作（删除、权限变更、导出）记录审计日志。
- 对象存储文件必须通过签名 URL 访问，禁止公开 bucket。
- 生产环境访问需 MFA。
- OnlyOffice 独立集群与主服务通过 VPC 对等连接，不暴露公网入口。
- 定期进行漏洞扫描与渗透测试。

### 17.4 AI 安全与可信

- AI 回答必须附带 evidence 引用，不可凭空生成。
- 对 LLM 输出做毒性/敏感内容过滤。
- 记录 AI 问答日志用于审计与质量分析。
- 用户可举报不当回答。

---

## 18. 决策记录

| 决策编号 | 决策内容 | 替代方案 | 选择原因 | 影响 | 负责人 |
|----------|----------|----------|----------|------|--------|
| D-01 | 文档统一转 PDF 后渲染 webp | 直接原格式预览 | 统一 viewer 体验，便于 bbox 提取与 AI 定位 | 增加转换依赖与成本 | 技术 |
| D-02 | 前端基于 Canvas 绘制页面 | 原生 PDF.js 渲染 | 更好的水印/高亮 overlay 控制，统一移动端体验 | 开发复杂度更高 | 技术 |
| D-03 | 使用 PAGE_IMAGE_NORMALIZED 坐标 | 像素坐标 | 适配不同渲染尺寸，支持响应式缩放 | 需要坐标换算 | 技术 |
| D-04 | AI 回答必须附带 evidence 引用 | 自由生成式回答 | 增强可信度，减少幻觉 | 需要证据服务与 bbox 对齐 | 产品 |
| D-05 | 搜索采用 hybrid（exact + fts + vector） | 仅 vector 或仅 fts | 兼顾精确匹配与语义匹配 | 增加 RRF 合并复杂度 | 技术 |
| D-06 | MVP 使用规则加权热度评分 | ML 模型 | 数据不足，规则可解释性强 | 准确度有限，二期优化 | 产品 |
| D-07 | 权限默认低摩擦，高敏感才强验证 | 全部强制登录 | 降低接收方流失，符合交易场景 | 需要精细权限设计 | 产品 |
| D-08 | 优先做 HubSpot/Salesforce 集成 | 先做 Pipedrive | HubSpot/Salesforce 用户基数最大 | 可能暂时不满足 Pipedrive 用户 | 产品 |
| D-09 | Office 转 PDF 采用 OnlyOffice | LibreOffice / 自研 | OnlyOffice 转换效果与排版还原更好 | 增加供应商依赖 | 技术 |
| D-10 | 产品永久不支持 Markdown 文档上传 | 支持 Markdown | Markdown 不是核心交易材料格式，支持成本高 | 用户需将 Markdown 转 PDF 上传 | 产品 |
| D-11 | 产品永久不支持 CSV 导出 | 保留 CSV 导出 | 导出非 DealSignal 核心闭环能力 | 用户可手动复制数据 | 产品 |
| D-12 | 动态水印采用前端 Canvas overlay | 服务端渲染水印 | 减少服务端图片处理成本，便于灵活调整；截图时水印仍保留 | 前端可被绕过（需配合签名与审计兜底） | 技术 |
| D-13 | 租户隔离采用"子域名 + Workspace"混合模式 | 纯 tenant 行级隔离 / 独立 schema | 对外 `{slug}.dealsignal.com` 子域名隔离租户品牌，支持自定义域名；对内 Workspace 支持同一用户多空间 | 实现复杂度高于纯行级隔离 | 技术 |
| D-14 | Workspace 创建权限为 admin | owner 才能创建 | 减轻 owner 负担，admin 可管理日常空间 | admin 误操作风险 | 产品 |
| D-15 | 用户通过 workspace 邀请注册 | 开放注册后分配默认 workspace | 邀请制保证用户直接进入正确工作空间 | 自然注册流程变长 | 产品 |
| D-16 | 支持企业自定义域名 | 仅子域名 | 提升企业品牌感知 | 增加域名解析、SSL、CDN 配置复杂度 | 产品/技术 |
| D-17 | 自定义域名 SSL 采用 Let's Encrypt 自动签发 | 手动上传证书 / 付费 CA | 降低运维成本，自动续期 | 当前场景下速率限制不构成瓶颈，无需额外预案 | 技术 |
| D-18 | Workspace 切换入口放在侧边栏 Settings 子菜单 | 顶部导航 / 头像下拉 | 不占用主导航空间，符合设置心智模型 | 切换路径多一步 | 设计 |
| D-19 | 邀请 token 有效期可配置 | 固定 7 天 | 满足不同安全策略需求 | 配置项增加 | 产品 |
| D-20 | OnlyOffice 自托管部署在独立集群 | 与主服务同集群 | 隔离资源，避免转换任务影响核心服务 | 增加跨集群通信复杂度 | 技术 |
| D-21 | 自定义域名采用 **CNAME 验证** | DNS TXT 验证 | 配置更简单，用户易操作 | 需要用户能修改 DNS 记录 | 技术 |
| D-22 | Workspace 切换后 URL 为 `/{workspaceSlug}/...` | 通过 header 切换 | URL 直观，便于分享与刷新 | 需要前端路由适配 | 技术 |
| D-23 | 邀请 token 最大有效期 **30 天** | 无上限 | 平衡安全性与邀请便利性 | 超期邀请需重新发送 | 产品 |
| D-24 | OnlyOffice 独立集群与主服务通过 **VPC 对等** | 公网 HTTPS | 降低暴露面，提升安全性 | 需要云网络配置 | 技术 |

---

## 19. 附录

### 19.1 术语表

| 术语 | 说明 |
|------|------|
| Tenant | 对外组织单元，拥有唯一 slug 和不可变 UUID，用于子域名隔离 |
| Workspace | 对内工作空间，同一用户可属于多个 Workspace，数据权限各自独立 |
| Slug | 租户子域名标识，如 `acme.dealsignal.com` 中的 `acme`，与 tenant UUID 在网关层做路由映射 |
| Smart Link | 带有权限控制与追踪能力的智能文档分享链接 |
| Deal Room | 多文件、多权限的数据室，常用于融资尽调或 M&A |
| Intent Score | 基于阅读行为计算出的 0-100 交易意图热度分 |
| Evidence | AI 回答引用的原文片段及其页面、bbox 信息 |
| PAGE_IMAGE_NORMALIZED | 相对于页面图片宽高的归一化坐标（0-1） |
| NDA Gating | 访问者在进入数据室前需先签署 NDA 的机制 |
| Ingestion | 文档解析流程，包括转 PDF、提取 bbox、渲染 webp、生成 chunks |
| URL Signing | Cloudflare 提供的签名 URL 能力，后端签发、CDN 边缘验证，保护私有对象存储 |
| Signed URL | 含过期时间与 HMAC 签名的临时 URL，用于授权访问单页文档图片 |
| Hybrid Search | 精确匹配 + 全文搜索 + 向量搜索的混合检索 |
| RRF | Reciprocal Rank Fusion，多路搜索结果的合并算法 |

### 19.2 参考文档

- `docs/PRD + 产品设计的完整文档草案.md`（已冻结）
- `docs/tasks/上传-查看-AI问答设计文.md`（已冻结）
- `docs/PRD-v1.0.0.md`
- `docs/database-model.md`
- `docs/roadmap-dealsignal-v2.md`
- `docs/go-to-market.md`

### 19.3 PRD 评审检查清单

- [ ] 产品战略与成功标准被所有关键方理解
- [ ] 范围边界清晰，In/Out of Scope 无歧义
- [ ] 三类核心用户与关键路径完整
- [ ] 16 个 FR 都已详细定义并关联 AC/EVT
- [ ] 数据模型与接口契约已确认
- [ ] 32 条验收标准覆盖正常/异常/边界/权限/性能/安全/租户隔离/注册流程
- [ ] 非功能需求可量化、可测试
- [ ] 风险与回滚方案已准备
- [ ] 合规、安全、隐私、AI 可信要求已纳入
- [ ] 实施计划与里程碑合理可行

### 19.4 编号规范

| 前缀 | 用途 |
|------|------|
| `PRD-YYYY-NNN` | PRD 文档编号 |
| `H-NN` | 产品假设 |
| `FR-NN` | 功能需求 |
| `AC-NN` | 验收标准 |
| `EVT-NN` | 埋点事件 |
| `API-NN` | 接口编号 |
| `INT-NN` | 内部/第三方集成 |
| `TASK-NN` | 开发任务 |
| `R-NN` | 风险 |
| `D-NN` | 决策记录 |

### 19.5 旧文件冻结说明

> 本文档为 DealSignal 的生产级 PRD v2.0.5，基于 `docs/PRD + 产品设计的完整文档草案.md` 与 `docs/tasks/上传-查看-AI问答设计文.md` 整合编制。
> 以下文件自本文档发布之日起冻结为参考材料，不再作为开发实施依据：
> - `docs/PRD + 产品设计的完整文档草案.md`
> - `docs/tasks/上传-查看-AI问答设计文.md`
> - `docs/PRD-v1.0.0.md`
> 后续所有需求变更、任务拆分、验收测试、上线运维均以本文档为准。

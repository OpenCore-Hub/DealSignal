# DealSignal 文档分享业务缺口修复：可执行任务计划

**计划版本**：v1.0.0  
**生成日期**：2026-07-05  
**依据报告**：`docs/reviews/document-sharing-design-vs-implementation-gap-report.md`  
**目标**：将“设计初衷 vs 代码实现”的缺口转化为可执行、可验收、可独立合并的 AGENT-TASK。

---

## 1. 计划总览

本次修复计划共拆分为 **14 个独立任务**，覆盖后端、前端、AI、安全、集成、实时化等维度。

| 阶段 | 时间 | 任务数 | 核心目标 |
|---|---|---|---|
| **短期** | 1-2 周 | 4 个 | 修复功能可用性与安全缺陷，消除 P0 风险 |
| **中期** | 2-6 周 | 5 个 | 补齐设计语义差异，扩展事件与通知能力 |
| **长期** | 6-12 周 | 5 个 | 向 AI 增强意图洞察与实时化演进 |

---

## 2. 任务总览表

| 任务文件 | Task ID | 标题 | 阶段 | 优先级 | 类型 | 工作量 | 依赖 |
|---|---|---|---|---|---|---|---|
| [TASK-SHARE-SHORT-001.md](./TASK-SHARE-SHORT-001.md) | TASK-SHARE-SHORT-001 | 公共 Viewer AI Copilot 权限与端点修复 | 短期 | P0 | fullstack | M | - |
| [TASK-SHARE-SHORT-002.md](./TASK-SHARE-SHORT-002.md) | TASK-SHARE-SHORT-002 | 通知收件人与邮件开关修复 | 短期 | P0 | backend | S | - |
| [TASK-SHARE-SHORT-003.md](./TASK-SHARE-SHORT-003.md) | TASK-SHARE-SHORT-003 | 安全审计事件记录 | 短期 | P0 | backend | M | - |
| [TASK-SHARE-SHORT-004.md](./TASK-SHARE-SHORT-004.md) | TASK-SHARE-SHORT-004 | 访问与页面浏览基础去重 | 短期 | P1 | backend | M | - |
| [TASK-SHARE-MID-001.md](./TASK-SHARE-MID-001.md) | TASK-SHARE-MID-001 | 后端 Key Page Views 语义修正 | 中期 | P1 | backend | M | TASK-SHARE-SHORT-004 |
| [TASK-SHARE-MID-002.md](./TASK-SHARE-MID-002.md) | TASK-SHARE-MID-002 | 扩展追踪事件体系 | 中期 | P1 | fullstack | L | TASK-SHARE-SHORT-004 |
| [TASK-SHARE-MID-003.md](./TASK-SHARE-MID-003.md) | TASK-SHARE-MID-003 | 通知规则引擎与事件合并 | 中期 | P1 | backend | L | TASK-SHARE-SHORT-002 |
| [TASK-SHARE-MID-004.md](./TASK-SHARE-MID-004.md) | TASK-SHARE-MID-004 | 公共 Viewer 动态水印 | 中期 | P1 | frontend | M | - |
| [TASK-SHARE-MID-005.md](./TASK-SHARE-MID-005.md) | TASK-SHARE-MID-005 | 页面与下载签名 URL | 中期 | P1 | backend | M | - |
| [TASK-SHARE-LONG-001.md](./TASK-SHARE-LONG-001.md) | TASK-SHARE-LONG-001 | Heat Score 时间衰减与权重校准 | 长期 | P2 | backend | M | TASK-SHARE-MID-001 |
| [TASK-SHARE-LONG-002.md](./TASK-SHARE-LONG-002.md) | TASK-SHARE-LONG-002 | AI 问答意图分析 | 长期 | P2 | ai/backend | L | TASK-SHARE-SHORT-001 |
| [TASK-SHARE-LONG-003.md](./TASK-SHARE-LONG-003.md) | TASK-SHARE-LONG-003 | 实时事件推送与 Dashboard 更新 | 长期 | P2 | infra/fullstack | L | TASK-SHARE-MID-002 |
| [TASK-SHARE-LONG-004.md](./TASK-SHARE-LONG-004.md) | TASK-SHARE-LONG-004 | CRM 深度集成（Timeline / Deal Stage / Task） | 长期 | P2 | backend | L | TASK-SHARE-MID-003 |
| [TASK-SHARE-LONG-005.md](./TASK-SHARE-LONG-005.md) | TASK-SHARE-LONG-005 | 预测性 Lead Scoring | 长期 | P3 | ai/backend | XL | TASK-SHARE-LONG-001, TASK-SHARE-LONG-002 |

---

## 3. 执行顺序

### 3.1 按阶段编排

```text
Sprint 1-2（短期：修复 P0 缺陷）
├── TASK-SHARE-SHORT-001  公共 Viewer AI Copilot 权限与端点修复
├── TASK-SHARE-SHORT-002  通知收件人与邮件开关修复
├── TASK-SHARE-SHORT-003  安全审计事件记录
└── TASK-SHARE-SHORT-004  访问与页面浏览基础去重

Sprint 3-6（中期：补齐设计差异）
├── TASK-SHARE-MID-001  后端 Key Page Views 语义修正
│       └── 依赖 TASK-SHARE-SHORT-004
├── TASK-SHARE-MID-002  扩展追踪事件体系
│       └── 依赖 TASK-SHARE-SHORT-004
├── TASK-SHARE-MID-003  通知规则引擎与事件合并
│       └── 依赖 TASK-SHARE-SHORT-002
├── TASK-SHARE-MID-004  公共 Viewer 动态水印
└── TASK-SHARE-MID-005  页面与下载签名 URL

Sprint 7-12（长期：架构演进）
├── TASK-SHARE-LONG-001  Heat Score 时间衰减与权重校准
│       └── 依赖 TASK-SHARE-MID-001
├── TASK-SHARE-LONG-002  AI 问答意图分析
│       └── 依赖 TASK-SHARE-SHORT-001
├── TASK-SHARE-LONG-003  实时事件推送与 Dashboard 更新
│       └── 依赖 TASK-SHARE-MID-002
├── TASK-SHARE-LONG-004  CRM 深度集成
│       └── 依赖 TASK-SHARE-MID-003
└── TASK-SHARE-LONG-005  预测性 Lead Scoring
        └── 依赖 TASK-SHARE-LONG-001, TASK-SHARE-LONG-002
```

### 3.2 关键路径

```text
TASK-SHARE-SHORT-004 ──→ TASK-SHARE-MID-001 ──→ TASK-SHARE-LONG-001 ──┐
                     └──→ TASK-SHARE-MID-002 ──→ TASK-SHARE-LONG-003   │
                                                                       ▼
TASK-SHARE-SHORT-002 ──→ TASK-SHARE-MID-003 ──→ TASK-SHARE-LONG-004    │
                                                                       ▼
TASK-SHARE-SHORT-001 ──→ TASK-SHARE-LONG-002 ───────────────────────→ TASK-SHARE-LONG-005
```

---

## 4. 各阶段目标与验收标准

### 4.1 短期（1-2 周）

**目标**：消除当前线上最严重的功能可用性、安全与数据质量缺陷。

| 任务 | 关键交付 | 验收标准 |
|---|---|---|
| TASK-SHARE-SHORT-001 | 公共 AI 端点 + viewer flag 控制 | 匿名用户仅在 `ai_copilot_enabled=true` 时可用 AI；AI 会话按 link+visitor 隔离 |
| TASK-SHARE-SHORT-002 | 通知按 link creator 发送 + 前端开关 | 通知收件人正确；用户可关闭邮件通知 |
| TASK-SHARE-SHORT-003 | 安全审计事件表 + 异常告警 | 密码/验证码失败、过期/越权访问被记录；异常模式可触发告警 |
| TASK-SHARE-SHORT-004 | 访问与页面浏览去重 | 30min 内同一 visitor 重复 open 去重；5min 内同一页重复 view 去重 |

### 4.2 中期（2-6 周）

**目标**：让实现与设计语义对齐，补齐事件、通知、安全能力。

| 任务 | 关键交付 | 验收标准 |
|---|---|---|
| TASK-SHARE-MID-001 | Key Page Views 按关键词匹配 | 后端评分使用关键词识别财务/团队/价格/安全页 |
| TASK-SHARE-MID-002 | 扩展事件体系 | 补齐 forward/return/scroll/ai 相关事件；前端统一上报 SDK |
| TASK-SHARE-MID-003 | 通知规则引擎 | 支持首次打开、重复关键页、转发、异常访问规则；10 分钟合并 |
| TASK-SHARE-MID-004 | 动态水印 | Canvas 渲染时叠加邮箱+时间+IP 哈希水印 |
| TASK-SHARE-MID-005 | 签名 URL | 页面图片与下载 URL 带 HMAC 签名，过期失效 |

### 4.3 长期（6-12 周）

**目标**：从规则驱动升级为 AI/实时驱动的意图洞察系统。

| 任务 | 关键交付 | 验收标准 |
|---|---|---|
| TASK-SHARE-LONG-001 | Heat Score 时间衰减 + 可配置权重 | 评分随时间衰减；各 circle 权重可实验调整 |
| TASK-SHARE-LONG-002 | AI 问题意图分析 | 对 AI 问题做主题分类、重复检测、紧迫度评分 |
| TASK-SHARE-LONG-003 | 实时推送 | WebSocket/SSE 推送事件、信号、通知；Dashboard 实时更新 |
| TASK-SHARE-LONG-004 | CRM 深度集成 | HubSpot/Salesforce 写入 timeline、更新 deal stage、创建 task |
| TASK-SHARE-LONG-005 | 预测性 Lead Scoring | 基于历史转化数据输出成交概率 |

---

## 5. 资源与依赖假设

- **人力**：每个 Sprint 建议 1 名后端 + 1 名前端/全栈，AI 任务需要 ML 经验。
- **基础设施**：长期任务需要 Kafka/Redis Stream/WebSocket 支持；若当前基础设施未准备，需先加 `TASK-SHARE-INFRA-001`。
- **数据**：预测性 Lead Scoring 依赖足够的转化样本；若样本不足，可先用规则/启发式模型替代。
- **第三方**：CRM 深度集成需要 HubSpot/Salesforce sandbox 账号。

---

## 6. 风险与注意事项

1. **热修复优先**：TASK-SHARE-SHORT-001 和 TASK-SHARE-SHORT-002 属于线上功能缺陷，建议立即执行。
2. **数据迁移**：去重与 Key Page Views 修正会改变历史评分，需明确是否回溯计算或仅作用于新数据。
3. **性能影响**：扩展事件体系与实时推送会增加写入与计算负载，需同步评估数据库索引与缓存策略。
4. **隐私合规**：动态水印、签名 URL、AI 问答分析涉及 PII，需确保符合 GDPR/CCPA/数据安全法。
5. **向后兼容**：`permission_type` 等 legacy 字段不要移除，仅逐步 deprecate。

---

## 7. 关联资源

- 缺口分析报告：`docs/reviews/document-sharing-design-vs-implementation-gap-report.md`
- 任务模板：`docs/templates/AGENT-TASK-template-v2.md`
- 当前版本任务目录：`docs/tasks/agent-tasks-v2.1.3/`
- PRD：`docs/backup/PRD-v2.1.0.md`
- TDD：`docs/backup/TDD-v2.1.0.md`
- Heat Score 算法：`docs/backup/HEAT-SCORE-ALGORITHM-v2.1.1.md`

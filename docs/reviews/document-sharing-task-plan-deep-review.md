# DealSignal 文档分享业务缺口修复任务计划：深度评审

> 评审角色：资深架构师 + 高级产品经理  
> 评审对象：`docs/tasks/document-sharing-gap-remediation/README.md` 及其 18 个任务文件  
> 评审日期：2026-07-08  
> 计划版本：v1.3.0

---

## 0. 总体判断

当前任务计划相比初版已经大幅收敛：剔除了伪需求（LONG-005 预测性 Lead Scoring）、合并了重复任务（MID-004 并入 MID-006）、把 Access Tab 的占位开关转为了真实任务，并保留了设计文档 `huntress-spectre-falcon.md` 的核心语义。**作为下一阶段 backlog 是合格的，但作为可立即进入开发的执行计划，仍有几个关键的架构级与产品级风险尚未消除。**

核心结论：

1. **P0 链路存在单点瓶颈**：SHORT-005（Deal Room / 文档链接后端核心）是大量下游 P0/P1 任务的硬依赖，但它仍处于“部分完成”且工作量 L，容易阻塞前端闭环。
2. **数据库迁移编号冲突风险极高**：多个任务自行规划了 `047`/`048`/`049`/`050` 迁移文件，并行分支几乎必然冲突。
3. **权限模型维度被混淆**：MID-009 把“文件收集”意图塞进了 `permission_type`，会与既有访问策略产生组合爆炸。
4. **异步/队列/可观测基础设施未显式化**：多个任务假设“通知异步”，但计划中缺少对 worker、死信、重试、监控的落地任务。
5. **数据规模与合规风险被低估**：事件体系扩展、Q&A、文件请求、上传文件会快速堆积 PII，计划缺少跨任务的 retention 与隐私设计。

下面从产品策略、架构设计、风险矩阵、可执行建议四个维度展开。

---

## 1. 产品策略评审（PM 视角）

### 1.1 方向正确：从“功能补齐”到“成交效率”

计划明确把目标限定为“直接提升成交效率或安全可信度”，这是对的。保留的 16 个 active 任务中，Sharing Core、Access Rules、Invite、Email、Security Audit、Signed URL、Watermark 都是**成交前的信任基础设施**，优先级合理。

### 1.2 P0 过于拥挤，Wave 1/2 不可执行

当前 P0 任务有 5 个，Wave 1（2 周）安排 3 个、Wave 2（2 周）安排 4 个。按现有工作量估算：

| Wave | 任务 | 工作量 | 关键依赖 |
|---|---|---|---|
| Wave 1 | SHORT-005 / SHORT-002 / SHORT-004 | L + S + M | 005 是后续所有任务父依赖 |
| Wave 2 | SHORT-006 / SHORT-007 / SHORT-008 / SHORT-009 | L + M + M + M | 全部依赖 005/006 |

**问题**：Wave 2 的 4 个任务都改 `PublicViewerPage` / `RightSidebar` / `AccessTab`，前端会严重冲突；同时 4 个 M/L 全栈任务在 2 周内由 1 名前端 + 1 名后端完成不现实。

**建议**：
- 把 Wave 2 拆成两周一个：先只做 **SHORT-006（Share/Invite/Access 弹窗收尾）+ SHORT-007（邮件异步 + 请求访问）**。
- **SHORT-008 / SHORT-009 延后到 Wave 3**，因为它们依赖 006/007 的 UI 框架（RightSidebar tab 体系、owner 管理入口）。
- 或者为 Wave 2 增加 1 名全栈人力。

### 1.3 命名混淆：两个 File Requests

- **SHORT-009**：访客 → owner 的“资料/文件请求”。
- **MID-009**：owner → 第三方的“文件收集链接”。

两者都被称为 File Request，会让销售、客服、文档、测试用例产生歧义。

**建议**：
- SHORT-009 改名为 **“访客资料请求（Visitor Material Request）”**。
- MID-009 改名为 **“文件收集链接（File Collection Link）”** 或 **“第三方上传链接”**。
- Access Tab 中 `fileRequestsEnabled` 只对应 SHORT-009；MID-009 通过 Share Tab 的 Link Type 选择。

### 1.4 Index File 自动生成（MID-008）优先级偏高

MID-008 被标为 P1，但它：
- 强依赖 LLM（OPENAI_API_KEY），如果客户未配置 AI，则功能完全不可用。
- 与 SHORT-001 AI Copilot 的能力有重叠：两者都读文档、都生成摘要。
- 会进一步增加 RightSidebar 的 tab 数量。

**建议**：降级为 **P2**，并在实现上做成“AI Copilot 摘要的缓存化/静态化”而不是独立大功能。这样复用 chunk 与 prompt，减少维护面。

### 1.5 Owner 管理入口分散

- SHORT-008 的 owner 回复入口：Analytics Tab 或新 Tab（待定）。
- SHORT-009 的 owner 审批入口：Analytics Tab 或新 Tab（待定）。
- MID-009 的 owner 审批入口：Analytics Tab。
- MID-007 本身就在新建 Analytics Tab。

**风险**：Analytics Tab 会被塞入“数据图表 + 访客列表 + Q&A 回复 + 文件请求审批 + 上传文件审批”，职责过载。

**建议**：在 MID-007 中把 Analytics Tab 升级为 **“Link Management / 管理” Tab**，下设子视图：
- Overview（数据）
- Visitors（访问者）
- Questions（Q&A）
- Requests（文件请求）
- Uploads（文件收集）
- Invitations（邀请）

这样 SHORT-008/009/MID-009 的 owner 入口都有统一归处。

---

## 2. 架构评审（架构师视角）

### 2.1 数据库迁移：必须集中编排

任务文件里出现的迁移文件：

| 任务 | 迁移文件 |
|---|---|
| SHORT-005 | `046_invitation_token_hash.up.sql` |
| SHORT-007 | `047_link_access_requests.up.sql` |
| SHORT-008 | `047_link_qa_enabled.up.sql` |
| SHORT-009 | `048_link_file_requests.up.sql` |
| MID-008 | `049_link_index_files.up.sql` |
| MID-009 | `050_file_request_links.up.sql` |

**问题**：
- SHORT-007 与 SHORT-008 都占 `047`。
- 多个任务同时新增 `links` 列：`qa_enabled`、`file_requests_enabled`、`index_file_enabled`、`target_folder_path`，并扩展 `permission_type` 枚举。
- 如果按“每个任务一个分支”执行，合并时迁移编号、列添加顺序、回滚脚本都会冲突。

**建议**：
- 新增一个 **INFRA/Schema 任务**（如 `TASK-SHARE-INFRA-001`），统一负责 v1.3 所有 schema 变更：
  - `links` 表新增列（`qa_enabled`、`file_requests_enabled`、`index_file_enabled`、`target_folder_path`、`security_version`、`link_type` 等）。
  - 新建表（`link_access_requests`、`link_visitor_questions`、`link_file_requests`、`link_index_files`、`link_uploaded_files` 等）。
  - 扩展或重构 `permission_type` / 新增 `link_type`。
- 其他任务只写业务逻辑，不新增 migration。
- 统一 migration 编号在一个 epic 分支中顺序提交，避免并行冲突。

### 2.2 权限模型：`permission_type` 不应承载“链接用途”

MID-009 把 `permission_type` 从访问策略枚举扩展为：

```sql
('public','email_required','whitelist','password','file_request')
```

**问题**：`file_request` 不是访问策略，而是链接用途。未来如果再增加“NDA gate”、“问卷收集”，枚举会不断膨胀，且与 `require_password`、`require_email` 等字段组合后语义矛盾。

**建议**：
- 新增 `link_type` 或 `link_purpose` 字段，取值：`view`（默认）、`file_request`。
- `permission_type` 保持为访问策略枚举：`public / email_required / whitelist / password`。
- `file_request` 链接必须满足 `deal_room_id IS NOT NULL` 且 `document_id IS NULL`。
- Access Tab 根据 `link_type` 动态显隐字段：file_request 链接隐藏 `fileRequestsEnabled`（inbound）但显示 `target_folder_path`。

### 2.3 异步通知基础设施未显式落地

SHORT-007、SHORT-008、SHORT-009、MID-009 都要求“通知异步”，但计划中没有任何任务专门建设或验证异步 worker。

**需要确认的问题**：
- 当前 `apps/api/internal/notification/worker.go` 是否以常驻 goroutine / cron / 独立进程运行？
- 是否有死信队列、重试策略、幂等键？
- 如果 worker 未部署，写入 `notifications` 表只是“延迟发送”而非真正异步。

**建议**：
- 若现有基础设施不足，新增 `TASK-SHARE-INFRA-002`：**可靠异步通知 worker**。
- 验收标准：通知入队 → worker 消费 → 重试 3 次 → 死信表；API 不阻塞。
- 所有依赖通知的业务任务（007/008/009）把“接入 INFRA-002”作为验收项。

### 2.4 会话安全：security_version 必须先于所有 session 功能

SHORT-005 提出用 `security_version` 替代 `updated_at` 做 session 失效。这是好设计，但：
- 它本身是一个 schema + service + handler 的全链路改造。
- 如果 005 的“剩余收尾”不包含 `security_version`，则 006/007/008/009 中所有“规则变更后旧 session 失效”的需求都无法实现。

**建议**：把 SHORT-005 明确拆成两个可验收的子任务：
- **SHORT-005-A**：token hash + `security_version` session 失效（硬安全）。
- **SHORT-005-B**：`link_access_requests` 请求访问 CRUD + 审批联动（业务闭环）。

只有 005-A 全绿后，才允许 006/007/008/009 进入开发。

### 2.5 签名 URL 与 viewer 体验的冲突

MID-005 设计签名 URL 有效期 15 分钟，用于页面图片和下载。问题在于：
- 用户在 viewer 中停留可能超过 15 分钟，翻页时图片 URL 会失效。
- 浏览器/ CDN 无法缓存签名 URL，每次都要重新生成。
- 如果同时做水印（MID-006），服务端动态水印 + 签名 URL 会让 MinIO/图片服务压力倍增。

**建议**：
- 对“页面图片”使用 **session-scoped signed cookie** 或短期 access token（如 1 小时、可刷新），而不是逐 URL 签名。
- 对“下载 URL”保留 15 分钟一次性/短时效签名。
- 在前端实现签名过期刷新逻辑（viewer 心跳或翻页前 refresh）。
- 把服务端动态水印作为可选实验，默认只做客户端文字水印 + 审计日志。

### 2.6 事件体系扩展会带来数据量激增

MID-002 计划新增 `scroll_depth_recorded`、`ai_question_asked`、`forward_signal`、`return_visit` 等事件。结合 SHORT-004 去重、SHORT-008 Q&A、SHORT-009 文件请求，未来 `access_logs` 及相关事件表会快速膨胀。

**当前计划缺少**：
- 事件 TTL / 分区策略。
- 采样率配置（尤其是 scroll depth）。
- 索引设计（link_id + visitor_id + created_at 复合索引）。
- 事件 schema 版本化（当前 enum 扩展方式成本高）。

**建议**：
- 新增 `TASK-SHARE-INFRA-003`：**事件存储与 retention 策略**。
- 对高频低价值事件（scroll depth）采用采样 + 单独表/分区，不要直接塞进 `access_logs`。
- 在 enum 之外增加 `metadata JSONB` 字段，避免每次新事件都改 schema。

### 2.7 Heat Score 与 Key Page 语义变更需要版本化

MID-001 把 `key_page_views` 从“停留 ≥3s”改为“关键词匹配”，LONG-001 又引入时间衰减。两者都会改变 Dashboard 上的数字。

**风险**：用户会看到分数/排名突然变化，无法解释。

**建议**：
- API 返回 `heat_score_version` 字段，前端在 tooltip 中说明“v2 算法”。
- 保留旧指标 `engaged_page_views`（MID-001 已考虑），并在 Dashboard 提供切换或并显。
- 算法上线后做 A/B 影子验证：新算法并行计算但不立即替换展示，观察 1-2 周后再切换。

### 2.8 LLM 能力的可降级性

SHORT-001（AI Copilot）和 MID-008（Index File）都依赖 LLM。当前项目 `OPENAI_API_KEY` 是可选的。

**建议**：
- 所有 LLM 功能必须带 feature flag / 可用性检测。
- 当 LLM 未配置时：AI tab 隐藏、Index File 开关禁用并提示“需配置 AI”。
- 单元测试使用 mock LLM，不能依赖真实 API。

---

## 3. 风险矩阵

| 风险 | 等级 | 影响任务 | 说明 |
|---|---|---|---|
| 数据库迁移编号/列变更冲突 | 🔴 高 | 005/007/008/009/MID-008/009 | 并行分支几乎必然冲突 |
| SHORT-005 阻塞下游 P0 | 🔴 高 | 006/007/008/009 | 005 未收尾，前端闭环无法联调 |
| 异步通知基础设施缺失 | 🟠 中高 | 007/008/009 | “异步”可能只是写入 DB，无 worker |
| `permission_type` 维度污染 | 🟠 中 | MID-009 | 会让访问策略与链接用途纠缠 |
| PII 数据堆积与合规 | 🟠 中 | 003/007/008/009/MID-009 | IP、邮箱、UA 多处新增 |
| 签名 URL 15min 过期影响 viewer | 🟠 中 | MID-005 | 长会话中图片会失效 |
| RightSidebar tab 过多 | 🟡 中低 | 008/009/MID-008 | 产品体验风险 |
| Heat Score 算法变更用户困惑 | 🟡 中低 | MID-001/LONG-001 | 缺少版本化与影子验证 |
| 16 个 active 任务超出现有资源 | 🟠 中 | 全部 | 1 后端 + 1 前端过于乐观 |
| 长期任务依赖未准备的基础设施 | 🟡 中 | LONG-003/LONG-004 | realtime、CRM sandbox、队列 |

---

## 4. 可执行建议

### 4.1 立即调整任务结构

1. **新增 `TASK-SHARE-INFRA-001`：Schema 统一编排**
   - 负责所有 v1.3 schema 变更与迁移编号分配。
   - 作为第一个合并进 epic 分支的任务。

2. **新增 `TASK-SHARE-INFRA-002`：可靠异步通知 worker（如需要）**
   - 确认现有 worker 是否满足；不满足则新建。
   - 验收：死信、重试、幂等、API 不阻塞。

3. **拆分 SHORT-005**
   - `SHORT-005-A`：token hash + `security_version`（硬安全）。
   - `SHORT-005-B`：`link_access_requests` 请求访问闭环。

4. **重构 MID-009 权限模型**
   - 引入 `link_type` 字段，`permission_type` 保持纯访问策略。
   - 更新 migration、service、AccessTab、ShareTab。

5. **重命名 SHORT-009 / MID-009**
   - 消除“File Request”歧义。

6. **降级 MID-008 为 P2**
   - 或合并进 AI Copilot 摘要缓存策略。

### 4.2 重新编排执行波次

建议把 16 个 active 任务重新分配到 5 个 2 周迭代， realistic 人力为 **2 后端 + 2 前端**：

| Wave | 任务 | 目标 |
|---|---|---|
| **Wave 1** | INFRA-001、SHORT-005-A、SHORT-002、SHORT-004 | Schema 定型 + 核心安全收尾 |
| **Wave 2** | SHORT-005-B、SHORT-006、SHORT-007 | 邀请/请求访问/邮件闭环 |
| **Wave 3** | SHORT-008、SHORT-009、INFRA-002 | Q&A、访客资料请求、通知 worker |
| **Wave 4** | MID-005、MID-006、MID-002 | 安全加固、水印、事件体系 |
| **Wave 5** | MID-007、MID-009、MID-001 | 分析/生命周期、文件收集、Key Page |
| **Wave 6+** | MID-008、LONG-001~004 | AI index、算法优化、实时、CRM（按需） |

> 注：原计划的 2-4 周短期 + 4-8 周中期共 10 周；按 realistic 资源需要 **10-12 周**完成 active 任务。

### 4.3 每个任务补充“运维级”验收项

在当前验收标准基础上，所有 P0/P1 任务增加：

- **Feature flag**：是否可灰度/关闭？
- **Metrics / Logs**：至少 1 个业务指标 + 1 个错误率指标。
- **Rollback plan**：schema 变更是否可回滚？旧客户端是否兼容？
- **性能预算**：API p99、前端 bundle 增量、DB 查询耗时。
- **Accessibility**：动画遵循 `prefers-reduced-motion`；水印不破坏屏幕阅读器。

### 4.4 引入跨任务合规任务

新增 `TASK-SHARE-COMPLIANCE-001`：**Sharing 链路 PII 最小化与 retention**：
- 明确 `access_logs`、`security_events`、`link_visitor_questions`、`link_file_requests`、`link_uploaded_files` 的保留期限。
- 统一 visitor_id / IP hash 生成规则。
- 导出/删除流程（GDPR/CCPA/数据安全法）。

### 4.5 分支与合并策略

当前“每个任务一个分支”在文件高度重叠（`link/service.go`、`handler.go`、`RightSidebar.tsx`、`AccessTab.tsx`）时不可行。

**建议**：
- 建立长生命周期的 epic 分支 `feat/share-v1.3`。
- 子任务分支从 epic 切出，频繁 rebase/merge 回 epic。
- 每个子任务 PR 必须跑 `gitnexus_impact`（项目 AGENTS.md 要求）。
- 合并到 main 前对 epic 分支跑一次完整的 `go test ./...` + `pnpm test` + `./e2e-test.sh`。

---

## 5. 需要产品/架构明确的开放问题

1. **Owner 管理入口**：Analytics Tab 是否改名为 Management Tab？Q&A / 文件请求 / 上传文件 / 邀请是否统一在此？
2. **`permission_type` 重构**：是否接受新增 `link_type` 字段？还是需要严格遵循设计文档的 `permission_type`？
3. **异步通知**：现有 notification worker 是否已常驻运行？是否需要独立 INFRA 任务？
4. **水印策略**：是否接受“服务端生成文本 + 客户端渲染 + 审计日志”作为 MVP，弱化 Print Screen 拦截？
5. **签名 URL**：是否接受 session-scoped signed cookie 替代逐 URL 签名？
6. **Index File**：是否愿意降级为 P2 或与 AI Copilot 摘要复用？
7. **资源投入**：是否能接受把 16 个 active 任务拆到 5-6 个迭代（10-12 周）？

---

## 6. 结论

当前任务计划在**需求筛选、依赖关系、验收标准**上已经做得较好，但距离“可立即进入并行开发”还差三步：

1. **Schema 与权限模型必须先定型**（INFRA-001 + MID-009 重构）。
2. **核心安全收尾必须先完成**（SHORT-005-A token hash + security_version）。
3. **异步/可观测/合规基础设施必须显式化**（INFRA-002、COMPLIANCE-001）。

完成这三步后，再按重新编排的 Wave 推进，才能避免中期大规模返工。

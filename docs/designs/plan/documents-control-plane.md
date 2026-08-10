# Documents 控制面 · 详细设计与迭代计划

> **Documents 是交易内容控制面（库存 → 分发 → 晋级尽调 → 驱动主理人动作），不是 DocSend 克隆，也不是第二套 VDR。**

状态：设计定稿 · Phase A Trust 进行中（A-1/A-2/A-3 已在 WIP；待拆 PR + A-7 部署）  
日期：2026-08-10  
承接：Cursor canvases `documents-commercial-review` · `documents-design-blueprint` · `documents-nav-e2e-map` · `documents-iteration-plan`  
范畴：左侧导航 **Documents（general 库）** + Sharing 模式 + Archive 吊销语义；**不含** Deal Room 内 VDR 重构、Agreements 产品化大改。

---

## 1. 北极星与反目标

### 1.1 北极星

在 workspace 内，主理人能把 general 文档当作**可分发、可吊销、可晋级**的内容资产：30 秒内发出受控链接；归档后访客默认不可再读；热度驱动下一动作；需要尽调时一键晋级 Deal Room。

### 1.2 成功判据

| ID | 判据 | 度量 |
|----|------|------|
| S1 | Archive 默认吊销访客读权限 | 归档后 Access 文档列表不含该 doc；signed-url/pages/download → deny |
| S2 | 库内完成首分享 | ≥70% 新建 document-link 不离开 Documents（或 Share drawer） |
| S3 | Trial 时间到首外链浏览 | P50 &lt; 5 min（upload → first public view） |
| S4 | 库 → Room 晋级 | 7 日内 attach 率作为 expansion 指标（基线后设目标） |
| S5 | Heat 可行动 | 有 heat 的 ready 行展示 ≥1 个 suggested action |

### 1.3 反目标（明确舍弃）

- 在 Documents 内做文件夹 ACL / Q&A / Spaces 式 VDR
- Documents 单独 SKU / 按文件数售卖
- 与 DocSend 功能清单式对标（告警生态、Dropbox SSO 等）
- 「已归档但仍可打开」且无明示的软归档
- 再造第二套分享收件箱（Sharing 是唯一主脊）
- 放开 `agreement` 进入 Deal Room

---

## 2. GitNexus 影响摘要（落地前）

索引：DealSignal（`npx gitnexus analyze --force`，2026-08-10）。

| 变更符号 | Upstream impact | Risk | 关键执行流 / 调用方 |
|----------|-----------------|------|---------------------|
| `listAuthorizedDocuments` | direct 2 · processes Access, RecordEvent | LOW→**HIGH 业务** | `documentsForAccessResponse` → Access；`AuthorizedDocumentIDs` → Ask AI / events |
| `evaluateLinkDocumentAccess` | direct 2 | LOW→**HIGH 业务** | `ensurePublicDocumentAccess` / `verifyLinkDocumentAccess` → PublicSignedURL, PublicDownloadURL, PublicDocumentPages |
| `Handler.Archive` | upstream callers 0（HTTP 注册） | LOW 图 · **HIGH 产品** | 仅改 status；吊销靠访客门禁而非改 Archive 本身 |
| `DocumentsTable` | DocumentsPage, AgreementDocumentsPage | LOW | FE 主表面；Agreements 复用需隔离回归 |
| `useDocumentColumns` | → DocumentsTable | LOW | 行 CTA / Archive confirm |
| `documentsSharePath` (web) | 9 direct · 6 modules | **CRITICAL** | LinksTable, LinkDetail, actionNavigatePath, Dashboard, StepReview — **勿改语义，只可增参** |
| `Handler.Access` | downstream 巨大 | **CRITICAL** | 勿重构 Access；只经 listAuthorizedDocuments 过滤归档 |

**落地约束**

1. 访客吊销优先改 `listAuthorizedDocuments` + `evaluateLinkDocumentAccess`（WIP 已有，需合入 + 测试 + 部署），**不要**大改 `Access`。
2. `documentsSharePath` 保持 `tab=shared` 契约；深链调用方多，禁止破坏 query 语义。
3. FE 改动集中在 `DocumentsTable` / `DocumentsColumns` / 新建 Share drawer；Agreements 路径 `category=agreement` 必须回归。

### 2.1 关键模块（GitNexus clusters）

| Module | 角色 |
|--------|------|
| Upload | Archive/List/Create/DeleteImpact |
| Link | Access、授权文档、公开资产、Ask |
| Share | DocumentsPage/Table/Detail |
| Dealroom | AddDocument 晋级 |
| Ingestion | ready 门禁（Share CTA 仅 ready） |
| Insights | documentsSharePath 消费方（勿回归） |

---

## 3. 目标体验与信息架构

### 3.1 三模式（文案可渐进，路由可暂用现 tab）

| 模式 | 现路由 | 职责 |
|------|--------|------|
| Library | `/documents`（无 tab / tab 清除） | ready general 库存 |
| Sharing | `/documents?tab=shared` | 库文档出站链接；Create/Disable |
| Lifecycle | `/documents?tab=archived` | 已归档；Unarchive / Delete |

Sibling 不变：`agreement-documents`、`deal-rooms`。

### 3.2 行级 CTA 契约

| 元素 | 行为 |
|------|------|
| 主 CTA | **Share**（打开库内 Share drawer；默认策略创建并复制） |
| 次 CTA | **Promote** → 现有 AddToDealRoomDialog |
| Overflow | Download · Archive · Delete ·（详情） |
| Archived | Unarchive + Delete；Share/Promote/Download disabled |
| LINKS · View | 仍走 `documentsSharePath({ documentId })` |

### 3.3 对象图

```
Document(general)
  ├─ owns → Link(s) [document / link_documents]
  │           └─ gates → Visitor Access → pages/signed-url/download/Ask
  └─ promotes → deal_room_documents → category=deal_room

Archive(document) ⇒ visitor evaluate + listAuthorized 拒绝该 doc
Disable(link)     ⇒ 既有 link.is_active 门禁（保留）
Delete(document)  ⇒ 既有级联 soft-delete links / link_documents / room rows
```

---

## 4. 技术方案

### 4.1 Phase A — Trust + Share（P0）

#### A1. Archive = 访客吊销（默认）

**行为**

- `listAuthorizedDocuments`：跳过 `status == "archived"`（deal-room meta / link_documents / legacy）。
- `evaluateLinkDocumentAccess`：membership 通过后 `GetDocumentByID`；`archived` → deny。
- 宿主 UI：Archive 确认框展示 `active_link_count`（复用 `getDocumentDeleteImpact` 或轻量 count）；文案明示「访客将无法打开」。
- **不做** Phase A：`keep_links_live` 高级选项（列入 Phase C 可选）。
- **不做**：改 `ServeSignedFile` 重查 DB（已签发 URL 在 TTL 内仍可读，接受 ≤15min；新 Access/SignedURL 立即失效）。

**文件**

- `apps/api/internal/link/authorized_docs.go`（+ tests）
- `apps/api/internal/link/document_access.go`（+ tests）
- `apps/web/src/components/documents/DocumentsColumns.tsx`（confirm copy）
- i18n：`documents.json` en + zh-CN

**验收**

- Unit：archived 不进 Access 列表；PublicSignedURL deny。
- API/E2E：创建 link → archive doc → public Access 无该 doc / signed-url 403。
- 合入后必须重建并部署 API（WIP 未部署则行为不变）。

**风险**：多文档 link 仅隐藏归档文档，其余仍可读 — **预期**。全 link disable 不在默认路径。

#### A2. 库内 Share drawer（MVP）

**行为**

- Library 行主 CTA「Share」打开 drawer（或 Dialog），预填 `documentId`。
- MVP：复用现有创建 link API + 安全默认值（与 `/links/new` 默认一致）；成功后 copy URL + toast；「高级设置」链到 `LinkDetail` 或现有 bundle/new。
- 允许过渡：drawer 内嵌精简 `LinkShareDialog` / 抽取 create-defaults，**避免**再造完整 policy UI。
- Agreements 表：不启用该主 CTA（或保持现有行为）。

**文件（预期）**

- 新建：`apps/web/src/components/documents/DocumentShareDrawer.tsx`（名可调整）
- `DocumentsColumns.tsx` / `DocumentsTable.tsx`
- 可能复用：`LinkShareDialog`、`api.createLink*`、`documentsCreateLinkPath`（高级出口）
- i18n：`documents` + `linkShare` / `links`

**验收**

- ready 文档：不离开 `/documents` 完成创建+复制。
- `documentsSharePath` 既有调用方无回归。
- 单元 + 组件测试；可选 e2e smoke。

#### A3. 上传后 Share CTA

**行为**

- Upload 成功 / `documents:uploaded` 后：toast action「Share」打开 A2 drawer（或 navigate Sharing + documentId）。
- 仅 `status` 将变为 ready 时可延迟 enable（processing 时 CTA disabled + 文案）。

**文件**：`UploadPage` / `useDocumentUploadConflict` / `DocumentsTable` 事件处理。

#### A4. Archive 确认 UX

- 调用 delete-impact 或 `active_link_count`；0 links 时简化确认。
- Archived 行已禁用 Create link / Add to room / Download — 保持。

---

### 4.2 Phase B — Signal act + 包装（P1）

#### B1. Heat → 动作芯片

在 `DocumentsColumns` FILE/HEAT 区：

| 条件 | Chip |
|------|------|
| heat hot + 有 link | Chase / 打开 Sharing |
| cold + ready + 0 links | Share |
| ready + not in room | Promote |
| 有 pending access / ask（若数据可得） | Review |

数据：优先用现有 row（heat, links, views）；Ask/access 若需额外 API，**单次** batch 或仅 Shared 模式展示，避免 List N+1。

#### B2. 包装文案

- Library 空态：库存说明 + CTA Upload / 指向 Deal Rooms「尽调在 Room」。
- Sidebar 不改 IA；可选 subtitle / 首次引导。
- i18n 全量 en + zh-CN。

#### B3. Sharing 模式抛光

- `documentId` 过滤、空态、创建入口与 A2 对齐。
- 保持 `actionNavigatePath(link_access_request)` → `documentsSharePath`。

---

### 4.3 Phase C — Expand（P2，可选）

| ID | 项 | 说明 |
|----|----|------|
| C1 | Bulk archive / promote | 多选 |
| C2 | DocSend/Drive import | 迁移叙事 |
| C3 | Archive advanced：keep links live | 显式 opt-in，默认仍吊销 |
| C4 | ServeSignedFile 重验 status | 仅当安全问卷要求零 TTL 窗口 |

---

## 5. 迭代计划任务（可执行 backlog）

### Phase A — Trust + Share（建议 2–4 周）

| Task ID | 标题 | 类型 | 影响符号 / 文件 | 依赖 | 验收 |
|---------|------|------|-----------------|------|------|
| **A-1** | 合入访客归档门禁 | BE | `listAuthorizedDocuments`, `evaluateLinkDocumentAccess` | — | 单测绿；Access/PublicSignedURL 行为 |
| **A-2** | 归档门禁集成/E2E | Test | `apps/api` e2e 或 link tests；可选 web e2e | A-1 | 归档后访客不可读 |
| **A-3** | Archive 确认文案 + impact count | FE | `DocumentsColumns`, i18n | A-1 可并行 | 确认框展示链接数与吊销说明 |
| **A-4** | DocumentShareDrawer MVP | FE | 新组件 + Columns/Table | — | 库内创建+复制 |
| **A-5** | Share drawer 接 create API + 默认策略 | FE/API | 复用 create link | A-4 | 与现默认安全策略一致 |
| **A-6** | 上传成功 → Share CTA | FE | Upload / documents:uploaded | A-4 | toast/action 可开 drawer |
| **A-7** | 部署门禁：API 镜像含 A-1 | Ops | docker-compose / staging | A-1,A-2 | staging 手工验证截图场景 |
| **A-8** | 回归 Agreements + documentsSharePath 调用方 | Test | AgreementDocumentsPage；LinksTable deep link | A-3–A-6 | 无路径回归 |

### Phase B — Signal + Packaging（建议 2–3 周）

| Task ID | 标题 | 类型 | 依赖 | 验收 |
|---------|------|------|------|------|
| **B-1** | Heat/行动作芯片 | FE | A-8 | 规则表芯片可见可点 |
| **B-2** | Library/Sharing 空态与控制面文案 | FE+i18n | — | en/zh-CN |
| **B-3** | Sharing 与 drawer 入口统一 | FE | A-4,A-5 | 单一创建路径 |
| **B-4** | 埋点：share_from_library / archive_revoke | Analytics | A-5,A-1 | 事件可查 |
| **B-5** | Trial 漏斗基线报告 | GTM/Data | B-4 | S3 基线 |

### Phase C — Expand（择机）

| Task ID | 标题 | 依赖 |
|---------|------|------|
| **C-1** | 多选 bulk archive/promote | B 稳定 |
| **C-2** | 导入迁移 MVP | 产品排期 |
| **C-3** | keep_links_live 高级归档 | 安全评审通过 |
| **C-4** | Signed file 重验 archived | 仅合规强需求 |

---

## 6. 测试矩阵

| 层 | 范围 |
|----|------|
| Go unit | `authorized_docs_test`, `document_access_test`；Archive 后 Access 列表 |
| Go integration | link public Access + signed-url（有 DB） |
| FE unit | Columns archive confirm；Share drawer；Agreements 无 Share 主 CTA |
| E2E | archive → visitor deny；library share copy；documents?tab=shared deep link |
| 手工 | 多文档 link 仅缺归档文档；TTL 内旧 signed URL；Unarchive 恢复 |

---

## 7. 发布与风险

| 风险 | 缓解 |
|------|------|
| 运行中 API 无 A-1 | A-7 强制；发布说明 |
| 多文档 link 部分不可见 | 产品预期；文档说明 |
| Share drawer 与 `/links/new` 分叉 | 共用 create defaults；高级回 LinkDetail |
| `documentsSharePath` 误改 | CRITICAL 调用方；只增不改 |
| Agreements 回归 | A-8 必跑 |

**Rollback**：A-1 可 feature-flag（可选 `ARCHIVE_REVOKES_VISITOR_ACCESS=1` 默认 on）；FE drawer 可回退到 `/links/new`。

---

## 8. 非本次范围

- Deal Room Documents Tab / Knowledge Desk
- Visitor Ask 引擎改造（见 `visitor-ask-unified-product.md`）
- DocSend 级告警/邮件产品
- 左侧导航信息架构大改名（Library/Sharing/Lifecycle 可先文案后路由）

---

## 9. 决策记录

| 决策 | 选择 | 理由 |
|------|------|------|
| 吊销实现点 | 访客授权层，非禁用全部 link | 多文档 link 安全；改动面小于 Access 重构 |
| Share UX | Drawer MVP + 高级外链 | 匹配 DocSend JTBD，避免重做 policy |
| 已签发 URL | 接受 TTL 窗口 | 成本低；新请求立即失效 |
| 英雄产品 | Room + Ask | 商业评审结论 |

---

## 10. 参考符号速查

```
FE:  DocumentsTable, useDocumentColumns, documentsSharePath, actionNavigatePath,
     AddToDealRoomDialog, LinkShareDialog, LinksTable
BE:  upload.Handler.Archive|List|Create|DeleteImpact
     link.listAuthorizedDocuments, evaluateLinkDocumentAccess,
     ensurePublicDocumentAccess, Handler.Access, PublicSignedURL
SQL: ArchiveDocument, GetDocumentByID, ListLinkDocumentsByPublicToken,
     GetDocumentDeleteImpact
```

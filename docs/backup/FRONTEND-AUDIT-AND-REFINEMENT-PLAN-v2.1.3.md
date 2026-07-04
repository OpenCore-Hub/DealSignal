# DealSignal Web v2.1.3 前端细胞级审计与专利级产品设计方案

## 0. 执行摘要

本计划基于第二轮四位专家视角的细胞级审计（前端架构师、高级产品总监、UI/UX 与交互设计师、用户心理学专家、文档审计师），针对 v2.1.2 已修改代码进行再次梳理。

**结论**：v2.1.2 已取得显著进展——侧边栏中文化、Dashboard 精简、复制反馈统一、生产 MSW 隔离、错误处理覆盖主要页面、设计 Token 基本收敛。但仍有**三类关键债务**需要在本轮解决：

1. **文档基线失真**：v2.1.2 审计计划包含大量已修复的过时指控，IMPLEMENTATION-PLAN-v2.1.1.md 虚报完成度，API-SPEC 与前端请求层严重脱节。
2. **核心交易闭环仍有占位**：SmartLinkCreator 复制无反馈、Security 审计日志按钮无响应、InsightsSuggestions 写邮件按钮无响应、TopNav 铃铛/头像无账户菜单、brand logo 未真正持久化。
3. **工程架构尚未收敛**：18+ 处重复 fetch 样板、api.ts 仍 re-export 工具函数、oversized 组件未拆分、统一数据层缺失、heatScore topKeyPages 算法与文档不符。

本计划提出**分阶段专利级产品设计方案**，优先修正文档基线并补齐剩余阻塞项，再建立统一数据层与拆分 oversized 组件，最后做 UI/UX 细节打磨与测试落地。

---

## 1. 审计发现总览（第二轮）

### 1.1 已修复（v2.1.2 成果）

- 根路径 `/` 改为 workspace 选择页；单 workspace 自动跳转。
- 生产构建不再启动 MSW。
- 侧边栏导航全中文。
- Dashboard 摘要指标精简为 2 个。
- 最近文档限制 3 条并增加「查看全部」。
- 复制操作统一使用 `copyToClipboard` + toast 反馈。
- 表格行支持键盘操作。
- 主要页面均已覆盖错误处理与重试 UI。
- 移除了 `text-[10px]` 与大部分 `transition-all`。
- 文档详情右侧 sidebar sticky 已生效。
- 删除了孤立 `PermissionSlider.tsx` 与 `components/insights/InsightsOverview.tsx`。

### 1.2 剩余阻塞项

| 类别 | 问题 | 位置 |
|------|------|------|
| 无响应按钮 | Security「查看审计日志」无 onClick 也未 disabled | `routes/settings/security.tsx:139` |
| 无响应按钮 | InsightsSuggestions「写跟进邮件」无 onClick 也未 disabled | `routes/insights/suggestions.tsx:94-97` |
| 无响应按钮 | TopNav 通知铃铛无 onClick | `components/layout/TopNav.tsx:41-45` |
| 复制反馈缺失 | SmartLinkCreator 生成链接后用 `navigator.clipboard.writeText`，无 toast/图标变化 | `components/links/SmartLinkCreator.tsx:465` |
| 表单提交无反馈 | SettingsGeneral/Brand/Integrations 保存/切换无 toast，部分无 catch | `routes/settings/general.tsx`、`brand.tsx`、`integrations.tsx` |
| 品牌持久化 | brand logo 上传仅本地 `URL.createObjectURL`，保存时发送 blob URL | `routes/settings/brand.tsx:92-105` |
| 头像硬编码 | TopNav 头像写死 "JD"，无账户菜单 | `components/layout/TopNav.tsx:49-51` |
| 删除确认不一致 | DocumentDetail/LinksTable 使用 `window.confirm` | `DocumentDetail.tsx`、`LinksTable.tsx` |
| 算法错误 | heatScore `topKeyPages` 用页码字符串匹配关键词，逻辑错误 | `lib/heat/heatScore.ts:136-143` |
| API 层 Bug | `request` 强制 `Content-Type: application/json`，与 FormData 上传冲突 | `lib/api.ts:49-58` |
| API 不对齐 | 路径 `/api${path}`、无认证头、无分页/幂等，与 API-SPEC 不一致 | `lib/api.ts:49-58` |

### 1.3 工程债务

| 类别 | 问题 | 影响 |
|------|------|------|
| 重复 fetch 样板 | 18+ 处组件自行管理 loading/error/data/retry | 可维护性差、无缓存、无乐观更新 |
| api.ts 职责混合 | 仍 re-export formatters/calculations | 工具函数与网络层耦合 |
| oversized 组件 | SmartLinkCreator 486 行、DocumentsTable 317 行、DocumentDetail 311 行、DashboardPage 322 行 | 可读性差、难测试 |
| 重复逻辑 | 日期格式化、事件类型中文映射、日趋势聚合多处重复 | DRY 违规 |
| 无测试 | 项目无任何单元/E2E 测试 | 回归风险高 |

### 1.4 UI/UX 债务

| 类别 | 问题 | 位置 |
|------|------|------|
| 英文/中英混杂 | `views` 残留（DocumentDetail）、`Deal Rooms` 残留（deal-rooms 返回文案） | 多处 |
| 营销标题 | contacts detail 用 "360° 视图"、deal-rooms/new 用 "数据室模板引擎" | `contacts/detail.tsx`、`deal-rooms/new.tsx` |
| 键盘可达缺失 | HeatMap 项、ContactDetail 浏览文档列表不可键盘操作 | `HeatMap.tsx`、`contacts/detail.tsx` |
| 空状态不一致 | DocumentDetail 链接空态用普通 div、insights/pages 用 emoji、contacts/detail 用内联文字 | 多处 |
| 阴影/圆角不一致 | AIAssistant `rounded-2xl` vs AIChat `rounded-xl`、卡片 hover shadow-sm 残留 | 多处 |
| 移动端 | AI 面板高度固定 520px、表格列数过多 | `AIAssistant.tsx`、`AIChat.tsx`、表格 |

### 1.5 文档债务

| 文档 | 问题 |
|------|------|
| `FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.2.md` | 包含大量已修复的过时指控 |
| `IMPLEMENTATION-PLAN-v2.1.1.md` | 虚报完成度，列出的文件多数不存在或已改名 |
| `API-SPEC-v2.1.0.md` | 与前端请求层路径/响应/认证不一致 |
| `README.md` | 使用 `npm install` 但项目用 pnpm |
| 缺少 | CHANGELOG、组件指南、可访问性检查清单、API 集成说明、测试策略 |

---

## 2. 专利级产品设计方案

### 2.1 设计原则（重申与细化）

1. **零无响应元素**  
   任何可见按钮/链接必须有明确响应或明确不可用说明（disabled + Tooltip/Dialog）。不允许存在无 onClick 也未 disabled 的按钮。

2. **即时反馈统一化**  
   所有变更操作（保存、删除、启用/停用、复制、重置）统一通过 sonner toast 反馈，危险操作使用 Dialog 二次确认。

3. **键盘与屏幕阅读器优先**  
   所有可点击元素优先使用 `<Link>`/`<button>`；无法避免时使用 `role`/`tabIndex`/`onKeyDown`。所有图标按钮必须有 `aria-label`。

4. **中文 SaaS 语境彻底化**  
   界面文案 100% 中文（保留 AI/NDA/CRM 等行业缩写）；数据单位本地化为「次访问」「次浏览」。

5. **数据诚实与样本量透明**  
   AI 洞察、热度解释必须标注样本量或置信度，避免基于单次访问下结论。

6. **工程可维护性**  
   统一数据层、拆分 oversized 组件、集中格式化/计算逻辑、补单元测试。

### 2.2 本轮设计目标

| 目标 | 可量化指标 |
|------|------------|
| 零无响应按钮 | 0 个既无 onClick 也无 disabled 的 Button/菜单项 |
| 复制反馈 100% 覆盖 | 所有 clipboard 调用走 `copyToClipboard` |
| 删除/危险操作 Dialog 化 | 0 个 `window.confirm` |
| 错误处理 100% 覆盖 | 所有 async 提交操作有 try/catch + toast |
| 英文微文案清零 | 0 个 `views`、`返回 Deal Rooms` 等 |
| API 层适配 | `request` 支持 FormData、预留认证/token/pagination 扩展点 |
| 热度算法对齐 | `topKeyPages` 与算法文档一致 |
| 统一数据层 | 引入 `useAsyncData` hook，替换 50% 以上重复 fetch 样板 |
| 测试起步 | 为 `heatScore.ts`、`formatters.ts`、`calculations.ts` 补单元测试 |

---

## 3. 实施路线图

### Phase A：文档基线修正 + 阻塞项清零

1. **更新 README.md**
   - 安装命令改为 `pnpm install`。

2. **更新 FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.2.md → v2.1.3**
   - 删除已修复指控，补充剩余问题。

3. **阻塞按钮处理**
   - Security「查看审计日志」：disabled + title「审计日志需后端支持」。
   - InsightsSuggestions「写跟进邮件」：disabled + title「邮件发送需后端支持」。
   - TopNav 通知铃铛：disabled + title「通知中心即将上线」。
   - SmartLinkCreator 复制：改用 `copyToClipboard` 并增加图标反馈。

4. **表单提交反馈与错误处理**
   - SettingsGeneral、SettingsBrand、SettingsIntegrations、DealRoomsNew、SmartLinkCreator docs 加载加 try/catch + toast。

5. **删除确认 Dialog 化**
   - DocumentDetail 删除、LinksTable 删除改用 shadcn Dialog。

6. **文案清理**
   - DocumentDetail `views` → `次访问`。
   - deal-rooms 返回文案 `Deal Rooms` → `数据室`。
   - contacts detail tab「360° 视图」→「概览」。
   - deal-rooms/new 标题「数据室模板引擎」→「新建数据室」。

7. **TopNav 头像/账户菜单**
   - 头像显示真实 workspace 首字母或用户 API 首字母（当前先使用 workspace 首字母，替代硬编码 JD）。
   - 增加账户下拉菜单（占位项 disabled + title）。

8. **Brand Logo 持久化**
   - Logo 上传调用 `api.uploadDocument` 或专用上传接口，保存返回的 CDN URL。

### Phase B：API 层与算法修复

1. **修复 `request` Content-Type**
   - 仅在非 FormData 时设置 `Content-Type: application/json`。

2. **API 适配层扩展**
   - `request` 支持可选 `token`、预留 `requestId`/`idempotencyKey` 参数。
   - 保持 `/api${path}` 不变，但新增注释说明与真实后端路径的迁移方式。

3. **修复 heatScore topKeyPages**
   - 根据 `HEAT-SCORE-ALGORITHM-v2.1.1.md` 实现：基于页面文本/标题关键词匹配，而非页码字符串。

### Phase C：统一数据层与组件拆分

1. **引入 `useAsyncData` hook**
   - 封装 `loading/error/data/refetch/cancelled`。
   - 逐步替换 DashboardPage、DocumentsTable、LinksTable、ContactsPage、DealRoomsPage 等重复样板。

2. **拆分 oversized 组件**
   - `SmartLinkCreator`：拆分 PermissionPanel、SecurityOptions、ScoreDisplay、LinkPreview。
   - `DocumentsTable`：提取 `columns.tsx`。
   - `DocumentDetail`：提取 `aggregateVisitors`、`heatDistribution`、LinksList。

3. **集中格式化/计算逻辑**
   - `formatShortDate` 统一日期格式化。
   - `getActivityLabel` 统一事件类型中文映射。
   - `groupLogsByDay` 统一日趋势聚合。

### Phase D：UI/UX 细节打磨

1. **空状态统一**
   - DocumentDetail 链接空态、DocumentContent 无 pageCount、contacts/detail 空态统一使用 `EmptyState`。
   - insights/pages 空态 emoji 改为 Phosphor 图标。

2. **键盘可达补齐**
   - HeatMap 项、ContactDetail 浏览文档列表改为 `<Link>` 或加 role/tabIndex/onKeyDown。

3. **移动端适配**
   - AIAssistant/AIChat 高度改为 `max-h-[calc(100dvh-2rem)]`。
   - 表格设置 `min-w-[640px]` 确保小屏横向滚动。

4. **视觉统一**
   - AIAssistant/AIChat 统一 `rounded-xl` + `shadow-lg`。
   - 移除剩余卡片 `hover:shadow-sm`，改为 `hover:bg-muted/50`。
   - Sidebar 折叠态图标增加 Tooltip。

### Phase E：测试落地

1. **单元测试**
   - `lib/heat/heatScore.ts`：权重、阈值、topKeyPages。
   - `lib/formatters.ts`：formatFileSize、formatDuration、formatRelativeTime。
   - `lib/calculations.ts`：calculateUniqueVisitors、calculateHeatDistribution。

2. **E2E/回归测试**
   - Playwright 关键路径：workspace 选择 → dashboard → documents → document detail → links。

---

## 4. 详细修改清单（Phase A + B 优先）

| # | 文件 | 修改内容 | 所属 Phase |
|---|------|----------|------------|
| 1 | `README.md` | `npm install` → `pnpm install` | A |
| 2 | `src/routes/settings/security.tsx` | 审计日志按钮 disabled + title | A |
| 3 | `src/routes/insights/suggestions.tsx` | 写跟进邮件按钮 disabled + title | A |
| 4 | `src/components/layout/TopNav.tsx` | 通知铃铛 disabled + title；头像改为 workspace/user 首字母；增加账户下拉菜单 | A |
| 5 | `src/components/links/SmartLinkCreator.tsx` | 复制生成链接改用 `copyToClipboard` + 图标反馈 | A |
| 6 | `src/routes/settings/general.tsx` | save 加 try/catch + toast | A |
| 7 | `src/routes/settings/brand.tsx` | save 加 try/catch + toast；logo 上传持久化 | A |
| 8 | `src/routes/settings/integrations.tsx` | toggle 加 try/catch + 乐观更新回滚 + toast | A |
| 9 | `src/routes/deal-rooms/new.tsx` | create 加 try/catch + toast | A |
| 10 | `src/components/links/SmartLinkCreator.tsx` | docs 加载加 catch + toast | A |
| 11 | `src/components/documents/DocumentDetail.tsx` | 删除改用 Dialog 确认 | A |
| 12 | `src/components/links/LinksTable.tsx` | 删除改用 Dialog 确认 + toast | A |
| 13 | `src/components/documents/DocumentDetail.tsx` | `views` → `次访问` | A |
| 14 | `src/routes/deal-rooms/detail.tsx` / `new.tsx` | `返回 Deal Rooms` → `返回数据室` | A |
| 15 | `src/routes/contacts/detail.tsx` | tab「360° 视图」→「概览」 | A |
| 16 | `src/routes/deal-rooms/new.tsx` | 标题「数据室模板引擎」→「新建数据室」 | A |
| 17 | `src/routes/settings/brand.tsx` | logo 上传持久化 | A |
| 18 | `src/lib/api.ts` | request 仅对非 FormData 设置 Content-Type；预留认证/token 参数 | B |
| 19 | `src/lib/heat/heatScore.ts` | 修复 topKeyPages 逻辑 | B |
| 20 | `src/hooks/useAsyncData.ts` | 新增统一数据 hook | C |
| 21 | 多个页面组件 | 用 useAsyncData 替换重复 fetch 样板 | C |
| 22 | `src/components/links/SmartLinkCreator.tsx` | 拆分子组件 | C |
| 23 | `src/components/documents/DocumentsTable.tsx` | 提取 columns.tsx | C |
| 24 | `src/lib/formatters.ts` | 增加 formatShortDate | C |
| 25 | 新增 `src/lib/activity.ts` | getActivityLabel | C |
| 26 | 新增 `src/lib/analytics.ts` | groupLogsByDay | C |
| 27 | 多处空状态 | 统一使用 EmptyState + Phosphor 图标 | D |
| 28 | `HeatMap.tsx` / `contacts/detail.tsx` | 键盘可达 | D |
| 29 | `AIAssistant.tsx` / `AIChat.tsx` | 高度/圆角/阴影统一 | D |
| 30 | 卡片 hover | 移除 shadow-sm，改为 bg-muted/50 | D |
| 31 | 测试文件 | 为 heat/formatters/calculations 补测试 | E |

---

## 5. 验收标准

### 5.1 阻塞项验收
- [ ] 0 个无响应按钮（无 onClick 也无 disabled）。
- [ ] 所有 clipboard 调用走 `copyToClipboard`。
- [ ] 所有 async 提交操作有 try/catch + toast。
- [ ] 删除/危险操作使用 Dialog 确认。
- [ ] 英文微文案清零（`views`、`Deal Rooms`、`360° 视图`、`数据室模板引擎`）。

### 5.2 工程验收
- [ ] `pnpm lint` 0 errors。
- [ ] `pnpm build` 通过。
- [ ] `pnpm tsc -b --noEmit` 通过。
- [ ] `pnpm test` 通过（新增单元测试）。

### 5.3 UI/UX 验收
- [ ] 所有可点击元素键盘可达。
- [ ] 所有图标按钮有 aria-label。
- [ ] 空状态统一使用 EmptyState。
- [ ] AI 面板移动端不溢出。

### 5.4 文档验收
- [ ] v2.1.3 审计计划文档准确反映当前代码。
- [ ] README 使用 pnpm。
- [ ] 新增/更新文档已保存。

---

## 6. 建议实施方式

- **方式一（推荐）：全面实施 Phase A → B → C → D → E**  
  按优先级逐阶段交付，每阶段验收后进入下一阶段。适合追求产品化与工程化双达标。

- **方式二：仅 Phase A + B**  
  快速清零剩余阻塞项并修复 API/算法层关键 Bug，适合需要尽快对外演示或接入真实后端的场景。

- **方式三：仅 Phase A**  
  只做文案、按钮、反馈等表面修复，风险最低但工程债务未解决。

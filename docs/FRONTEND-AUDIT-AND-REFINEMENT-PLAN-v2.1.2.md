# DealSignal Web v2.1.2 前端细胞级审计与专利级产品设计方案

## 0. 执行摘要

本次审计基于四位专家视角（前端架构师、高级产品总监、UI/UX 与交互设计师、用户心理学与行为心理学专家）对 `apps/web` 进行细胞级扫描，并结合 `docs/` 全部设计文档进行一致性校验。

**结论**：当前代码是一个视觉完整度高、Mock 数据详尽的**高保真前端 Demo**，但距离可交付的 SaaS 产品仍有大量工程与体验债务。主要问题集中在：

1. **工程债务**：重复代码、缺失错误处理、N+1 请求、API/Mock 未与生产隔离。
2. **功能占位**：大量核心操作 `disabled`、`onClick={() => {}}`、伪上传、伪 AI、伪文档渲染。
3. **设计偏差**：侧边栏英文、英文微文案残留、Dashboard 摘要指标超标、可点击元素键盘不可达。
4. **体验风险**：复制无反馈、AI 重置无确认、空状态不一致、移动端 AI 面板溢出。

本计划提出**分阶段专利级产品设计方案**，在保持现有视觉骨架的前提下，优先补齐阻塞项，再系统性提升交互、可访问性与工程可维护性。

---

## 1. 审计发现总览

### 1.1 前端架构现状

- **技术栈**：React 19 + React Router 8 + Vite 8 + Tailwind CSS 4 + Base UI + Zustand + Motion + TanStack Table + MSW。
- **文件规模**：96 个 TS/TSX 文件，约 9,755 行源码；25 个 routes，56 个 components（17 个 UI 基础组件）。
- **路由体系**：workspace slug 顶层参数，但根路径硬编码跳转 `/acme-capital/dashboard`；`/viewer/:documentId` 独立于 AppShell。
- **状态管理**：三个 Zustand store（ui / signal / ai），但各页面仍大量使用 `useState` + `useEffect` 自行请求，无统一数据获取层。

### 1.2 占位符 / 硬编码 / 未实现清单

#### 🔴 高优先级阻塞项

| 类别 | 问题 | 位置 | 影响 |
|------|------|------|------|
| 核心操作 disabled | 邀请成员、管理文档、下载、编辑链接 | `deal-rooms/detail.tsx`、`DocumentDetail.tsx`、`CanvasViewer.tsx`、`LinkDetail.tsx` | 用户无法完成核心闭环 |
| 点击无响应 | 导出访问数据、仅允许下载、删除、编辑角色、移除成员、上传推荐文件 | `LinksTable.tsx`、`DocumentsTable.tsx`、`settings/members.tsx`、`deal-rooms/detail.tsx` | 假交互导致信任崩塌 |
| 伪上传 | Uploader 仅用 setInterval 模拟进度，未调用 API | `components/upload/Uploader.tsx` | 无法真正创建文档 |
| 伪 AI | aiStore 基于关键词匹配，未调用真实模型 | `stores/aiStore.ts` | AI 助手价值为 0 |
| 伪文档渲染 | CanvasViewer 显示占位 div | `components/viewer/CanvasViewer.tsx` | 文档预览不可用 |
| 根路径硬编码 | `/` 永远跳 `/acme-capital/dashboard` | `router.tsx` | 多租户入口被锁死 |
| 直接引用 mock | `WorkspaceSwitcher`、`insights/pages.tsx` 直接 import mock 数据 | `WorkspaceSwitcher.tsx`、`routes/insights/pages.tsx` | 后端替换 mock 后仍展示死数据 |
| 生产包含 MSW | `main.tsx` 无条件启动 worker | `main.tsx` | 生产包体积与行为风险 |

#### 🟠 中优先级体验与工程项

| 类别 | 问题 | 位置 |
|------|------|------|
| 错误处理缺失 | 几乎所有页面 `useEffect` 无 `catch` | 全项目页面组件 |
| 表单无校验 | 未引入 zod/yup，邮箱、slug、域名无格式校验 | SmartLinkCreator、settings/*、deal-rooms/new |
| 重复代码 | SmartLinkCreator vs PermissionSlider；AIAssistant vs AIChat；InsightsOverview 重复 | `components/links/*`、`components/ai/*`、`components/insights/*` |
| 空状态不一致 | InsightsSuggestions、LinkAccessLog、DocumentAnalytics、Security 审计日志 | 多处 |
| N+1 请求 | DocumentDetail 先取 links 再逐个取 logs；insights/overview 同理 | `DocumentDetail.tsx`、`insights/overview.tsx` |
| 可点击元素不可键盘操作 | Dashboard 最近文档、HeatMap、DocumentContent 页面卡片、Insights 列表行、表格行 | 多处 |
| 复制无反馈 | 6 处调用 clipboard 后无 toast/图标变化 | `LinksTable`、`DocumentsTable`、`DocumentDetail`、`SmartLinkCreator`、`LinkDetail` |
| 移动端 AI 面板溢出 | `w-[360px]` 在 375px 以下设备溢出 | `AIAssistant.tsx`、`AIChat.tsx` |
| 侧边栏英文 | 导航标签未中文化 | `Sidebar.tsx` |

#### 🟡 低优先级打磨项

| 类别 | 问题 | 位置 |
|------|------|------|
| 文案单位 | `views` 未替换为中文 | `DocumentsTable`、`LinksTable`、`CanvasViewer`、`InsightsOverview` |
| 阴影/过渡滥用 | `hover:shadow-sm/md`、`transition-all` 多处 | Dashboard、SignalCard、ActionList、SmartLinkCreator |
| 字体大小 | `text-[10px]` 低于 Token 规范 | `DocumentAnalytics`、`DocumentContent`、`CanvasViewer`、`RowActions` |
| 品牌未接入 | Sidebar Logo 硬编码 "D"，未使用 settings.logoUrl | `Sidebar.tsx`、`settings/brand.tsx` |
| 用户头像 | 硬编码 "JD"，无账户菜单 | `TopNav.tsx` |
| Dialog 关闭文案 | `sr-only` 为英文 "Close" | `components/ui/dialog.tsx` |

### 1.3 设计系统一致性评估

**遵循良好**：色彩 Token、Dark Mode、Typography、CVA 变体、图标策略、动效与 `prefers-reduced-motion`、空态组件。

**主要偏差**：
- 侧边栏英文标签与 `PRODUCT-DESIGN-v2.1.1-REFINED` 要求的全站中文冲突。
- Dashboard 摘要卡 4 个指标 vs 规范要求 2 个。
- 卡片 hover 滥用阴影，与 Token 规范 5.1 冲突。
- 可点击区域未做键盘与屏幕阅读器适配。
- `views` 等英文微文案残留。

### 1.4 文档一致性评估

- `DESIGN-TOKENS-v2.1.1.md` 与代码高度一致。
- `PRODUCT-DESIGN-v2.1.1-REFINED.md` 与代码存在文案、指标数量、键盘可访问性偏差。
- `HEAT-SCORE-ALGORITHM-v2.1.1.md` 与 `src/lib/heat/heatScore.ts` 在权重、阈值、关键页分类上不一致。
- `API-SPEC-v2.1.0.md` 与 `src/lib/api.ts` 路径、认证、分页、幂等均未对齐。
- 项目根目录缺少 `README.md` 与 `AGENTS.md`。

---

## 2. 专利级产品设计方案

### 2.1 设计原则（基于用户心理学与行为心理学）

1. **认知负荷最小化（Cognitive Load Theory）**  
   每屏决策组 ≤3；Dashboard 摘要仅保留"高热度信号"与"待办行动"两个核心指标；复杂表单默认折叠高级选项。

2. **即时反馈与确定性（Operant Conditioning）**  
   所有复制操作必须提供 1s 内的视觉反馈；危险/重置操作必须二次确认；保存/删除/启用必须 toast 反馈。

3. **控制感与自主权（Self-Determination Theory）**  
   禁用按钮必须说明原因或提供替代路径；假功能必须从 UI 下架或明确标注"即将上线"。

4. **信任与数据诚实（Truth Default Theory）**  
   占位页明确说明状态；AI 回复必须引用真实数据；空状态给出下一步行动。

5. **无障碍与包容性（Universal Design）**  
   所有可点击元素必须键盘可达；关键操作满足 WCAG 2.1 AA 对比度；动态内容使用 `aria-live`。

6. **中文 SaaS 语境一致性**  
   全站中文标签；英文缩写（PRO、Workspace）统一为"高级版"、"工作区"；数据单位本地化为"次访问"。

### 2.2 设计目标

| 目标 | 定义 | 可量化指标 |
|------|------|------------|
| **零假交互** | 用户点击的任何元素都产生明确响应或明确不可用说明 | 0 个 `onClick={() => {}}` |
| **零硬编码入口** | 根路径、workspace、用户头像不依赖 mock | 根路径支持 workspace 选择；头像来自用户 API |
| **全键盘可达** | 所有可点击区域可通过 Tab + Enter/Space 触发 | 键盘遍历无遗漏 |
| **即时反馈** | 复制、保存、删除、重置均有反馈 | 100% 核心操作带反馈 |
| **设计系统 100% 对齐** | 无 `text-[10px]`、无 `transition-all`、无任意阴影滥用 | lint 级约束 |

### 2.3 页面级设计决策

#### Dashboard（交易雷达）
- 摘要指标从 4 个精简为 2 个：高热度信号数、待办行动数。
- 风险提醒保持独立模块（符合 Signal-First）。
- 最近文档限制 3 条，其余折叠到「查看全部」。
- 所有卡片/列表行改为 `<button>` 或 `<Link>`，支持键盘操作。

#### 文档库 / 文档详情
- 文档表格行整行可点击跳转；行操作按钮明确可聚焦。
- 文档详情右侧 sidebar sticky 跟随滚动（已实施）。
- "此文档的链接" 放进「总览」Tab 内部（已实施）。
- 复制链接/URL 后图标变为 `Check` 并 toast「已复制」。
- 删除/下载按钮若后端未就绪，改为 `disabled + Tooltip`，不保留无响应按钮。

#### 链接管理 / 链接详情
- 合并 `SmartLinkCreator` 与 `PermissionSlider`：PermissionSlider 作为受控子组件，评分逻辑下沉到 hook。
- 自定义安全选项默认折叠。
- 复制、启用/停用、删除均有反馈。
- 删除必须二次确认。

#### 数据室
- 邀请成员、管理文档按钮若后端未就绪，使用 Tooltip 说明，不保留假按钮。
- 推荐文件的上传按钮实现真实文件选择（调用 Uploader 或说明未上线）。

#### 设置
- 成员管理：编辑角色使用 Select 下拉；移除需二次确认。
- 安全：2FA 配置入口明确状态；审计日志按钮打开日志弹窗或说明未上线。
- 品牌：Logo 上传真正持久化（调用上传接口保存 CDN URL）。

#### AI 助手
- 全局 AI 助手与 Viewer AI 助手统一 UI 组件。
- 重置对话二次确认。
- 消息区 `aria-live="polite"`，pending 时 `aria-busy="true"`。
- 移动端宽度改为 `max-w-[calc(100vw-2rem)]`。

#### 文档查看器
- CanvasViewer 占位页文案使用 `text-muted-foreground` 保证对比度。
- 下载按钮根据后端状态显示可用或不可用说明。

---

## 3. 实施路线图

### Phase A：零阻塞项（ must have ）
目标：让产品所有可见交互都是真实或有明确不可用说明的。

1. **移除或禁用所有无响应点击**
   - `DocumentsTable` 下载/删除：若后端未就绪，改为 disabled + Tooltip。
   - `LinksTable` 导出/仅允许下载/删除：同上。
   - `settings/members.tsx` 编辑角色/移除：同上或实现。
   - `deal-rooms/detail.tsx` 推荐文件上传：同上。

2. **修复错误处理**
   - 为所有页面级 `useEffect` 增加 `try/catch` + error 状态 + 重试按钮。
   - 路由增加 `errorElement`。

3. **接入真实数据入口**
   - `WorkspaceSwitcher` 调用 `/api/workspaces`。
   - 根路径 `/` 改为 workspace 选择页或从持久化读取当前 workspace。
   - `insights/pages.tsx` 不再直接 import `mockDocuments`。

4. **生产禁用 MSW**
   - `main.tsx` 仅在开发环境启动 MSW。

### Phase B：核心功能补齐
1. **Uploader 真实上传**
   - 调用 `api.uploadDocument`（修复 FormData header 冲突）。
   - 使用 `xhr` 或 fetch 进度事件替代 setInterval。

2. **AI 助手真实化接口**
   - 接入 `/api/ai/chat` 或 SSE 流式接口。
   - 统一全局与 Viewer AI 组件。

3. **文档渲染**
   - CanvasViewer 接入签名 URL / PDF 渲染占位改进。
   - 下载按钮对接 `/documents/:id/download`。

4. **表单校验**
   - 引入 zod，校验邮箱、slug、域名、密码、有效期。

### Phase C：体验与可访问性
1. **中文化与文案**
   - Sidebar 标签改为中文。
   - `views` → `次访问`。
   - Workspace 文案本地化。
   - Dialog 关闭按钮 sr-only 改为中文。

2. **复制反馈**
   - 所有复制按钮点击后图标变 `Check` + toast「已复制」。
   - 封装 `copyToClipboard` 工具函数。

3. **键盘可达**
   - 所有 `onClick` 的 `<div>/<li>/<tr>` 改为 `<button>/<Link>` 或加 `role="button" tabIndex={0}` + Enter/Space。

4. **AI 体验**
   - 重置二次确认。
   - `aria-live`、`aria-busy`。
   - 移动端宽度修复。

5. **空状态与错误状态**
   - InsightsSuggestions、LinkAccessLog、DocumentAnalytics 增加 EmptyState。
   - 全局 ErrorBoundary。

### Phase D：设计系统收敛
1. **移除反 Token 写法**
   - `text-[10px]` → `text-caption`。
   - `transition-all` → `transition-colors` 或具体属性。
   - 卡片 hover 阴影收敛。

2. **Dashboard 精简**
   - 摘要指标 4 → 2。
   - 最近文档限制 3 条。

3. **品牌接入**
   - Sidebar Logo 使用 `settings.logoUrl`。
   - 主色动态应用。

4. **热度算法对齐**
   - `heatScore.ts` 与 `HEAT-SCORE-ALGORITHM-v2.1.1.md` 同步。

### Phase E：工程重构
1. **统一数据层**
   - 引入 React Query / SWR / 自研 useApi，替换各页面自行请求。

2. **拆分 api.ts**
   - client / resources / formatters / calculations 分层。

3. **合并重复组件**
   - SmartLinkCreator + PermissionSlider。
   - AIAssistant + AIChat。
   - 删除孤立 `components/insights/InsightsOverview.tsx`。

4. **测试**
   - 补充单元测试与 E2E 测试。

---

## 4. 详细修改清单（Phase A + 部分 Phase C 优先项）

### 4.1 零阻塞项修改清单

| # | 文件 | 修改内容 | 所属 Phase |
|---|------|----------|------------|
| 1 | `src/main.tsx` | MSW 启动加 `import.meta.env.DEV` 判断 | A |
| 2 | `src/router.tsx` | 根路径 `/` 改为 `/workspaces` 选择页；增加 `errorElement` | A |
| 3 | `src/routes/workspaces.tsx` | 新增 workspace 选择页（若不存在） | A |
| 4 | `src/components/layout/WorkspaceSwitcher.tsx` | 改为调用 `api.getWorkspaces()`，fallback 处理空列表 | A |
| 5 | `src/components/layout/Sidebar.tsx` | 导航标签中文化 | C |
| 6 | `src/components/documents/DocumentsTable.tsx` | 下载/删除按钮 disabled + Tooltip；行点击改为 Link/button | A/C |
| 7 | `src/components/links/LinksTable.tsx` | 导出/仅允许下载/删除按钮 disabled + Tooltip；行点击可键盘 | A/C |
| 8 | `src/routes/settings/members.tsx` | 编辑角色/移除按钮 disabled + Tooltip 或实现 | A |
| 9 | `src/routes/deal-rooms/detail.tsx` | 邀请成员/管理文档 disabled + Tooltip；推荐文件上传 disabled + Tooltip | A |
| 10 | `src/components/links/LinkDetail.tsx` | 编辑按钮 disabled + Tooltip | A |
| 11 | `src/components/documents/DocumentDetail.tsx` | 下载按钮 disabled + Tooltip | A |
| 12 | `src/components/viewer/CanvasViewer.tsx` | 下载按钮 disabled + Tooltip；占位文案对比度修复 | A/C |
| 13 | `src/routes/settings/security.tsx` | 审计日志按钮打开弹窗或 disabled + Tooltip | A |
| 14 | `src/routes/settings/billing.tsx` | 升级按钮跳转/弹窗或 disabled + Tooltip | A |
| 15 | `src/routes/insights/pages.tsx` | 移除 `mockDocuments` import，改用 API | A |
| 16 | 全页面组件 | 统一加 `try/catch` + error 状态 + 重试按钮 | A |
| 17 | `src/components/common/DetailLayout.tsx` | sidebar sticky 已实施，保持 | - |
| 18 | `src/components/documents/DocumentDetail.tsx` | 链接列表移入总览 Tab 已实施，保持 | - |

### 4.2 体验与可访问性优先项

| # | 文件 | 修改内容 | 所属 Phase |
|---|------|----------|------------|
| 19 | `src/lib/utils.ts` 或新增 `src/lib/clipboard.ts` | 封装 `copyToClipboard(text)`，返回 Promise，失败 toast | C |
| 20 | 所有复制调用点 | 替换为封装函数，点击后图标变 Check + toast | C |
| 21 | `src/components/ai/AIAssistant.tsx` | 重置按钮加二次确认 Dialog | C |
| 22 | `src/components/ai/AIAssistant.tsx`、`src/components/viewer/AIChat.tsx` | 消息区加 `aria-live`、`aria-busy`；移动端宽度修复 | C |
| 23 | `src/routes/insights/suggestions.tsx` | 空状态使用 `EmptyState` | C |
| 24 | `src/components/links/LinkAccessLog.tsx` | 空表格状态 | C |
| 25 | `src/components/documents/DocumentAnalytics.tsx` | analytics 为空时显示 EmptyState | C |
| 26 | `src/components/ui/dialog.tsx` | `sr-only` 文案改为"关闭" | C |
| 27 | 多处 `views` 文案 | 替换为"次访问"或"次浏览" | C |

---

## 5. 验收标准

### 5.1 阻塞项验收
- [ ] 项目中不存在任何 `onClick={() => {}}`（除非明确注释为占位且已 disabled）。
- [ ] 所有 disabled 按钮均有 Tooltip 或 title 说明原因。
- [ ] 所有页面级数据请求有错误状态 UI。
- [ ] 生产构建不启动 MSW。
- [ ] 根路径支持 workspace 选择或持久化读取。
- [ ] `WorkspaceSwitcher` 与 `insights/pages.tsx` 不再直接引用 mock。

### 5.2 体验验收
- [ ] 复制操作有 1s 内的图标变化或 toast 反馈。
- [ ] 删除、重置、移除等危险操作有二次确认。
- [ ] 所有可点击卡片/列表行可通过 Tab 聚焦并按 Enter/Space 触发。
- [ ] AI 消息区屏幕阅读器可感知新消息。
- [ ] 移动端 AI 面板不超出视口。

### 5.3 设计系统验收
- [ ] 侧边栏导航为中文。
- [ ] 无 `text-[10px]` 手写。
- [ ] 无 `transition-all` 滥用。
- [ ] Dashboard 摘要指标为 2 个。
- [ ] `views` 等英文微文案替换为中文单位。

### 5.4 工程验收
- [ ] `pnpm lint` 通过。
- [ ] `pnpm build` 通过。
- [ ] `pnpm tsc -b --noEmit` 通过。

---

## 6. 建议实施方式

考虑到用户希望"完整梳理后逐一代码实现"，推荐采用**分阶段交付 + 每阶段验收**的方式：

- **方式一（推荐）：全面实施 Phase A → C → D → E**  
  优先让产品从 Demo 变为"可交互原型"，再逐步补齐真实后端能力、设计系统收敛与工程重构。每完成一个 Phase 进行一轮 QA 截图与验收。

- **方式二：仅修复高优先级阻塞项（Phase A）**  
  快速消除所有假交互与硬编码入口，适合需要尽快对外演示的场景。

- **方式三：一次性大规模重构（Phase A+E 同步）**  
  同时解决阻塞项与工程架构（数据层、api 拆分、重复组件合并）。改动面大，风险高，但长期可维护性最好。

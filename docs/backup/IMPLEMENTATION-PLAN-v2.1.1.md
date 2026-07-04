# DealSignal v2.1.1 实施计划

> Signal-First 重设计落地路线图
> 日期：2026-06-20
> 状态：已批准

---

## 1. 总体节奏

预计周期：9-12 天
工作模式：按阶段推进，每阶段完成后 `pnpm build` + `pnpm lint` 验证

```
阶段 A：基础设施与数据层      1-2 天    ✅ 已完成
阶段 B：核心组件专利级实现    3-4 天    ✅ 已完成
阶段 C：页面重构与路由        2-3 天    ✅ 已完成
阶段 D：Viewer 体验与 AI      2-3 天    ✅ 已完成
阶段 E：数据室模板引擎        1-2 天    ✅ 已完成
```

> 实际完成时间：2026-06-20。所有阶段已通过 `pnpm build` 与 `pnpm lint` 验证。原 TanStack Table 的 3 个 `incompatible-library` warning 已通过 `"use no memo"` 指令与 ESLint disable 注释消除。

---

## 2. 阶段 A：基础设施与数据层

### 2.1 目标

建立新数据模型、Mock 数据、API 接口，为上层组件提供数据支撑。

### 2.2 任务清单

| # | 任务 | 文件 | 验收标准 |
|---|---|---|---|
| A1 | 扩展类型定义 | `src/types/index.ts` | 新增 Signal/Action/HeatScoreConfig/KeyPage/ContactProfile/DealRoomTemplate/AIConversation |
| A2 | 扩展 Mock 数据 | `src/lib/mocks/data.ts` | 包含信号流、行动队列、热度配置、模板数据 |
| A3 | 新增 MSW handlers | `src/lib/mocks/handlers.ts` | 新增 /api/signals, /api/actions, /api/heat-config, /api/templates |
| A4 | 扩展 API 层 | `src/lib/api.ts` | 新增对应 API 方法，含热度评分计算函数 |
| A5 | 热度评分函数 | `src/lib/heatScore.ts`（新建） | 实现规则版评分算法，含单元测试 |

### 2.3 新增类型

```typescript
// Signal 信号
interface Signal {
  id: string;
  type: "hot" | "warm" | "cold" | "risk";
  title: string;
  description: string;
  explanation: string;
  suggestion: string;
  documentId?: string;
  contactId?: string;
  linkId?: string;
  createdAt: string;
  priority: "high" | "medium" | "low";
}

// Action 待办行动
interface Action {
  id: string;
  signalId: string;
  title: string;
  impact: "high" | "medium" | "low";
  dueAt: string;
  status: "pending" | "done" | "snoozed" | "ignored";
  actionType: "email" | "call" | "share" | "review";
}

// 热度评分配置
interface HeatScoreConfig {
  name: "founder" | "investor_ir" | "sales";
  weights: {
    opens: number;
    revisits: number;
    avgDurationMinutes: number;
    keyPageViews: number;
    forwardSignals: number;
    downloads: number;
    bouncePenalty: number;
  };
  keyPages: Record<string, string[]>;
  thresholds: { hot: number; warm: number; cold: number };
}

// 联系人画像
interface ContactProfile {
  id: string;
  email: string;
  name: string;
  organization?: string;
  role?: string;
  heatLevel: HeatLevel;
  score: number;
  scoreHistory: { date: string; score: number }[];
  relatedContacts: string[];
  notes?: string;
}

// 数据室模板
interface DealRoomTemplate {
  id: string;
  name: string;
  description: string;
  scenario: "seed" | "series_a" | "series_b" | "lp_update" | "sales_proposal" | "ma";
  folderStructure: { name: string; description?: string }[];
  recommendedFiles: string[];
  defaultPermissionLevel: "low" | "medium" | "high";
  ndaEnabled: boolean;
}

// AI 对话
interface AIConversation {
  id: string;
  documentId: string;
  messages: {
    id: string;
    role: "user" | "assistant";
    content: string;
    evidences?: Evidence[];
    createdAt: string;
  }[];
}
```

---

## 3. 阶段 B：核心组件专利级实现

### 3.1 目标

实现七大交互创新的核心组件。

### 3.2 任务清单

| # | 组件 | 文件 | 依赖 |
|---|---|---|---|
| B1 | SignalStream | `src/components/dashboard/SignalStream.tsx` | Signal 类型、RowActions、HeatBadge |
| B2 | HeatMap | `src/components/dashboard/HeatMap.tsx` | Document/Link 数据、HeatBadge |
| B3 | ActionQueue | `src/components/dashboard/ActionQueue.tsx` | Action 类型、Button |
| B4 | SmartLinkComposer | `src/components/links/SmartLinkComposer.tsx` | Slider、Select、PermissionBadge |
| B5 | DocumentHeatmap | `src/components/documents/DocumentHeatmap.tsx` | PageAnalytics、Canvas 或 div 实现 |
| B6 | DocumentDetail 重构 | `src/components/documents/DocumentDetail.tsx` | DocumentHeatmap、Tabs、StatCard |
| B7 | ContactProfile | `src/components/contacts/ContactProfile.tsx` | ActivityTimeline、HeatBadge、StatCard |
| B8 | DealRoomTemplatePicker | `src/components/deal-rooms/DealRoomTemplatePicker.tsx` | Card、Button |
| B9 | AIAssistant | `src/components/viewer/AIAssistant.tsx` | ChatMessage、Evidence |

### 3.3 组件详细要求

#### SignalStream

- 接收 `signals: Signal[]`
- 支持 `onAction(signal, action)` 回调
- 支持滑动/归档（移动端）
- 空状态友好

#### HeatMap

- 接收 `items: { id, title, score, level, metric }[]`
- 支持 `onSelect(id)`
- 响应式网格

#### ActionQueue

- 接收 `actions: Action[]`
- 支持完成/推迟/忽略
- 一键生成邮件草稿

#### SmartLinkComposer

- 5 级安全强度滑块
- 动态显示接收方步骤
- 推荐配置
- 生成链接后显示复制/邮件模板

#### DocumentHeatmap

- 在文档缩略图上叠加热力
- 可切换热力图/正常视图
- 点击缩略图跳转到对应页

#### DocumentDetail 重构

- 左侧 60% 预览 + 热力图
- 右侧 40% 标签页
- 标签：Overview / Visitors / Links / Settings

#### ContactProfile

- 名片区
- 关系图（同机构联系人）
- 活动时间线
- 热度趋势
- 推荐行动

#### DealRoomTemplatePicker

- 模板卡片网格
- 悬停显示结构预览
- 选中后进入配置流程

#### AIAssistant

- 悬浮按钮
- 对话面板
- Evidence 列表
- 向发件人提问

---

## 4. 阶段 C：页面重构与路由

### 4.1 目标

将新组件接入页面，完成交易雷达 Dashboard 等核心页面改造。

### 4.2 任务清单

| # | 页面 | 文件 | 主要变更 |
|---|---|---|---|
| C1 | Dashboard | `src/routes/dashboard.tsx` | 改为 SignalStream + HeatMap + ActionQueue 布局 |
| C2 | Documents | `src/routes/documents.tsx` | 添加文档表现摘要、快速创建链接入口 |
| C3 | Document Detail | `src/routes/documents/detail.tsx` | 接入重构后的 DocumentDetail 组件 |
| C4 | Links | `src/routes/links.tsx` | 接入 SmartLinkComposer |
| C5 | Deal Rooms | `src/routes/deal-rooms.tsx` | 接入模板选择入口 |
| C6 | New Deal Room | `src/routes/deal-rooms/new.tsx` | 接入 DealRoomTemplatePicker |
| C7 | Contacts | `src/routes/contacts.tsx` | 列表页增强 |
| C8 | Contact Detail | `src/routes/contacts/detail.tsx` | 接入 ContactProfile 组件 |
| C9 | Insights | `src/routes/insights/overview.tsx` | 添加热度评分解释、关键页分析 |

### 4.3 Dashboard 布局

```tsx
<div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
  <div className="lg:col-span-5">
    <SignalStream signals={signals} />
  </div>
  <div className="lg:col-span-4">
    <HeatMap items={heatItems} />
  </div>
  <div className="lg:col-span-3">
    <ActionQueue actions={actions} />
  </div>
</div>
<RiskAlerts alerts={riskAlerts} />
```

---

## 5. 阶段 D：Viewer 体验与 AI

### 5.1 目标

升级文档查看器，集成 AI 悬浮助手 2.0。

### 5.2 任务清单

| # | 任务 | 文件 | 验收标准 |
|---|---|---|---|
| D1 | Viewer 页面布局 | `src/routes/viewer.tsx` | Canvas 区域 + 缩略图导航 + AI 助手 |
| D2 | Canvas 渲染 | `src/components/viewer/DocumentCanvas.tsx` | 加载 WebP/占位图、缩放、平移 |
| D3 | 缩略图导航 | `src/components/viewer/ThumbnailNav.tsx` | 页面缩略图、当前页高亮 |
| D4 | 高亮框绘制 | `src/components/viewer/HighlightOverlay.tsx` | 根据 bbox 绘制、pulse 动画 |
| D5 | AI 问答 Mock | `src/lib/mocks/handlers.ts` | `/api/assistant/chat` 返回 answer + evidence |
| D6 | AIAssistant 集成 | `src/components/viewer/AIAssistant.tsx` | 问答 + evidence 跳转 |
| D7 | 动态水印 | `src/components/viewer/WatermarkOverlay.tsx` | Canvas 上叠加邮箱/时间/IP |

### 5.3 Viewer 布局

```
┌─────────────────────────────────────────────────────────┐
│  顶部工具栏：缩放、翻页、下载控制                          │
├──────────────────┬──────────────────────────────────────┤
│                  │                                      │
│  缩略图导航       │  Canvas 渲染区                       │
│  (左侧 180px)    │  + Highlight Overlay                 │
│                  │  + Watermark Overlay                 │
│                  │                                      │
├──────────────────┴──────────────────────────────────────┤
│  AI 悬浮按钮（右下角）                                    │
└─────────────────────────────────────────────────────────┘
```

---

## 6. 阶段 E：打磨与验收

### 6.1 目标

修复构建/lint 问题，优化响应式与动效，完成验收。

### 6.2 任务清单

| # | 任务 | 验收标准 |
|---|---|---|
| E1 | 响应式适配 | 桌面/平板/移动端核心功能可用 |
| E2 | 无障碍检查 | 键盘导航、对比度、aria-label |
| E3 | 动效调优 | 无卡顿、减少动效模式生效 |
| E4 | 构建验证 | `pnpm build` 通过 |
| E5 | Lint 验证 | `pnpm lint` 无 error（warnings 可接受） |
| E6 | 文档检查 | 5 份设计文档已写入 docs/ |

---

## 7. 新增/修改文件清单

### 7.1 新增文件

```
docs/
  RESEARCH-INSIGHTS-v2.1.1.md
  PRODUCT-DESIGN-v2.1.1.md
  INTERACTION-SPEC-v2.1.1.md
  HEAT-SCORE-ALGORITHM-v2.1.1.md
  IMPLEMENTATION-PLAN-v2.1.1.md

src/
  lib/
    heatScore.ts
  components/
    dashboard/
      SignalStream.tsx
      HeatMap.tsx
      ActionQueue.tsx
      RiskAlerts.tsx
    links/
      SmartLinkComposer.tsx
    documents/
      DocumentHeatmap.tsx
    contacts/
      ContactProfile.tsx
    deal-rooms/
      DealRoomTemplatePicker.tsx
    viewer/
      DocumentCanvas.tsx
      ThumbnailNav.tsx
      HighlightOverlay.tsx
      WatermarkOverlay.tsx
      AIAssistant.tsx
```

### 7.2 修改文件

```
src/
  types/index.ts
  lib/mocks/data.ts
  lib/mocks/handlers.ts
  lib/api.ts
  components/
    documents/DocumentDetail.tsx
    links/PermissionSlider.tsx（可能替换为 SmartLinkComposer）
  routes/
    dashboard.tsx
    documents.tsx
    documents/detail.tsx
    links.tsx
    deal-rooms.tsx
    deal-rooms/new.tsx
    contacts.tsx
    contacts/detail.tsx
    insights/overview.tsx
    viewer.tsx
```

---

## 8. 验收标准

- [ ] Dashboard 交易雷达首屏包含信号流、热度地图、行动队列
- [ ] 智能链接创建器支持可视化安全强度滑块
- [ ] AI 悬浮助手支持多轮对话、evidence 跳转、原文高亮
- [ ] 文档详情页展示三位一体视图
- [ ] 联系人页面展示 360° 视图
- [ ] 数据室创建支持模板选择
- [ ] 热度评分按圈层计算并展示
- [ ] 构建通过，lint 无错误
- [ ] 新文档已持久化至 docs/

---

## 9. 风险与应对

| 风险 | 影响 | 应对 |
|---|---|---|
| Canvas 实现复杂 | 中 | MVP 先用 div/图片占位，Canvas 二期迭代 |
| AI 功能超出前端能力 | 中 | 前端 mock AI 回答，后端可后续替换 |
| 信号流数据过载 | 低 | 默认过滤 Cold，只展示 Hot/Warm |
| 热度评分主观争议 | 低 | 标注"MVP 规则版"，提供权重配置入口 |
| 移动端体验差 | 中 | Dashboard 改为卡片堆叠，简化 Viewer 手势 |

---

## 10. 实际落地补充说明（2026-06-21 文档对齐）

以下内容为 v2.1.1 实际实现与原始计划的差异记录，供后续迭代与真实后端接入时参考。

### 10.1 i18n 与 Settings Language 页面

| 项 | 计划 | 实际 |
|---|---|---|
| 范围 | 未计划 | 已提前落地：双语 `en` / `zh-CN`，默认 `en` |
| 实现 | - | `i18next` + `react-i18next` + `i18next-resources-to-backend`，11 个 namespace 懒加载，自定义 detector |
| 路由 | - | `/:workspaceSlug/settings/language` |
| 持久化 | - | `localStorage` key `i18nextLng` |

### 10.2 主题与全局状态

- 主题状态由 Zustand `uiStore` 管理，支持 `light` / `dark` / `system`，持久化到 `localStorage`。
- `ThemeProvider` 负责切换 `html.dark` 类并监听 `prefers-color-scheme`。
- 其他 Zustand store：`workspaceStore`（当前 workspace）、`aiStore`（AI 会话）、`documentStore`、`linkStore` 等。

### 10.3 Workspace 切换器

- 原始计划：入口放在侧边栏 Settings 子菜单。
- 实际实现：入口放在侧边栏底部 **Workspace Switcher**（`src/components/layout/WorkspaceSwitcher.tsx`）。
- 工作区名称在 mock 数据中存为 i18n key（`mock.workspaces.acme.name`），组件通过 `t(workspace.name)` 渲染。
- "Create workspace" 当前仅展示 coming-soon toast，未实现真实创建流程。

### 10.4 组件清单修正

原始 §3.2 / §7.1 的组件名与文件和实际代码不完全一致，实际文件如下：

| 实际文件 | 对应原始计划组件 | 说明 |
|---|---|---|
| `src/components/dashboard/SignalCard.tsx` | `SignalStream.tsx` | 重命名/拆分；实际为可交互信号卡片 |
| `src/components/dashboard/ActionList.tsx` | `ActionQueue.tsx` | 重命名 |
| `src/components/dashboard/RiskAlertList.tsx` | `RiskAlerts.tsx` | 内联风险提醒列表 |
| `src/components/links/SmartLinkCreator.tsx` + `src/components/links/smart-link/*` | `SmartLinkComposer.tsx` | 拆分为 Creator + 多个子组件 |
| `src/components/documents/DocumentDetail.tsx` / `DocumentContent.tsx` / `DocumentAnalytics.tsx` / `DocumentAIInsights.tsx` | `DocumentHeatmap.tsx` | 未做页面热力图 overlay，改为详情页多标签 |
| `src/components/contacts/ContactDetail.tsx` | `ContactProfile.tsx` | 重命名 |
| `src/components/deal-rooms/DealRoomCreator.tsx` | `DealRoomTemplatePicker.tsx` | 重命名；模板选择内嵌在创建流程中 |
| `src/components/viewer/CanvasViewer.tsx` | `DocumentCanvas.tsx` | 重命名 |
| `src/components/ai/AIAssistant.tsx` / `src/routes/viewer.tsx` | `viewer/AIAssistant.tsx` | AI 助手组件提升到通用组件目录 |

> 注：原始计划中的 `ThumbnailNav`、`HighlightOverlay`、`WatermarkOverlay` 尚未作为独立组件实现；相关能力在 `CanvasViewer` 与 Viewer 路由中内联处理。

### 10.5 React Compiler 与 TanStack Table 兼容处理

- 项目使用 React 19，但未启用 React Compiler 插件。
- `DocumentsTable.tsx`、`LinksTable.tsx`、`LinkAccessLog.tsx` 使用 TanStack Table 时触发 `react-hooks/incompatible-library` ESLint 规则。
- 处理方式：在每个组件顶部添加 `"use no memo";` 指令，并在 `useReactTable` 调用处添加 `// eslint-disable-next-line react-hooks/incompatible-library`。
- 该处理是临时方案；若后续启用 React Compiler，需重新评估这些 opt-out。

### 10.6 Mock 数据与 i18n key 约定

- 为保持中英文切换一致，mock 数据中的用户可见文案统一返回 i18n key：
  - 信号：`dashboard.mock.signals.sig_1.title` / `.description` / `.explanation` / `.suggestion`
  - 行动项：`dashboard.mock.actions.*`
  - 风险：`dashboard.mock.risks.*`
  - 工作区名：`mock.workspaces.acme.name` / `mock.workspaces.ventura.name`
- 组件通过 `t(key)` 渲染，支持 `data` 插值。
- **后端接入风险**：真实 API 若返回 plain text，需要改为 plain text 渲染或让后端保持相同的 key 契约。建议在 API-SPEC 中明确后端文案返回格式。

### 10.7 路由补充

实际路由在 `src/router.tsx` 中定义，除原始计划页面外新增：

- `/`：Workspaces 选择页（`src/routes/workspaces.tsx`）
- `/:workspaceSlug/settings/language`：语言设置页
- 其他 Settings 子页面：`general`、`brand`、`security`、`members`、`billing`、`integrations`

### 10.8 验收清单更新

- [x] Dashboard 交易雷达首屏包含信号流、热度卡片、行动队列
- [x] 智能链接创建器支持可视化安全强度与摩擦提示
- [x] AI 悬浮助手支持多轮对话、evidence 跳转
- [x] 文档详情页展示 Overview / Visitors / Links / Settings 标签
- [x] 联系人页面展示详情与时间线
- [x] 数据室创建支持模板选择
- [x] 热度评分按圈层计算并展示
- [x] 构建通过，`pnpm lint` 无错误
- [x] i18n 中英文切换可用
- [x] Settings Language 页面可用
- [ ] 真实后端接入时确认 mock-i18n key 模式或迁移为 plain text

## 11. 版本历史

| 版本 | 日期 | 变更 |
|---|---|---|
| v2.1.1 | 2026-06-20 | Signal-First 重设计实施计划 |
| v2.1.1-sync | 2026-06-21 | 补充实际落地差异：i18n、Theme、Workspace Switcher、组件清单修正、React Compiler opt-out、mock-i18n 约定 |

# DealSignal Web — 语义化设计系统

> **版本**：v2.1.0  
> **适用范围**：`apps/web`（Vite + React + Tailwind v4 + shadcn/ui）  
> **设计来源**：`docs/UI-DESIGN-DELIVERABLE-v2.1.0.md` + `docs/COMPETITIVE-ANALYSIS-Papermark-v2.1.0.md`  
> **Dial 设定**：DESIGN_VARIANCE = 5 / MOTION_INTENSITY = 4 / VISUAL_DENSITY = 6

---

## 1. 设计哲学

### 1.1 产品气质

DealSignal 是面向融资创始人、投资机构 IR、B2B 销售 AE 的**专业交易工具**。界面应传递：

- **可信**：深 Slate 主色、充足留白、稳定布局。
- **高效**：核心动作始终可达，信息密度适中，列表优先于网格。
- **安全可见**：权限、水印、访问控制等安全能力要“被看见但不增加摩擦”。
- **行动导向**：热度、提醒、跟进建议必须可解释、可行动。

### 1.2 与 Papermark 的差异定位

Papermark 偏向通用文档分享；DealSignal 聚焦**交易信号**。因此：

- 弱化通用“文件管理”感，强化**热度/意图/跟进**叙事。
- Dashboard 不是“浏览统计”，而是**交易雷达**。
- 文档列表不只展示文件，而是展示**传播状态与热度入口**。

---

## 2. Token 系统

### 2.1 色彩

色彩语义全部通过 CSS 变量暴露，设计/开发使用语义名而非硬编码色值。

| Token | Light | Dark | 用途 |
|-------|-------|------|------|
| `--background` | `#ffffff` | `#020617` | 页面背景 |
| `--foreground` | `#0f172a` | `#f8fafc` | 主文本 |
| `--card` | `#ffffff` | `#0f172a` | 卡片背景 |
| `--muted` | `#f1f5f9` | `#1e293b` | 悬停/次要背景 |
| `--muted-foreground` | `#64748b` | `#94a3b8` | 次要文本 |
| `--border` | `#e2e8f0` | `rgba(148,163,184,0.2)` | 边框、分隔线 |
| `--primary` | `#0f172a` | `#f8fafc` | 主按钮、强调 |
| `--primary-foreground` | `#ffffff` | `#0f172a` | 主按钮文字 |
| `--destructive` | `#ef4444` | `#ef4444` | 删除/危险 |

功能色（保持恒定，不随主题反转语义）：

| Token | 色值 | 用途 |
|-------|------|------|
| `--color-success-500` | `#10b981` | 成功、低摩擦权限 |
| `--color-warning-500` | `#f59e0b` | 警告、中热度 |
| `--color-error-500` | `#ef4444` | 错误、高热度 |
| `--color-info-500` | `#3b82f6` | 信息、低热度 |
| `--color-hot-500` | `#ef4444` | Hot 热度 |
| `--color-warm-500` | `#f59e0b` | Warm 热度 |
| `--color-cold-500` | `#3b82f6` | Cold 热度 |

### 2.2 字体

- **主字体**：`Geist Variable`（`@fontsource-variable/geist`）
- **回退**：`system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`
- **字号比例**：

| 工具类 | 字号 | 字重 | 行高 | 用途 |
|--------|------|------|------|------|
| `.text-display` | 32px | 700 | 40px | 登录/落地页大标题 |
| `.text-h1` | 24px | 600 | 32px | 页面标题 |
| `.text-h2` | 20px | 600 | 28px | 卡片/区块标题 |
| `.text-h3` | 16px | 600 | 24px | 小标题、列表主文本 |
| `.text-body` | 14px | 400 | 22px | 正文（默认 body 字号） |
| `.text-caption` | 12px | 400 | 18px | 辅助说明、元数据 |

### 2.3 间距

基于 4px 网格：

| Token | 值 | 用途 |
|-------|-----|------|
| `space-1` | 4px | 图标与文字间距 |
| `space-2` | 8px | 行内元素间距 |
| `space-3` | 12px | 卡片内部紧凑间距 |
| `space-4` | 16px | 卡片内边距、表单间距 |
| `space-6` | 24px | 区块间距 |
| `space-8` | 32px | 页面级间距 |
| `space-12` | 48px | 大区块分隔 |

### 2.4 圆角与阴影

| Token | 值 | 用途 |
|-------|-----|------|
| `--radius-sm` | 4px | 标签、输入框 |
| `--radius-md` | 8px | 卡片、按钮 |
| `--radius-lg` | 12px | 大卡片、弹窗 |
| `--radius-xl` | 16px | 模态框、大容器 |
| `shadow-sm` | `0 1px 2px rgba(0,0,0,0.05)` | 悬停卡片 |
| `shadow-md` | `0 4px 6px rgba(0,0,0,0.07)` | 下拉菜单、悬浮面板 |
| `shadow-lg` | `0 10px 15px rgba(0,0,0,0.1)` | 模态框、AI 助手展开 |

---

## 3. 图标策略

### 3.1 图标库

- **主图标库**：`@phosphor-icons/react`
- **权重规则**：
  - 导航/列表：`weight="regular"`
  - 强调/状态：`weight="fill"` 或 `weight="bold"`
  - 小尺寸操作（12-14px）：`weight="bold"`
- **逐步替换**：shadcn/ui 组件内部仍依赖 `lucide-react`；自定义组件全部使用 Phosphor，避免两套图标混用。

### 3.2 常用图标映射

| 场景 | 图标（Phosphor） |
|------|------------------|
| Dashboard | `ChartPie` |
| Documents | `FileText` |
| Links | `Link` |
| Deal Rooms | `FolderOpen` |
| Contacts | `Users` |
| Insights | `ChartLineUp` |
| Settings | `Gear` |
| Upload | `UploadSimple` |
| Search | `MagnifyingGlass` |
| Copy link | `Copy` |
| More actions | `DotsThree` |
| Active toggle | `Switch`（组件） |
| Delete | `Trash` |
| Download | `DownloadSimple` |
| Preview | `Eye` |
| PRO / Crown | `Crown` |

---

## 4. 组件模式

### 4.1 页面头（Page Header）

所有列表/管理页统一结构：

```text
┌─────────────────────────────────────────────────────────────┐
│ 标题（.text-h1）                                              │
│ 副标题（.text-body text-muted-foreground）                   │
│                                            [主按钮] [次按钮]  │
└─────────────────────────────────────────────────────────────┘
```

- 标题左对齐，操作右对齐。
- 分析类页面右上角可放置**时间范围选择器**。

### 4.2 列表项（List Row）

DealSignal 列表采用**卡片式行**，而非密集表格。每行包含：

```text
┌────────────────────────────────────────────────────────────────┐
│ [Icon] 主文本                    [badge1] [badge2] [actions ⋮] │
│        元数据 · 元数据                                          │
└────────────────────────────────────────────────────────────────┘
```

规则：
- 主文本最多一行，溢出截断。
- 元数据使用 `.text-caption text-muted-foreground`。
- 行内操作聚合为 `RowActions` 下拉菜单，避免超过 2 个按钮并列。
- 行悬停时背景变为 `--muted`。

### 4.3 表格（Data Table）

当需要批量选择/排序/筛选时使用 `@tanstack/react-table` + shadcn `Table`：

- 表头使用 `.text-caption text-muted-foreground`、大写/正常大小写均可，保持全站一致。
- 行高 56-64px，保证可点击区域。
- 排序图标使用 `CaretUp` / `CaretDown`。
- 空状态占据整行，居中展示。

### 4.4 空状态（Empty State）

必须包含三要素：

1. 图标或插画（使用 Phosphor 大图标，64px）。
2. 明确文案，说明“当前没有数据”以及“为什么需要数据”。
3. 主操作按钮，引导用户创建/分享。

示例：

```text
┌─────────────────────────────────────┐
│           [大图标]                  │
│      暂无热度数据                   │
│   分享第一份文档即可看到热度分析     │
│        [去创建链接]                 │
└─────────────────────────────────────┘
```

### 4.5 行内操作菜单（RowActions）

使用 `DropdownMenu` 聚合：

- 复制链接
- 创建链接
- 下载
- 重命名
- 删除（红色，带 `Trash` 图标）
- PRO 功能（带 `Crown` 图标与 `PRO` badge）

### 4.6 热度标签（HeatBadge）

沿用现有实现：

| 状态 | 背景 | 文字 | 边框 |
|------|------|------|------|
| Hot | `bg-hot-500/10` | `text-hot-500` | `border-hot-500/20` |
| Warm | `bg-warm-500/10` | `text-warm-500` | `border-warm-500/20` |
| Cold | `bg-cold-500/10` | `text-cold-500` | `border-cold-500/20` |

- 使用圆角全角 pill（`rounded-full`）。
- 文字为中文标签：高热度 / 中热度 / 低热度。

### 4.7 权限强度卡片

沿用 `PermissionSlider` 组件：

- 低摩擦：绿色，图标 `LockKeyOpen`
- 中强度：黄色，图标 `Lock`
- 高强度：红色，图标 `Shield`

 slider 三档：0 / 1 / 2，步进 1。

### 4.8 文件类型图标（FileTypeIcon）

根据 `fileType` 渲染不同图标与颜色：

| 类型 | 图标 | 颜色 |
|------|------|------|
| pdf | `FilePdf` | `text-red-500` |
| docx | `FileDoc` | `text-blue-500` |
| pptx | `FilePpt` | `text-orange-500` |
| xlsx | `FileXls` | `text-green-500` |

 fallback 使用 `FileText`。

---

## 5. 页面设计规范

### 5.1 Dashboard（交易雷达）

```text
┌─────────────────────────────────────────────────────────────────────┐
│ 交易雷达                                   [Last 7 days ▼]          │
│ 追踪文档热度，识别高意向对象，快速采取行动。                          │
├─────────────────────────────────────────────────────────────────────┤
│ [Hot card] [Warm card] [Cold card]                                  │
├─────────────────────────────────────────────────────────────────────┤
│ [最近文档 card]              [最近链接 card]                         │
├─────────────────────────────────────────────────────────────────────┤
│ [高热度提醒 card]                                                   │
└─────────────────────────────────────────────────────────────────────┘
```

规则：
- 热度卡片可点击，跳转 Insights 对应 tab。
- 空状态使用统一 EmptyState 组件，文案带 CTA。
- 后续加入 Views Overview 趋势图时，放置在热度卡片下方。

### 5.2 Documents（文档库）

采用**卡片式列表 + 表头**混合布局：

```text
┌─────────────────────────────────────────────────────────────────────┐
│ 文档库                                      [+ 上传文档] [More ▼]   │
│ 管理所有已上传材料，追踪传播与热度。                                 │
│ [🔍 搜索...] [筛选 ▼]                                               │
│ 📄 3 个文档                                                         │
├─────────────────────────────────────────────────────────────────────┤
│ [PDF] Acme Pitch Deck.pdf                  [Hot] [47 views] [⋯]    │
│       18 页 · 4.2 MB · 2 天前 · 3 个链接                             │
├─────────────────────────────────────────────────────────────────────┤
│ [XLS] Financial Model 2026-2028            [Warm] [12 views] [⋯]   │
│       12 页 · 1.8 MB · 3 天前 · 1 个链接                             │
└─────────────────────────────────────────────────────────────────────┘
```

规则：
- 标题使用 `.text-h1`，副标题 `.text-body text-muted-foreground`。
- 主按钮“上传文档”跳转 `/documents/upload`。
- 行点击跳转文档详情或 Viewer。
- 行内操作包含：创建链接、预览、下载、删除。

### 5.3 Links（链接管理）

上下分区：

```text
┌─────────────────────────────────────────────────────────────────────┐
│ 链接管理                                                            │
│ 创建并管理可追踪的分享链接。                                        │
├─────────────────────────────────────────────────────────────────────┤
│ [PermissionSlider 创建区]                                           │
├─────────────────────────────────────────────────────────────────────┐
│ 全部链接                                                            │
│ [Name] [Document] [Views] [Avg Duration] [Last Viewed] [Active]    │
│ Link #b9xkv  Pitch Deck    0    -          -          [●] Yes  [⋯]  │
└─────────────────────────────────────────────────────────────────────┘
```

规则：
- 链接列表使用 TanStack Table。
- `Active` 列使用 `Switch`。
- 复制按钮在 Link 列直接可用。

### 5.4 Deal Rooms / Contacts / Insights / Settings

当前为占位页，后续按此规范扩展：

- 统一 PageHeader。
- 列表优先于网格。
- 空状态使用 EmptyState 组件。

---

## 6. 交互与动效

### 6.1 动效强度

MOTION_INTENSITY = 4（克制但愉悦）：

| 场景 | 动画 | 时长 | 缓动 |
|------|------|------|------|
| 页面进入 | `opacity` + `translateY(12px → 0)` | 400ms | `cubic-bezier(0.16, 1, 0.3, 1)` |
| 卡片悬停 | `background-color` + `box-shadow` | 150ms | ease |
| 下拉菜单 | `scale` + `opacity` | 150ms | ease-out |
| AI 助手展开/收起 | `scale` + `opacity` | 200ms | `cubic-bezier(0.16, 1, 0.3, 1)` |

### 6.2 Reduced Motion

- 使用 `useReducedMotion` hook 检测系统偏好。
- `prefers-reduced-motion: reduce` 时禁用所有位移动画，仅保留透明度或完全不动画。
- `motion.exit` 不能传 `false`，应传 `undefined`。

---

## 7. 无障碍与反 AI Slop

### 7.1 无障碍

- 所有图标按钮必须带 `aria-label`。
- 表单输入必须关联 `Label`。
- 颜色对比度 ≥ 4.5:1（主文本与背景）。
- 聚焦环使用 `focus-visible:ring-2 focus-visible:ring-ring`。

### 7.2 反 AI Slop 清单

- ✅ 不使用默认 `Inter`，使用 `Geist`。
- ✅ 不使用低对比度占位灰色作为 UI 主色。
- ✅ 不使用随机圆角，统一 token。
- ✅ 不在卡片内再套多层卡片。
- ✅ 不滥用渐变，背景使用纯色或极淡阴影。
- ✅ 按钮文案具体：用“创建链接”代替“提交”。
- ✅ 列表项有明确悬停反馈。
- ✅ 空状态不只有文字，必须有图标和 CTA。

---

## 8. 商业化提示设计

### 8.1 PRO badge

对付费功能使用统一 PRO badge：

```text
[Crown] PRO
```

- badge 使用 `variant="outline"` + `text-xs`。
- 不阻断操作，hover 时显示 tooltip 说明升级后可用。

### 8.2 用量提示

在资源管理页和 Sidebar 展示：

```text
1 / 50 links
████░░░░░░
```

- 进度条颜色：正常为 `primary`，接近上限为 `warning`，超出为 `error`。
- 文案简洁，点击可跳转账单/升级。

### 8.3 升级卡片

在合适位置（Dashboard 空状态旁、Sidebar 底部）放置升级卡片：

- 标题带 ✨ 或 Crown 图标。
- 说明升级后可解锁的具体能力（如自定义域名、数据室）。
- CTA 按钮明确：“升级至 Business”。

---

## 9. 响应式策略

| 断点 | 行为 |
|------|------|
| `< 768px` | Sidebar 变为抽屉式，TopNav 显示汉堡菜单，列表行操作聚合为 ⋮ |
| `768px ~ 1024px` | Sidebar 可折叠，主内容区保持两列网格 |
| `> 1024px` | Sidebar 默认展开，支持三列热度卡片 + 两列文档/链接卡片 |

---

## 4.4 页面头（Page Header）

所有列表/管理页统一结构：

```text
┌─────────────────────────────────────────────────────────────┐
│ 标题（.text-h1）                                              │
│ 副标题（.text-body text-muted-foreground）                   │
│                                            [主按钮] [次按钮]  │
└─────────────────────────────────────────────────────────────┘
```

- 标题左对齐，操作右对齐。
- 分析类页面右上角可放置**时间范围选择器**。

### 4.5 详情布局（Detail Layout）

详情页采用两栏布局：

```text
┌──────────────────────────────┬─────────────────────────────┐
│ 主内容区                      │ 侧边摘要区                   │
│ 图表 / 时间线 / 列表          │ 统计卡片 / 操作 / 元信息     │
└──────────────────────────────┴─────────────────────────────┘
```

- 主内容区最小宽度保证可读性。
- 侧边栏宽度固定 320px，在 lg 以下断点自动堆叠。

### 4.6 统计卡片（Stat Card）

用于详情页侧边栏：

- 标签使用 `.text-caption text-muted-foreground`。
- 数值使用 `.text-h2 tabular-nums`。
- 可选图标居右上。

### 4.7 趋势图（Trend Chart）

当前为占位组件，使用 CSS 柱状图：

- 高度固定 192px。
- 柱子使用 `bg-primary/10`，hover 为 `bg-primary/20`。
- 后续可替换为 Recharts。

### 4.8 活动时间线（Activity Timeline）

- 左侧圆点 + 竖线连接。
- 时间使用 `.text-caption text-muted-foreground`。
- 标题使用 `.text-sm font-medium`。
- 描述可选。

### 4.9 访客列表（Visitor List）

- 头像占位（圆形 muted 背景 + User 图标）。
- 邮箱主文本，组织/访问次文本。
- 右侧 HeatBadge。
- 点击跳转 Contacts 详情。

### 4.10 用量条（Usage Bar）

- 标签与数值左右分布。
- 进度条颜色：正常 `primary`，≥80% `warning`，≥100% `error`。

### 4.11 权限标签（Permission Badge）

- `public` / `email` / `whitelist` / `password` / `nda` 五种状态。
- 使用 shadcn Badge 变体区分。

---

## 5. 新增页面设计规范

### 5.8 Document Detail（文档详情）

- 使用 PageHeader + 返回链接。
- 主内容：页面停留柱状图（占位）。
- 侧边：文档统计卡片、热度分布。
- 下方：此文档的链接列表、最近访客。

### 5.9 Link Detail（链接详情）

- 使用 PageHeader + 返回链接。
- 主内容：访问量趋势图（占位）、访问者时间线。
- 侧边：链接统计、权限配置摘要。
- 下方：访问日志表格。

### 5.10 Deal Rooms（数据室）

- 列表页：卡片式列表，展示模板、文档数、成员数、待审批数。
- 创建页：模板选择 → 基础信息 → 添加文档 → 预览。
- 详情页：三栏布局（文件夹树 / 文档列表 / 成员与活动）。

### 5.11 Contacts（访问者）

- 列表页：以人为中心的卡片列表，展示组织、热度、跨文档行为。
- 详情页：两栏布局（活动时间线 / 热度评分与跟进建议）。

### 5.12 Insights（洞察）

- Overview：热度分布 + 趋势 + Top 10 排名。
- Pages：文档选择器 + 页面停留柱状图 + 退出率 + 页面排名。
- Suggestions：优先级排序的建议卡片列表。

### 5.13 Settings（设置）

- 左侧子导航，右侧内容区。
- General：Workspace 信息。
- Brand：Logo、颜色、欢迎语、自定义域名。
- Members：成员列表、邀请、角色管理。
- Integrations：CRM / Slack 授权与映射。
- Billing：计划、用量、升级。
- Security：2FA、审计日志、数据保留。

---

## 10. 文件约定

| 目录 | 用途 |
|------|------|
| `src/components/ui` | shadcn/ui 基础组件，尽量不直接修改 |
| `src/components/layout` | AppShell、Sidebar、TopNav、WorkspaceSwitcher |
| `src/components/dashboard` | Dashboard 专属组件 |
| `src/components/documents` | Documents 列表/详情组件 |
| `src/components/links` | PermissionSlider、LinksTable、LinkDetail |
| `src/components/deal-rooms` | DealRoomList、DealRoomCreate、DealRoomDetail |
| `src/components/contacts` | ContactList、ContactDetail |
| `src/components/insights` | InsightsOverview、InsightsPages、InsightsSuggestions |
| `src/components/settings` | SettingsNav、BrandSettings、MemberSettings 等 |
| `src/components/viewer` | CanvasViewer、AIChat |
| `src/components/common` | 跨页面复用组件：EmptyState、FileTypeIcon、HeatBadge、RowActions、PageHeader、DetailLayout、StatCard、TrendChart、ActivityTimeline、VisitorList、PermissionBadge、UsageBar |
| `src/routes` | 页面级路由组件，尽量薄 |
| `src/lib/mocks` | MSW / 本地 mock 数据 |
| `src/stores` | zustand 状态管理 |

---

## 11. 变更日志

| 版本 | 日期 | 修改内容 |
|------|------|----------|
| v2.1.0 | 2026-06-20 | 初始版本：整合 UI-DESIGN-DELIVERABLE token、Papermark 竞品模式、反 AI Slop 规则、组件与页面规范。 |

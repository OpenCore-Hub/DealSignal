# DealSignal v2.1.1 设计 Token 规范

> 用于收敛当前界面色彩、字号、间距、阴影语义混乱的问题，统一 Signal-First 视觉语言。

---

## 1. 色彩 Token

### 1.1 品牌/中性色

| Token | Light | Dark | 用途 |
|-------|-------|------|------|
| `--background` | `#ffffff` | `#020617` | 页面背景 |
| `--foreground` | `#0f172a` | `#f8fafc` | 主要文字 |
| `--card` | `#ffffff` | `#0f172a` | 卡片背景 |
| `--card-foreground` | `#0f172a` | `#f8fafc` | 卡片文字 |
| `--popover` | `#ffffff` | `#0f172a` | 浮层背景 |
| `--primary` | `#0f172a` | `#f8fafc` | 主按钮/链接 |
| `--primary-foreground` | `#ffffff` | `#0f172a` | 主按钮文字 |
| `--secondary` | `#f1f5f9` | `#1e293b` | 次级背景 |
| `--muted` | `#f1f5f9` | `#1e293b` | 静音背景 |
| `--muted-foreground` | `#64748b` | `#94a3b8` | 辅助文字 |
| `--border` | `#e2e8f0` | `rgba(148,163,184,0.2)` | 边框 |
| `--input` | `#e2e8f0` | `rgba(148,163,184,0.2)` | 输入框边框 |
| `--ring` | `#0f172a` | `#f8fafc` | focus ring |

### 1.2 热度 Scale（独立，不与状态色混淆）

| Token | 值 | 用途 |
|-------|-----|------|
| `--color-hot-500` | `#e11d48` (rose-600) | 高热度信号、紧急行动 |
| `--color-hot-100` | `#ffe4e6` | 高热度背景 |
| `--color-warm-500` | `#d97706` (amber-600) | 中热度信号 |
| `--color-warm-100` | `#fef3c7` | 中热度背景 |
| `--color-cold-500` | `#64748b` (slate-500) | 低热度/沉寂 |
| `--color-cold-100` | `#f1f5f9` | 低热度背景 |

### 1.3 风险 Scale

| Token | 值 | 用途 |
|-------|-----|------|
| `--color-risk-500` | `#7c3aed` (violet-600) | 异常访问、过期链接、高敏下载 |
| `--color-risk-100` | `#ede9fe` | 风险背景 |

### 1.4 状态 Scale

| Token | 值 | 用途 |
|-------|-----|------|
| `--color-success-500` | `#10b981` | 成功、完成、正向 |
| `--color-warning-500` | `#f59e0b` | 警告、待审批 |
| `--color-error-500` | `#ef4444` | 错误、删除、破坏性操作 |
| `--color-info-500` | `#3b82f6` | 提示、信息 |

---

## 2. 字体 Token

### 2.1 字号（以 rem 为基准，body 16px）

| Class | Size | Line Height | Weight | 用途 |
|-------|------|-------------|--------|------|
| `.text-display` | `2rem` (32px) | `2.5rem` | 700 | 登录/空态大标题 |
| `.text-h1` | `1.5rem` (24px) | `2rem` | 600 | 页面标题 |
| `.text-h2` | `1.25rem` (20px) | `1.75rem` | 600 | 页面级模块标题 |
| `.text-h3` | `1rem` (16px) | `1.5rem` | 600 | 卡片标题、sidebar 标题 |
| `.text-body` | `0.875rem` (14px) | `1.375rem` | 400 | 正文 |
| `.text-caption` | `0.75rem` (12px) | `1.125rem` | 400 | 辅助说明 |
| `.text-stat` | `2.25rem` (36px) | `2.5rem` | 700 | 摘要数字 |

### 2.2 数字与等宽

- 所有数据数字使用 `font-variant-numeric: tabular-nums`。
- 分数、访问次数、金额等使用 `.text-stat`。

---

## 3. 间距 Token

| Token | 值 | 用途 |
|-------|-----|------|
| `--space-1` | `0.25rem` (4px) | 图标与文字间距 |
| `--space-2` | `0.5rem` (8px) | 紧凑内联间距 |
| `--space-3` | `0.75rem` (12px) | 表单行间距 |
| `--space-4` | `1rem` (16px) | 卡片内部小间距 |
| `--space-5` | `1.25rem` (20px) | 卡片内部标准间距 |
| `--space-6` | `1.5rem` (24px) | 模块之间间距 |
| `--space-8` | `2rem` (32px) | 大模块间距 |
| `--space-10` | `2.5rem` (40px) | 页面级间距 |

---

## 4. 圆角与阴影

### 4.1 圆角

| Token | 值 | 用途 |
|-------|-----|------|
| `--radius-sm` | `0.25rem` (4px) | 小按钮、标签 |
| `--radius-md` | `0.5rem` (8px) | 输入框、小卡片 |
| `--radius-lg` | `0.75rem` (12px) | 卡片 |
| `--radius-xl` | `1rem` (16px) | 弹窗、大卡片 |

### 4.2 阴影

| Token | 值 | 用途 |
|-------|-----|------|
| `--shadow-card` | `0 1px 3px rgba(0,0,0,0.05)` | 卡片默认 |
| `--shadow-dropdown` | `0 4px 12px rgba(0,0,0,0.08)` | 下拉菜单、浮层 |
| `--shadow-modal` | `0 12px 40px rgba(0,0,0,0.12)` | 弹窗、AI 助手面板 |

---

## 5. 组件语义规范

### 5.1 卡片

- 默认背景 `bg-card`，边框 `border border-border`，圆角 `rounded-lg`，阴影 `shadow-card`。
- 可点击卡片 hover：`hover:bg-muted/50` + `hover:border-muted-foreground/20`；不使用 `hover:shadow-sm`。
- 真正需要强调浮起的卡片（如 Dashboard 摘要卡）才使用 `hover:shadow-sm`。

### 5.2 按钮

| 变体 | 用途 |
|------|------|
| `default` | 页面主行动 |
| `secondary` | 次行动/折叠面板触发 |
| `outline` | 低频操作 |
| `ghost` | 图标按钮、文本链接 |
| `destructive` | 删除、断开连接等不可逆操作 |

### 5.3 Badge

| 变体 | 用途 |
|------|------|
| `default` | 普通标签 |
| `secondary` | 状态标签（public、未启用） |
| `outline` | 权限/配置标签 |
| `hot/warm/cold/risk` | 热度与风险语义 |

### 5.4 图标

- 标题前图标仅在概念陌生或需要快速识别时使用；常规数据卡片不再强制加图标。
- 图标尺寸：标题旁 16px，行内 14px，按钮内 16px。

---

## 6. 响应式断点

| 断点 | 宽度 | 行为 |
|------|------|------|
| `sm` | 640px | 小屏手机 |
| `md` | 768px | 平板，侧边栏可折叠 |
| `lg` | 1024px | 桌面，双栏布局 |
| `xl` | 1280px | 大屏，三栏/宽表格 |

---

## 7. 动效

- 默认进入动画：`opacity 0→1`, `translateY 12px→0`, `duration 300ms`, `ease [0.16,1,0.3,1]`。
- `prefers-reduced-motion: reduce` 时所有 JS 动画禁用（通过 `useReducedMotion`）。
- hover 过渡使用 `transition-colors` 或 `transition-opacity`，避免 `transition-all`。

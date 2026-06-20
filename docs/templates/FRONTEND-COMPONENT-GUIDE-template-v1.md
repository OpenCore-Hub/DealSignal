---
id: "FCG-YYYY-NNN"
version: "{vX.Y.Z}"
status: "{草稿 / 评审中 / 已批准 / 已归档}"
owner: "{负责人}"
---

# {项目名称} 前端组件规范指南

> **文档编号**：`FCG-YYYY-NNN`  
> **版本**：`{vX.Y.Z}`  
> **模板版本**：`v1`  
> **状态**：`{草稿 / 评审中 / 已批准 / 已归档}`  
> **编写人/适用对象**：`前端负责人 / 高级前端工程师`  
> **编写日期**：`{YYYY-MM-DD}`  
> **关联文档**：  
> - `docs/PRD-vX.Y.Z.md`  
> - `docs/TDD-vX.Y.Z.md`  
> - `docs/templates/UI-DESIGN-DELIVERABLE-template-v1.md`  
> - `docs/templates/ACCESSIBILITY-CONFORMANCE-template-v1.md`  
> - `docs/templates/API-SPEC-template-v1.md`  
> - `docs/DESIGN.md`（如有设计系统）  
> **评审人**：`前端负责人、产品设计师、测试负责人、架构师`

---

## 0. 文档使用说明

本文档定义 `{项目名称}` 前端组件的开发规范、目录结构、Storybook 使用、可访问性实现、状态管理约定与交付流程。

**目标**：
- 建立统一、可维护、可复用的前端组件体系。
- 提升设计与开发交付一致性，降低沟通成本。
- 确保组件具备良好的可测试性、可访问性与性能表现。

**适用范围**：
- 所有前端项目，包括 Web App、管理后台、H5、小程序（如适用）。

---

## 1. 文档控制信息

### 1.1 变更日志

| 版本 | 日期 | 修改人 | 修改内容 | 影响范围 |
|------|------|--------|----------|----------|
| v0.1.0 | YYYY-MM-DD | {编写人} | 初始版本 | 全文档 |

### 1.2 技术栈

| 层级 | 选型 | 说明 |
|------|------|------|
| 框架 | `{React / Vue / Svelte / Angular}` | {版本与选型理由} |
| 语言 | `{TypeScript}` | 强制启用严格模式 |
| 样式 | `{CSS Modules / Tailwind / Styled Components / SCSS}` | {样式方案} |
| 组件文档 | `{Storybook}` | 组件开发与验收平台 |
| 测试 | `{Vitest / Jest + React Testing Library}` | 单元与集成测试 |
| 状态管理 | `{Zustand / Redux Toolkit / Pinia / Context}` | {选型} |
| 构建工具 | `{Vite / Next.js / Webpack}` | {构建方案} |

---

## 2. 范围与目标

### 2.1 范围

本文档覆盖：

- 项目目录结构与文件组织。
- 组件分类与命名规范。
- 组件接口（Props）设计原则。
- Storybook 编写与维护要求。
- 可访问性（a11y）实现标准。
- 状态管理、数据获取与错误处理约定。
- 性能、安全与代码质量规范。

### 2.2 目标

- 组件职责单一、接口清晰、易于复用。
- 新组件开发有标准模板，降低个人风格差异。
- UI 组件可在 Storybook 中独立运行、测试与验收。
- 所有组件满足 WCAG 2.1 AA 基线要求。
- 状态管理边界清晰，避免过度全局化。

---

## 3. 目录结构

### 3.1 推荐目录结构

```text
src/
├── assets/              # 静态资源（图片、字体、图标）
├── components/          # 通用 UI 组件
│   ├── atoms/           # 原子组件：Button, Input, Icon
│   ├── molecules/       # 分子组件：FormField, SearchBar
│   ├── organisms/       # 有机体组件：Header, DataTable
│   └── templates/       # 页面级模板：DashboardLayout
├── features/            # 业务功能模块
│   └── {feature-name}/
│       ├── api/         # 该功能相关的 API 调用
│       ├── components/  # 功能私有组件
│       ├── hooks/       # 功能私有 hooks
│       ├── stores/      # 功能私有状态
│       └── types/       # 功能私有类型
├── hooks/               # 全局通用 hooks
├── lib/                 # 工具函数与第三方封装
├── providers/           # 全局 Context/Provider
├── routes/              # 路由定义
├── stores/              # 全局状态管理
├── styles/              # 全局样式、主题变量
├── types/               # 全局类型定义
└── utils/               # 通用工具函数
```

### 3.2 组件文件组织

每个组件目录建议包含：

```text
Button/
├── index.ts             # 统一导出
├── Button.tsx           # 组件实现
├── Button.types.ts      # Props 与类型定义（可合并到 .tsx）
├── Button.stories.tsx   # Storybook 故事
├── Button.test.tsx      # 单元测试
└── Button.module.css    # 组件样式（按项目选型）
```

---

## 4. 组件规范

### 4.1 组件分类

| 分类 | 说明 | 示例 |
|------|------|------|
| 基础组件（Base） | 无业务语义，可被任何项目复用 | Button, Input, Modal, Toast |
| 业务组件（Business） | 包含业务逻辑或特定数据结构 | UserCard, OrderList, PaymentForm |
| 布局组件（Layout） | 控制页面结构与响应式布局 | Sidebar, PageHeader, Grid |
| 页面组件（Page） | 路由级入口，组合多个组件 | DashboardPage, SettingsPage |

### 4.2 命名规范

| 项目 | 规范 | 示例 |
|------|------|------|
| 组件文件 | PascalCase | `UserProfile.tsx` |
| Hook 文件 | camelCase 前缀 use | `useAuth.ts` |
| 工具函数 | camelCase | `formatDate.ts` |
| 类型/接口 | PascalCase | `UserProfileProps` |
| 常量 | SCREAMING_SNAKE_CASE | `MAX_RETRY_COUNT` |
| CSS 类名 | BEM / utility-first | `button--primary` / `flex gap-2` |

### 4.3 Props 设计原则

| 原则 | 说明 |
|------|------|
| 接口最小化 | 只暴露必要的 Props，避免过度配置 |
| 默认值合理 | 为可选 Props 提供符合最常见场景的默认值 |
| 事件命名 | 使用 `onXxx` 命名事件回调 |
| 类型严格 | 所有 Props 必须定义 TypeScript 类型 |
| 转发 ref | 需要时支持 `React.forwardRef` |
| 样式扩展 | 提供 `className` / `style` 扩展点，但避免样式污染 |

### 4.4 Props 模板

```tsx
export interface ButtonProps {
  variant?: 'primary' | 'secondary' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  disabled?: boolean;
  loading?: boolean;
  children: React.ReactNode;
  onClick?: (event: React.MouseEvent<HTMLButtonElement>) => void;
  className?: string;
  // 其他业务相关 props
}
```

---

## 5. Storybook 规范

### 5.1 Storybook 使用目标

- 作为组件开发环境，支持孤立开发与快速迭代。
- 作为设计-开发验收平台，确保实现与设计一致。
- 作为组件文档，供新人学习与跨团队复用。

### 5.2 每个组件必须包含的故事

| 故事 | 说明 |
|------|------|
| Default | 默认状态，展示最常用形态 |
| Variants | 所有变体（如 primary/secondary/danger） |
| Sizes | 所有尺寸 |
| States | 禁用、加载、错误、空状态 |
| Interactive | 可交互示例，展示事件回调 |
| Accessibility | 键盘导航、屏幕阅读器状态 |

### 5.3 Story 编写模板

```tsx
import type { Meta, StoryObj } from '@storybook/react';
import { Button } from './Button';

const meta: Meta<typeof Button> = {
  title: 'Components/Button',
  component: Button,
  tags: ['autodocs'],
  argTypes: {
    variant: { control: 'select', options: ['primary', 'secondary', 'danger'] },
    size: { control: 'select', options: ['sm', 'md', 'lg'] },
  },
};

export default meta;
type Story = StoryObj<typeof Button>;

export const Default: Story = {
  args: {
    children: 'Click me',
  },
};

export const Loading: Story = {
  args: {
    children: 'Saving',
    loading: true,
  },
};
```

---

## 6. 可访问性（a11y）实现

### 6.1 基础要求

- 所有项目以 WCAG 2.1 AA 为最低标准。
- 关键组件需通过 axe-core 自动扫描。
- 复杂组件需配合屏幕阅读器与键盘进行手动测试。

### 6.2 通用规范

| 组件 | 要求 |
|------|------|
| 按钮 | 必须有可访问名称，支持 Enter/Space 激活 |
| 链接 | 使用原生 `<a>` 或具备 role/link 行为 |
| 表单输入 | 必须关联 `<label>` 或 `aria-labelledby` |
| 错误提示 | 使用 `aria-describedby` / `aria-invalid` 关联 |
| 模态框 | `role="dialog"`、`aria-modal="true"`、焦点管理 |
| 下拉菜单 | 支持方向键导航、Escape 关闭、焦点回归 |
| 通知/Toast | `role="status"` 或 `role="alert"`，提供关闭按钮 |
| 加载状态 | 使用 `aria-busy` / `aria-live` 告知辅助技术 |

### 6.3 焦点管理

- 焦点顺序符合视觉顺序。
- 模态框打开时焦点锁定在框内，关闭时返回触发元素。
- 所有交互元素必须有可见焦点样式。

### 6.4 可访问性测试

| 测试类型 | 工具/方式 | 频率 |
|----------|-----------|------|
| 自动扫描 | axe-core / Storybook a11y addon | 每次提交 |
| 键盘测试 | 手动 Tab 导航 | 开发自测 |
| 屏幕阅读器 | NVDA / JAWS / VoiceOver | 复杂组件 |
| 对比度检查 | Lighthouse / axe | 每次视觉调整 |

---

## 7. 状态管理约定

### 7.1 状态分类

| 状态类型 | 位置 | 示例 |
|----------|------|------|
| UI 局部状态 | 组件内部 useState | 模态框开关、表单输入 |
| 共享 UI 状态 | 局部 Context / 轻量 store | 主题、语言、侧边栏展开 |
| 服务端状态 | 数据获取库（TanStack Query / SWR） | 用户数据、列表数据 |
| 全局业务状态 | 全局 store | 购物车、登录态 |

### 7.2 状态管理原则

- 状态尽量靠近使用它的组件，避免不必要的全局化。
- 服务端状态优先使用专门的数据获取库管理缓存、重试、去重。
- 全局状态应分组管理，避免单一大 store。
- 状态更新不可变，禁止直接修改原对象。

### 7.3 数据获取与错误处理

| 场景 | 约定 |
|------|------|
| 数据加载 | 显示 Skeleton 或 Loading 状态 |
| 加载失败 | 显示 Error Boundary 或局部错误提示 |
| 空数据 | 显示 Empty 占位组件 |
| 重试 | 提供用户可触发的重试按钮 |
| 乐观更新 | 在提交后立即更新 UI，失败时回滚 |

---

## 8. 性能与质量

### 8.1 性能规范

| 项目 | 要求 |
|------|------|
| 包体积 | 首屏 JS 体积 ≤ {X} KB（gzip） |
| 图片优化 | 使用 WebP/AVIF、懒加载、响应式图片 |
| 代码分割 | 按路由与大型组件进行动态导入 |
| 渲染优化 | 避免不必要重渲染，善用 memo/useMemo/useCallback |
| 字体加载 | 使用 font-display: swap，避免 FOIT |

### 8.2 代码质量

- 启用 ESLint + Prettier + TypeScript 严格模式。
- 关键组件与 hooks 必须编写单元测试。
- 禁止在代码中硬编码颜色、间距等设计 Token，应从主题获取。
- 禁止直接操作 DOM，除非必要且已评审。

---

## 9. 检查清单

- [ ] 目录结构符合规范，组件分类清晰。
- [ ] 所有组件都有 TypeScript 类型定义。
- [ ] 通用组件已在 Storybook 中编写并维护。
- [ ] 组件满足 WCAG 2.1 AA 基线要求。
- [ ] 键盘导航、焦点管理、屏幕阅读器已验证。
- [ ] 状态管理按分类使用合适的方案。
- [ ] 数据获取具备加载、错误、空状态、重试处理。
- [ ] 性能优化措施已纳入开发 checklist。
- [ ] 组件代码通过 ESLint/Prettier/TypeScript 检查。
- [ ] 关键组件已编写单元测试并通过。

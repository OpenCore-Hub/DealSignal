# Agent 工作规范

## 项目背景

DealSignal 是一个面向创始人、投资人和销售团队的智能文档分享与交易信号平台。前端采用 React + Vite + Tailwind CSS + Base UI 构建。

## 工作原则

1. **设计优先**：任何 UI 改动前，先对照 `docs/DESIGN-TOKENS-v2.1.1.md` 与 `docs/INTERACTION-SPEC-v2.1.1-REFINED.md`。
2. **零假交互**：不要保留 `onClick={() => {}}`。未实现的功能应 `disabled` 并加 Tooltip 说明，或从 UI 移除。
3. **即时反馈**：复制、删除、保存、重置等操作必须提供图标变化、toast 或二次确认。
4. **键盘可达**：所有可点击元素必须可通过 Tab 聚焦，Enter/Space 触发。
5. **中文 SaaS 语境**：界面标签、微文案、数据单位使用中文。
6. **不要引用 Mock**：业务组件应调用 `src/lib/api.ts` 中的 API，不要直接 import `src/lib/mocks/data.ts`。
7. **MSW 仅开发**：生产构建不要启动 MSW；Mock 逻辑只写在 `src/lib/mocks/` 中。

## 目录约定

- `apps/web/src/routes/`：页面入口，尽量保持薄。
- `apps/web/src/components/`：业务与通用组件。
- `apps/web/src/components/ui/`：基础 UI 组件，基于 Base UI 封装。
- `apps/web/src/lib/`：API 客户端、工具函数、类型。
- `apps/web/src/stores/`：Zustand 状态管理。

## 代码风格

- 使用 TypeScript 严格模式。
- Tailwind 类名优先使用语义化 Token，避免任意值（如 `text-[10px]`）。
- 组件 props 必须显式类型化。
- 异步请求必须包含错误处理。

## 当前重点

参见 [前端审计与优化计划 v2.1.2](./docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.2.md)。

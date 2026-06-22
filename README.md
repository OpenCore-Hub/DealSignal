# DealSignal

智能文档分享与交易信号平台（Signal-First Document Intelligence）。

## 项目结构

```
├── apps/web          # React + Vite 前端应用
├── docs/             # PRD、TDD、设计文档、实施计划
└── README.md         # 本文件
```

## 快速开始

```bash
# 安装依赖
cd apps/web
pnpm install

# 启动开发服务器（含 MSW Mock）
pnpm dev

# 构建
pnpm build

# 类型检查
npx tsc -b --noEmit

# 代码检查
pnpm lint
```

## 技术栈

- React 19 + TypeScript
- React Router 8
- Vite 8
- Tailwind CSS 4
- Base UI + 自定义 shadcn 风格组件
- Zustand
- Motion（Framer Motion）
- TanStack Table
- MSW（开发环境 Mock）

## 设计文档

- [前端审计与优化计划 v2.1.3](./docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md)
- [产品设计 v2.1.1](./docs/PRODUCT-DESIGN-v2.1.1-REFINED.md)
- [交互规范 v2.1.1](./docs/INTERACTION-SPEC-v2.1.1-REFINED.md)
- [设计令牌 v2.1.1](./docs/DESIGN-TOKENS-v2.1.1.md)
- [API 规范 v2.1.0](./docs/API-SPEC-v2.1.0.md)

## 安全扫描

安全策略与风险接受项见 [`docs/SECURITY.md`](./docs/SECURITY.md)。

常用命令：

```bash
# Go 漏洞扫描
cd apps/api && make security

# Trivy 文件系统扫描
cd apps/api && make trivy-fs

# 前端依赖审计
cd apps/web && pnpm security
```

## 贡献指南

1. 遵循 `apps/web/src/index.css` 中的语义化 Token。
2. 所有新增可点击元素必须支持键盘操作（Tab + Enter/Space）。
3. 所有复制、删除、保存等操作必须提供即时反馈。
4. 不要在生产代码中直接引用 `mock*` 数据；Mock 仅用于 MSW handler。

---
task_id: "TASK-FRONTEND-002"
parent_issue: "DS-010"
agent_task_id: "AGENT-TASK-002"
version: "v2.1.2"
priority: "P1"
status: "已完成"
type: "frontend"
effort: "M"
branch: "feat/agent-task-002-viewer-components"
estimated_files: "8"
max_lines: "400"
project_stack: "React 19 / React Router 8 / Vite 8 / TypeScript / Tailwind CSS 4 / Base UI"
ai_red_flags:
  - "保持现有 Viewer 行为不回归"
  - "Canvas 相关实现需考虑 SSR/hydration 安全"
  - "不得破坏现有路由与公开链接查看"
  - "不得引入未使用的依赖"
ai_confidence: "medium"
pending_confirmation: []
available_tools:
  - "test"
  - "lint"
  - "browse"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-FRONTEND-002` |
> | `parent_issue` | `DS-010` |
> | `agent_task_id` | `AGENT-TASK-002` |
> | **版本** | `v2.1.2` |
> | **模板版本** | `v2` |
> | **优先级** | `P1` |
> | **状态** | `待执行` |
> | **类型** | `frontend` |
> | **预计工作量** | `M` |
> | **分支名** | `feat/agent-task-002-viewer-components` |
> | **AI 置信度** | `medium` |
> | **依赖** | `TASK-FRONTEND-001` |
> | **待人工确认事项** | `已确定：v2.1.2 采用 div/图片占位 + 后端签名 URL，真实 Canvas 渲染延后` |
> | **可用工具/技能** | `test / lint / browse` |

# TASK-FRONTEND-002 Viewer 子组件拆分与 Canvas 体验增强

> **父 Issue**：`DS-010`  
> **版本**：`v2.1.2`  
> **模板版本**：`v2`  
> **优先级**：`P1`  
> **状态**：`待执行`  
> **类型**：`frontend`  
> **预计工作量**：`M`  
> **分支名**：`feat/agent-task-002-viewer-components`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

将 `CanvasViewer` 中内联的缩略图导航、高亮框、水印抽离为独立组件，提升 Viewer 的可测试性与可维护性，同时保持公开/私有文档查看行为不变。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8.4 Viewer |
| TDD | `docs/TDD-v2.1.0.md` §6.3 |
| IMPLEMENTATION-PLAN | `docs/IMPLEMENTATION-PLAN-v2.1.1.md` §5.2 |
| 父 Issue | `DS-010` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 中等；先读 Viewer 路由与 CanvasViewer，再按需读取 store/types。 |
| **必读顺序** | front matter → `ai_red_flags` → 目标 → 边界条件 → 输出。 |
| **分块策略** | 若上下文受限，先读 `CanvasViewer.tsx`，再读 `viewer.tsx` 和测试。 |

### 3.1 已有代码/表（执行前必须阅读）

- `apps/web/src/components/viewer/CanvasViewer.tsx`
- `apps/web/src/routes/viewer.tsx`
- `apps/web/src/lib/mocks/handlers.ts`（Viewer 相关 mock）
- `apps/web/src/i18n/locales/en/viewer.json` 与 `zh-CN/viewer.json`

### 3.2 数据模型/接口

```typescript
interface ViewerPage {
  id: string;
  pageNumber: number;
  imageUrl: string;
  width: number;
  height: number;
}

interface Evidence {
  id: string;
  pageNumber: number;
  bbox: { x: number; y: number; w: number; h: number };
  text: string;
}

interface WatermarkInfo {
  email?: string;
  ip?: string;
  viewedAt?: string;
}
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 公开链接 | 无认证访客可查看 | 组件不依赖用户 store |
| 响应式 | 支持 1280px+ 与移动端 | 缩略图在窄屏可折叠 |
| 性能 | 大文档不卡顿 | 仅渲染可视页缩略图 |
| 最大变更行数 | ≤ 400 | 超出需拆分 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 无 evidence | bbox 为空 | 不高亮 |
| 无水印信息 | visitor 未提供 | 不绘制水印 |
| 单页文档 | pageCount=1 | 缩略图区仍可渲染 |
| 小屏幕 | 宽度 < 768px | 缩略图可折叠/隐藏 |

### 3.5 测试数据 / Mock / Fixture

```json
{
  "document": {
    "id": "doc-001",
    "title": "Q3 Pitch",
    "pages": [
      { "id": "p1", "pageNumber": 1, "imageUrl": "/mock/page1.webp", "width": 800, "height": 1131 }
    ]
  },
  "evidence": {
    "id": "ev-001",
    "pageNumber": 1,
    "bbox": { "x": 100, "y": 200, "w": 300, "h": 40 },
    "text": "Revenue grew 3x."
  },
  "watermark": {
    "email": "visitor@example.test",
    "ip": "203.0.113.1",
    "viewedAt": "2026-06-21T14:00:00Z"
  }
}
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/web/src/components/viewer/ThumbnailNav.tsx` | 新增 | 页面缩略图导航 |
| `apps/web/src/components/viewer/HighlightOverlay.tsx` | 新增 | 根据 bbox 绘制 pulse 高亮 |
| `apps/web/src/components/viewer/WatermarkOverlay.tsx` | 新增 | Canvas/图片上叠加邮箱/时间/IP |
| `apps/web/src/components/viewer/CanvasViewer.tsx` | 修改 | 接入三个子组件 |
| `apps/web/src/components/viewer/CanvasViewer.test.tsx` | 新增 | 子组件渲染与交互测试 |

### 4.2 行为定义

- `ThumbnailNav` 接收 `pages`、`currentPage`、`onSelect`，渲染可点击缩略图列表；当前页高亮。
- `HighlightOverlay` 接收 `evidence[]`、`pageWidth`、`pageHeight`，按相对坐标绘制半透明 pulse 框。
- `WatermarkOverlay` 接收 `watermark`，在页面角落渲染斜向/平铺水印，不影响正文可读性。
- `CanvasViewer` 组合上述组件，保持原有 props 与状态逻辑不变。

---

## 5. 验收标准

- [x] Viewer 页面正常渲染文档、缩略图、高亮、水印
- [x] `pnpm test` 通过
- [x] `pnpm lint` 0 errors
- [x] `pnpm build` 成功
- [x] 公开 Viewer 路由 `/viewer/:documentId` 仍可匿名访问
- [x] 组件拆分后无功能回归

---

## 6. 实现步骤建议

1. 阅读 `CanvasViewer.tsx` 与 `routes/viewer.tsx`，梳理现有 props 与状态。
2. 提取 `ThumbnailNav.tsx`，保持当前页高亮与点击跳转逻辑。
3. 提取 `HighlightOverlay.tsx`，仅做纯展示，接受 bbox 数组。
4. 提取 `WatermarkOverlay.tsx`，默认在页面右下角渲染单条斜向水印；可选平铺模式。
5. 在 `CanvasViewer.tsx` 中替换内联实现为子组件组合。
6. 新增 `CanvasViewer.test.tsx`，覆盖正常渲染、无 evidence、无水印场景。
7. 运行 `pnpm lint && pnpm test && pnpm build`。
8. 提交 PR。

---

## 7. 测试验证

### 7.1 单元测试

```bash
cd apps/web && pnpm test -- CanvasViewer
```

### 7.2 集成/回归测试

```bash
cd apps/web && pnpm lint && pnpm test && pnpm build
```

### 7.3 手动验证

```bash
cd apps/web && pnpm dev
# 访问 /viewer/doc-001，确认缩略图、高亮、水印正常显示
```

### 7.4 回归测试命令

```bash
cd apps/web && pnpm lint && pnpm test && pnpm build
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`。
- **范围锁定**：不得修改本任务范围外的文件；如必须修改，需在 Agent 备注中说明理由并征得审批。
- **保持现有测试通过**：运行全量回归测试命令全绿才能提交。
- **不要提前实现**：范围外的功能（如真实 PDF 渲染、注释工具）不要碰。
- **禁止把敏感数据发送给 LLM**：Token、密码、密钥、PII、生产数据一律不得出现在 prompt、日志或测试数据中。
- **代码风格**：遵循项目已有目录和命名约定。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

| CODE-REVIEW 3.11 检查项 | 本任务自检要求 |
|------------------------|----------------|
| 无硬编码示例域名/邮箱/密码 | 测试数据使用 `example.test` 域名。 |
| 无未清理的 TODO / FIXME / placeholder | 实现完成后全局搜索，确保无残留。 |
| 无幻觉常量 | 颜色、尺寸从 Tailwind theme 或现有组件引用。 |
| 错误处理不过度 try-catch，不吞掉异常 | 组件 props 类型校验明确。 |
| 未引入未使用的依赖或代码 | 提交前运行 lint。 |
| 未擅自实现范围外功能 | 仅拆分 Viewer 子组件，不新增复杂交互。 |
| 测试数据与生产数据隔离 | mock 数据为 fixture。 |

---

## 10. Definition of Done

- [x] 代码实现完成
- [x] 测试通过（单元 + 集成）
- [x] lint / typecheck / build 通过
- [ ] 代码审查通过
- [x] 与父 Issue 的验收标准对齐
- [ ] PR 已关联父 Issue：`Relates to #DS-010`
- [x] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- 如果用户确认使用真实 Canvas 渲染，优先使用 `OffscreenCanvas`/client-only effect，避免 SSR 问题。
- 若保持 div/图片占位，`WatermarkOverlay` 可用绝对定位 div 实现，不必引入 Canvas API。
- 缩略图在移动端建议默认折叠，提供 toggle 按钮。

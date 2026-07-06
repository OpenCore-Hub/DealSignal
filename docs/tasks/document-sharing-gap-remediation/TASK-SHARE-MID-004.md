---
task_id: "TASK-SHARE-MID-004"
parent_issue: "DS-SHARE-008"
agent_task_id: "AGENT-TASK-SHARE-008"
version: "v1.0.0"
priority: "P1"
status: "待执行"
type: "frontend"
effort: "M"
branch: "feat/share-mid-004-dynamic-watermark"
estimated_files: "8"
max_lines: "400"
project_stack: "React 19 + TypeScript + Vite 8 + Canvas API"
ai_red_flags:
  - "水印信息必须来自后端 Access 响应，不可前端伪造"
  - "水印必须覆盖在 Canvas 层之上，截图不易去除"
  - "水印不得影响文字可读性"
  - "性能：水印渲染不能明显拖慢翻页"
ai_confidence: "medium"
pending_confirmation:
  - "水印内容格式：单行还是多行？字体/透明度？"
  - "是否对下载的 PDF 也加水印？"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

# TASK-SHARE-MID-004 公共 Viewer 动态水印

> **父 Issue**：`DS-SHARE-008`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **类型**：`frontend`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-mid-004-dynamic-watermark`

---

## 1. 目标

在公共文档查看器中实现动态水印：当 `watermark_enabled=true` 时，在 Canvas 渲染层上叠加包含访问者邮箱、访问时间、IP 哈希的半透明水印。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.7 |
| PRD | `docs/backup/PRD-v2.1.0.md` FR-09 |
| TDD | `docs/backup/TDD-v2.1.0.md` C-05 |

### 2.1 已有代码

- `apps/web/src/components/viewer/CanvasViewer.tsx` — Canvas 渲染
- `apps/web/src/components/viewer/PublicViewerPage.tsx` — 公共 viewer
- `apps/api/internal/link/handler.go` — `respondAccessSuccess` 返回 `watermarkEnabled`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 水印内容 | 邮箱 + 访问时间 + IP 哈希 | 来自后端 Access 响应 |
| 透明度 | 0.08 ~ 0.15 | 不遮挡正文 |
| 字体 | 系统无衬线字体 | 如 14px Inter/sans-serif |
| 布局 | 倾斜 30° 平铺 | 覆盖整个 Canvas |
| 性能 | < 5ms 每页 | 避免重绘时卡顿 |

### 3.2 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 水印未启用 | `watermarkEnabled=false` | 不渲染水印 |
| 无邮箱 | 匿名访问 | 使用 visitor_id 或 IP 哈希替代 |
| 高 DPI | Retina 屏幕 | 水印按 devicePixelRatio 缩放 |
| 下载 | 用户点击下载 | 可选：下载文件也带水印（本期可不做） |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/web/src/components/viewer/WatermarkOverlay.tsx` | 新增 | 水印 Canvas 覆盖层组件 |
| `apps/web/src/components/viewer/CanvasViewer.tsx` | 修改 | 渲染 WatermarkOverlay |
| `apps/web/src/components/viewer/PublicViewerPage.tsx` | 修改 | 传递 watermarkInfo |
| `apps/api/internal/link/handler.go` | 修改 | Access 响应增加 `watermarkText`（邮箱/时间/IP 哈希） |
| `apps/web/src/lib/apiAdapters.ts` | 修改 | 映射 watermark 字段 |
| `apps/web/src/types/index.ts` | 修改 | 补充 WatermarkInfo 类型 |

### 4.2 行为定义

- 后端 `Access` 响应中返回 `watermarkText`。
- 前端 Canvas 绘制页面后，在其上绘制倾斜平铺的水印文字。
- 水印文字可包含多行：`email`、`accessed at 2026-07-05 10:00`、`hash: abc123`。

---

## 5. 验收标准

- [ ] `watermarkEnabled=true` 时页面显示半透明倾斜水印。
- [ ] 水印内容包含邮箱、访问时间、IP 哈希。
- [ ] `watermarkEnabled=false` 时不显示水印。
- [ ] 翻页/缩放后水印正常重绘。
- [ ] 前端 `pnpm test` / `pnpm lint` / `pnpm typecheck` 全绿。

---

## 6. 实现步骤建议

1. 后端 Access 响应增加 `watermarkText` 字段。
2. 创建 `WatermarkOverlay` 组件，接收 page dimensions 和 watermark text。
3. 在 `CanvasViewer` 页面绘制完成后渲染水印覆盖层。
4. 使用 `CanvasRenderingContext2D` 绘制倾斜文字。
5. 处理高 DPI 缩放。
6. 补测试/截图对比。

---

## 7. 测试验证

```bash
cd apps/web
pnpm test CanvasViewer
pnpm test WatermarkOverlay
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 水印内容必须由后端生成，前端不可自行构造。
- 水印不得写入 page 图片缓存，必须每次渲染时叠加。
- 透明度必须足够低，不影响阅读。
- 不得在 worker 线程外做大量 Canvas 像素操作。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-008`

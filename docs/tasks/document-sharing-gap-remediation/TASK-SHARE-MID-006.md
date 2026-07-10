---
task_id: TASK-SHARE-MID-006
parent_issue: DS-SHARE-018
agent_task_id: AGENT-TASK-SHARE-018
version: v1.0.0
priority: P1
status: 已完成
type: fullstack
effort: M
branch: feat/share-mid-006-server-side-watermark
estimated_files: '10'
max_lines: '500'
project_stack: Go 1.25 + Gin + PostgreSQL + React 19 + Canvas API
dependencies:
- INFRA-001
- TASK-SHARE-SHORT-005
ai_red_flags:
- 水印内容必须由后端生成，前端不可自行构造
- IP 哈希必须不可逆，避免泄露访客真实 IP
- 必须防止打印、右键保存、DevTools 简单删除水印
- 水印透明度必须足够低，不影响阅读
- 性能开销 < 5ms 每页
ai_confidence: medium
pending_confirmation:
- IP 哈希算法：SHA-256 前 8 位还是 HMAC？
- 是否对下载 PDF 也做水印（服务端渲染）？
available_tools:
- test
- lint
- browse
---

# TASK-SHARE-MID-006 服务端水印与防绕过

> **父 Issue**：`DS-SHARE-018`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **状态**：`待执行`  
> **类型**：`fullstack`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-mid-006-server-side-watermark`

---

## 1. 目标

增强公共 Viewer 水印的不可伪造性与信息完整性：
- 后端 `Access` 响应生成 `watermarkText`（邮箱 + 访问时间 + IP 哈希）。
- 前端 `WatermarkOverlay` 只负责渲染后端返回的文本。
- 增加前端防绕过措施：禁用右键保存、Print Screen 时模糊/警告、监听 DOM 删除尝试。
- （可选）对页面图片做服务端动态水印（与 MID-005 签名 URL 结合）。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 功能设计 | `/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §8.7.6 |
| 对齐报告 | ../../reviews/DESIGN-ALIGNMENT-huntress-spectre-falcon.md |
| 最终评审 | ../../reviews/FINAL-REVIEW.md §3.2 |
| 已有代码 | `apps/api/internal/link/handler.go`、`apps/web/src/components/viewer/WatermarkOverlay.tsx`、`apps/web/src/components/viewer/ViewerCanvas.tsx` |

---

## 3. 输入

### 3.1 水印内容

| 字段 | 来源 | 说明 |
|---|---|---|
| email | `effectiveEmail` 或 visitor_id | 已验证邮箱优先 |
| accessedAt | 后端当前 UTC 时间 | ISO 8601 格式 |
| ipHash | `SHA-256(ip)` 前 8 位 | 不可逆，用于追溯 |

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 透明度 | 0.08 ~ 0.15 | 不遮挡正文 |
| 布局 | 倾斜 30° 平铺 | 覆盖整个 Canvas |
| 性能 | < 5ms 每页 | 避免重绘卡顿 |
| 匿名访问 | 无邮箱时用 visitor_id 前 8 位 | — |

### 3.3 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 水印未启用 | `watermarkEnabled=false` | 不返回 `watermarkText`，不渲染 |
| 高 DPI | Retina 屏幕 | 按 devicePixelRatio 缩放 |
| 打印/截图 | 用户按 Print Screen | 显示警告并模糊文档内容 |
| 删除水印 DOM | DevTools 删除 `WatermarkOverlay` | 文档内容保持可见，但操作被记录（可选） |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/link/handler.go` | 修改 | `respondAccessSuccess` 增加 `watermarkText` |
| `apps/api/internal/link/service.go` | 修改 | 生成水印文本辅助函数 |
| `apps/web/src/lib/apiAdapters.ts` | 修改 | 映射 `watermarkText` |
| `apps/web/src/components/viewer/WatermarkOverlay.tsx` | 修改 | 只渲染后端文本，不再本地构造 |
| `apps/web/src/components/viewer/ViewerCanvas.tsx` | 修改 | 监听 Print Screen、右键、DOM 删除 |
| `apps/web/src/types/index.ts` | 修改 | 补充 `watermarkText` 字段 |

### 4.2 行为定义

- 后端在 `Access` 成功响应中返回 `watermarkText`。
- 前端 `WatermarkOverlay` 直接使用该文本，不自行拼接。
- 打印/截图时显示覆盖层警告。
- 右键菜单在文档区域被禁用。

---

## 5. 验收标准

- [ ] 后端返回 `watermarkText` 包含邮箱/时间/IP 哈希。
- [ ] 前端渲染后端返回的水印文本。
- [ ] `watermarkEnabled=false` 时不渲染水印。
- [ ] Print Screen 触发警告/模糊。
- [ ] 右键保存被禁用。
- [ ] `pnpm test WatermarkOverlay ViewerCanvas`、`go test ./internal/link/...` 全绿。

---

## 6. 实现步骤建议

1. 后端新增 `buildWatermarkText(email, ip, accessedAt)`。
2. 修改 `respondAccessSuccess` 返回 `watermarkText`。
3. 前端 `WatermarkOverlay` 改为纯渲染组件。
4. 在 `ViewerCanvas` 增加键盘/鼠标/打印事件监听。
5. 补测试。

---

## 7. 测试验证

```bash
# 后端
cd apps/api
go test ./internal/link/...
make lint

# 前端
cd apps/web
pnpm test WatermarkOverlay ViewerCanvas
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 水印内容必须由后端生成。
- 不得明文传输或存储 IP。
- 防绕过措施不得破坏无障碍访问（保留 screen reader 摘要）。
- 动画与模糊必须遵循 `prefers-reduced-motion`。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-018`

---
task_id: "TASK-SHARE-MID-002"
parent_issue: "DS-SHARE-006"
agent_task_id: "AGENT-TASK-SHARE-006"
version: "v1.0.0"
priority: "P1"
status: "已完成"
type: "fullstack"
effort: "L"
branch: "feat/share-mid-002-extended-event-tracking"
estimated_files: "16"
max_lines: "800"
project_stack: "Go 1.25 + Gin + PostgreSQL + React 19 + TypeScript"
ai_red_flags:
  - "新增事件类型必须追加 enum，不能破坏已有事件查询"
  - "前端事件上报必须防抖动，避免频繁请求"
  - "AI 交互事件不得把用户问题原文写入公开可访问的表"
  - "所有事件必须带 visitor_id / link_id 便于归因"
ai_confidence: "medium"
pending_confirmation:
  - "scroll_depth 事件是实时上报还是页面切换时批量上报？"
  - "forward_signal 如何定义：新 visitor_id 首次出现即算转发？"
available_tools:
  - "test"
  - "lint"
  - "browse"
---

# TASK-SHARE-MID-002 扩展追踪事件体系

> **父 Issue**：`DS-SHARE-006`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **类型**：`fullstack`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-mid-002-extended-event-tracking`

---

## 1. 目标

将当前 3 种事件扩展为更完整的事件体系，补齐 PRD 中定义的 `forward_signal`、`return_visit`、`scroll_depth_recorded`、`ai_question_asked`、`ai_answer_viewed`、`ai_evidence_clicked` 等事件。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 缺口分析报告 | `docs/reviews/document-sharing-design-vs-implementation-gap-report.md` §4.1 |
| PRD | `docs/backup/PRD-v2.1.0.md` EVT-01 ~ EVT-18 |
| API 契约 | `docs/backup/API-SPEC-v2.1.0.md` API-05 |

### 2.1 已有代码

- `apps/api/internal/analytics/service.go` — 事件记录
- `apps/api/internal/link/handler.go` — 公共事件接收
- `apps/web/src/components/viewer/useViewerDocument.ts` — 页面事件
- `apps/web/src/components/viewer/CanvasViewer.tsx` — 下载事件

---

## 3. 输入

### 3.1 新增事件类型

| 事件 | 来源 | 数据 |
|---|---|---|
| `forward_signal` | 后端 | 新 `visitor_id` 首次打开同一 link |
| `return_visit` | 后端 | 老 visitor 30min 后再次打开 |
| `scroll_depth_recorded` | 前端 | page_number, depth（0-1） |
| `ai_question_asked` | 前端/后端 | question_length, topic（可选） |
| `ai_answer_viewed` | 前端 | — |
| `ai_evidence_clicked` | 前端 | document_id, page_number, box |

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 上报频率 | scroll 事件防抖 500ms | 避免海量请求 |
| 批量上报 | 可选 | 页面 unload 时批量发送未发送事件 |
| 去重 | 依赖 TASK-SHARE-SHORT-004 | 本任务只负责产生事件 |
| 隐私 | AI 问题可存储 | 但需符合隐私政策 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/0XX_event_types.up.sql` | 新增 | 扩展 event_type enum |
| `apps/api/internal/analytics/service.go` | 修改 | 新增 `RecordScrollDepth`, `RecordAIEvent`, `RecordForwardSignal`, `RecordReturnVisit` |
| `apps/api/internal/analytics/handler.go` | 修改 | 支持新 event_type 接收 |
| `apps/api/internal/link/handler.go` | 修改 | `Access` 中检测 forward/return |
| `apps/web/src/lib/analytics.ts` | 新增 | 统一事件上报 SDK |
| `apps/web/src/components/viewer/useViewerDocument.ts` | 修改 | 滚动深度监听 |
| `apps/web/src/components/viewer/CanvasViewer.tsx` | 修改 | AI evidence 点击上报 |
| `apps/web/src/components/ai/AIAssistant.tsx` / `SidebarAIChat.tsx` | 修改 | AI 问题/回答事件上报 |
| `apps/api/internal/db/queries.sql` | 新增 | 新事件写入查询 |

---

## 5. 验收标准

- [ ] `forward_signal` 在新 visitor 首次打开时生成。
- [ ] `return_visit` 在老 visitor 30min 后再次打开时生成。
- [ ] `scroll_depth_recorded` 在前端滚动时防抖上报。
- [ ] `ai_question_asked` / `ai_answer_viewed` / `ai_evidence_clicked` 在 AI 交互时上报。
- [ ] 新事件进入 `access_logs` 或专用事件表。
- [ ] 前端事件 SDK 覆盖公开/认证 viewer。
- [ ] 单元/组件测试覆盖主要事件触发。

---

## 6. 实现步骤建议

1. 扩展 `access_logs.event_type` enum 或创建 `link_events` 通用事件表。
2. 后端新增各类 `RecordXxx` 方法。
3. 前端创建 `analytics.ts` SDK，统一 `recordPublicEvent` / `recordViewerEvent`。
4. 在 `useViewerDocument` 中增加 scroll 监听与防抖。
5. 在 AI 组件中埋点。
6. 在 `Access` 中根据 visitor_id 历史检测 forward/return。
7. 补测试。

---

## 7. 测试验证

```bash
# 后端
cd apps/api
go test ./internal/analytics/...
go test ./internal/link/...
make lint

# 前端
cd apps/web
pnpm test useViewerDocument
pnpm test CanvasViewer
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 新增事件 enum 值必须向后兼容；旧代码忽略未知事件类型。
- scroll 事件必须防抖，不能直接每次 scroll 都发请求。
- AI 事件上报必须在请求成功后触发，避免误报。
- 不得把用户问题明文写入前端日志。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-006`

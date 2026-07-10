---
task_id: TASK-SHARE-MID-008
parent_issue: DS-SHARE-022
agent_task_id: AGENT-TASK-SHARE-022
version: v1.0.0
priority: P1
status: 已完成
type: fullstack
effort: L
branch: feat/share-mid-008-index-file-generation
estimated_files: '20'
max_lines: '1000'
project_stack: Go 1.25 + Gin + PostgreSQL + React 19 + TypeScript + LLM
dependencies:
- INFRA-001
- TASK-SHARE-SHORT-001
- TASK-SHARE-SHORT-005
- TASK-SHARE-SHORT-006
ai_red_flags:
- 生成内容必须基于当前 link 可见文档，不能泄露其他工作区资料
- LLM 输出可能包含幻觉，必须标注 'AI generated' 并建议核对原始文件
- 大文档聚合可能触发 LLM token/超时限制，必须分页/分块 + 缓存
- 生成失败时不能阻塞访客访问主文档
- 索引文件内容必须做 XSS 过滤后才能渲染
ai_confidence: medium
pending_confirmation:
- Index File 是只读 HTML，还是可下载 PDF？
- 生成触发时机：开关开启时立即生成 / 首次访问时生成 / 文档变更时重新生成？
- 是否对 deal room 多文档做统一目录 + 每文档摘要？
available_tools:
- test
- lint
- browse
---

# TASK-SHARE-MID-008 索引文件自动生成（Index File Generation）

> **父 Issue**：`DS-SHARE-022`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **类型**：`fullstack`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-mid-008-index-file-generation`

---

## 1. 目标

让 Access Tab 中的 `indexFileEnabled` 开关产生真实价值：

- 当 owner 开启 "Index File" 后，系统自动为当前 link（单文档或 Deal Room 多文档）生成一份**执行摘要 + 目录 + 关键信息索引**。
- 访客在公共 Viewer 侧边栏看到 "Index" tab，可快速掌握资料全貌，再决定是否深入阅读。
- 生成结果缓存到 `link_index_files` 表，避免每次访问重复调用 LLM。
- 文档集发生变化时，支持手动或自动重新生成。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 功能设计 | `/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §8.7.6 |
| 相关任务 | TASK-SHARE-SHORT-001、TASK-SHARE-SHORT-005、TASK-SHARE-SHORT-009 |
| 依赖能力 | 文档 chunk 搜索、LLM completion、deal room 多文档关联 |

### 2.1 当前问题

- `indexFileEnabled` 是纯前端占位字段，开启后没有任何效果。
- 数据室场景下，投资人常先看 1 页摘要再决定要不要翻 100 页 BP；缺少自动摘要能力降低转化率。

---

## 3. 输入

### 3.1 数据模型

```sql
ALTER TABLE links ADD COLUMN IF NOT EXISTS index_file_enabled boolean NOT NULL DEFAULT false;

CREATE TABLE link_index_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL UNIQUE REFERENCES links(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','generating','ready','failed')),
    content_html TEXT,
    error_message TEXT,
    generated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
```

### 3.2 API 契约

**Owner 端点**

```http
POST /api/v1/links/:id/index-file/generate
```

```http
GET /api/v1/links/:id/index-file
```

**公共端点（访客）**

```http
GET /api/v1/public/links/:token/index-file
X-Link-Session: <session-token>
```

### 3.3 生成策略

| 策略 | 说明 |
|---|---|
| 输入 | link 关联文档的高优先级 chunks（限制 token） |
| 提示词 | "基于以下资料生成执行摘要、目录、关键数据表格。不要添加资料中没有的信息。" |
| 输出 | Markdown/JSON，后端 sanitize 后转 HTML |
| 缓存 | 写入 `link_index_files`，状态 `ready` |
| 失败 | 状态 `failed`，返回 error_message，不影响主 viewer |
| 重新生成 | owner 点击 regenerate；文档集变化时也可触发 |

### 3.4 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 功能开关 | `links.index_file_enabled = true` | 关闭时返回 `403 index_file_disabled` |
| 文档范围 | 仅限 link 可见 documents | deal room 取所有关联文档 |
| token 限制 | 单次生成不超过 100k 字符输入 | 超长文档做分块摘要再聚合 |
| 并发 | 同 link 同时只能有一个生成任务 | 通过 DB status `generating` 乐观锁 |
| 缓存 | 生成后 24h 内不自动重跑 | 避免高频 LLM 调用 |

### 3.5 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 开关关闭 | `index_file_enabled=false` | `403 index_file_disabled` |
| 生成中 | status=`generating` | 返回 `202 generation_in_progress` |
| 生成失败 | status=`failed` | 返回 `500` + error_message，viewer 显示重试按钮 |
| LLM 未配置 | `OPENAI_API_KEY` 为空 | status=`failed`，提示管理员配置 AI |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/db/migrations/051_link_index_files.up.sql` | 新增 | `links.index_file_enabled` + `link_index_files`（由 INFRA-001 统一编排） |
| `apps/api/internal/db/migrations/051_link_index_files.down.sql` | 新增 | 回滚 |
| `apps/api/internal/db/queries.sql` | 修改 | CreateLink/UpdateLink 增加 `index_file_enabled` |
| `apps/api/internal/db/queries.sql.go` | 重新生成 | sqlc |
| `apps/api/internal/db/models.go` | 重新生成 | sqlc |
| `apps/api/internal/link/service.go` | 修改 | 新增 `GenerateIndexFile`、`GetIndexFile` |
| `apps/api/internal/link/handler.go` | 修改 | 注册 owner + 公共 API |
| `apps/api/internal/llm/prompts/index_file.go` | 新增 | 索引文件生成 prompt |
| `apps/api/internal/assistant/service.go`（或 search） | 修改 | 复用 chunk 聚合 |
| `apps/web/src/lib/api.ts` | 新增 | `generateIndexFile`、`getIndexFile` |
| `apps/web/src/types/index.ts` | 修改 | 补充 `LinkIndexFile` |
| `apps/web/src/components/links/share/AccessTab.tsx` | 修改 | `indexFileEnabled` 开关保持可见 |
| `apps/web/src/components/viewer/RightSidebar.tsx` | 修改 | 增加 "Index" tab（仅开启时） |
| `apps/web/src/components/viewer/IndexFilePanel.tsx` | 新增 | 渲染 HTML 摘要 + 重试/刷新按钮 |
| `apps/web/src/components/links/share/AnalyticsTab.tsx` | 修改 | Owner 手动生成/重新生成入口 |
| `apps/web/src/i18n/locales/en/linkShare.json` | 修改 | 文案 |
| `apps/web/src/i18n/locales/zh-CN/linkShare.json` | 修改 | 文案 |

### 4.2 行为定义

```text
Access Tab / Advanced
└── Index File [开关]
    开启后：
    - Owner 可一键 "Generate index"；生成后可在 Analytics Tab 预览。
    - 公共 Viewer 侧边栏出现 "Index" tab，访客看到 AI 生成的摘要/目录。
    关闭后：
    - 已生成的缓存保留，但不再对访客展示；再次开启时直接复用或提示重新生成。
```

---

## 5. 验收标准

- [ ] 后端新增 `link_index_files` 表与 `links.index_file_enabled` 列。
- [ ] Owner 可手动触发生成，公共访客在开关开启时可见 Index tab。
- [ ] 生成内容限制在当前 link 文档范围内，不能跨 link。
- [ ] 生成失败不影响主 viewer，且向 owner 展示失败原因。
- [ ] 输出 HTML 经过 sanitize，无 XSS 风险。
- [ ] 前端 `pnpm lint` / `pnpm typecheck` / `pnpm test` 全绿。
- [ ] 后端 `go test ./internal/link/...` 全绿。

---

## 6. 实现步骤建议

1. 基于 INFRA-001 已创建的 051 migration 修改代码，**本任务不再新增 migration**。
2. 实现 `GenerateIndexFile` service：聚合 chunks → 调用 LLM → sanitize → 写缓存。
3. 注册 owner / 公共 GET 与 owner POST regenerate。
4. 前端 `IndexFilePanel.tsx` 渲染，支持 loading / error / ready 状态。
5. `RightSidebar.tsx` 条件渲染 Index tab。
6. Analytics Tab 增加 owner 生成入口与状态显示。
7. 补单元 + 集成测试（LLM 用 mock）。

---

## 7. 测试验证

```bash
# 后端
cd apps/api
go test ./internal/link/...
./e2e-test.sh

# 前端
cd apps/web
pnpm test IndexFilePanel RightSidebar AnalyticsTab
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- **必须**对 LLM 输出做 HTML sanitize（推荐 `bluemonday` 或等价方案）。
- **必须**标注 "AI generated"，避免访客把摘要当作原始文件。
- **禁止**在生成任务中泄露非当前 link 的文档内容。
- 生成任务必须是异步或带超时保护，不能阻塞 HTTP 请求超过 30s。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 单元测试 + e2e P0 通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-022`

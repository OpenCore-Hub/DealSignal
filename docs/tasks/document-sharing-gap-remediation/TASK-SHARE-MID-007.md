---
task_id: TASK-SHARE-MID-007
parent_issue: DS-SHARE-019
agent_task_id: AGENT-TASK-SHARE-019
version: v1.0.0
priority: P1
status: 已完成
type: fullstack
effort: M
branch: feat/share-mid-007-link-analytics-lifecycle
estimated_files: '14'
max_lines: '700'
project_stack: Go 1.25 + Gin + PostgreSQL + React 19 + TypeScript
dependencies:
- INFRA-001
- TASK-SHARE-SHORT-005
- TASK-SHARE-SHORT-004
ai_red_flags:
- Link 级 Analytics 必须租户隔离，不能泄露其他 workspace 数据
- 过期提醒邮件必须尊重 email_enabled 开关
- 旧 /r/:slug 重定向必须保留访问归因
- 归档/删除 link 必须使已签发 session 立即失效
- 不得修改或删除历史 access_logs
ai_confidence: medium
pending_confirmation:
- Analytics Tab 是否展示 AI 问答记录？
- 过期提醒提前 24h/7d 还是仅 24h？
available_tools:
- test
- lint
- browse
---

# TASK-SHARE-MID-007 Link 级 Analytics 与生命周期管理

> **父 Issue**：`DS-SHARE-019`  
> **版本**：`v1.0.0`  
> **优先级**：`P1`  
> **状态**：`待执行`  
> **类型**：`fullstack`  
> **预计工作量**：`M`  
> **分支名**：`feat/share-mid-007-link-analytics-lifecycle`

---

## 1. 目标

为分享链接补齐分析与生命周期管理能力：
- Link 级 Analytics Tab：最近访问者、停留时长、下载次数、关键页、AI 问答记录。
- 链接过期前提醒：cron 任务在 `expires_at` 前 24h/7d 发送提醒邮件。
- 旧 `/r/:slug` 入口治理：重定向到默认 share link `/l/:token`，保留访问归因。
- 链接归档/续期：支持 soft-archive 与一键续期。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 功能设计 | `/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §8.7.1 / §8.7.7 |
| 对齐报告 | ../../reviews/DESIGN-ALIGNMENT-huntress-spectre-falcon.md |
| 最终评审 | ../../reviews/FINAL-REVIEW.md §2.2 / §3.2 |
| 已有代码 | `apps/web/src/components/deal-rooms/DealRoomShareDialog.tsx`、`apps/api/internal/analytics/service.go` |

---

## 3. 输入

### 3.1 Analytics Tab 数据需求

| 指标 | 来源 | 说明 |
|---|---|---|
| 最近访问者 | `access_logs` | email / visitor_id / 时间 / IP 哈希 |
| 停留时长 | `page_views` | 汇总或平均值 |
| 下载次数 | `access_logs` `download_attempted` | — |
| 关键页 | `heat.IsKeyPage` + `page_views` | 按关键词匹配 |
| AI 问答 | `assistant_messages` | 需按 link_id 过滤（可选） |

### 3.2 生命周期事件

| 事件 | 触发 | 行为 |
|---|---|---|
| 即将过期 | cron 扫描 `expires_at` | 发送提醒邮件给创建者 |
| 已过期 | 访客访问 | 返回 `410 link_expired`，允许创建者续期 |
| 归档 | 创建者手动归档 | `status='archived'`，session 失效 |
| 续期 | 创建者点击续期 | 更新 `expires_at`，重新激活 |

### 3.3 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 无权限查看 Analytics | 非 link 创建者/管理员 | `403 forbidden` |
| 已过期 link 访问 | 访客打开 | `410 link_expired` + 提示创建者续期 |
| 归档 link 访问 | 访客打开 | `410 link_expired` / `403 link_disabled` |
| 续期到过去时间 | 创建者选择过去日期 | 校验失败 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/api/internal/analytics/service.go` | 修改 | 新增 `GetLinkAnalytics` |
| `apps/api/internal/link/service.go` | 修改 | 续期、归档、过期检查 |
| `apps/api/internal/link/handler.go` | 修改 | Analytics / renew / archive 端点 |
| `apps/api/internal/db/queries.sql` | 新增 | Analytics 聚合查询 |
| `apps/api/internal/cron/` 或 worker | 新增/修改 | 过期提醒任务 |
| `apps/web/src/components/links/share/AnalyticsTab.tsx` | 新增 | Analytics Tab |
| `apps/web/src/components/deal-rooms/DealRoomShareDialog.tsx` | 修改 | 增加 Analytics Tab |
| `apps/web/src/i18n/locales/*/dealRooms.json` | 修改 | Analytics 文案 |

### 4.2 行为定义

- `DealRoomShareDialog` / `LinkShareDialog` 增加第 4 个 Tab “Analytics”。
- cron 每天扫描即将过期链接，发送提醒邮件。
- `/r/:slug` 301/302 重定向到默认 `/l/:token`；访问记录归入该 share link。
- 归档后公共访问立即拒绝，已签发 session 失效。

---

## 5. 验收标准

- [ ] Analytics Tab 展示最近访问者、停留时长、下载次数。
- [ ] 过期前 24h/7d 发送提醒邮件。
- [ ] `/r/:slug` 重定向到 `/l/:token` 并保留访问归因。
- [ ] 归档/续期 API 可用，归档后访问被拒绝。
- [ ] `go test ./internal/link/...`、`go test ./internal/analytics/...` 全绿。
- [ ] `pnpm lint`、`pnpm typecheck`、`pnpm test` 全绿。

---

## 6. 实现步骤建议

1. 后端新增 `GetLinkAnalytics` 聚合查询。
2. 新增 `/links/:id/analytics` 端点。
3. 新增 renew/archive 端点。
4. 新增 cron 任务扫描过期链接。
5. 修改 `/r/:slug` handler 为重定向。
6. 前端新增 Analytics Tab。
7. 补测试。

---

## 7. 测试验证

```bash
# 后端
cd apps/api
go test ./internal/link/...
go test ./internal/analytics/...
./e2e-test.sh
make lint

# 前端
cd apps/web
pnpm lint
pnpm typecheck
pnpm test DealRoomShareDialog LinkShareDialog
```

---

## 8. 约束与红线

- Analytics 数据必须租户隔离。
- 不得修改或删除历史 `access_logs`。
- 过期提醒必须可配置开关。
- 重定向不得丢失 UTM 或查询参数。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-SHARE-019`

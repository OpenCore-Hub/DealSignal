---
task_id: TASK-SHARE-SHORT-006
parent_issue: DS-SHARE-016
agent_task_id: AGENT-TASK-SHARE-016
version: v1.0.0
priority: P0
status: 已完成
type: frontend
effort: L
branch: feat/share-short-006-share-dialog-polish
estimated_files: '18'
max_lines: '900'
project_stack: React 19 + TypeScript + Vite 8 + Tailwind CSS 4 + shadcn/ui
dependencies:
- INFRA-001
- TASK-SHARE-SHORT-005
ai_red_flags:
- 所有 UI 文案必须通过 i18n t()，禁止硬编码英文/中文
- Share Tab 只保留分享属性，访问控制开关统一在 Access Tab
- Preset 自动填充后必须给用户明确反馈
- Revoke 操作必须有二次确认，避免误触
- Dialog 打开/关闭、Tab 切换、Chip 进入/删除必须遵循动画规范
ai_confidence: medium
pending_confirmation:
- Preset Custom 状态是否需要在偏离预设时自动切换，还是手动标记？
- Analytics Tab 是否在 MVP 内实现，还是放到 MID-007？
available_tools:
- test
- lint
- browse
---

# TASK-SHARE-SHORT-006 前端 Share / Invite / Access 三 Tab 弹窗

> **父 Issue**：`DS-SHARE-016`  
> **版本**：`v1.0.0`  
> **优先级**：`P0`  
> **状态**：`部分完成`  
> **类型**：`frontend`  
> **预计工作量**：`L`  
> **分支名**：`feat/share-short-006-share-dialog-frontend`

---

## 1. 目标

实现 Deal Room / 文档链接侧的统一分享弹窗：
- `DealRoomShareDialog`：Deal Room 详情页入口，支持新建/编辑 Link。
- `LinkShareDialog`：文档链接侧入口，复用 Share / Invite / Access Tab。
- Share Tab：名称、域名、预设、过期时间、标签、访问规则摘要。
- Invite Tab：邮箱 TagInput、发送邀请、邀请状态列表、Resend / Revoke。
- Access Tab：认证开关、允许/阻止邮箱与域名、水印/NDA/下载/AI 等高级选项。

三 Tab 弹窗已收尾：preset 覆盖二次确认、字段 200ms 高亮反馈、保存成功态、未保存离开提示、完整 i18n、Resend tooltip 与成功 toast。

---

## 2. 上下文

| 文档 | 链接/章节 |
|---|---|
| 功能设计 | `/Users/mg/.kimi/plans/huntress-spectre-falcon.md` §8 |
| 对齐报告 | ../../reviews/DESIGN-ALIGNMENT-huntress-spectre-falcon.md |
| 最终评审 | ../../reviews/FINAL-REVIEW.md §2.2 / §3.2 |
| 已有代码 | `apps/web/src/components/deal-rooms/DealRoomShareDialog.tsx`、`apps/web/src/components/links/LinkShareDialog.tsx`、`apps/web/src/components/links/share/*` |

---

## 3. 输入

### 3.1 组件结构

```text
components/deal-rooms/
├─ DealRoomShareButton.tsx
├─ DealRoomShareDialog.tsx
└─ ...

components/links/
├─ LinkShareDialog.tsx
└─ share/
   ├─ ShareTab.tsx
   ├─ InviteTab.tsx
   ├─ AccessTab.tsx
   ├─ AccessSummaryCard.tsx
   ├─ EmailTagInput.tsx
   ├─ SecuritySwitch.tsx
   └─ CollapsibleSection.tsx
```

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|---|---|---|
| 新建模式 | 无 link id，Header 显示输入框，Footer 文案 "Create link" | — |
| 编辑模式 | 有 link id，Header 显示 link name + short_url，Footer 文案随 Tab 变化 | — |
| Preset 切换 | 选择 preset 后自动填充 Access Tab 开关；用户手动修改后 preset 变为 custom | 需二次确认覆盖 |
| allow/block 冲突 | 同一邮箱/域名不能同时存在 allow 与 block | 保存时校验 |
| 移动端 | 弹窗接近全屏，Advanced 默认折叠 | — |

### 3.3 失败用例

| 场景 | 输入 | 预期行为 |
|---|---|---|
| 无 link name | Share Tab name 为空 | inline error + 禁用保存 |
| allow 规则但 require_email 关闭 | Access Tab 配置 | 自动开启 require_email 并提示 |
| 密码长度不足 | require_password=true + 密码 < 8 | inline error |
| 撤销邀请误触 | 点击 Revoke | 二次确认弹窗 |
| 未保存离开 | 切换 Tab 或关闭 Dialog | 提示 unsaved changes |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|---|---|---|
| `apps/web/src/components/links/share/ShareTab.tsx` | 修改 | 完善 Preset Custom 状态、Access Summary 卡片 |
| `apps/web/src/components/links/share/InviteTab.tsx` | 修改 | Resend tooltip、Revoke 二次确认、空状态 |
| `apps/web/src/components/links/share/AccessTab.tsx` | 修改 | 字段分层、校验、动画反馈 |
| `apps/web/src/components/links/share/AccessSummaryCard.tsx` | 修改 | 跳转到 Access Tab |
| `apps/web/src/components/deal-rooms/DealRoomShareDialog.tsx` | 修改 | 新建/编辑模式、Footer 文案 |
| `apps/web/src/components/links/LinkShareDialog.tsx` | 修改 | 复用三 Tab |
| `apps/web/src/i18n/locales/en/dealRooms.json` | 修改 | 补全 share/invite/accessRules 键 |
| `apps/web/src/i18n/locales/zh-CN/dealRooms.json` | 修改 | 同步中文键 |

### 4.2 行为定义

- Share Tab 只展示“分享出去”相关属性；访问控制真实来源是 Access Tab。
- Preset 切换后，受影响字段高亮 200ms 后淡出。
- 用户手动修改任一受 preset 控制字段后，preset 下拉自动变为 `custom`。
- 保存成功：按钮文案短暂变为 "Saved"，随后关闭弹窗或显示成功 Alert。
- 所有动画遵循 `prefers-reduced-motion`。

---

## 5. 验收标准

- [x] Share / Invite / Access 三 Tab 职责分离，无重复字段。
- [x] Preset Custom 状态与二次确认逻辑完整。
- [x] allow/block 冲突 inline 校验。
- [x] Revoke 操作有二次确认。
- [x] 所有新增文案同步 `en` / `zh-CN`。
- [x] `pnpm lint`、`pnpm typecheck`、`pnpm test` 全绿。

---

## 6. 实现步骤建议

1. 完善 `ShareTab`：Preset 状态机、Access Summary 卡片、旧 slug 提示。
2. 完善 `InviteTab`：Resend tooltip、Revoke 确认、空状态、表格行撤销动画。
3. 完善 `AccessTab`：字段分层、校验、chip 动画、折叠面板 Badge。
4. 统一 Dialog Footer 保存按钮文案与 loading/success 状态。
5. 补充 i18n 键。
6. 补组件测试与截图对比（可选）。

---

## 7. 测试验证

```bash
cd apps/web
pnpm test DealRoomShareDialog LinkShareDialog ShareTab InviteTab AccessTab
pnpm lint
pnpm typecheck
```

---

## 8. 约束与红线

- 禁止硬编码文案。
- 禁止在 Share Tab 直接修改 Access Tab 字段（只能通过跳转或 Preset）。
- 动画时长 150-250ms，必须支持 `prefers-reduced-motion: reduce`。
- 移动端必须可正常操作。

---

## 9. Definition of Done

- [x] 代码实现完成
- [x] 测试通过
- [x] lint / typecheck 通过
- [x] PR 已关联父 Issue：`Closes #DS-SHARE-016`（PR #88）

---
task_id: "TASK-FRONTEND-008"
parent_issue: "DS-030"
agent_task_id: "AGENT-TASK-030"
version: "v2.1.3"
priority: "P1"
status: "待执行"
type: "frontend"
effort: "S"
branch: "feat/agent-task-030-i18n-cleanup"
estimated_files: "8"
max_lines: "200"
project_stack: "React 19 + TypeScript + Vite 8 + i18next"
ai_red_flags:
  - "不得破坏现有 i18n key 结构"
  - "不得新增中英混杂文案"
  - "行业缩写（AI/NDA/CRM）可保留"
  - "敏感数据不得发送给 LLM"
ai_confidence: "high"
pending_confirmation:
  - "是否允许在代码中保留 'DealSignal' 品牌名"
available_tools:
  - "test"
  - "lint"
---

# TASK-FRONTEND-008 前端文案与中英混杂清理

> **父 Issue**：`DS-030`

---

## 1. 目标

清理前端界面残留英文与营销化文案，确保 v2.1.3 中文 SaaS 语境一致：
- `views` → `次访问`
- `Deal Rooms` → `数据室`
- `360° 视图` → `概览`
- `数据室模板引擎` → `新建数据室`

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| 前端审计计划 | `docs/FRONTEND-AUDIT-AND-REFINEMENT-PLAN-v2.1.3.md` §2.4 / §3 Phase A.6 |
| PRD | `docs/PRD-v2.1.0.md` §11.2 |
| 父 Issue | `DS-030` |

### 2.1 已有代码

- `apps/web/src/i18n/locales/en/**/*.json`
- `apps/web/src/i18n/locales/zh-CN/**/*.json`
- `apps/web/src/components/documents/DocumentContent.tsx`
- `apps/web/src/routes/deal-rooms/*.tsx`
- `apps/web/src/routes/contacts/detail.tsx`

---

## 3. 输入

### 3.1 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 行业缩写 | 可保留 | AI / NDA / CRM / LP 等 |
| 品牌名 | 保留 | DealSignal |
| 数字/单位 | 本地化 | `次访问`、`次浏览` |

---

## 4. 输出

### 4.1 需要修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `i18n/locales/en/documents.json` | 修改 | 移除/替换残留英文微文案 |
| `i18n/locales/zh-CN/documents.json` | 修改 | 统一中文单位 |
| `i18n/locales/en/settings.json` | 修改 | 清理 settings 文案 |
| `i18n/locales/zh-CN/settings.json` | 修改 | 清理 settings 文案 |
| `components/documents/DocumentContent.tsx` | 修改 | `views` → `次访问` |
| `routes/deal-rooms/*.tsx` | 修改 | `Deal Rooms` → `数据室` |
| `routes/contacts/detail.tsx` | 修改 | `360° 视图` → `概览` |
| `routes/deal-rooms/new.tsx` | 修改 | `数据室模板引擎` → `新建数据室` |

---

## 5. 验收标准

- [ ] 0 处 `views`、`返回 Deal Rooms`、`360° 视图`、`数据室模板引擎` 等残留文案。
- [ ] 中文 namespace 中数据单位统一为「次访问」「次浏览」。
- [ ] `pnpm lint && pnpm typecheck && pnpm test` 全绿。
- [ ] i18n 测试不报错。

---

## 6. 实现步骤建议

1. `grep -R "views\|Deal Rooms\|360°" apps/web/src` 定位残留。
2. 替换硬编码文案为 i18n key；补充 zh-CN/en 翻译。
3. 运行 lint/typecheck/test。

---

## 7. 测试验证

```bash
cd apps/web
pnpm lint
pnpm typecheck
pnpm test
```

---

## 8. 约束与红线

- 不要直接写死中文，优先使用 i18n key。
- 不要删除已有 key 导致其他页面引用失败。

---

## 9. Definition of Done

- [ ] 代码实现完成
- [ ] 测试通过
- [ ] lint / typecheck 通过
- [ ] PR 已关联父 Issue：`Closes #DS-030`

---

## 10. Agent 备注

- 可用 `pnpm exec tsc --noEmit` 快速检查 i18n key 缺失。
- 营销标题建议改为功能描述式文案，避免过度包装。

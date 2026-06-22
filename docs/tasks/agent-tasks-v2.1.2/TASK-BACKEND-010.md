---
task_id: "TASK-BACKEND-010"
parent_issue: "DS-025"
agent_task_id: "AGENT-TASK-013"
version: "v2.1.0"
priority: "P0"
status: "已完成"
type: "security"
effort: "M"
branch: "feat/agent-task-013-security-scan"
estimated_files: "6"
max_lines: "400"
project_stack: "Go 1.22+ / Gin / Docker / PostgreSQL"
ai_red_flags:
  - "漏洞扫描必须作为 CI 门禁"
  - "发现的 HIGH/CRITICAL 漏洞必须修复或记录风险接受"
  - "不得为绕过扫描而禁用规则"
  - "secret 扫描不得误报生产凭据"
ai_confidence: "high"
pending_confirmation: []
available_tools:
  - "test"
  - "lint"
  - "docker"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-BACKEND-010` |
> | `parent_issue` | `DS-025` |
> | `agent_task_id` | `AGENT-TASK-013` |
> | **版本** | `v2.1.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `已完成` |
> | **类型** | `security` |
> | **预计工作量** | `M` |
> | **分支名** | `feat/agent-task-013-security-scan` |
> | **AI 置信度** | `high` |
> | **依赖** | `TASK-BACKEND-006, TASK-BACKEND-009` |
> | **待人工确认事项** | `-` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-BACKEND-010 安全扫描与修复

> **父 Issue**：`DS-025`  
> **版本**：`v2.1.0`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`已完成`  
> **类型**：`security`  
> **预计工作量**：`M`  
> **分支名**：`feat/agent-task-013-security-scan`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

在功能开发完成后，对 Go 后端与前端依赖进行安全扫描，修复 HIGH/CRITICAL 漏洞，并将扫描纳入 CI 门禁。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §17 |
| TDD | `docs/TDD-v2.1.0.md` §7.x |
| 父 Issue | `DS-025` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.1 扫描工具

- `govulncheck ./...`（Go 漏洞库）
- `trivy image` 或 `trivy filesystem`（容器与依赖）
- `gitleaks` 或 `trufflehog`（secret 扫描）
- `npm audit`（前端依赖）

### 3.2 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 扫描时机 | 每次 PR / nightly | CI 配置 |
| 门禁 | 无 HIGH/CRITICAL | 若无法修复需记录风险接受 |
| 最大变更行数 | ≤ 400 | |

---

## 4. 输出

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/Makefile` | 修改 | 增加 `make security` |
| `apps/web/package.json` | 修改 | 增加 `pnpm security` |
| `.github/workflows/security.yml` | 新增 | 安全扫描 CI |
| `docs/SECURITY.md` | 新增 | 漏洞处理流程 |

---

## 5. 验收标准

- [x] `make security` 无 HIGH/CRITICAL Go 漏洞
- [x] `pnpm security` 无高危前端依赖
- [x] `make trivy-fs` 无 HIGH/CRITICAL 依赖漏洞
- [x] CI 安全扫描通过（含 trivy fs/image、gitleaks、govulncheck、pnpm audit）
- [x] 发现的 secret 已轮换（本次扫描无新泄露）

---

## 6. Definition of Done

- [x] 代码/配置实现完成
- [x] 扫描通过
- [x] 风险接受项已记录到 `docs/SECURITY.md`
- [ ] PR 已关联父 Issue：`Closes #DS-025`

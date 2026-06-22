---
task_id: "TASK-BACKEND-007"
parent_issue: "DS-004"
agent_task_id: "AGENT-TASK-010"
version: "v2.1.0"
priority: "P1"
status: "已完成"
type: "infra"
effort: "L"
branch: "feat/agent-task-010-subdomain-ssl"
estimated_files: "8"
max_lines: "600"
project_stack: "Go 1.22+ / Gin / Docker / PostgreSQL / Redis / Let's Encrypt / Traefik"
ai_red_flags:
  - "不得提交 TLS 私钥到仓库"
  - "CNAME 验证必须幂等且可重试"
  - "自定义域名配置必须 tenant 隔离"
  - "SSL 证书续期必须自动化"
ai_confidence: "medium"
pending_confirmation:
  - "生产使用 Traefik / Caddy / cert-manager？"
  - "域名注册商 API 是否需要？"
available_tools:
  - "test"
  - "lint"
  - "docker"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-BACKEND-007` |
> | `parent_issue` | `DS-004` |
> | `agent_task_id` | `AGENT-TASK-010` |
> | **版本** | `v2.1.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P1` |
> | **状态** | `已完成` |
> | **类型** | `infra` |
> | **预计工作量** | `L` |
> | **分支名** | `feat/agent-task-010-subdomain-ssl` |
> | **AI 置信度** | `medium` |
> | **依赖** | `TASK-BACKEND-001` |
> | **待人工确认事项** | `TLS 基础设施选型 / 域名商 API` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-BACKEND-007 子域名/自定义域名与 SSL 自动签发

> **父 Issue**：`DS-004`  
> **版本**：`v2.1.0`  
> **模板版本**：`v2`  
> **优先级**：`P1`  
> **状态**：`已完成`  
> **类型**：`infra`  
> **预计工作量**：`L`  
> **分支名**：`feat/agent-task-010-subdomain-ssl`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

实现租户子域名分配、自定义域名 CNAME 验证、SSL 证书自动签发/续期，以及品牌分享页域名路由，覆盖 API-15。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §4.2 In Scope #9、D-16、D-21 |
| TDD | `docs/TDD-v2.1.0.md` §7.4 |
| API 契约 | `docs/API-SPEC-v2.1.0.md` API-15 |
| DB | `docs/database-model-v2.1.0.md` §4.2.1 `tenant_domains` |
| 父 Issue | `DS-004` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 大；先读 API-15 与 `tenant_domains` 表结构。 |
| **必读顺序** | front matter → `ai_red_flags` → 目标 → 边界条件 → 输出。 |
| **分块策略** | 不需要分块；本任务以设计 + 占位 API 为主。 |

### 3.1 已有代码/表（执行前必须阅读）

- `docs/API-SPEC-v2.1.0.md` API-15
- `docs/database-model-v2.1.0.md` §4.2.1 `tenant_domains`
- `apps/api/internal/server/server.go`（来自 TASK-BACKEND-001）

### 3.2 数据模型/接口

```sql
CREATE TABLE tenant_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    domain TEXT UNIQUE NOT NULL,
    domain_type TEXT NOT NULL CHECK (domain_type IN ('subdomain','custom','public_link')),
    verification_token TEXT,
    verified_at TIMESTAMPTZ,
    certificate_status TEXT DEFAULT 'pending',
    certificate_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 子域名 | 全局唯一、小写、字母数字连字符 | 非法返回 400 |
| CNAME | 指向指定目标（如 `cname.dealsignal.com`） | 未验证不可启用 |
| SSL | 使用 Let's Encrypt HTTP-01 或 DNS-01 | 自动续期 |
| 证书状态 | pending / active / expired / failed | 可查询 |
| 最大变更行数 | ≤ 600 | 超出需拆分 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 域名已被占用 | 重复 subdomain | 409 |
| CNAME 未解析 | 自定义域名 | 422 未验证 |
| 证书签发失败 | Let's Encrypt 限制 | 500 并记录 |
| 越权 | 非 tenant admin | 403 |

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/internal/db/migrations/006_tenant_domains.up.sql` | 新增 | tenant_domains 表 |
| `apps/api/internal/db/queries.sql` | 修改 | 新增域名查询 |
| `apps/api/internal/domain/service.go` | 新增 | 域名验证、证书状态 |
| `apps/api/internal/domain/handler.go` | 新增 | 路由 handler |
| `apps/api/internal/server/server.go` | 修改 | 多租户 Host 路由 |
| `apps/api/docker-compose.yml` | 修改 | 可选 Traefik/Caddy 示例 |

### 4.2 行为定义

- `POST /api/tenant/domains` 注册域名。
- `GET /api/tenant/domains/:id/verify` 检查 CNAME/证书状态。
- 请求到达自定义域名时，Gin 根据 Host 解析 tenant/workspace。

---

## 5. 验收标准

- [x] 子域名可分配并解析到 tenant
- [x] 自定义域名 CNAME 验证通过后才可启用
- [x] SSL 证书自动签发且可续期（RenewalWorker + 占位 Provider）
- [x] `go test ./...` 通过
- [x] `make lint` 通过

---

## 6. 实现步骤建议

1. 编写 migration。
2. 实现 domain service/handler。
3. 在 server 中增加 Host-based tenant 解析。
4. 集成证书管理（本地可用 self-signed 或 Caddy）。
5. 编写测试。
6. 提交 PR。

---

## 7. 测试验证

```bash
cd apps/api && make lint && make test
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`。
- **范围锁定**：聚焦域名/SSL；不做 CDN/WAF 高级配置。
- **禁止把敏感数据发送给 LLM**：私钥、token 不得出现在 prompt/日志/代码中。
- **代码风格**：遵循 Go 标准项目布局。

---

## 9. Definition of Done

- [x] 代码实现完成
- [x] 测试通过
- [x] lint / build 通过
- [ ] PR 已关联父 Issue：`Closes #DS-004`

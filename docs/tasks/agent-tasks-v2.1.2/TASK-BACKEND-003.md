---
task_id: "TASK-BACKEND-003"
parent_issue: "DS-005 / DS-006 / DS-007"
agent_task_id: "AGENT-TASK-006"
version: "v2.1.0"
priority: "P0"
status: "已完成"
type: "backend"
effort: "L"
branch: "feat/agent-task-006-document-ingestion"
estimated_files: "12"
max_lines: "800"
project_stack: "Go 1.22+ / Gin / PostgreSQL / S3 / OnlyOffice / pdfcpu / Redis"
ai_red_flags:
  - "文件校验必须在服务端完成"
  - "签名 URL 不得携带用户/访客身份"
  - "ingestion 任务必须可重试、可观测"
  - "对象存储 key 必须按 tenant/workspace 隔离"
ai_confidence: "medium"
pending_confirmation:
  - "OnlyOffice 自托管地址与凭证"
  - "对象存储使用 S3 还是 MinIO 本地开发？"
available_tools:
  - "test"
  - "lint"
  - "docker"
---

> **模板元信息**
> | 字段 | 值 |
> |------|------|
> | `task_id` | `TASK-BACKEND-003` |
> | `parent_issue` | `DS-005 / DS-006 / DS-007` |
> | `agent_task_id` | `AGENT-TASK-006` |
> | **版本** | `v2.1.0` |
> | **模板版本** | `v2` |
> | **优先级** | `P0` |
> | **状态** | `待执行` |
> | **类型** | `backend` |
> | **预计工作量** | `L` |
> | **分支名** | `feat/agent-task-006-document-ingestion` |
> | **AI 置信度** | `medium` |
> | **依赖** | `TASK-BACKEND-002` |
> | **待人工确认事项** | `OnlyOffice 配置 / 对象存储选型 / PAGE_WEBP 与 image_object_key 事实源` |
> | **可用工具/技能** | `test / lint / docker` |

# TASK-BACKEND-003 文档上传、对象存储与 ingestion pipeline

> **父 Issue**：`DS-005 / DS-006 / DS-007`  
> **版本**：`v2.1.0`  
> **模板版本**：`v2`  
> **优先级**：`P0`  
> **状态**：`待执行`  
> **类型**：`backend`  
> **预计工作量**：`L`  
> **分支名**：`feat/agent-task-006-document-ingestion`  
> **AI 红线**：执行前必须通读本模板 `ai_red_flags` 与第 8 节「约束与红线」。

---

## 1. 目标

实现文档上传、生成签名直传 URL、异步解析 PDF/Office 生成 page webp、chunks、bboxes，覆盖 API-05、API-06；明确 `document_pages.image_object_key` 与 `document_files(PAGE_WEBP)` 的单一事实源。

---

## 2. 上下文

| 文档 | 链接/章节 |
|------|-----------|
| PRD | `docs/PRD-v2.1.0.md` §8.2 |
| TDD | `docs/TDD-v2.1.0.md` §6.1、§6.2 |
| API 契约 | `docs/API-SPEC-v2.1.0.md` API-05、API-06 |
| DB | `docs/database-model-v2.1.0.md` |
| 父 Issue | `DS-005 / DS-006 / DS-007` |
| CODE-REVIEW 检查项 | `docs/templates/CODE-REVIEW-template-v1.md` 3.11 |

---

## 3. 输入

### 3.0 LLM 上下文预算与分块策略

| 项 | 建议 |
|------|------|
| **上下文预算** | 大；优先读 API-05/06 与 database-model 文档相关表。 |
| **必读顺序** | front matter → `ai_red_flags` → 目标 → 边界条件 → 输出。 |
| **分块策略** | 若上下文受限，先读上传接口与 documents/pages/chunks 表结构。 |

### 3.1 已有代码/表（执行前必须阅读）

- `apps/api/internal/server/routes.go`（来自 TASK-BACKEND-002）
- `apps/api/internal/middleware/auth.go`
- `docs/API-SPEC-v2.1.0.md` API-05、API-06
- `docs/database-model-v2.1.0.md`

### 3.2 数据模型/接口

```sql
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    storage_key TEXT NOT NULL,
    page_count INT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE ingestion_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id),
    status TEXT NOT NULL DEFAULT 'queued',
    attempts INT DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE pages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id),
    page_number INT NOT NULL,
    image_url TEXT,
    width INT,
    height INT,
    UNIQUE (document_id, page_number)
);

CREATE TABLE chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    page_id UUID NOT NULL REFERENCES pages(id),
    text TEXT NOT NULL,
    bbox JSONB,
    embedding VECTOR(1536)
);
```

### 3.3 边界条件

| 维度 | 约束 | 说明 |
|------|------|------|
| 文件大小 | ≤ 100MB | 超过返回 413 |
| 格式 | pdf / docx / pptx / xlsx | 非法返回 415 |
| page webp | 最长边 ≤ 2048px | 缩放生成 |
| 任务重试 | 最多 3 次 | Redis 队列或 DB 轮询 |
| 对象存储 key | `tenants/{tenant_id}/workspaces/{workspace_id}/documents/{doc_id}/{filename}` | 隔离 |
| 最大变更行数 | ≤ 400 | 超出需拆分 |

### 3.4 失败用例

| 场景 | 输入 | 预期行为 |
|------|------|----------|
| 文件过大 | 101MB | 413 `payload_too_large` |
| 非法格式 | `.exe` | 415 `unsupported_media_type` |
| 上传后解析失败 | 损坏 PDF | job 状态 `failed`，错误信息可查询 |
| 越权上传 | 非 workspace 成员 | 403 `forbidden` |
| 签名 URL 过期 | 超过 TTL | 对象存储返回 403 |

### 3.5 测试数据 / Mock / Fixture

```json
{
  "document": {
    "title": "Q3 Pitch",
    "filename": "q3-pitch.pdf"
  },
  "uploadPolicy": {
    "maxSize": 104857600,
    "allowedTypes": ["application/pdf", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"]
  }
}
```

---

## 4. 输出

### 4.1 需要创建/修改的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `apps/api/internal/db/migrations/002_documents_ingestion.up.sql` | 新增 | documents / ingestion_jobs / pages / chunks 表 |
| `apps/api/internal/db/migrations/002_documents_ingestion.down.sql` | 新增 | 回滚 |
| `apps/api/internal/db/queries.sql` | 修改 | 新增相关 sqlc 查询 |
| `apps/api/internal/upload/service.go` | 新增 | 上传与直传 URL |
| `apps/api/internal/upload/handler.go` | 新增 | 上传路由 |
| `apps/api/internal/ingestion/service.go` | 新增 | 解析任务调度 |
| `apps/api/internal/ingestion/pdf.go` | 新增 | PDF 处理（pdfcpu / image） |
| `apps/api/internal/ingestion/office.go` | 新增 | OnlyOffice 调用 |
| `apps/api/internal/storage/s3.go` | 新增 | S3/MinIO 客户端 |
| `apps/api/internal/server/routes.go` | 修改 | 注册 documents 路由 |

### 4.2 行为定义

- `POST /api/documents` 接收文件元数据，返回上传 URL 或签名直传参数。
- 上传完成后创建 `document` 记录与 `ingestion_jobs` 记录。
- worker 消费 job，解析文件生成 `pages` 与 `chunks`。
- `GET /api/documents/:id/status` 返回 ingestion 状态。

---

## 5. 验收标准

- [x] 上传文件返回 document 记录
- [x] ingestion 完成后生成 pages 与 chunks
- [x] 签名 URL 可访问对象存储
- [x] 任务失败可重试，状态可查询
- [x] 越权访问返回 403
- [x] `go test ./...` 通过
- [x] `make lint` 通过

---

## 6. 实现步骤建议

1. 编写 migration `002_documents_ingestion`。
2. 更新 `queries.sql` 与 sqlc 生成代码。
3. 实现 `internal/storage/s3.go`，支持 S3 兼容 API（MinIO/S3）。
4. 实现 `internal/upload/service.go` 与 `handler.go`。
5. 实现 `internal/ingestion/service.go` 任务调度（可先用同步或 Redis 队列）。
6. 实现 `internal/ingestion/pdf.go` 解析 PDF 为图片与文本块。
7. 实现 `internal/ingestion/office.go` 调用 OnlyOffice 转换。
8. 注册路由。
9. 编写测试。
10. 运行 `make lint && make test`。
11. 提交 PR。

---

## 7. 测试验证

### 7.1 单元测试

```bash
cd apps/api && go test ./internal/upload/... ./internal/ingestion/... ./internal/storage/...
```

### 7.2 集成测试

```bash
cd apps/api && docker compose up -d
go test ./tests/integration/... -tags integration
docker compose down
```

### 7.3 手动验证

```bash
# 获取上传 URL
curl -X POST http://localhost:8080/api/documents \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"title":"Q3 Pitch","filename":"q3-pitch.pdf"}'
```

### 7.4 回归测试命令

```bash
cd apps/api && make lint && make test
```

---

## 8. 约束与红线

- **单 PR 规模**：修改文件数不得超过 `estimated_files`，变更行数建议不超过 `max_lines`。
- **范围锁定**：聚焦上传与 ingestion 核心链路；不做高级 PDF 排版、OCR、视频解析。
- **租户隔离**：对象存储 key 与所有查询必须带 `tenant_id` / `workspace_id`。
- **认证授权**：上传/查询必须通过 auth middleware。
- **不要提前实现**：范围外的功能（如公开链接、AI search）不要碰。
- **禁止把敏感数据发送给 LLM**：Token、密码、密钥、PII、生产数据一律不得出现在 prompt、日志或测试数据中。
- **代码风格**：遵循 Go 标准项目布局。

---

## 9. 与 CODE-REVIEW AI 检查项的交叉引用

| CODE-REVIEW 3.11 检查项 | 本任务自检要求 |
|------------------------|----------------|
| 无硬编码示例域名/邮箱/密码 | 对象存储 endpoint/credential 来自环境变量。 |
| 无未清理的 TODO / FIXME / placeholder | 全局搜索无残留。 |
| 无幻觉常量 | 文件大小、重试次数、缩放尺寸使用常量/配置。 |
| 错误处理不过度 try-catch，不吞掉异常 | ingestion 错误写入 job 记录并抛出。 |
| 未引入未使用的依赖或代码 | `go mod tidy` 与 lint 通过。 |
| 未擅自实现范围外功能 | 仅 upload + ingestion。 |
| 测试数据与生产数据隔离 | fixture 数据不引用生产。 |

---

## 10. Definition of Done

- [x] 代码实现完成
- [x] 测试通过（单元 + 集成）
- [x] lint / build 通过
- [ ] 代码审查通过
- [x] 与父 Issue 的验收标准对齐
- [ ] PR 已关联父 Issue：`Closes #DS-005` / `Relates to #DS-006 #DS-007`
- [x] 已按第 9 节完成 CODE-REVIEW 3.11 自检

---

## 11. Agent 备注

- 本地开发推荐 MinIO，通过 docker-compose 启动；生产配置 S3。
- OnlyOffice 转换可用其 `/converter` API；开发环境可用容器 `onlyoffice/documentserver`。
- PDF 解析可用 `pdfcpu` 提取文本，用 `github.com/nfnt/resize` 或 `golang.org/x/image/draw` 生成 webp（需 libwebp 或保存为 png 先用）。
- 若文件数超出，可将 Office pipeline 拆为单独 task。

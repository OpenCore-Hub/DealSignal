# Performance Report v2.1.2

## 1. 目标 (Targets)

> 以下基线为示例值，需在真实部署环境中根据机器规格与业务预期校准。

| Endpoint | 目标 P99 延迟 | 目标 QPS | 优先级 |
|----------|---------------|----------|--------|
| 公开链接访问 (`GET /api/v1/public/links/{token}`) | < 200ms | 1000/s | P0 |
| 签名 URL 生成 (`POST /api/v1/workspaces/{slug}/documents/{id}/signed-url`) | < 100ms | 500/s | P0 |
| 搜索 (`GET /api/v1/workspaces/{slug}/search`) | < 500ms | 100/s | P1 |
| AI 问答 (`POST /api/v1/workspaces/{slug}/assistant/chat`) | < 3000ms | 30/s | P1 |

## 2. 工具 (Tools)

- [k6](https://k6.io/)：主要压测脚本与吞吐量/延迟度量。
- vegeta（可选）：补充恒定速率的 HTTP 压力。
- `go test -race -cover`：并发正确性验证。

脚本位置：`apps/api/scripts/loadtest/`

```bash
# 示例
make loadtest-public-link TARGET=http://localhost:8080 LINK_TOKEN=abc123
make loadtest-signed-url TARGET=http://localhost:8080 WORKSPACE_SLUG=acme DOCUMENT_ID=xxx API_TOKEN=xxx
```

## 3. 环境 (Environment)

| 项 | 值 |
|----|----|
| 部署方式 | Local Docker Compose |
| 数据库 | PostgreSQL 16 |
| 缓存 | Redis 7 |
| 对象存储 | MinIO (local S3) |
| 压测机 | macOS / 16 GB RAM / Apple Silicon |

## 4. 结果 (Results)

> 待在实际环境中执行后填写。

| Endpoint | RPS | P50 | P95 | P99 | 错误率 | 备注 |
|----------|-----|-----|-----|-----|--------|------|
| 公开链接访问 | - | - | - | - | - | 未执行 |
| 签名 URL 生成 | - | - | - | - | - | 未执行 |
| 搜索 | - | - | - | - | - | 未执行 |
| AI 问答 | - | - | - | - | - | 未执行 |

## 5. 发现与优化项 (Action Items)

- [ ] 在真实生产镜像上复现压测并回填结果。
- [ ] 根据结果确认数据库连接池大小与 Redis 缓存命中率。
- [ ] 对公开链接访问考虑 CDN / 边缘缓存。
- [ ] 对 AI 问答实施流式响应与限流。

## 6. 红线声明

- 压测脚本不得在正式生产环境直接执行。
- 压测数据必须与生产数据隔离。

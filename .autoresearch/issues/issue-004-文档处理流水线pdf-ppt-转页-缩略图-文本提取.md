# 文档处理流水线（PDF/PPT 转页、缩略图、文本提取）

## Description
异步处理上传的文档，生成页级记录、缩略图、文本摘录，为页面级分析提供数据。

## Source
US-001, FR-12, database-model.md Section 2.3

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] PDF 上传后自动解析为 document_pages 记录
- [ ] 每页生成缩略图 storage key 与 text_excerpt
- [ ] 处理状态机支持 uploaded / processing / ready / failed
- [ ] 处理失败时记录 processing_error 并允许重试
- [ ] PPT 与 DOC 至少解析为单页/结构化记录（MVP 可降级）

## Validation
- [ ] 上传 10 页 PDF 后 document_pages 出现 10 条记录
- [ ] 文档 ready 后可查询到 page_count

## Dependencies
#3

## Type
backend

## Priority
high

## Risk Class
unknown

## PRD Reference
PRD.md Section 7.1, database-model.md Section 2.3

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 3.1 document_page_tiles, 5.1 Tile Pipeline (steps 2-5)
- API endpoints: Internal pg-boss processing job; no public API
- Data model: document_pages, document_page_tiles
- Testing guidance: 10-page PDF produces 10 document_page_tiles rows

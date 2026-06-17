# 文档上传与版本管理后端

## Description
实现文档上传 API、对象存储写入、版本记录与基本元数据管理，支持 PDF / PPT / DOC / XLS / 图片 / 视频等文件类型。

## Source
US-001, FR-1, FR-27

## Hard Constraints
- 所有多租户表必须按 workspace_id 隔离

## Acceptance Criteria
- [ ] 用户可向工作区上传支持的文件类型
- [ ] 文件内容写入对象存储，数据库存储 bucket/key/大小/checksum
- [ ] 同一文档支持多版本（document_versions）
- [ ] 上传失败返回具体错误信息
- [ ] API 返回上传进度或状态

## Validation
- [ ] 上传 PDF 后 documents 与 document_versions 表生成对应记录
- [ ] 重复上传同一 document 产生递增的 version_number

## Dependencies
#1, #2

## Type
backend

## Priority
high

## Risk Class
build_failure

## PRD Reference
PRD.md Section 7.1, US-001, FR-1, FR-27

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 Documents, 5.1 Tile Pipeline (step 1), 7.3 Data Protection
- API endpoints: POST /documents, GET /documents, GET /documents/:id, POST /documents/:id/versions
- Data model: documents, document_versions, R2 source bucket
- Testing guidance: Integration: upload PDF, verify DB + R2 records

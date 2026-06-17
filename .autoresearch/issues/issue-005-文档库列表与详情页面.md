# 文档库列表与详情页面

## Description
实现桌面端文档列表与文档详情页，支持查看文档状态、版本、链接数、打开次数、Top pages 等指标。

## Source
US-001, UI/page-prototypes.md Section 6-7

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 文档列表展示名称、类型、状态、链接数、打开数、更新时间
- [ ] 支持按状态 / 类型 / 所有者筛选
- [ ] 文档详情页包含 Overview / Pages / Links / Versions / Settings 标签
- [ ] Overview 展示总打开数、独立收件人、平均阅读时长、下载数
- [ ] Pages 标签展示每页平均停留时间与跳出率

## Validation
- [ ] 在浏览器中打开 Documents 页面可见已上传文档
- [ ] 点击文档进入 Document Detail 后数据加载正常

## Dependencies
#3, #4

## Type
frontend

## Priority
high

## Risk Class
test_failure

## PRD Reference
PRD.md US-001, UI/page-prototypes.md Section 6-7

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 Documents, 9.4 Acceptance Mapping (US-001)
- API endpoints: GET /documents, GET /documents/:id
- Data model: documents, document_versions, document_page_tiles (counts)
- Testing guidance: Browser test shows uploaded documents and detail tabs

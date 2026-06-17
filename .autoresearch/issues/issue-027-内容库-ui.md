# 内容库 UI

## Description
实现 Content Library 页面，支持按 Approved / Drafts / Archived / Templates 分类查看、审批、归档与使用统计。

## Source
US-010, UI/page-prototypes.md Section 17

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 内容库页面展示文档状态与集合
- [ ] Admin 可审批或归档文档
- [ ] 可配置仅允许从 Approved 内容创建 Smart Link
- [ ] 展示文档内容表现（链接数、打开数、转化率）

## Validation
- [ ] 在浏览器中打开 Content Library 可看到文档列表
- [ ] 审批后该文档状态变为 Approved

## Dependencies
#26

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
PRD.md US-010, UI/page-prototypes.md Section 17

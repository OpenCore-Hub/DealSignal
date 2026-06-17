# 链接详情与管理页面

## Description
实现 Link Detail 页面，展示链接状态、安全模式、复制链接、撤销链接、收件人活动摘要等。

## Source
US-002, UI/page-prototypes.md Section 9

## Hard Constraints
- 访问控制必须在文档内容加载前执行

## Acceptance Criteria
- [ ] 页面头部展示链接名称、文档名、状态、安全模式
- [ ] 支持一键复制链接与撤销链接
- [ ] 展示意图评分卡片（占位或真实数据）
- [ ] 展示最近活动摘要
- [ ] 撤销后链接状态立即更新

## Validation
- [ ] 点击 Revoke 后链接状态变为 revoked 且 viewer 访问被拒绝
- [ ] 浏览器中 Link Detail 页面加载无报错

## Dependencies
#6

## Type
frontend

## Priority
high

## Risk Class
test_failure

## PRD Reference
PRD.md US-002, UI/page-prototypes.md Section 9

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 Smart Links, 9.4 Acceptance Mapping (US-002)
- API endpoints: GET /smart-links/:id, POST /smart-links/:id/revoke
- Data model: smart_links, activity_events
- Testing guidance: Revoke action blocks viewer access

# 收件人活动时间线与分析展示

## Description
在 Link Detail 等页面展示收件人活动时间线、页面级分析、转发/新收件人检测、下载事件等。

## Source
US-004, FR-14, FR-15, UI/page-prototypes.md Section 9

## Hard Constraints
- 分析事件表为 append-only，不得更新历史记录

## Acceptance Criteria
- [ ] Link Detail 展示 Activity Timeline：打开、页面浏览、转发、下载
- [ ] Page Analytics 展示每页平均停留时间
- [ ] 识别并展示新收件人/转发信号
- [ ] 数据更新延迟不超过 10 秒
- [ ] 在浏览器中验证数据准确性

## Validation
- [ ] 打开链接并翻页后，Link Detail 时间线出现对应事件
- [ ] 多一个邮箱打开同一链接时，显示转发/新收件人提示

## Dependencies
#11, #12

## Type
fullstack

## Priority
high

## Risk Class
test_failure

## PRD Reference
PRD.md US-004, FR-14, FR-15, UI/page-prototypes.md Section 9

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 Analytics, 9.4 Acceptance Mapping (US-004)
- API endpoints: GET /analytics/links/:id, GET /analytics/documents/:id
- Data model: page_view_events, activity_events, view_sessions
- Testing guidance: Browser test shows timeline after events

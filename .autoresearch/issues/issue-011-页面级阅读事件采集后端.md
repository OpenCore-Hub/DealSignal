# 页面级阅读事件采集后端

## Description
实现 view_sessions、page_view_events、activity_events 的写入接口，记录首次打开、页面停留时长、翻页、关闭等行为。

## Source
US-004, FR-11, FR-12

## Hard Constraints
- 分析事件表为 append-only，不得更新历史记录

## Acceptance Criteria
- [ ]  viewer 打开时创建 view_sessions 记录
- [ ] 每页可见时间记录到 page_view_events（duration_ms）
- [ ] activity_events 记录 document_opened / document_closed / page_viewed 等事件
- [ ] 事件写入延迟在正常条件下小于 10 秒
- [ ] 失败写入支持重试或降级记录

## Validation
- [ ] 打开文档并浏览 3 页后，page_view_events 出现 3 条记录
- [ ] activity_events 按 workspace_id + occurred_at 可查询

## Dependencies
#9, #10

## Type
backend

## Priority
high

## Risk Class
test_failure

## PRD Reference
PRD.md US-004, FR-11, FR-12

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 events, 5.1 Analytics flow, 9.4 Acceptance Mapping (US-004)
- API endpoints: POST /v/:slug/events (beacon)
- Data model: view_sessions, page_view_events, activity_events
- Testing guidance: Events recorded after browsing; latency < 60s

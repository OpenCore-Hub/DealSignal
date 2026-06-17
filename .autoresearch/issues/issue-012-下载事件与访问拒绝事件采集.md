# 下载事件与访问拒绝事件采集

## Description
记录下载尝试与结果、访问被拒绝/过期/撤销的原因，为审计与风险分析提供数据。

## Source
US-004, FR-13, FR-29

## Hard Constraints
- 分析事件表为 append-only，不得更新历史记录

## Acceptance Criteria
- [ ] 成功下载记录到 download_events（status=allowed）
- [ ] 下载被禁用时记录 blocked 与原因
- [ ] 访问被拒绝、过期、撤销分别记录 activity_events
- [ ] 事件包含 actor_email、ip_address、watermarked 等上下文
- [ ] 支持通过 API 查询下载事件列表

## Validation
- [ ] 关闭下载后点击下载按钮，download_events 出现 blocked 记录
- [ ] 访问过期链接产生 access_denied 事件

## Dependencies
#9

## Type
backend

## Priority
high

## Risk Class
test_failure

## PRD Reference
PRD.md US-004, FR-13, FR-29

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 events, 5.4 Edge Cases
- API endpoints: POST /v/:slug/events
- Data model: download_events, activity_events
- Testing guidance: Blocked download creates record; access_denied events logged

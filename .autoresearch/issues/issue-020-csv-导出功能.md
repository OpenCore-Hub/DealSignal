# CSV 导出功能

## Description
为链接、文档、数据室的分析数据提供 CSV 导出能力，方便用户离线使用与周会汇报。

## Source
FR-30

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] Link Detail 支持导出该链接的收件人活动 CSV
- [ ] Document Detail 支持导出文档表现 CSV
- [ ] Room Detail 支持导出 room activity CSV
- [ ] CSV 包含时间、收件人、事件类型、页面、时长等字段
- [ ] 导出响应在合理时间内完成（< 10 秒对于千行数据）

## Validation
- [ ] 点击导出后生成可下载 CSV 文件
- [ ] CSV 内容包含预期列且编码正确

## Dependencies
#13

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
PRD.md FR-30

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 Exports, 8.3 Database Considerations
- API endpoints: GET /exports/links/:id.csv, GET /exports/documents/:id.csv, GET /exports/rooms/:id.csv
- Data model: activity_events, page_view_events, view_sessions
- Testing guidance: Downloaded CSV has expected columns

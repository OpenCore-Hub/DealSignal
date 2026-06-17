# 基础数据室后端

## Description
实现 Deal Room 的创建、文件夹结构、文件关联、外部成员邀请、访问规则与活动日志。

## Source
US-006, FR-18~FR-21

## Hard Constraints
- 所有多租户表必须按 workspace_id 隔离
- 访问控制必须在文档内容加载前执行

## Acceptance Criteria
- [ ] 可创建 Deal Room 并设置名称、类型、默认权限
- [ ] 支持嵌套文件夹（deal_room_folders）
- [ ] 可向文件夹添加文档版本（deal_room_files）
- [ ] 可邀请外部收件人为 room members
- [ ] 支持按 contact / account / domain 设置访问规则
- [ ] 记录 room 级 activity_events

## Validation
- [ ] 创建 room 后 deal_rooms、deal_room_folders、deal_room_members 生成记录
- [ ] 外部成员访问 room 时按访问规则校验

## Dependencies
#3, #2

## Type
backend

## Priority
medium

## Risk Class
build_failure

## PRD Reference
PRD.md Section 7.5, US-006, FR-18~FR-21

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 3.1 Rooms schema, 4.1 Rooms, 5.1 Room logic
- API endpoints: POST /rooms, GET /rooms, GET /rooms/:id, POST /rooms/:id/folders, POST /rooms/:id/files, POST /rooms/:id/members
- Data model: deal_rooms, deal_room_folders, deal_room_files, deal_room_members, deal_room_access_rules
- Testing guidance: DB records created; access rules enforced

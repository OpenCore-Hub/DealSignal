# 数据室创建与管理 UI

## Description
实现 Deal Rooms 列表、创建流程、Room Detail（Overview/Files/Recipients/Activity/Q&A/Settings）等管理界面。

## Source
US-006, UI/page-prototypes.md Section 12-14

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] Deal Rooms 列表展示名称、模板、成员数、热信号、最后活动
- [ ] 创建流程支持选择模板、命名、添加文档、邀请成员
- [ ] Room Detail Files 标签展示文件夹树与文件列表
- [ ] Recipients 标签展示成员权限与活动
- [ ] Activity 标签展示时间线
- [ ] Q&A 支持提问与回答（MVP 可简化）

## Validation
- [ ] 在浏览器中创建 Seed Fundraising Room 后可在列表看到
- [ ] 邀请成员后 Room Detail Recipients 标签显示该成员

## Dependencies
#18

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
PRD.md US-006, UI/page-prototypes.md Section 12-14

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 Rooms, 9.4 Acceptance Mapping (US-006)
- API endpoints: All /rooms endpoints
- Data model: deal_rooms, deal_room_folders, deal_room_files, deal_room_members
- Testing guidance: Browser test creates room from template and invites member

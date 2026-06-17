# 品牌化 LP 门户 UI（LP Portal）

## Description
为投资机构实现品牌化 LP 门户界面，LP 登录后可见 fund deck、季度报告、税务文件等聚合材料，支持按 LP 权限展示不同内容。

## Source
P2: LP Portal, UI/page-prototypes.md Section 10.3 Mobile Room Viewer

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 门户首页展示工作区品牌、最新报告、未读内容
- [ ] 按 LP 账户/联系人权限过滤可见房间和文件
- [ ] 支持文件夹导航和文件搜索
- [ ] 展示通知和新内容上线提醒
- [ ] 响应式布局支持桌面和移动端

## Validation
- [ ] LP 登录门户后可见被授权的报告列表
- [ ] 不同 LP 账户看到的内容按权限区分

## Dependencies
#18, #29

## Type
frontend

## Priority
low

## Risk Class
unknown

## PRD Reference
PRD.md Section 11 P2, UI/page-prototypes.md Section 10.3

## SPEC Reference
SPEC file: `tasks/spec-dealsignal-v1.md`; relevant sections: Section 4.1 Rooms; API endpoints: GET /rooms/:id, GET /rooms/:id/files; data model: deal_rooms, deal_room_members, deal_room_access_rules

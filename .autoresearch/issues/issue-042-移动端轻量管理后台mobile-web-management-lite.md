# 移动端轻量管理后台（Mobile Web Management Lite）

## Description
实现发送方在移动设备上的轻量管理界面，包括底部导航、Activity Feed、Hot Signals、Link/Room Summary、Access Requests 和通知设置。复杂的数据室搭建和文档上传仍保留在桌面端。

## Source
UI/page-prototypes.md Section 11

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 底部导航包含 Activity / Hot / Links / Rooms / Me
- [ ] Activity Feed 展示 first open、repeat open、hot score、forward、access request 等事件
- [ ] Hot Signals 卡片展示收件人、评分、解释、建议动作
- [ ] Link Summary 支持复制链接、发送跟进、撤销、打开桌面分析
- [ ] Room Summary 支持批准访问、查看活跃收件人、打开桌面房间
- [ ] Access Requests 支持一键批准/拒绝/批准域名
- [ ] 在 iOS Safari 和 Chrome Android 上验证可用

## Validation
- [ ] 在移动端浏览器打开管理后台，Hot Signals 列表正常显示
- [ ] 点击 Approve 后 access_grant 状态更新为 approved

## Dependencies
#15, #17

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
UI/page-prototypes.md Section 11

## SPEC Reference
SPEC file: `tasks/spec-dealsignal-v1.md`; relevant sections: Section 2.1 System Context, 4.1 Analytics; API endpoints: GET /analytics/dashboard, GET /analytics/links/:id, POST /rooms/:id/members; data model: recommendations, access_grants, intent_scores

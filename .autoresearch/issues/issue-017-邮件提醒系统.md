# 邮件提醒系统

## Description
实现邮件通知队列，支持首次打开、Hot score 等事件的邮件提醒，并链接到对应分析页。

## Source
US-008, FR-22

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 用户可配置是否接收邮件提醒
- [ ] 首次打开文档时向发送方发送邮件
- [ ] Hot score 事件触发邮件提醒
- [ ] 邮件包含链接到 Link Detail 的按钮
- [ ] 通知发送失败进入重试队列

## Validation
- [ ] 收件人打开链接后，发送方邮箱收到 first-open 邮件
- [ ] 模拟高意图行为后发送方收到 hot-score 邮件

## Dependencies
#14

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
PRD.md US-008, FR-22

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 Alerts, 6.2 Retry Strategy, 9.4 Acceptance Mapping (US-008)
- API endpoints: Internal alert job triggered by analytics events
- Data model: notifications, notification_preferences
- Testing guidance: Email received after first-open and hot-score events

# Slack 提醒集成

## Description
连接 Slack workspace，将首次打开、Hot score、转发检测等事件发送到指定频道。

## Source
P1: Slack alerts, FR-23

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 用户可通过 OAuth 连接 Slack
- [ ] 可配置提醒事件类型与目标频道
- [ ] Hot score 事件触发 Slack 消息
- [ ] 消息包含链接到 DealSignal 的按钮

## Validation
- [ ] 配置后模拟 Hot score 事件，Slack 频道收到消息

## Dependencies
#17

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
PRD.md Section 7.8, FR-23, Section 11 P1

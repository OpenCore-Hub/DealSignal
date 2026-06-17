# CRM 活动同步

## Description
将文档打开、高意图等事件写入 CRM timeline，并在启用时自动创建跟进任务。

## Source
US-009, FR-25

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] Smart Link 可与 CRM deal / contact 关联
- [ ] 文档打开事件写入关联 CRM 对象 timeline
- [ ] Hot score 事件触发 CRM task 创建（可配置）
- [ ] 失败同步进入重试队列

## Validation
- [ ] 模拟文档打开后，HubSpot/Salesforce timeline 出现对应事件

## Dependencies
#23, #11

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
PRD.md US-009, FR-25

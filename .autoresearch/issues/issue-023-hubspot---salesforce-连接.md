# HubSpot / Salesforce 连接

## Description
实现 CRM 集成连接，支持 OAuth 授权并存储 access token，建立 DealSignal 对象与 CRM 对象的映射。

## Source
US-009, FR-24

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 支持连接 HubSpot 与 Salesforce
- [ ] 存储加密后的 integration credentials
- [ ] 支持将 contact / account / smart_link / deal_room 映射到 CRM 对象
- [ ] 连接状态可显示在设置页

## Validation
- [ ] 完成 OAuth 后 integrations 表生成 connected 记录
- [ ] crm_mappings 可保存对象映射

## Dependencies
#2

## Type
backend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
PRD.md Section 7.8, US-009, FR-24

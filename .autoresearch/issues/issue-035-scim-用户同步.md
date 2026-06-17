# SCIM 用户同步

## Description
提供 SCIM 2.0 接口，允许企业通过身份提供商自动同步用户、分配角色、禁用账户。

## Source
P2: SCIM

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 实现 SCIM /Users 与 /Groups 端点
- [ ] 支持创建、更新、停用用户
- [ ] 支持通过 group 映射工作区角色
- [ ] 同步事件记录审计日志

## Validation
- [ ] 从 IdP 推送用户后 DealSignal 工作区出现对应成员

## Dependencies
#34

## Type
backend

## Priority
low

## Risk Class
unknown

## PRD Reference
PRD.md Section 11 P2

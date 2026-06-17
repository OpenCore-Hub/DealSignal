# 认证与工作区模型

## Description
实现用户认证、工作区创建、成员关系与角色权限，为后续多租户功能提供身份与权限基础。

## Source
database-model.md Section 2.1, PRD.md Section 4

## Hard Constraints
- 所有多租户表必须按 workspace_id 隔离

## Acceptance Criteria
- [ ] 用户可通过邮箱注册并登录
- [ ] 用户可创建多个工作区并在其间切换
- [ ] 工作区成员角色支持 owner / admin / member / viewer
- [ ] 所有 API 默认按 workspace_id 过滤
- [ ] 单元测试覆盖认证与角色校验

## Validation
- [ ] 调用非当前工作区的资源返回 403
- [ ] 数据库中存在 owner 角色的 workspace_memberships 记录

## Dependencies
#1

## Type
backend

## Priority
high

## Risk Class
build_failure

## PRD Reference
PRD.md Section 4, database-model.md Section 2.1

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 7.1 Authentication & Authorization, 9.1 Unit Tests
- API endpoints: POST /auth/register, POST /auth/login, GET /auth/me, POST /workspaces
- Data model: users, workspaces, workspace_memberships
- Testing guidance: Unit tests for workspace isolation middleware; cross-workspace returns 403

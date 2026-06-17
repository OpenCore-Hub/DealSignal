# 设置中心（Settings）

## Description
实现 Settings 页面，支持工作区配置、成员管理、角色权限、品牌设置、安全默认值、集成连接、账单和数据隐私设置。

## Source
UI/page-prototypes.md Section 18

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] Workspace 设置：名称、slug、模式（founder/investment_firm/sales/mixed）
- [ ] Members 设置：邀请成员、分配角色 owner/admin/member/viewer、移除成员
- [ ] Branding 设置：上传 logo、设置主色、预览品牌化阅读器
- [ ] Security defaults：默认访问模式、下载策略、水印策略
- [ ] Integrations：连接/断开 Slack、HubSpot、Salesforce
- [ ] Billing：展示当前计划与使用配额（可占位）
- [ ] Data and privacy：数据保留、删除请求入口

## Validation
- [ ] 在浏览器中打开 Settings 可切换各子页面
- [ ] 修改品牌设置后 viewer 顶部栏显示自定义 logo

## Dependencies
#2

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
UI/page-prototypes.md Section 18

## SPEC Reference
SPEC file: `tasks/spec-dealsignal-v1.md`; relevant sections: Section 7.1 Authentication & Authorization, 7.2 External Interfaces; API endpoints: GET/PUT /workspaces/:id, GET/POST /workspaces/:id/memberships, GET/POST /integrations; data model: workspaces, workspace_memberships, integrations, notification_preferences

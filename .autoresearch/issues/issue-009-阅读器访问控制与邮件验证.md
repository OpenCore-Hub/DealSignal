# 阅读器访问控制与邮件验证

## Description
实现公开访问链接的入口校验、过期/撤销检测、邮箱验证流程、允许列表/密码校验以及被阻止状态的友好提示页。

## Source
US-003, FR-6, FR-29, UI/page-prototypes.md Section 10.1

## Hard Constraints
- 访问控制必须在文档内容加载前执行
- 收件人只有在发送方策略明确要求时才需要创建账户

## Acceptance Criteria
- [ ] 打开有效链接时直接进入 viewer
- [ ] 过期或撤销链接显示清晰原因与联系发送方入口
- [ ] email_verification 模式发送一次性验证邮件/验证码
- [ ] allowlist 模式拒绝非允许邮箱并提示
- [ ] password 模式要求输入密码后方可查看
- [ ] 所有访问检查在返回文档内容前完成

## Validation
- [ ] 直接访问已撤销链接返回 403/阻止页面，不暴露文档内容
- [ ] 邮箱验证通过后可在同一浏览器会话内查看
- [ ] 移动端浏览器中验证通过

## Dependencies
#6

## Type
fullstack

## Priority
high

## Risk Class
test_failure

## PRD Reference
PRD.md US-003, FR-6, FR-29, UI/page-prototypes.md Section 10.1

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 Viewer, 5.1 Access Resolution, 7.3 Data Protection, 9.4 (FR-29)
- API endpoints: GET /v/:slug, POST /v/:slug/verify, POST /v/:slug/password, POST /v/:slug/request-access
- Data model: smart_links, smart_link_recipients, access_grants, view_sessions
- Testing guidance: Revoked/expired links show block page without content; public link opens without account

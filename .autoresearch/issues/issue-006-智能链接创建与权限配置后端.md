# 智能链接创建与权限配置后端

## Description
实现 Smart Link 的创建、命名、slug 生成、访问模式、过期、下载策略、水印开关、密码、白名单等权限配置后端。

## Source
US-002, FR-2~FR-10

## Hard Constraints
- 访问控制必须在文档内容加载前执行

## Acceptance Criteria
- [ ] 可为文档生成一个或多个唯一 slug 链接
- [ ] 支持 access_mode: public / email_verification / allowlist / password / approval_required
- [ ] 支持 expires_at、revoked_at、download_policy、watermark_enabled
- [ ]  allowlist 支持邮箱域名或具体邮箱列表
- [ ] 密码模式必须设置 password_hash
- [ ] 链接状态包含 active / expired / revoked

## Validation
- [ ] API 创建链接后 smart_links 表生成记录
- [ ] 过期链接自动返回 expired 状态
- [ ] 撤销链接后 status 变为 revoked

## Dependencies
#3

## Type
backend

## Priority
high

## Risk Class
build_failure

## PRD Reference
PRD.md Section 7.1, US-002, FR-2~FR-10

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 Smart Links, 5.2 Validation Rules, 5.3 State Machine
- API endpoints: POST /smart-links, GET /smart-links, GET /smart-links/:id, POST /smart-links/:id/revoke
- Data model: smart_links, smart_link_recipients
- Testing guidance: Create link with all access modes; verify slug + settings

# 智能链接创建表单 UI

## Description
实现创建 Smart Link 的表单界面，包含链接命名、收件人邮箱、访问预设、安全控件、接收方摩擦提示与创建结果展示。

## Source
US-002, UI/page-prototypes.md Section 8

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 用户可选择 Fast Share / Balanced / High Security 预设
- [ ] 表单显示每个安全选项对接收方摩擦的影响
- [ ] 支持开关：邮箱验证、下载、水印、NDA、过期时间
- [ ] 创建成功后显示可复制链接
- [ ] 在桌面和移动端浏览器中验证可用

## Validation
- [ ] 在浏览器中创建链接后可复制 slug URL
- [ ] 选择 High Security 预设时 recipient friction 显示为 High

## Dependencies
#6

## Type
frontend

## Priority
high

## Risk Class
test_failure

## PRD Reference
PRD.md US-002, UI/page-prototypes.md Section 8

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.2 POST /smart-links, 9.4 Acceptance Mapping (US-002)
- API endpoints: POST /smart-links
- Data model: smart_links, documents
- Testing guidance: Browser test creates link and copies URL; friction indicator accurate

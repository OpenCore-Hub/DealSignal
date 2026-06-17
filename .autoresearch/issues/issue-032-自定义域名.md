# 自定义域名

## Description
支持工作区绑定自定义域名（如 investor.fund.com），使阅读器与门户展示企业自有域名。

## Source
P2: Custom domain

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 工作区可配置自定义域名
- [ ] 提供 DNS 验证指引
- [ ] viewer 链接可通过自定义域名打开
- [ ] HTTPS 证书自动申请或支持上传

## Validation
- [ ] 配置自定义域名后，Smart Link 可通过该域名访问

## Dependencies
#10, #31

## Type
infra

## Priority
low

## Risk Class
unknown

## PRD Reference
PRD.md Section 11 P2

# 企业 DLP 集成

## Description
与常见 DLP/CASB 方案集成，支持内容扫描、敏感数据检测、外发策略联动等企业安全需求。

## Source
P3: Enterprise DLP integrations

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 提供 API 或 webhook 供 DLP 系统查询/扫描内容
- [ ] 支持上传前敏感信息扫描
- [ ] 支持按 DLP 策略阻止下载或分享
- [ ] 记录 DLP 相关审计事件

## Validation
- [ ] 上传含敏感信息文档时触发 DLP 策略

## Dependencies
#36

## Type
backend

## Priority
low

## Risk Class
unknown

## PRD Reference
PRD.md Section 11 P3

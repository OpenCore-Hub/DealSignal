# 数据驻留

## Description
支持企业客户选择数据存储区域（如 US/EU/Asia），满足合规与本地化要求。

## Source
P3: Data residency

## Hard Constraints
- 所有多租户表必须按 workspace_id 隔离

## Acceptance Criteria
- [ ] 企业工作区可选择数据驻留区域
- [ ] 文档、事件、数据库按区域隔离
- [ ] 跨区域访问遵循策略限制

## Validation
- [ ] 选择 EU 区域后，该工作区数据存储在 EU

## Dependencies
#1

## Type
infra

## Priority
low

## Risk Class
unknown

## PRD Reference
PRD.md Section 11 P3

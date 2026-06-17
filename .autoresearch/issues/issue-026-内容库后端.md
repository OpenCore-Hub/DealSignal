# 内容库后端

## Description
实现内容库的数据模型，支持文档状态 Draft / In Review / Approved / Archived、集合管理与使用统计。

## Source
US-010, FR-26, FR-28

## Hard Constraints
- 所有多租户表必须按 workspace_id 隔离

## Acceptance Criteria
- [ ] library_collections 与 library_items 表可用
- [ ] 文档状态可在 Draft / In Review / Approved / Archived 间切换
- [ ] 支持将文档加入集合
- [ ] 可追踪文档被使用的链接数与打开数

## Validation
- [ ] 标记文档为 Approved 后状态更新并记录审批人

## Dependencies
#3

## Type
backend

## Priority
medium

## Risk Class
build_failure

## PRD Reference
PRD.md Section 7.7, US-010, FR-26, FR-28

# 高级审计导出

## Description
提供合规级审计导出，包含完整访问日志、IP、设备、下载记录、权限变更等，支持 PDF/CSV。

## Source
P2: Advanced audit export

## Hard Constraints
- 分析事件表为 append-only，不得更新历史记录

## Acceptance Criteria
- [ ] 可按时间范围导出完整审计日志
- [ ] 导出包含 IP、设备、邮箱、事件类型、结果
- [ ] 支持 tamper-evident 摘要或签名（可选）
- [ ] 导出文件包含工作区与生成时间元数据

## Validation
- [ ] 导出审计日志后文件包含所有事件类型

## Dependencies
#20

## Type
backend

## Priority
low

## Risk Class
test_failure

## PRD Reference
PRD.md Section 11 P2

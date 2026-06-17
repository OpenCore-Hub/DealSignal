# 高级水印模板

## Description
扩展水印能力，支持自定义水印文本、位置、透明度、颜色，以及下载文件的水印嵌入。

## Source
P1: Advanced watermark templates

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 支持配置水印内容模板（邮箱、时间、IP、自定义文本）
- [ ] 支持调整水印位置与样式
- [ ] 下载 PDF 时可在文件上嵌入水印
- [ ] 不同链接可使用不同水印模板

## Validation
- [ ] 配置自定义水印后 viewer 与下载文件均显示对应水印

## Dependencies
#16

## Type
backend

## Priority
medium

## Risk Class
unknown

## PRD Reference
PRD.md Section 11 P1

# 基础动态水印

## Description
在文档 viewer 中叠加动态水印，显示收件人邮箱与访问时间，用于威慑与追溯；MVP 阶段 viewer 层水印即可。

## Source
US-007, FR-10

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] watermark_enabled=true 时 viewer 显示半透明水印
- [ ] 水印内容包含收件人邮箱与当前时间戳
- [ ] 水印不遮挡文档主体内容
- [ ] 水印设置状态在 Link Detail 中可见
- [ ] 下载时至少标记是否含水印（MVP 可仅在 viewer 层实现）

## Validation
- [ ] 启用水印后 viewer 截图可见邮箱与时间
- [ ] 关闭水印后 viewer 不再显示水印

## Dependencies
#10

## Type
backend

## Priority
medium

## Risk Class
unknown

## PRD Reference
PRD.md US-007, FR-10

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 5.1 Watermark, 7.3 Data Protection, 9.4 Acceptance Mapping (US-007)
- API endpoints: GET /v/:slug/manifest (watermarkText), viewer rendering
- Data model: smart_links (watermark_enabled)
- Testing guidance: Screenshot shows both server-baked and client-overlay watermarks

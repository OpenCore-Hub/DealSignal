# 文档阅读器渲染与页面导航

## Description
实现文档阅读器前端，支持 PDF 页面渲染、翻页、缩放、大纲导航、移动端适配，并在允许时提供下载按钮。

## Source
US-003, UI/page-prototypes.md Section 10.2

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 桌面与移动端均可打开并阅读文档
- [ ] 支持上一页/下一页、页码指示器、缩略图/大纲
- [ ] 支持 pinch zoom / 双击适配宽度
- [ ] 下载按钮仅在 download_policy 为 allowed 时显示
- [ ]  viewer 首屏渲染在 2 秒内完成（典型 PDF）

## Validation
- [ ] 在浏览器中打开 Smart Link 可逐页浏览 PDF
- [ ] 下载禁用时 viewer 不显示下载入口
- [ ] Chrome DevTools 移动视图下页面导航可用

## Dependencies
#4, #9

## Type
frontend

## Priority
high

## Risk Class
test_failure

## PRD Reference
PRD.md US-003, UI/page-prototypes.md Section 10.2

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 2.1 System Context, 4.2 GET /v/:slug/manifest, 5.1 Viewer Rendering (Canvas 2D + OffscreenCanvas + Web Worker)
- API endpoints: GET /v/:slug/manifest, GET /v/:slug/tiles/:token
- Data model: document_page_tiles, view_sessions
- Testing guidance: Browser and mobile viewport test; tiles decrypt and assemble correctly

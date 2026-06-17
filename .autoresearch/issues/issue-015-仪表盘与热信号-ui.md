# 仪表盘与热信号 UI

## Description
实现登录后首屏 Dashboard，展示今日热信号、推荐跟进、最近打开、活跃数据室、风险提醒与表现最佳内容。

## Source
US-005, US-008, UI/page-prototypes.md Section 5

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 首屏展示 Hot Signals / Opens / Risks 汇总卡片
- [ ] Recommended Follow-ups 列表展示高意图收件人与建议动作
- [ ] Recent Activity 展示最近事件
- [ ] 风险面板展示过期/可疑/被阻止访问
- [ ] 支持创始人/基金/销售三种 segment 的文案变体

## Validation
- [ ] 在浏览器打开 Dashboard 可见热信号卡片
- [ ] 有 Hot score 事件时 Recommended Follow-ups 出现对应条目

## Dependencies
#13, #14

## Type
frontend

## Priority
high

## Risk Class
test_failure

## PRD Reference
PRD.md US-005, US-008, UI/page-prototypes.md Section 5

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 4.1 Dashboard, 9.4 Acceptance Mapping (US-005, US-008)
- API endpoints: GET /analytics/dashboard
- Data model: intent_scores, activity_events, recommendations
- Testing guidance: Hot events appear on dashboard

# 意图评分计算与解释

## Description
基于阅读行为计算 0-100 的意图评分，输出 Cold/Warm/Hot 标签与解释文本，并随新活动更新。

## Source
US-005, FR-16, FR-17

## Hard Constraints
- 分析事件表为 append-only，不得更新历史记录

## Acceptance Criteria
- [ ] 系统为每个收件人生成 0-100 的 intent score
- [ ] 评分映射为 cold (0-39) / warm (40-69) / hot (70-100)
- [ ] 评分包含自然语言解释（如“重复查看财务页”）
- [ ] 新活动发生后评分在 1 分钟内重新计算
- [ ] 支持 founder / investment_firm / sales 三类评分类型

## Validation
- [ ] 模拟多次打开与关键页停留后，score 从 cold 升至 warm/hot
- [ ] 数据库 intent_scores 记录包含 explanation 与 factors

## Dependencies
#11, #12

## Type
backend

## Priority
high

## Risk Class
test_failure

## PRD Reference
PRD.md Section 7.4, US-005, FR-16, FR-17

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 5.1 Intent Scoring, 8.2 Optimization Strategy, 9.4 Acceptance Mapping (US-005)
- API endpoints: Internal pg-boss scoring job; GET /analytics/dashboard reads scores
- Data model: intent_scores, activity_events, page_view_events
- Testing guidance: Simulated activity changes score from cold to warm/hot

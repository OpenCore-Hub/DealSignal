# 行动助手推荐

## Description
基于意图评分与行为模式生成下一步行动建议（如跟进时机、推荐材料、建议会议），并展示在 Dashboard 与 Link Detail。

## Source
P1: Action Assistant recommendations, PRD.md Section 7.6

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] 检测高意图、停滞、异常访问等模式
- [ ] 生成推荐标题、正文与建议动作
- [ ] 推荐展示在 Dashboard 与 Link Detail
- [ ] 用户可 dismiss 或 mark done

## Validation
- [ ] 模拟高意图行为后 Dashboard 出现跟进建议
- [ ] 点击 mark done 后 recommendations 状态更新

## Dependencies
#14

## Type
backend

## Priority
medium

## Risk Class
unknown

## PRD Reference
PRD.md Section 7.6, Section 11 P1

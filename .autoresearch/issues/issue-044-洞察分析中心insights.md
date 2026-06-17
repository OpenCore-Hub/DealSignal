# 洞察分析中心（Insights）

## Description
实现 Insights 页面，包含 Intent Analytics、Content Performance、Page Performance、Team Performance 和 Risk & Audit 视图，帮助用户优化内容并识别机会与风险。

## Source
UI/page-prototypes.md Section 16

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] Intent Analytics 展示 Hot / Warm / Cold 收件人、停滞收件人、活跃度上升的账户
- [ ] Content Performance 展示 Top converting documents、drop-off pages、最高/最低互动页面
- [ ] Page Performance 展示每页平均停留时间、重读率、跳出率
- [ ] Team Performance 展示成员活跃度、发送链接数、产生的高意图信号
- [ ] Risk and Audit 展示被阻止访问、异常地区、下载事件、撤销/过期链接
- [ ] 支持按时间范围和 segment 筛选

## Validation
- [ ] 打开 Insights 页面可见 Intent Analytics 卡片
- [ ] 筛选时间范围后图表与表格数据更新

## Dependencies
#13, #14

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
UI/page-prototypes.md Section 16

## SPEC Reference
SPEC file: `tasks/spec-dealsignal-v1.md`; relevant sections: Section 4.1 Analytics, 8.3 Database Considerations; API endpoints: GET /analytics/insights/*; data model: activity_events, page_view_events, intent_scores, download_events

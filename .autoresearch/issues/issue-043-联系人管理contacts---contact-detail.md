# 联系人管理（Contacts + Contact Detail）

## Description
实现 Contacts 列表与 Contact Detail 页面，展示投资人/LP/客户/合伙人的互动历史、数据室访问记录、总体热度评分和推荐下一步动作。支持公司与账户级视图。

## Source
UI/page-prototypes.md Section 15

## Hard Constraints
- 无针对本 Issue 的额外硬约束；遵循 PRD 全局安全与隐私约束。

## Acceptance Criteria
- [ ] Contacts 列表展示姓名、邮箱、组织、细分标签、总体评分
- [ ] 支持按 segment、组织、评分筛选
- [ ] Contact Detail 展示个人资料、看过的文档、访问过的数据室、时间线
- [ ] 展示 Overall engagement score 和 Recommended next action
- [ ] Company/Account Detail 展示关联联系人、账户级评分、相关链接和房间
- [ ] 支持与 CRM 映射状态联动（P1）

## Validation
- [ ] 在浏览器中打开 Contacts 页面可见联系人列表
- [ ] 点击联系人进入 Detail 后时间线与评分加载正常

## Dependencies
#2, #13

## Type
frontend

## Priority
medium

## Risk Class
test_failure

## PRD Reference
UI/page-prototypes.md Section 15

## SPEC Reference
SPEC file: `tasks/spec-dealsignal-v1.md`; relevant sections: Section 4.1 Analytics, 7.2 External Interfaces; API endpoints: GET /contacts, GET /contacts/:id, GET /analytics/contacts/:id; data model: contacts, accounts, account_contacts, intent_scores, activity_events

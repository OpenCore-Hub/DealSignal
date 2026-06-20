---
id: "API-DEPRECATION-YYYY-NNN"
version: "{vX.Y.Z}"
status: "{草稿 / 评审中 / 已批准 / 已归档}"
owner: "{API 负责人}"
linked_docs:
  - "{docs/API-SPEC-vX.Y.Z.md}"
  - "{docs/openapi-vX.Y.Z.yaml}"
  - "{docs/TDD-vX.Y.Z.md}"
---

# {模块名} API 弃用策略

> **文档编号**：`API-DEPRECATION-YYYY-NNN`  
> **版本**：`{vX.Y.Z}`  
> **模板版本**：`v1`  
> **状态**：`{草稿 / 评审中 / 已批准 / 已归档}`  
> **负责人**：`{API 负责人}`  
> **关联文档**：
> - `docs/API-SPEC-vX.Y.Z.md`
> - `docs/openapi-vX.Y.Z.yaml`
> - `docs/TDD-vX.Y.Z.md`

---

## 0. 文档使用说明

本文档定义 `{产品名}` 如何有序、可预测地弃用 API 端点、字段或版本，确保外部/内部集成方有充足迁移时间。

**目标**：
- 建立统一的 API 弃用生命周期与沟通机制。
- 降低因 API 变更导致的集成方故障。
- 为 Sunset 日期、迁移路径、兼容性策略提供决策依据。

---

## 1. 弃用原则

1. **提前通知**：对外 API 至少提前 {N} 个主版本或 {M} 个月通知。
2. **提供替代方案**：每个弃用必须指向推荐的替代端点/字段/版本。
3. **渐进式关闭**：先标记 `Deprecated`，再返回 `Sunset` 头，最后停止服务。
4. **可观测**：监控被弃用 API 的调用量，确保在关闭前降至安全阈值。
5. **文档同步**：弃用信息必须同步到 OpenAPI、Release Notes、开发者文档。

---

## 2. 弃用生命周期

| 阶段 | API 行为 | 持续时间 | 沟通方式 |
|------|----------|----------|----------|
| **Active** | 正常运行 | — | — |
| **Deprecated** | 正常响应，响应头增加 `Deprecation` | {N} 个月 | 邮件、开发者文档、Release Notes |
| **Sunset** | 继续响应，响应头增加 `Sunset` 具体日期 | {M} 个月 | 再次通知、埋点告警 |
| **Removed** | 返回 `410 Gone` 或 `404 Not Found` | — | 最终通知 |

---

## 3. 弃用清单

### 3.1 API-01：{接口/字段名称}

| 字段 | 内容 |
|------|------|
| **类型** | {端点 / 字段 / 参数 / 版本} |
| **当前状态** | {Deprecated / Sunset / Removed} |
| **弃用版本** | `{vX.Y.Z}` |
| **Sunset 日期** | `{YYYY-MM-DD}` |
| **推荐替代** | `{新端点/字段/版本}` |
| **影响范围** | {内部 / 外部 / 特定客户} |
| **调用量基线** | {X QPS / 占总量 Y%} |
| **迁移负责人** | {姓名} |
| **迁移状态** | {进行中 / 已完成} |

---

## 4. 沟通模板

### 4.1 对外开发者通知

```
主题：[Action Required] {产品名} API {端点} 将于 {YYYY-MM-DD} 下线

正文：
- 受影响端点：{endpoint}
- 下线日期：{YYYY-MM-DD}
- 推荐替代：{alternative}
- 迁移文档：{link}
- 如有问题请联系：{support email}
```

### 4.2 内部同步

- Release Notes：`{docs/RELEASE-NOTES-vX.Y.Z.md}`
- 变更日志：`{docs/CHANGELOG-vX.Y.Z.md}`
- 内部 Wiki：{link}

---

## 5. 监控与告警

| 监控项 | 阈值 | 动作 |
|--------|------|------|
| 被弃用 API 调用量 | {> X QPS} | 通知迁移负责人 |
| 调用方数量 | {> N 个} | 扩大通知范围 |
| 距离 Sunset 日期 | {≤ 30 天} | 升级告警 |

---

## 6. 回滚与例外

- 若 Sunset 前仍有高调用量，可申请延长，需 {角色} 审批。
- 已 Removed 的 API 原则上不再恢复；确需恢复按重大变更流程处理。

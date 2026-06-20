---
id: "CL-YYYY-NNN"
version: "{vX.Y.Z}"
status: "{草稿 / 评审中 / 已批准 / 已归档}"
owner: "{负责人}"
---

# {产品名} 变更日志

> **文档编号**：`CL-YYYY-NNN`  
> **版本**：`{vX.Y.Z}`  
> **模板版本**：`v1`  
> **状态**：`{草稿 / 评审中 / 已批准 / 已归档}`  
> **编写人/适用对象**：`{产品/技术负责人}`  
> **编写日期**：`{YYYY-MM-DD}`  
> **关联文档**：  
> - `docs/RELEASE-NOTES-vX.Y.Z.md`  
> - `docs/PRD-vX.Y.Z.md`  
> - `docs/TDD-vX.Y.Z.md`  
> **评审人**：`{产品负责人、技术负责人}`  
> **发布状态（CHANGELOG 专用）**：`{草稿 / 已发布}`

---

## 说明

本文档记录产品所有版本的变更历史，面向开发者、运维、客户成功及高级用户。与 `RELEASE-NOTES` 不同：

- **RELEASE-NOTES**：面向用户/市场，强调价值与影响。
- **CHANGELOG**：面向技术团队，强调具体变更、兼容性、迁移事项。

---

## 格式规范

每个版本条目按以下结构编写：

```markdown
## [vX.Y.Z] - YYYY-MM-DD

### Added
- 新增功能 A

### Changed
- 功能 B 行为调整

### Deprecated
- 功能 C 已标记弃用，计划 vX.Y.Z 移除

### Removed
- 移除功能 D

### Fixed
- 修复问题 E

### Security
- 修复安全漏洞 F（CVE-XXXX-XXXXX，如适用）

### Migration
- 从 vX.Y.Z 升级需执行 ...
```

---

## 变更历史

### [v1.0.0] - {YYYY-MM-DD}

#### Added
- `{新增功能 1}`
- `{新增功能 2}`

#### Changed
- `{行为变更 1}`

#### Fixed
- `{缺陷修复 1}`

#### Security
- `{安全修复 1}`

#### Migration
- `{升级/迁移说明}`

---

## 兼容性矩阵

| 版本 | 数据库 schema | API 版本 | 最低客户端版本 |
|------|---------------|----------|----------------|
| `v1.0.0` | `schema-v5` | `v1` | `v1.0.0` |
| `v0.9.0` | `schema-v4` | `v1` | `v0.9.0` |

---

## 检查清单

- [ ] 每个版本都有发布日期
- [ ] 所有变更按 `Added/Changed/Deprecated/Removed/Fixed/Security/Migration` 分类
- [ ] 破坏性变更已标注并附迁移说明
- [ ] 安全修复已关联 CVE 或内部安全编号
- [ ] 与 RELEASE-NOTES 保持一致

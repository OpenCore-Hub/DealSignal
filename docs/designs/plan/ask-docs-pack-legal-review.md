# Ask Docs Pack 法务润色流程（D18）

> 配套：`docs/designs/plan/ask-docs-intent-first-clue-engine.md` §15.3  
> Pack 源：`apps/api/internal/assistant/jobs/{financing_dd_v1,ma_redflag_v1}.yaml`

## 允许改

- `label_en` / `label_zh`
- `query_en` / `query_zh`（仍必须是**空格分隔关键词串**，禁止完整问句 / `?` / `？`）

## 禁止改（须另开新 Pack 或版本提案）

- `item_id`
- `value_type`（含从空改为有值，或反之）
- `pack_id` / `pack_version` 语义（发新版 Pack 另议）
- 项数与顺序所隐含的稳定 id 表（§15.1 / §15.2）

## PR 检查清单

1. 独立 PR，标题含 `ask-docs pack legal` 或同等标识；**不要**与行为/检索改动混提。
2. Diff 仅触及 YAML 文案字段（及必要的设计文档措辞）；CI `go test ./internal/assistant/jobs/...` 通过（锁定 id / value_type / 禁问句）。
3. 融资包：P2 coverage **生产开启前**至少一轮法务/产品审阅，在 PR 描述记审阅人与日期。
4. 并购包：P2.2 上线前同等审阅。
5. 不在本 PR 修改 `ASK_DOCS_DD_BOUNDARY_*` 默认（阈值走 D16）。

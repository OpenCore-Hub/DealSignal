# Ask Docs boundary calibration (D16)

Offline tool for reviewing DD coverage boundary thresholds before changing §7.2 defaults.

## When to run

1. Run DD coverage (`financing_dd_v1`) on **≥3** real financing rooms and export ClaimPack snapshots.
2. Manually label false-supported rows (`false_supported` in the JSON).
3. Run the calibrator; keep the mechanism (relative score band **or** weak Jaccard). Only open a PR to change defaults if distributions clearly disagree with `0.35` / `0.75` / `0.5`.

## Command

```bash
cd apps/api
go run ./cmd/askdocs-boundary-calibrate \
  -in testdata/ask_docs_eval/boundary_calibrate_sample.json \
  -out testdata/ask_docs_eval/boundary_calibrate_report.md
```

Optional: set `ASK_DOCS_DD_BOUNDARY_*` / `APP_ENV` so the tool uses the same thresholds as the server (`config.AskDocsFromEnv`).

## Input schema

See `coverage.CalibrateInput`:

- `rooms[]`: `room_id`, `rows` (CoverageRow JSON), `query_by_item_id` (required for each supported item)
- `false_supported[]`: `{room_id, item_id, note?}` human mislabel tags

## Report template fields

| Field | Meaning |
| ----- | ------- |
| Rooms / supported counts | Coverage of the offline corpus |
| Defaults under test | Current low/high/Jaccard |
| False caught by defaults | How many labeled misses already enter the boundary band |
| Distributions | min/p50/p90/max/mean for rel_score and Jaccard |
| Suggestion | Keep defaults vs consider a §7.2 PR |

Sample fixture: `boundary_calibrate_sample.json`.

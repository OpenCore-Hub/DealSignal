# Ask Docs boundary calibration (D16)

- Rooms: **3** (design recommends ≥3 financing rooms)
- Supported rows: **4**; human false-supported: **1**
- Defaults under test: score ∈ [0.35, 0.75]×globalMax OR Jaccard < 0.50
- False-supported caught by defaults: **1** / 1

## Distributions

| Cohort | Metric | n | min | p50 | p90 | max | mean |
| ------ | ------ | - | --- | --- | --- | --- | ---- |
| all supported | rel_score | 4 | 0.450 | 1.000 | 1.000 | 1.000 | 0.863 |
| all supported | jaccard | 4 | 0.000 | 0.339 | 0.595 | 0.667 | 0.336 |
| false supported | rel_score | 1 | 0.450 | 0.450 | 0.450 | 0.450 | 0.450 |
| false supported | jaccard | 1 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 |

## Suggestion

Current defaults already flag all labeled false-supported rows as boundary. Keep 0.35 / 0.75 / 0.5 unless precision (over-boundary) is too high on clean supported rows.

# P3.1a real xlsx ingest smoke — CFI Three Statement Model

Date: 2026-07-24  
File: `/Users/mg/Downloads/CFI-Case-Study-Three-Statement-Model.xlsx` (344K)  
Command:

```bash
cd apps/api
APP_ENV=development go run ./cmd/askdocs-table-ingest-smoke \
  -in "/Users/mg/Downloads/CFI-Case-Study-Three-Statement-Model.xlsx"
```

## Result

| Metric | Value |
| ------ | ----- |
| `ASK_DOCS_TABLE_INGEST` (APP_ENV=development) | on |
| Limits | 20 sheets / 5k rows·sheet / 20k rows·file |
| Sheets in workbook | Cover Page, Three Statement Model |
| `table_row` chunks | **87** |
| Soft-limit warnings | **0** |
| Cover Page chunks | 0 (no mappable data rows after trim) |
| Three Statement Model chunks | 87 (of 1000 GetRows data lines; rest empty/whitespace after trim) |

## Samples

- first: sheet=`Three Statement Model` row=1 — year headers / historical vs forecast labels  
- last: sheet=`Three Statement Model` row=115 — Financing Cash Flow line

## Gate read

- Truncation path **not** exercised (file ≪ soft caps) — OK for first real sample.  
- Extract path is healthy: no errors, meta has sheet/row/headers.  
- Full `ProcessDocument` (PDF preview + persist chunks to DB) needs API rebuilt with P3.1a + OnlyOffice; this smoke validates the **native table_row** cut used by ingestion.

## Next

1. Rebuild/restart API with migration `100` + `ASK_DOCS_TABLE_INGEST`.  
2. Upload this xlsx into a deal room and confirm `chunk_type='table_row'` rows in DB + Ask still excludes them.  
3. Add ≥2 more real rooms/files before grilling `struct.tabular`.

-- Sheet → viewer page ranges for XLSX citation deep-links.
-- Built during OnlyOffice per-sheet convert + PDF merge (ingestion).

CREATE TABLE IF NOT EXISTS document_sheet_page_ranges (
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    sheet_name  TEXT NOT NULL,
    page_start  INT  NOT NULL CHECK (page_start > 0),
    page_end    INT  NOT NULL CHECK (page_end >= page_start),
    PRIMARY KEY (document_id, sheet_name)
);

CREATE INDEX IF NOT EXISTS idx_document_sheet_page_ranges_document
    ON document_sheet_page_ranges (document_id);

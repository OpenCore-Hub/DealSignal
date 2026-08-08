-- Persist the document being viewed so bundle / deal-room page views can resolve
-- page titles for key-page heat, compliance, and notifications.
-- Historical rows without lineage keep document_id NULL (honest: no fake join).

ALTER TABLE page_views
    ADD COLUMN IF NOT EXISTS document_id UUID REFERENCES documents(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_page_views_document_id
    ON page_views (document_id)
    WHERE document_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_page_views_link_document_page
    ON page_views (link_id, document_id, page_number)
    WHERE document_id IS NOT NULL;

-- Backfill from primary link document (single-doc shares).
UPDATE page_views pv
SET document_id = l.document_id
FROM links l
WHERE pv.link_id = l.id
  AND pv.document_id IS NULL
  AND l.document_id IS NOT NULL;

-- Prefer reading-session document when the open session already tracked one
-- (covers some multi-doc traffic recorded after reading_sessions landed).
UPDATE page_views pv
SET document_id = rs.document_id
FROM reading_sessions rs
WHERE pv.reading_session_id = rs.id
  AND pv.document_id IS NULL
  AND rs.document_id IS NOT NULL;

-- chunk_boxes: precise bounding boxes for text chunks (design: PAGE_IMAGE_NORMALIZED coordinate space)
CREATE TABLE IF NOT EXISTS chunk_boxes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    page_number INT NOT NULL,
    coordinate_space TEXT NOT NULL DEFAULT 'PAGE_IMAGE_NORMALIZED',
    x DOUBLE PRECISION NOT NULL,
    y DOUBLE PRECISION NOT NULL,
    w DOUBLE PRECISION NOT NULL,
    h DOUBLE PRECISION NOT NULL,
    source TEXT NOT NULL DEFAULT 'PDF_TEXT_LAYER',
    confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chunk_boxes_chunk ON chunk_boxes(chunk_id);
CREATE INDEX IF NOT EXISTS idx_chunk_boxes_doc_page ON chunk_boxes(document_id, page_number);

-- Enhance chunks table with document_id, chunk_index, chunk_type
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS document_id UUID REFERENCES documents(id) ON DELETE CASCADE;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS chunk_index INT DEFAULT 0;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS chunk_type TEXT DEFAULT 'paragraph';

CREATE INDEX IF NOT EXISTS idx_chunks_document ON chunks(document_id);

-- Backfill document_id for existing chunks via pages
UPDATE chunks c
SET document_id = p.document_id
FROM pages p
WHERE c.page_id = p.id AND c.document_id IS NULL;

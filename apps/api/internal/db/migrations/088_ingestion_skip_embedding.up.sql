-- Allow deal-room uploads to queue preview-only ingestion (no auto-embed).
ALTER TABLE ingestion_jobs
  ADD COLUMN IF NOT EXISTS skip_embedding BOOLEAN NOT NULL DEFAULT false;

-- Irreversible data backfill: allow rules may have been created by later
-- CreateLink/UpdateLink sync as well. Down migration is intentionally a no-op.
SELECT 1;

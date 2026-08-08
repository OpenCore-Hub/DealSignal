-- Revert access_logs.event_type to link_opened / download_attempted only.
-- Classification marker rows must be removed first.
-- Parent-only CHECK changes (partitions inherit).

DELETE FROM access_logs
WHERE event_type IN ('forward_signal', 'return_visit');

DO $$
DECLARE
  r RECORD;
  allow_list text :=
    'CHECK (event_type IN ('
    || '''link_opened'','
    || '''download_attempted'''
    || '))';
BEGIN
  FOR r IN
    SELECT c.conrelid::regclass AS tbl, c.conname
    FROM pg_constraint c
    JOIN pg_attribute a
      ON a.attrelid = c.conrelid
     AND a.attnum = ANY (c.conkey)
    WHERE c.contype = 'c'
      AND c.conislocal
      AND a.attname = 'event_type'
      AND (
        c.conrelid = 'access_logs'::regclass
        OR c.conrelid IN (
          SELECT i.inhrelid
          FROM pg_inherits i
          WHERE i.inhparent = 'access_logs'::regclass
        )
      )
  LOOP
    EXECUTE format('ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I', r.tbl, r.conname);
  END LOOP;

  ALTER TABLE access_logs
    DROP CONSTRAINT IF EXISTS access_logs_event_type_check;
  ALTER TABLE access_logs
    DROP CONSTRAINT IF EXISTS access_logs_event_type_check1;

  EXECUTE format(
    'ALTER TABLE access_logs ADD CONSTRAINT access_logs_event_type_check %s',
    allow_list
  );
END $$;

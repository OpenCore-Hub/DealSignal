-- Expand access_logs.event_type to persist DetectForwardOrReturn classifications
-- alongside link_opened (forward_signal / return_visit markers).
--
-- access_logs is RANGE-partitioned: CHECK constraints must be dropped/added on
-- the parent only. Inherited partition constraints (e.g. *_check1 on
-- access_logs_y2026m02) cannot be dropped from the child (SQLSTATE 42P16).

DO $$
DECLARE
  r RECORD;
  allow_list text :=
    'CHECK (event_type IN ('
    || '''link_opened'','
    || '''download_attempted'','
    || '''forward_signal'','
    || '''return_visit'''
    || '))';
BEGIN
  -- Drop any *local* (non-inherited) event_type CHECKs that still omit the new
  -- kinds. Skip inherited partition constraints — those disappear when the
  -- parent constraint is dropped below.
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
      AND pg_get_constraintdef(c.oid) NOT LIKE '%forward_signal%'
  LOOP
    EXECUTE format('ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I', r.tbl, r.conname);
  END LOOP;

  -- Parent: drop current allow-list (also removes inherited copies on partitions).
  ALTER TABLE access_logs
    DROP CONSTRAINT IF EXISTS access_logs_event_type_check;
  ALTER TABLE access_logs
    DROP CONSTRAINT IF EXISTS access_logs_event_type_check1;

  -- Parent only — declarative partitions inherit the new CHECK automatically.
  EXECUTE format(
    'ALTER TABLE access_logs ADD CONSTRAINT access_logs_event_type_check %s',
    allow_list
  );
END $$;

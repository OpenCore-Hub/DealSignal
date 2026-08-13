-- Revert to post-159 allow list (without link_archived / link_renewed).

DO $$
DECLARE
  r RECORD;
  allow_list text :=
    'CHECK (event_type IN ('
    || '''security_gate_failed'','
    || '''expired_link_accessed'','
    || '''max_access_reached'','
    || '''revoked_link_accessed'','
    || '''abnormal_access_pattern'','
    || '''access_rules_updated'','
    || '''invite_token_failed'','
    || '''invite_token_expired'','
    || '''invite_token_revoked'','
    || '''invite_token_redeemed'','
    || '''invalid_password'','
    || '''blocked_email'','
    || '''blocked_domain'','
    || '''allowed_email'','
    || '''allowed_domain'','
    || '''not_in_allow_list'','
    || '''no_allow_match'','
    || '''no_rules'','
    || '''rate_limit_exceeded'','
    || '''scope_violation'','
    || '''ask_ai_rate_limited'','
    || '''ask_escalated'','
    || '''ask_formal_submitted'','
    || '''session_security_config_changed'','
    || '''capture_attempt'''
    || '))';
BEGIN
  FOR r IN
    SELECT c.conrelid::regclass AS tbl, c.conname
    FROM pg_constraint c
    JOIN pg_attribute a
      ON a.attrelid = c.conrelid
     AND a.attnum = ANY (c.conkey)
    WHERE c.contype = 'c'
      AND a.attname = 'event_type'
      AND c.conrelid::regclass::text LIKE 'security_events%'
  LOOP
    EXECUTE format('ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I', r.tbl, r.conname);
  END LOOP;

  ALTER TABLE security_events
    DROP CONSTRAINT IF EXISTS security_events_event_type_check;
  EXECUTE format(
    'ALTER TABLE security_events ADD CONSTRAINT security_events_event_type_check %s',
    allow_list
  );

  FOR r IN
    SELECT c.oid::regclass AS tbl
    FROM pg_class c
    JOIN pg_inherits i ON i.inhrelid = c.oid
    JOIN pg_class p ON p.oid = i.inhparent
    WHERE p.relname = 'security_events'
      AND NOT EXISTS (
        SELECT 1
        FROM pg_constraint con
        JOIN pg_attribute a
          ON a.attrelid = con.conrelid
         AND a.attnum = ANY (con.conkey)
        WHERE con.conrelid = c.oid
          AND con.contype = 'c'
          AND a.attname = 'event_type'
      )
  LOOP
    EXECUTE format(
      'ALTER TABLE %s ADD CONSTRAINT %I %s',
      r.tbl,
      'security_events_event_type_check',
      allow_list
    );
  END LOOP;
END $$;

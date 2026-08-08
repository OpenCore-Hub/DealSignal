-- Speed scheduled Formal Q&A due sweeps (worker + lazy-on-read).

CREATE INDEX IF NOT EXISTS idx_link_ask_turns_formal_due
    ON link_ask_turns (formal_publish_at ASC)
    WHERE formal_status = 'scheduled'
      AND formal_publish_at IS NOT NULL;

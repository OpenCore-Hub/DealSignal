ALTER TABLE workspace_billing
    ADD COLUMN IF NOT EXISTS stripe_customer_id TEXT,
    ADD COLUMN IF NOT EXISTS stripe_subscription_id TEXT,
    ADD COLUMN IF NOT EXISTS stripe_price_id TEXT,
    ADD COLUMN IF NOT EXISTS billing_status TEXT
        CHECK (billing_status IS NULL OR billing_status IN ('active', 'past_due', 'canceled')),
    ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS past_due_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS workspace_billing_stripe_customer_uidx
    ON workspace_billing (stripe_customer_id)
    WHERE stripe_customer_id IS NOT NULL AND stripe_customer_id <> '';

CREATE TABLE IF NOT EXISTS billing_stripe_events (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

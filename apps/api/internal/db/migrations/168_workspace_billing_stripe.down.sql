DROP TABLE IF EXISTS billing_stripe_events;

DROP INDEX IF EXISTS workspace_billing_stripe_customer_uidx;

ALTER TABLE workspace_billing
    DROP COLUMN IF EXISTS past_due_at,
    DROP COLUMN IF EXISTS current_period_end,
    DROP COLUMN IF EXISTS billing_status,
    DROP COLUMN IF EXISTS stripe_price_id,
    DROP COLUMN IF EXISTS stripe_subscription_id,
    DROP COLUMN IF EXISTS stripe_customer_id;

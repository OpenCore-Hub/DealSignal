package config

import (
	"strings"
	"testing"
)

func TestEmailQueueEnabledByDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("REDIS_URL", "localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("URL_SIGNING_SECRET", "test-url-secret")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_ACCESS_KEY", "key")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("APP_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("EMAIL_QUEUE_ENABLED", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.EmailQueueEnabled {
		t.Fatal("expected email queue to be enabled by default")
	}
}

func TestUnpaidPlanChangeRejectedInProductionLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("REDIS_URL", "localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("URL_SIGNING_SECRET", "test-url-secret")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_ACCESS_KEY", "key")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("APP_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("APP_ENV", "production")
	t.Setenv("TURNSTILE_SITE_KEY", "1x00000000000000000000AA")
	t.Setenv("TURNSTILE_SECRET", "1x0000000000000000000000000000000AA")
	t.Setenv("BILLING_ALLOW_UNPAID_PLAN_CHANGE", "true")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "BILLING_ALLOW_UNPAID_PLAN_CHANGE") {
		t.Fatalf("expected production unpaid-plan rejection, got %v", err)
	}
}

func TestUnpaidPlanChangeOffByDefaultInProduction(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("REDIS_URL", "localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("URL_SIGNING_SECRET", "test-url-secret")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_ACCESS_KEY", "key")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("APP_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("APP_ENV", "production")
	t.Setenv("TURNSTILE_SITE_KEY", "1x00000000000000000000AA")
	t.Setenv("TURNSTILE_SECRET", "1x0000000000000000000000000000000AA")
	t.Setenv("BILLING_ALLOW_UNPAID_PLAN_CHANGE", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.AllowUnpaidPlanChange {
		t.Fatal("production must not allow unpaid plan changes")
	}
}

func TestStripeWebhookSecretRequiredInProductionLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("REDIS_URL", "localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("URL_SIGNING_SECRET", "test-url-secret")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_ACCESS_KEY", "key")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("APP_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("APP_ENV", "production")
	t.Setenv("TURNSTILE_SITE_KEY", "1x00000000000000000000AA")
	t.Setenv("TURNSTILE_SECRET", "1x0000000000000000000000000000000AA")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_x")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "STRIPE_WEBHOOK_SECRET") {
		t.Fatalf("expected production stripe webhook rejection, got %v", err)
	}
}

func TestFormalAskEntitlementStubRejectedInProductionLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("REDIS_URL", "localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("URL_SIGNING_SECRET", "test-url-secret")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_ACCESS_KEY", "key")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("APP_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("APP_ENV", "production")
	t.Setenv("TURNSTILE_SITE_KEY", "1x00000000000000000000AA")
	t.Setenv("TURNSTILE_SECRET", "1x0000000000000000000000000000000AA")
	t.Setenv("FORMAL_ASK_ENTITLEMENT_STUB_PLAN", "trial")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "FORMAL_ASK_ENTITLEMENT_STUB_PLAN") {
		t.Fatalf("expected production stub rejection, got %v", err)
	}
}

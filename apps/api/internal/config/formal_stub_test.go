package config

import (
	"strings"
	"testing"
)

func TestFormalAskEntitlementStubRejectedInProductionLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("REDIS_URL", "localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("URL_SIGNING_SECRET", "test-url-secret")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_ACCESS_KEY", "key")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("APP_ENV", "production")
	t.Setenv("FORMAL_ASK_ENTITLEMENT_STUB_PLAN", "trial")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "FORMAL_ASK_ENTITLEMENT_STUB_PLAN") {
		t.Fatalf("expected production stub rejection, got %v", err)
	}
}

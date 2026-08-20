package config

import (
	"strings"
	"testing"
)

func TestLoadRequiresAppBaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("REDIS_URL", "localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("URL_SIGNING_SECRET", "test-url-secret")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_ACCESS_KEY", "key")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("APP_BASE_URL", "")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "APP_BASE_URL") {
		t.Fatalf("expected APP_BASE_URL required, got %v", err)
	}
}

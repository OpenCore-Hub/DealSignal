package config

import (
	"strings"
	"testing"
)

func requiredLoadEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("REDIS_URL", "localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("URL_SIGNING_SECRET", "test-url-secret")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_ACCESS_KEY", "key")
	t.Setenv("S3_SECRET_KEY", "secret")
}

func TestTurnstileOptionalOutsideProduction(t *testing.T) {
	requiredLoadEnv(t)
	t.Setenv("APP_ENV", "development")
	t.Setenv("TURNSTILE_SITE_KEY", "")
	t.Setenv("TURNSTILE_SECRET", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.TurnstileSiteKey != "" || cfg.TurnstileSecret != "" {
		t.Fatal("expected empty turnstile config")
	}
}

func TestTurnstilePairRequired(t *testing.T) {
	requiredLoadEnv(t)
	t.Setenv("APP_ENV", "development")
	t.Setenv("TURNSTILE_SITE_KEY", "1x00000000000000000000AA")
	t.Setenv("TURNSTILE_SECRET", "")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "TURNSTILE_SITE_KEY") {
		t.Fatalf("expected pair rejection, got %v", err)
	}
}

func TestTurnstileRequiredInProduction(t *testing.T) {
	requiredLoadEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("TURNSTILE_SITE_KEY", "")
	t.Setenv("TURNSTILE_SECRET", "")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "TURNSTILE") {
		t.Fatalf("expected production turnstile rejection, got %v", err)
	}
}

func TestTurnstileAcceptedInProduction(t *testing.T) {
	requiredLoadEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("TURNSTILE_SITE_KEY", "1x00000000000000000000AA")
	t.Setenv("TURNSTILE_SECRET", "1x0000000000000000000000000000000AA")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.TurnstileSiteKey == "" || cfg.TurnstileSecret == "" {
		t.Fatal("expected turnstile keys")
	}
}

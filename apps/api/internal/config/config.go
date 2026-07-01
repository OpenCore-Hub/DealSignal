package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	JWTSecret         string
	LinkSessionSecret string
	LogLevel          string
	Version     string

	S3Endpoint       string
	S3PublicEndpoint string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3Region         string
	S3UsePathStyle   string

	OnlyOfficeURL       string
	OnlyOfficeJWTSecret string

	OpenAIAPIKey         string
	OpenAIBaseURL        string
	OpenAIEmbeddingModel string
	OpenAIChatModel      string
	OpenAIReferer        string // optional, e.g. for OpenRouter
	OpenAIAppTitle       string // optional, e.g. for OpenRouter

	BaseDomain          string
	CNAMETarget         string
	CertProvider        string
	AppBaseURL          string
	FrontendURL         string
	ViewerBaseURL       string
	SMTPHost            string
	SMTPPort            string
	SMTPUser            string
	SMTPPass            string
	SMTPFrom            string
	ResendAPIKey        string
	ResendFromEmail     string
	SlackClientID       string
	SlackClientSecret   string
	HubSpotClientID     string
	HubSpotClientSecret string

	RateLimitPublicRPM     int
	RateLimitAuthRPM       int
	RateLimitUploadRPM     int
	RateLimitWorkspaceRPM  int
	IdempotencyTTLHours    int
	IdempotencyMaxBodySize int

	CORSAllowedOrigins string
	MetricsEnabled     bool
	PprofEnabled       bool
}

// Load parses environment variables into Config and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		Version:     getEnv("VERSION", "v2.1.2"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		LinkSessionSecret: os.Getenv("LINK_SESSION_SECRET"),

		S3Endpoint:       os.Getenv("S3_ENDPOINT"),
		S3PublicEndpoint: os.Getenv("S3_PUBLIC_ENDPOINT"),
		S3Bucket:         os.Getenv("S3_BUCKET"),
		S3AccessKey:      os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:      os.Getenv("S3_SECRET_KEY"),
		S3Region:         os.Getenv("S3_REGION"),
		S3UsePathStyle:   os.Getenv("S3_USE_PATH_STYLE"),

		OnlyOfficeURL:       os.Getenv("ONLYOFFICE_URL"),
		OnlyOfficeJWTSecret: os.Getenv("ONLYOFFICE_JWT_SECRET"),

		OpenAIAPIKey:         os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:        os.Getenv("OPENAI_BASE_URL"),
		OpenAIEmbeddingModel: os.Getenv("OPENAI_EMBEDDING_MODEL"),
		OpenAIChatModel:      os.Getenv("OPENAI_CHAT_MODEL"),
		OpenAIReferer:        os.Getenv("OPENAI_REFERER"),
		OpenAIAppTitle:       os.Getenv("OPENAI_APP_TITLE"),

		BaseDomain:          getEnv("BASE_DOMAIN", "dealsignal.com"),
		CNAMETarget:         getEnv("CNAME_TARGET", "cname.dealsignal.com"),
		CertProvider:        getEnv("CERT_PROVIDER", "noop"),
		AppBaseURL:          getEnv("APP_BASE_URL", "http://localhost:8080"),
		FrontendURL:         getEnv("FRONTEND_URL", "http://localhost:5173"),
		ViewerBaseURL:       getEnv("VIEWER_BASE_URL", getEnv("FRONTEND_URL", "http://localhost:5173")),
		SMTPHost:            os.Getenv("SMTP_HOST"),
		SMTPPort:            getEnv("SMTP_PORT", "587"),
		SMTPUser:            os.Getenv("SMTP_USER"),
		SMTPPass:            os.Getenv("SMTP_PASS"),
		SMTPFrom:            os.Getenv("SMTP_FROM"),
		ResendAPIKey:        os.Getenv("RESEND_API_KEY"),
		ResendFromEmail:     getEnv("RESEND_FROM_EMAIL", getEnv("SMTP_FROM", "noreply@dealsignal.com")),
		SlackClientID:       os.Getenv("SLACK_CLIENT_ID"),
		SlackClientSecret:   os.Getenv("SLACK_CLIENT_SECRET"),
		HubSpotClientID:     os.Getenv("HUBSPOT_CLIENT_ID"),
		HubSpotClientSecret: os.Getenv("HUBSPOT_CLIENT_SECRET"),

		RateLimitPublicRPM:     getEnvInt("RATE_LIMIT_PUBLIC_RPM", 100),
		RateLimitAuthRPM:       getEnvInt("RATE_LIMIT_AUTH_RPM", 20),
		RateLimitUploadRPM:     getEnvInt("RATE_LIMIT_UPLOAD_RPM", 10),
		RateLimitWorkspaceRPM:  getEnvInt("RATE_LIMIT_WORKSPACE_RPM", 200),
		IdempotencyTTLHours:    getEnvInt("IDEMPOTENCY_TTL_HOURS", 24),
		IdempotencyMaxBodySize: getEnvInt("IDEMPOTENCY_MAX_BODY_SIZE", 1<<20),

		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
		MetricsEnabled:     strings.ToLower(getEnv("METRICS_ENABLED", "true")) == "true",
		PprofEnabled:       strings.ToLower(getEnv("PPROF_ENABLED", "false")) == "true",
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.LinkSessionSecret == "" {
		cfg.LinkSessionSecret = cfg.JWTSecret
	}
	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required")
	}
	if cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY and S3_SECRET_KEY are required")
	}

	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return nil, fmt.Errorf("PORT must be a valid integer: %w", err)
	}

	return cfg, nil
}

// MustLoad is like Load but exits the process on error.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

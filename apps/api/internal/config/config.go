package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port               string
	AppEnv             string
	DatabaseURL        string
	RedisURL           string
	JWTSecret          string
	IPHashKey          string
	LinkSessionSecret  string
	InviteTokenHashKey string
	LogLevel           string
	Version            string

	S3Endpoint       string
	S3PublicEndpoint string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3Region         string
	S3UsePathStyle   string

	OnlyOfficeURL       string
	OnlyOfficeJWTSecret string

	OpenAIAPIKey            string
	OpenAIBaseURL           string
	OpenAIEmbeddingModel    string
	OpenAIEmbeddingEndpoint string // "embeddings" (default) or "chat_completions"
	OpenAIChatModel         string
	OpenAIReferer           string // optional, e.g. for OpenRouter
	OpenAIAppTitle          string // optional, e.g. for OpenRouter

	BaseDomain             string
	CNAMETarget            string
	CertProvider           string
	AppBaseURL             string
	FrontendURL            string
	ViewerBaseURL          string
	SMTPHost               string
	SMTPPort               string
	SMTPUser               string
	SMTPPass               string
	SMTPFrom               string
	SMTPInsecureSkipVerify bool
	SMTPTimeout            time.Duration
	SMTPMaxRetries         int
	SMTPPoolMaxConns       int
	SMTPPoolIdleTimeout    time.Duration
	SMTPPoolMaxLifetime    time.Duration
	SMTPPoolMaxUses        int
	ResendAPIKey           string
	ResendFromEmail        string
	ResendTimeout          time.Duration
	ResendMaxRetries       int
	ResendWebhookSecret    string

	EmailQueueEnabled         bool
	EmailQueueStream          string
	EmailWorkerCount          int
	EmailWorkerInterval       time.Duration
	EmailQueueMaxAttempts     int
	EmailBatchSize            int
	EmailWorkerBatchSize      int
	EmailTrackingSecret       string
	EmailTrackingTTL          time.Duration
	RetryBackoffBase          time.Duration
	RetryBackoffMax           time.Duration
	DefaultBrandName          string
	VerificationTokenTTLHours int
	SlackClientID             string
	SlackClientSecret         string
	HubSpotClientID           string
	HubSpotClientSecret       string

	RateLimitPublicRPM     int
	RateLimitAuthRPM       int
	RateLimitUploadRPM     int
	RateLimitWorkspaceRPM  int
	IdempotencyTTLHours    int
	IdempotencyMaxBodySize int

	LinkOpenDedupWindow time.Duration
	PageViewDedupWindow time.Duration
	DedupRedisEnabled   bool

	URLSigningSecret string

	SecurityAnomalyWindow    time.Duration
	SecurityAnomalyThreshold int

	AccessLogsRetentionDays     int
	PageViewsRetentionDays      int
	SecurityEventsRetentionDays int

	SignalRulesPath string

	FeatureWorkerEnabled  bool
	FeatureWorkerInterval time.Duration

	EventsEnabled       bool
	EventsStreamName    string
	EventsConsumerGroup string

	CORSAllowedOrigins string
	MetricsEnabled     bool
	PprofEnabled       bool
}

// Load parses environment variables into Config and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		AppEnv:             getEnv("APP_ENV", "development"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		Version:            getEnv("VERSION", "v2.5.0"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		RedisURL:           os.Getenv("REDIS_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		IPHashKey:          os.Getenv("IP_HASH_KEY"),
		LinkSessionSecret:  os.Getenv("LINK_SESSION_SECRET"),
		InviteTokenHashKey: os.Getenv("INVITE_TOKEN_HASH_KEY"),

		S3Endpoint:       os.Getenv("S3_ENDPOINT"),
		S3PublicEndpoint: os.Getenv("S3_PUBLIC_ENDPOINT"),
		S3Bucket:         os.Getenv("S3_BUCKET"),
		S3AccessKey:      os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:      os.Getenv("S3_SECRET_KEY"),
		S3Region:         os.Getenv("S3_REGION"),
		S3UsePathStyle:   os.Getenv("S3_USE_PATH_STYLE"),

		OnlyOfficeURL:       os.Getenv("ONLYOFFICE_URL"),
		OnlyOfficeJWTSecret: os.Getenv("ONLYOFFICE_JWT_SECRET"),

		OpenAIAPIKey:            os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:           os.Getenv("OPENAI_BASE_URL"),
		OpenAIEmbeddingModel:    os.Getenv("OPENAI_EMBEDDING_MODEL"),
		OpenAIEmbeddingEndpoint: os.Getenv("OPENAI_EMBEDDING_ENDPOINT"),
		OpenAIChatModel:         os.Getenv("OPENAI_CHAT_MODEL"),
		OpenAIReferer:           os.Getenv("OPENAI_REFERER"),
		OpenAIAppTitle:          os.Getenv("OPENAI_APP_TITLE"),

		BaseDomain:             getEnv("BASE_DOMAIN", "dealsignal.com"),
		CNAMETarget:            getEnv("CNAME_TARGET", "cname.dealsignal.com"),
		CertProvider:           getEnv("CERT_PROVIDER", "noop"),
		AppBaseURL:             getEnv("APP_BASE_URL", "http://localhost:8080"),
		FrontendURL:            getEnv("FRONTEND_URL", "http://localhost:5173"),
		ViewerBaseURL:          getEnv("VIEWER_BASE_URL", getEnv("FRONTEND_URL", "http://localhost:5173")),
		SMTPHost:               os.Getenv("SMTP_HOST"),
		SMTPPort:               getEnv("SMTP_PORT", "587"),
		SMTPUser:               os.Getenv("SMTP_USER"),
		SMTPPass:               os.Getenv("SMTP_PASS"),
		SMTPFrom:               os.Getenv("SMTP_FROM"),
		SMTPInsecureSkipVerify: strings.ToLower(getEnv("SMTP_INSECURE_SKIP_VERIFY", "false")) == "true",
		SMTPTimeout:            time.Duration(getEnvInt("SMTP_TIMEOUT_SECONDS", 10)) * time.Second,
		SMTPMaxRetries:         getEnvInt("SMTP_MAX_RETRIES", 3),
		SMTPPoolMaxConns:       getEnvInt("SMTP_POOL_MAX_CONNS", 10),
		SMTPPoolIdleTimeout:    time.Duration(getEnvInt("SMTP_POOL_IDLE_TIMEOUT_SECONDS", 60)) * time.Second,
		SMTPPoolMaxLifetime:    time.Duration(getEnvInt("SMTP_POOL_MAX_LIFETIME_SECONDS", 300)) * time.Second,
		SMTPPoolMaxUses:        getEnvInt("SMTP_POOL_MAX_USES", 100),
		ResendAPIKey:           os.Getenv("RESEND_API_KEY"),
		ResendFromEmail:        getEnv("RESEND_FROM_EMAIL", getEnv("SMTP_FROM", "noreply@dealsignal.com")),
		ResendTimeout:          time.Duration(getEnvInt("RESEND_TIMEOUT_SECONDS", 10)) * time.Second,
		ResendMaxRetries:       getEnvInt("RESEND_MAX_RETRIES", 3),
		ResendWebhookSecret:    os.Getenv("RESEND_WEBHOOK_SECRET"),

		EmailQueueEnabled:         strings.ToLower(getEnv("EMAIL_QUEUE_ENABLED", "false")) == "true",
		EmailQueueStream:          getEnv("EMAIL_QUEUE_STREAM", "mail:queue"),
		EmailWorkerCount:          getEnvInt("EMAIL_WORKER_COUNT", 2),
		EmailWorkerInterval:       time.Duration(getEnvInt("EMAIL_WORKER_INTERVAL_MS", 1000)) * time.Millisecond,
		EmailQueueMaxAttempts:     getEnvInt("EMAIL_QUEUE_MAX_ATTEMPTS", 3),
		EmailBatchSize:            getEnvInt("EMAIL_BATCH_SIZE", 100),
		EmailWorkerBatchSize:      getEnvInt("EMAIL_WORKER_BATCH_SIZE", 10),
		EmailTrackingSecret:       getEnv("EMAIL_TRACKING_SECRET", ""),
		EmailTrackingTTL:          time.Duration(getEnvInt("EMAIL_TRACKING_TTL_HOURS", 168)) * time.Hour,
		RetryBackoffBase:          time.Duration(getEnvInt("EMAIL_RETRY_BACKOFF_BASE_SECONDS", 5)) * time.Second,
		RetryBackoffMax:           time.Duration(getEnvInt("EMAIL_RETRY_BACKOFF_MAX_SECONDS", 3600)) * time.Second,
		DefaultBrandName:          getEnv("DEFAULT_BRAND_NAME", "DealSignal"),
		VerificationTokenTTLHours: getEnvInt("VERIFICATION_TOKEN_TTL_HOURS", 24),
		SlackClientID:             os.Getenv("SLACK_CLIENT_ID"),
		SlackClientSecret:         os.Getenv("SLACK_CLIENT_SECRET"),
		HubSpotClientID:           os.Getenv("HUBSPOT_CLIENT_ID"),
		HubSpotClientSecret:       os.Getenv("HUBSPOT_CLIENT_SECRET"),

		RateLimitPublicRPM:     getEnvInt("RATE_LIMIT_PUBLIC_RPM", 100),
		RateLimitAuthRPM:       getEnvInt("RATE_LIMIT_AUTH_RPM", 20),
		RateLimitUploadRPM:     getEnvInt("RATE_LIMIT_UPLOAD_RPM", 10),
		RateLimitWorkspaceRPM:  getEnvInt("RATE_LIMIT_WORKSPACE_RPM", 200),
		IdempotencyTTLHours:    getEnvInt("IDEMPOTENCY_TTL_HOURS", 24),
		IdempotencyMaxBodySize: getEnvInt("IDEMPOTENCY_MAX_BODY_SIZE", 1<<20),

		LinkOpenDedupWindow: time.Duration(getEnvInt("LINK_OPEN_DEDUP_WINDOW_MINUTES", 30)) * time.Minute,
		PageViewDedupWindow: time.Duration(getEnvInt("PAGE_VIEW_DEDUP_WINDOW_MINUTES", 5)) * time.Minute,
		DedupRedisEnabled:   strings.ToLower(getEnv("DEDUP_REDIS_ENABLED", "true")) == "true",

		URLSigningSecret: getEnv("URL_SIGNING_SECRET", ""),

		SecurityAnomalyWindow:    time.Duration(getEnvInt("SECURITY_ANOMALY_WINDOW_MINUTES", 5)) * time.Minute,
		SecurityAnomalyThreshold: getEnvInt("SECURITY_ANOMALY_THRESHOLD", 5),

		SignalRulesPath: getEnv("SIGNAL_RULES_PATH", "config/signal_rules.yaml"),

		FeatureWorkerEnabled:  strings.ToLower(getEnv("FEATURE_WORKER_ENABLED", "true")) == "true",
		FeatureWorkerInterval: time.Duration(getEnvInt("FEATURE_WORKER_INTERVAL_MINUTES", 5)) * time.Minute,

		EventsEnabled:       strings.ToLower(getEnv("EVENTS_ENABLED", "true")) == "true",
		EventsStreamName:    getEnv("EVENTS_STREAM_NAME", "events:signal"),
		EventsConsumerGroup: getEnv("EVENTS_CONSUMER_GROUP", "signal-sync"),

		AccessLogsRetentionDays:     getEnvInt("ACCESS_LOGS_RETENTION_DAYS", 90),
		PageViewsRetentionDays:      getEnvInt("PAGE_VIEWS_RETENTION_DAYS", 90),
		SecurityEventsRetentionDays: getEnvInt("SECURITY_EVENTS_RETENTION_DAYS", 180),

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
	if cfg.URLSigningSecret == "" {
		return nil, fmt.Errorf("URL_SIGNING_SECRET is required")
	}
	if cfg.LinkSessionSecret == "" {
		cfg.LinkSessionSecret = cfg.JWTSecret
	}
	if cfg.IPHashKey == "" {
		cfg.IPHashKey = cfg.JWTSecret
		fmt.Fprintf(os.Stderr, "warning: IP_HASH_KEY is not set; falling back to JWT_SECRET. Set IP_HASH_KEY explicitly in production.\n")
	}
	if cfg.InviteTokenHashKey == "" {
		cfg.InviteTokenHashKey = cfg.JWTSecret
		fmt.Fprintf(os.Stderr, "warning: INVITE_TOKEN_HASH_KEY is not set; falling back to JWT_SECRET. Set INVITE_TOKEN_HASH_KEY explicitly in production.\n")
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

	if cfg.SMTPInsecureSkipVerify && strings.ToLower(cfg.AppEnv) == "production" {
		return nil, fmt.Errorf("SMTP_INSECURE_SKIP_VERIFY must not be enabled in production")
	}
	if cfg.EmailQueueEnabled {
		if cfg.RedisURL == "" {
			return nil, fmt.Errorf("EMAIL_QUEUE_ENABLED requires REDIS_URL")
		}
		if cfg.EmailQueueStream == "" {
			return nil, fmt.Errorf("EMAIL_QUEUE_STREAM must not be empty")
		}
		if cfg.EmailWorkerCount <= 0 {
			return nil, fmt.Errorf("EMAIL_WORKER_COUNT must be positive")
		}
		if cfg.EmailWorkerInterval <= 0 {
			return nil, fmt.Errorf("EMAIL_WORKER_INTERVAL_MS must be positive")
		}
		if cfg.EmailQueueMaxAttempts <= 0 {
			return nil, fmt.Errorf("EMAIL_QUEUE_MAX_ATTEMPTS must be positive")
		}
		if cfg.EmailBatchSize <= 0 {
			return nil, fmt.Errorf("EMAIL_BATCH_SIZE must be positive")
		}
		if cfg.EmailWorkerBatchSize <= 0 {
			return nil, fmt.Errorf("EMAIL_WORKER_BATCH_SIZE must be positive")
		}
	}
	if cfg.SMTPHost != "" {
		if cfg.SMTPTimeout <= 0 {
			return nil, fmt.Errorf("SMTP_TIMEOUT_SECONDS must be positive")
		}
		if cfg.SMTPMaxRetries < 0 {
			return nil, fmt.Errorf("SMTP_MAX_RETRIES must be non-negative")
		}
		if cfg.SMTPPoolMaxConns <= 0 {
			return nil, fmt.Errorf("SMTP_POOL_MAX_CONNS must be positive")
		}
		if cfg.SMTPPoolIdleTimeout <= 0 {
			return nil, fmt.Errorf("SMTP_POOL_IDLE_TIMEOUT_SECONDS must be positive")
		}
		if cfg.SMTPPoolMaxLifetime <= 0 {
			return nil, fmt.Errorf("SMTP_POOL_MAX_LIFETIME_SECONDS must be positive")
		}
		if cfg.SMTPPoolMaxUses <= 0 {
			return nil, fmt.Errorf("SMTP_POOL_MAX_USES must be positive")
		}
	}
	if cfg.ResendAPIKey != "" {
		if cfg.ResendTimeout <= 0 {
			return nil, fmt.Errorf("RESEND_TIMEOUT_SECONDS must be positive")
		}
		if cfg.ResendMaxRetries < 0 {
			return nil, fmt.Errorf("RESEND_MAX_RETRIES must be non-negative")
		}
		if strings.ToLower(cfg.AppEnv) == "production" && cfg.ResendWebhookSecret == "" {
			return nil, fmt.Errorf("RESEND_WEBHOOK_SECRET is required in production when RESEND_API_KEY is set")
		}
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

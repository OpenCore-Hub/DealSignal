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

	S3Endpoint     string
	S3Bucket       string
	S3AccessKey    string
	S3SecretKey    string
	S3Region       string
	S3UsePathStyle string

	OnlyOfficeURL       string
	OnlyOfficeJWTSecret string

	OpenAIAPIKey    string
	OpenAIBaseURL   string
	OpenAIChatModel string

	BaseDomain   string
	CNAMETarget  string
	CertProvider string
	// AppBaseURL is the public API origin (emails, OAuth, HMAC file proxy). Required.
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

	RateLimitPublicRPM      int
	RateLimitAuthRPM        int
	RateLimitRegisterLimit  int
	RateLimitRegisterWindow time.Duration
	RateLimitResendLimit    int
	RateLimitResendWindow   time.Duration
	RateLimitUploadRPM      int
	RateLimitWorkspaceRPM   int
	IdempotencyTTLHours     int
	IdempotencyMaxBodySize  int

	LinkOpenDedupWindow time.Duration
	PageViewDedupWindow time.Duration
	DedupRedisEnabled   bool

	URLSigningSecret string

	SecurityAnomalyWindow    time.Duration
	SecurityAnomalyThreshold int

	AccessLogsRetentionDays     int
	PageViewsRetentionDays      int
	SecurityEventsRetentionDays int
	// KnowledgeQARetentionDays is hot-data retention for knowledge_qa_sessions (0 disables).
	KnowledgeQARetentionDays int
	// KnowledgeQAMemberRPM caps session asks per room member per minute (0 disables RPM; single-flight remains).
	KnowledgeQAMemberRPM int
	// KnowledgeQAFollowUpRPM caps follow-up chip generations per room member per minute (0 disables RPM).
	KnowledgeQAFollowUpRPM int
	// KnowledgeQARewriteEnabled toggles elliptical retrieve-query rewrite (default on).
	// Independent of follow-up chip LLM — set false to kill rewrite without killing chips.
	KnowledgeQARewriteEnabled bool
	// KnowledgeQARewriteCacheEnabled toggles provenanced rewrite cache after grounding (default on).
	KnowledgeQARewriteCacheEnabled bool
	// KnowledgeQATableLaneEnabled merges local table_row chunks into Knowledge Query (default on).
	// Requires TABLE_INGEST_* to have produced rows for room spreadsheets.
	KnowledgeQATableLaneEnabled bool
	// KnowledgeQAMultiHopEnabled runs deterministic second-hop retrieve on the session path (default on).
	KnowledgeQAMultiHopEnabled bool

	// VisitorAskUnifiedEnabled gates the unified visitor Ask UI (Phase A rollout).
	// Set VISITOR_ASK_UNIFIED=1 to enable; API dual-write remains active regardless.
	VisitorAskUnifiedEnabled bool
	// Visitor Ask monthly caps come from workspace plan_code
	// (see plan.Limits.VisitorAskAIMonthly). Per-link links.ask_ai_monthly_quota
	// remains an optional tighter cap.
	// VisitorAskAIRPM caps AI lane requests per visitor+link per minute (abuse guard).
	VisitorAskAIRPM int
	// VisitorAskAIDailyLimit caps AI lane requests per visitor+link per day.
	VisitorAskAIDailyLimit int
	// VisitorAskFormalDailyLimit caps Formal-mode asks per visitor+link per day (stricter than host).
	VisitorAskFormalDailyLimit int

	SignalRulesPath string

	FeatureWorkerEnabled  bool
	FeatureWorkerInterval time.Duration

	HeatScoreRefreshInterval time.Duration

	// InsightsDigestHourUTC is the earliest UTC hour when daily digests may enqueue (0–23, default 8).
	InsightsDigestHourUTC int
	// InsightsDigestInterval is how often the digest scheduler ticks (default 15m).
	InsightsDigestInterval time.Duration

	// FormalPublishInterval is how often the Formal Q&A due-sweep worker runs (0 → 15s).
	FormalPublishInterval time.Duration
	// FormalPublishBatchSize caps turns published per worker tick (0 → 50).
	FormalPublishBatchSize int
	// FileRequestPendingTTL is how long a pending visitor upload may occupy
	// object storage before the expire worker deletes it (0 → 168h).
	FileRequestPendingTTL time.Duration
	// FileRequestExpireInterval is how often the pending-upload expire worker ticks (0 → 15m).
	FileRequestExpireInterval time.Duration
	// FileRequestExpireBatchSize caps pending uploads expired per worker tick (0 → 100).
	FileRequestExpireBatchSize int
	// FormalAskEntitledPlanCodes controls which control-plane plan codes may use Formal Q&A.
	FormalAskEntitledPlanCodes []string
	// FormalAskEntitlementStubPlan is a non-production escape hatch when docling-rag
	// is unset or unreachable (CI / local). Must be empty in production.
	FormalAskEntitlementStubPlan string
	// AllowUnpaidPlanChange lets PUT /billing/plan persist pro/business without
	// checkout. Forced false in production (Load rejects an explicit true).
	AllowUnpaidPlanChange bool
	// StripeSecretKey is the Stripe API secret (sk_...). Empty disables checkout.
	StripeSecretKey string
	// StripeWebhookSecret verifies POST /stripe/webhook. Required in production
	// when StripeSecretKey is set.
	StripeWebhookSecret string
	// StripeAPIBase overrides the Stripe API host (tests / proxies).
	StripeAPIBase              string
	StripePriceProMonthly      string
	StripePriceProYearly       string
	StripePriceBusinessMonthly string
	StripePriceBusinessYearly  string

	EventsEnabled       bool
	EventsStreamName    string
	EventsConsumerGroup string

	// TurnstileSiteKey is the public widget key. Empty disables the widget
	// (local / e2e). Must be set together with TurnstileSecret.
	TurnstileSiteKey string
	// TurnstileSecret is the siteverify secret. Never expose to clients.
	TurnstileSecret string
	// TurnstileTimeout is the siteverify HTTP timeout (default 5s).
	TurnstileTimeout time.Duration

	CORSAllowedOrigins string
	MetricsEnabled     bool
	PprofEnabled       bool

	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration

	// TableIngest holds TABLE_INGEST_* spreadsheet chunking flags.
	TableIngest TableIngestConfig

	// DoclingRAG holds DOCLING_RAG_* external knowledge-base settings.
	DoclingRAG DoclingRAGConfig
}

// Production rate-limit defaults. Sized for global SaaS: corporate NAT/CGNAT,
// multi-tab dashboards, batch diligence uploads, and stream reconnects.
// These are abuse guards, not billing quotas.
const (
	DefaultRateLimitPublicRPM             = 600
	DefaultRateLimitAuthRPM               = 120
	DefaultRateLimitRegisterLimit         = 10
	DefaultRateLimitRegisterWindowMinutes = 15
	DefaultRateLimitResendLimit           = 8
	DefaultRateLimitResendWindowMinutes   = 15
	DefaultRateLimitUploadRPM             = 240
	DefaultRateLimitWorkspaceRPM          = 600
	DefaultKnowledgeQAMemberRPM           = 60
	DefaultKnowledgeQAFollowUpRPM         = 80
	DefaultVisitorAskAIRPM                = 30
	DefaultVisitorAskAIDailyLimit         = 150
	DefaultVisitorAskFormalDailyLimit     = 50
)

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

		S3Endpoint:     os.Getenv("S3_ENDPOINT"),
		S3Bucket:       os.Getenv("S3_BUCKET"),
		S3AccessKey:    os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:    os.Getenv("S3_SECRET_KEY"),
		S3Region:       os.Getenv("S3_REGION"),
		S3UsePathStyle: os.Getenv("S3_USE_PATH_STYLE"),

		OnlyOfficeURL:       os.Getenv("ONLYOFFICE_URL"),
		OnlyOfficeJWTSecret: os.Getenv("ONLYOFFICE_JWT_SECRET"),

		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:   os.Getenv("OPENAI_BASE_URL"),
		OpenAIChatModel: os.Getenv("OPENAI_CHAT_MODEL"),

		BaseDomain:             getEnv("BASE_DOMAIN", "dealsignal.com"),
		CNAMETarget:            getEnv("CNAME_TARGET", "cname.dealsignal.com"),
		CertProvider:           getEnv("CERT_PROVIDER", "noop"),
		AppBaseURL:             strings.TrimSpace(os.Getenv("APP_BASE_URL")),
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

		EmailQueueEnabled:         strings.ToLower(getEnv("EMAIL_QUEUE_ENABLED", "true")) == "true",
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

		RateLimitPublicRPM: getEnvInt("RATE_LIMIT_PUBLIC_RPM", DefaultRateLimitPublicRPM),
		RateLimitAuthRPM:   getEnvInt("RATE_LIMIT_AUTH_RPM", DefaultRateLimitAuthRPM),
		// Register is tighter than login/refresh so one IP cannot mint Trial accounts
		// in a burst. Window is minutes; 0/negative falls back to 15.
		RateLimitRegisterLimit:  getEnvInt("RATE_LIMIT_REGISTER_LIMIT", DefaultRateLimitRegisterLimit),
		RateLimitRegisterWindow: time.Duration(getEnvInt("RATE_LIMIT_REGISTER_WINDOW_MINUTES", DefaultRateLimitRegisterWindowMinutes)) * time.Minute,
		RateLimitResendLimit:    getEnvInt("RATE_LIMIT_RESEND_LIMIT", DefaultRateLimitResendLimit),
		RateLimitResendWindow:   time.Duration(getEnvInt("RATE_LIMIT_RESEND_WINDOW_MINUTES", DefaultRateLimitResendWindowMinutes)) * time.Minute,
		// Batch deal-room folder uploads issue one create per file; 60/min still
		// clips a 100-file diligence pack. Default 240/min.
		RateLimitUploadRPM:     getEnvInt("RATE_LIMIT_UPLOAD_RPM", DefaultRateLimitUploadRPM),
		RateLimitWorkspaceRPM:  getEnvInt("RATE_LIMIT_WORKSPACE_RPM", DefaultRateLimitWorkspaceRPM),
		IdempotencyTTLHours:    getEnvInt("IDEMPOTENCY_TTL_HOURS", 24),
		IdempotencyMaxBodySize: getEnvInt("IDEMPOTENCY_MAX_BODY_SIZE", 1<<20),

		LinkOpenDedupWindow: time.Duration(getEnvInt("LINK_OPEN_DEDUP_WINDOW_MINUTES", 30)) * time.Minute,
		PageViewDedupWindow: time.Duration(getEnvInt("PAGE_VIEW_DEDUP_WINDOW_MINUTES", 5)) * time.Minute,
		DedupRedisEnabled:   strings.ToLower(getEnv("DEDUP_REDIS_ENABLED", "true")) == "true",

		URLSigningSecret: getEnv("URL_SIGNING_SECRET", ""),

		SecurityAnomalyWindow:    time.Duration(getEnvInt("SECURITY_ANOMALY_WINDOW_MINUTES", 5)) * time.Minute,
		SecurityAnomalyThreshold: getEnvInt("SECURITY_ANOMALY_THRESHOLD", 5),

		SignalRulesPath: getEnv("SIGNAL_RULES_PATH", "config/signal_rules.yaml"),

		FeatureWorkerEnabled:         strings.ToLower(getEnv("FEATURE_WORKER_ENABLED", "true")) == "true",
		FeatureWorkerInterval:        time.Duration(getEnvInt("FEATURE_WORKER_INTERVAL_MINUTES", 5)) * time.Minute,
		HeatScoreRefreshInterval:     time.Duration(getEnvInt("HEAT_SCORE_REFRESH_INTERVAL_SECONDS", 120)) * time.Second,
		InsightsDigestHourUTC:        getEnvInt("INSIGHTS_DIGEST_HOUR_UTC", 8),
		InsightsDigestInterval:       time.Duration(getEnvInt("INSIGHTS_DIGEST_INTERVAL_MINUTES", 15)) * time.Minute,
		FormalPublishInterval:        time.Duration(getEnvInt("FORMAL_PUBLISH_INTERVAL_SECONDS", 15)) * time.Second,
		FormalPublishBatchSize:       getEnvInt("FORMAL_PUBLISH_BATCH_SIZE", 50),
		FileRequestPendingTTL:        time.Duration(getEnvInt("FILE_REQUEST_PENDING_TTL_HOURS", 168)) * time.Hour,
		FileRequestExpireInterval:    time.Duration(getEnvInt("FILE_REQUEST_EXPIRE_INTERVAL_MINUTES", 15)) * time.Minute,
		FileRequestExpireBatchSize:   getEnvInt("FILE_REQUEST_EXPIRE_BATCH_SIZE", 100),
		FormalAskEntitledPlanCodes:   parseDelimitedList(getEnv("FORMAL_ASK_ENTITLED_PLAN_CODES", "enterprise trial")),
		FormalAskEntitlementStubPlan: strings.TrimSpace(getEnv("FORMAL_ASK_ENTITLEMENT_STUB_PLAN", "")),

		StripeSecretKey:            strings.TrimSpace(getEnv("STRIPE_SECRET_KEY", "")),
		StripeWebhookSecret:        strings.TrimSpace(getEnv("STRIPE_WEBHOOK_SECRET", "")),
		StripeAPIBase:              strings.TrimSpace(getEnv("STRIPE_API_BASE", "")),
		StripePriceProMonthly:      strings.TrimSpace(getEnv("STRIPE_PRICE_PRO_MONTHLY", "")),
		StripePriceProYearly:       strings.TrimSpace(getEnv("STRIPE_PRICE_PRO_YEARLY", "")),
		StripePriceBusinessMonthly: strings.TrimSpace(getEnv("STRIPE_PRICE_BUSINESS_MONTHLY", "")),
		StripePriceBusinessYearly:  strings.TrimSpace(getEnv("STRIPE_PRICE_BUSINESS_YEARLY", "")),

		EventsEnabled:       strings.ToLower(getEnv("EVENTS_ENABLED", "true")) == "true",
		EventsStreamName:    getEnv("EVENTS_STREAM_NAME", "events:signal"),
		EventsConsumerGroup: getEnv("EVENTS_CONSUMER_GROUP", "signal-sync"),

		AccessLogsRetentionDays:        getEnvInt("ACCESS_LOGS_RETENTION_DAYS", 90),
		PageViewsRetentionDays:         getEnvInt("PAGE_VIEWS_RETENTION_DAYS", 90),
		SecurityEventsRetentionDays:    getEnvInt("SECURITY_EVENTS_RETENTION_DAYS", 180),
		KnowledgeQARetentionDays:       getEnvInt("KNOWLEDGE_QA_RETENTION_DAYS", 90),
		KnowledgeQAMemberRPM:           getEnvInt("KNOWLEDGE_QA_MEMBER_RPM", DefaultKnowledgeQAMemberRPM),
		KnowledgeQAFollowUpRPM:         getEnvInt("KNOWLEDGE_QA_FOLLOWUP_RPM", DefaultKnowledgeQAFollowUpRPM),
		KnowledgeQARewriteEnabled:      strings.ToLower(getEnv("KNOWLEDGE_QA_REWRITE_ENABLED", "true")) == "true",
		KnowledgeQARewriteCacheEnabled: strings.ToLower(getEnv("KNOWLEDGE_QA_REWRITE_CACHE_ENABLED", "true")) == "true",
		KnowledgeQATableLaneEnabled:    strings.ToLower(getEnv("KNOWLEDGE_QA_TABLE_LANE_ENABLED", "true")) == "true",
		KnowledgeQAMultiHopEnabled:     strings.ToLower(getEnv("KNOWLEDGE_QA_MULTI_HOP_ENABLED", "true")) == "true",
		VisitorAskUnifiedEnabled:       visitorAskUnifiedEnabledFromEnv(),
		VisitorAskAIRPM:                getEnvInt("VISITOR_ASK_AI_RPM", DefaultVisitorAskAIRPM),
		VisitorAskAIDailyLimit:         getEnvInt("VISITOR_ASK_AI_DAILY_LIMIT", DefaultVisitorAskAIDailyLimit),
		VisitorAskFormalDailyLimit:     getEnvInt("VISITOR_ASK_FORMAL_DAILY_LIMIT", DefaultVisitorAskFormalDailyLimit),

		TurnstileSiteKey: strings.TrimSpace(os.Getenv("TURNSTILE_SITE_KEY")),
		TurnstileSecret:  strings.TrimSpace(os.Getenv("TURNSTILE_SECRET")),
		TurnstileTimeout: time.Duration(getEnvInt("TURNSTILE_TIMEOUT_SECONDS", 5)) * time.Second,

		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
		MetricsEnabled:     strings.ToLower(getEnv("METRICS_ENABLED", "true")) == "true",
		PprofEnabled:       strings.ToLower(getEnv("PPROF_ENABLED", "false")) == "true",
	}
	cfg.TableIngest = TableIngestFromEnv(cfg.AppEnv)
	cfg.DoclingRAG = DoclingRAGFromEnv()
	cfg.HTTPReadTimeout = time.Duration(getEnvInt("HTTP_READ_TIMEOUT_SECONDS", 30)) * time.Second
	writeOverrideSec := getEnvInt("HTTP_WRITE_TIMEOUT_SECONDS", 0)
	minWrite := LongRunningWriteDeadline(cfg.DoclingRAG.HTTPTimeout)
	if writeOverrideSec > 0 && time.Duration(writeOverrideSec)*time.Second < minWrite {
		fmt.Fprintf(os.Stderr,
			"warning: HTTP_WRITE_TIMEOUT_SECONDS=%d is below the knowledge minimum %s; clamping to minimum\n",
			writeOverrideSec, minWrite,
		)
	}
	cfg.HTTPWriteTimeout = resolveHTTPWriteTimeout(cfg.DoclingRAG, writeOverrideSec)

	if cfg.AppBaseURL == "" {
		return nil, fmt.Errorf("APP_BASE_URL is required")
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
	if cfg.FormalAskEntitlementStubPlan != "" && strings.ToLower(cfg.AppEnv) == "production" {
		return nil, fmt.Errorf("FORMAL_ASK_ENTITLEMENT_STUB_PLAN must be empty in production")
	}
	unpaidFlag := strings.TrimSpace(os.Getenv("BILLING_ALLOW_UNPAID_PLAN_CHANGE"))
	if strings.ToLower(cfg.AppEnv) == "production" {
		if unpaidFlag == "1" || strings.EqualFold(unpaidFlag, "true") {
			return nil, fmt.Errorf("BILLING_ALLOW_UNPAID_PLAN_CHANGE must not be enabled in production")
		}
		cfg.AllowUnpaidPlanChange = false
	} else if unpaidFlag == "" {
		cfg.AllowUnpaidPlanChange = true
	} else {
		cfg.AllowUnpaidPlanChange = unpaidFlag == "1" || strings.EqualFold(unpaidFlag, "true")
	}
	if strings.ToLower(cfg.AppEnv) == "production" && cfg.StripeSecretKey != "" && cfg.StripeWebhookSecret == "" {
		return nil, fmt.Errorf("STRIPE_WEBHOOK_SECRET is required when STRIPE_SECRET_KEY is set in production")
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

	if cfg.TurnstileTimeout <= 0 {
		cfg.TurnstileTimeout = 5 * time.Second
	}
	siteSet := cfg.TurnstileSiteKey != ""
	secretSet := cfg.TurnstileSecret != ""
	if siteSet != secretSet {
		return nil, fmt.Errorf("TURNSTILE_SITE_KEY and TURNSTILE_SECRET must both be set or both empty")
	}
	if strings.EqualFold(cfg.AppEnv, "production") && !secretSet {
		return nil, fmt.Errorf("TURNSTILE_SITE_KEY and TURNSTILE_SECRET are required in production")
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

func visitorAskUnifiedEnabledFromEnv() bool {
	v := strings.TrimSpace(os.Getenv("VISITOR_ASK_UNIFIED"))
	return v == "1" || strings.EqualFold(v, "true")
}

// parseDelimitedList splits on commas and/or whitespace and lowercases tokens.
func parseDelimitedList(raw string) []string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return nil
	}
	normalized = strings.ReplaceAll(normalized, ",", " ")
	return strings.Fields(normalized)
}

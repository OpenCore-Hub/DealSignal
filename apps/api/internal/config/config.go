package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
	LogLevel    string
	Version     string

	S3Endpoint       string
	S3PublicEndpoint string
	S3Bucket         string
	S3AccessKey    string
	S3SecretKey    string
	S3Region       string
	S3UsePathStyle string

	OnlyOfficeURL         string
	OnlyOfficeJWTSecret   string

	OpenAIAPIKey         string
	OpenAIBaseURL        string
	OpenAIEmbeddingModel string
	OpenAIChatModel      string
	OpenAIReferer        string // optional, e.g. for OpenRouter
	OpenAIAppTitle       string // optional, e.g. for OpenRouter

	BaseDomain   string
	CNAMETarget  string
	CertProvider string
	AppBaseURL            string
	SMTPHost              string
	SMTPPort              string
	SMTPUser              string
	SMTPPass              string
	SMTPFrom              string
	SlackClientID         string
	SlackClientSecret     string
	HubSpotClientID       string
	HubSpotClientSecret   string
}

// Load parses environment variables into Config and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Port:     getEnv("PORT", "8080"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
		Version:  getEnv("VERSION", "v2.1.2"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),

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

		BaseDomain:   getEnv("BASE_DOMAIN", "dealsignal.com"),
		CNAMETarget:  getEnv("CNAME_TARGET", "cname.dealsignal.com"),
		CertProvider: getEnv("CERT_PROVIDER", "noop"),
		AppBaseURL:          getEnv("APP_BASE_URL", "http://localhost:8080"),
		SMTPHost:            os.Getenv("SMTP_HOST"),
		SMTPPort:            getEnv("SMTP_PORT", "587"),
		SMTPUser:            os.Getenv("SMTP_USER"),
		SMTPPass:            os.Getenv("SMTP_PASS"),
		SMTPFrom:            os.Getenv("SMTP_FROM"),
		SlackClientID:       os.Getenv("SLACK_CLIENT_ID"),
		SlackClientSecret:   os.Getenv("SLACK_CLIENT_SECRET"),
		HubSpotClientID:     os.Getenv("HUBSPOT_CLIENT_ID"),
		HubSpotClientSecret: os.Getenv("HUBSPOT_CLIENT_SECRET"),
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

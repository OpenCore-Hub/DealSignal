interface DurableObjectNamespace<T = unknown> {
	idFromName(name: string): unknown;
	get(id: unknown): DurableObjectStub<T>;
}

interface DurableObjectStub<T = unknown> {
	fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response>;
}

declare namespace Cloudflare {
	interface Env {
		DEALSIGNAL: DurableObjectNamespace;

		APP_ENV: string;
		PORT: string;
		LOG_LEVEL: string;
		PPROF_ENABLED: string;
		METRICS_ENABLED: string;

		APP_BASE_URL: string;
		FRONTEND_URL: string;
		VIEWER_BASE_URL: string;
		CORS_ALLOWED_ORIGINS: string;

		S3_ENDPOINT: string;
		S3_BUCKET: string;
		S3_REGION: string;
		S3_USE_PATH_STYLE: string;

		DATABASE_URL: string;
		REDIS_URL: string;
		JWT_SECRET: string;
		URL_SIGNING_SECRET: string;
		LINK_SESSION_SECRET: string;
		IP_HASH_KEY: string;
		INVITE_TOKEN_HASH_KEY: string;
		S3_ACCESS_KEY: string;
		S3_SECRET_KEY: string;
		TURNSTILE_SITE_KEY: string;
		TURNSTILE_SECRET: string;

		OPENAI_API_KEY?: string;
		OPENAI_BASE_URL?: string;
		OPENAI_CHAT_MODEL?: string;
		RESEND_API_KEY?: string;
		RESEND_FROM_EMAIL?: string;
		RESEND_WEBHOOK_SECRET?: string;
		STRIPE_SECRET_KEY?: string;
		STRIPE_WEBHOOK_SECRET?: string;
		STRIPE_PRICE_PRO_MONTHLY?: string;
		STRIPE_PRICE_PRO_YEARLY?: string;
		STRIPE_PRICE_BUSINESS_MONTHLY?: string;
		STRIPE_PRICE_BUSINESS_YEARLY?: string;
		ONLYOFFICE_URL?: string;
		ONLYOFFICE_JWT_SECRET?: string;
		DOCLING_RAG_BASE_URL?: string;
		DOCLING_RAG_PLATFORM_ADMIN_KEY?: string;
		DOCLING_RAG_HTTP_TIMEOUT_MS?: string;
		DOCLING_RAG_DEFAULT_MODE?: string;
		DOCLING_RAG_TOP_K?: string;
		EMAIL_TRACKING_SECRET?: string;
		SLACK_CLIENT_ID?: string;
		SLACK_CLIENT_SECRET?: string;
		HUBSPOT_CLIENT_ID?: string;
		HUBSPOT_CLIENT_SECRET?: string;
		SMTP_HOST?: string;
		SMTP_PORT?: string;
		SMTP_USER?: string;
		SMTP_PASS?: string;
		SMTP_FROM?: string;
	}
}

interface Env extends Cloudflare.Env {}

declare module "cloudflare:workers" {
	export const env: Cloudflare.Env;
}

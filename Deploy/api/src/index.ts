import { Container, getContainer } from "@cloudflare/containers";
import { env } from "cloudflare:workers";

function containerOSEnv(
	values: Record<string, string | undefined>,
): Record<string, string> {
	const out: Record<string, string> = {};
	for (const [key, value] of Object.entries(values)) {
		if (value !== undefined && value !== "") {
			out[key] = value;
		}
	}
	return out;
}

export class DealSignal extends Container {
	defaultPort = 8080;
	sleepAfter = "24h";
	enableInternet = true;

	envVars = containerOSEnv({
		PORT: env.PORT ?? "8080",
		APP_ENV: env.APP_ENV ?? "production",
		LOG_LEVEL: env.LOG_LEVEL ?? "info",
		PPROF_ENABLED: env.PPROF_ENABLED ?? "false",
		METRICS_ENABLED: env.METRICS_ENABLED ?? "true",

		APP_BASE_URL: env.APP_BASE_URL,
		FRONTEND_URL: env.FRONTEND_URL ?? env.APP_BASE_URL,
		VIEWER_BASE_URL: env.VIEWER_BASE_URL ?? env.FRONTEND_URL ?? env.APP_BASE_URL,
		CORS_ALLOWED_ORIGINS: env.CORS_ALLOWED_ORIGINS ?? env.APP_BASE_URL,

		DATABASE_URL: env.DATABASE_URL,
		REDIS_URL: env.REDIS_URL,
		JWT_SECRET: env.JWT_SECRET,
		URL_SIGNING_SECRET: env.URL_SIGNING_SECRET,
		LINK_SESSION_SECRET: env.LINK_SESSION_SECRET,
		IP_HASH_KEY: env.IP_HASH_KEY,
		INVITE_TOKEN_HASH_KEY: env.INVITE_TOKEN_HASH_KEY,

		S3_ENDPOINT: env.S3_ENDPOINT,
		S3_BUCKET: env.S3_BUCKET,
		S3_ACCESS_KEY: env.S3_ACCESS_KEY,
		S3_SECRET_KEY: env.S3_SECRET_KEY,
		S3_REGION: env.S3_REGION ?? "auto",
		S3_USE_PATH_STYLE: env.S3_USE_PATH_STYLE ?? "true",

		TURNSTILE_SITE_KEY: env.TURNSTILE_SITE_KEY,
		TURNSTILE_SECRET: env.TURNSTILE_SECRET,

		OPENAI_API_KEY: env.OPENAI_API_KEY,
		OPENAI_BASE_URL: env.OPENAI_BASE_URL,
		OPENAI_CHAT_MODEL: env.OPENAI_CHAT_MODEL,
		RESEND_API_KEY: env.RESEND_API_KEY,
		RESEND_FROM_EMAIL: env.RESEND_FROM_EMAIL,
		RESEND_WEBHOOK_SECRET: env.RESEND_WEBHOOK_SECRET,
		STRIPE_SECRET_KEY: env.STRIPE_SECRET_KEY,
		STRIPE_WEBHOOK_SECRET: env.STRIPE_WEBHOOK_SECRET,
		STRIPE_PRICE_PRO_MONTHLY: env.STRIPE_PRICE_PRO_MONTHLY,
		STRIPE_PRICE_PRO_YEARLY: env.STRIPE_PRICE_PRO_YEARLY,
		STRIPE_PRICE_BUSINESS_MONTHLY: env.STRIPE_PRICE_BUSINESS_MONTHLY,
		STRIPE_PRICE_BUSINESS_YEARLY: env.STRIPE_PRICE_BUSINESS_YEARLY,
		ONLYOFFICE_URL: env.ONLYOFFICE_URL,
		ONLYOFFICE_JWT_SECRET: env.ONLYOFFICE_JWT_SECRET,
		DOCLING_RAG_BASE_URL: env.DOCLING_RAG_BASE_URL,
		DOCLING_RAG_PLATFORM_ADMIN_KEY: env.DOCLING_RAG_PLATFORM_ADMIN_KEY,
		DOCLING_RAG_HTTP_TIMEOUT_MS: env.DOCLING_RAG_HTTP_TIMEOUT_MS,
		DOCLING_RAG_DEFAULT_MODE: env.DOCLING_RAG_DEFAULT_MODE,
		DOCLING_RAG_TOP_K: env.DOCLING_RAG_TOP_K,
		EMAIL_TRACKING_SECRET: env.EMAIL_TRACKING_SECRET,
		SLACK_CLIENT_ID: env.SLACK_CLIENT_ID,
		SLACK_CLIENT_SECRET: env.SLACK_CLIENT_SECRET,
		HUBSPOT_CLIENT_ID: env.HUBSPOT_CLIENT_ID,
		HUBSPOT_CLIENT_SECRET: env.HUBSPOT_CLIENT_SECRET,
		SMTP_HOST: env.SMTP_HOST,
		SMTP_PORT: env.SMTP_PORT,
		SMTP_USER: env.SMTP_USER,
		SMTP_PASS: env.SMTP_PASS,
		SMTP_FROM: env.SMTP_FROM,
	});

	async onActivityExpired(): Promise<void> {
		this.renewActivityTimeout();
	}
}

export default {
	async fetch(
		request: Request,
		workerEnv: { DEALSIGNAL: DurableObjectNamespace },
	): Promise<Response> {
		return getContainer(workerEnv.DEALSIGNAL).fetch(request);
	},
};

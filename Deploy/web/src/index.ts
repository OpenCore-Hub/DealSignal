/**
 * Edge reverse-proxy for same-origin `/api` (replaces production nginx).
 * Static SPA files are served by Workers Static Assets and never reach this
 * script except for `/api`, via `assets.run_worker_first`.
 */
export interface Env {
	API: {
		fetch: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
	};
}

function withForwardingHeaders(request: Request): Request {
	const url = new URL(request.url);
	const headers = new Headers(request.headers);
	if (!headers.has("X-Forwarded-Proto")) {
		headers.set("X-Forwarded-Proto", url.protocol.replace(":", ""));
	}
	const ip = headers.get("CF-Connecting-IP");
	if (ip) {
		if (!headers.has("X-Forwarded-For")) {
			headers.set("X-Forwarded-For", ip);
		}
		if (!headers.has("X-Real-IP")) {
			headers.set("X-Real-IP", ip);
		}
	}
	return new Request(request, { headers });
}

export default {
	async fetch(request: Request, env: Env): Promise<Response> {
		const { pathname } = new URL(request.url);
		if (pathname !== "/api" && !pathname.startsWith("/api/")) {
			return new Response("Not found", { status: 404 });
		}
		// Pass the Request through so POST bodies and SSE streams are not buffered.
		return env.API.fetch(withForwardingHeaders(request));
	},
};

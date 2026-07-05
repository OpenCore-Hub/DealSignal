import i18next from "i18next";

export interface ApiErrorDetails {
  field: string;
  issue: string;
}

export interface ApiPagination {
  page: number;
  page_size: number;
  total: number;
  has_more: boolean;
}

export interface BaseResponse<T> {
  code: string;
  message: string;
  request_id: string;
  data?: T;
  pagination?: ApiPagination;
  details?: ApiErrorDetails[];
}

export interface GateErrorFlags {
  requiresEmail?: boolean;
  requiresEmailVerification?: boolean;
  requiresPassword?: boolean;
  requiresNda?: boolean;
}

export class ApiError extends Error {
  status: number;
  code: string;
  requestId: string;
  details?: ApiErrorDetails[];
  requiresEmail?: boolean;
  requiresEmailVerification?: boolean;
  requiresPassword?: boolean;
  requiresNda?: boolean;

  constructor({
    status,
    code,
    message,
    requestId,
    details,
    requiresEmail,
    requiresEmailVerification,
    requiresPassword,
    requiresNda,
  }: {
    status: number;
    code: string;
    message: string;
    requestId: string;
    details?: ApiErrorDetails[];
  } & GateErrorFlags) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
    this.details = details;
    this.requiresEmail = requiresEmail;
    this.requiresEmailVerification = requiresEmailVerification;
    this.requiresPassword = requiresPassword;
    this.requiresNda = requiresNda;
  }
}

export interface RequestOptions extends RequestInit {
  token?: string;
  idempotencyKey?: string;
  skipAuth?: boolean;
  /** Called when backend returns X-Link-Session-Refresh — the
   *  frontend should store the refreshed session token so the
   *  idle timeout keeps sliding during document viewing. */
  onSessionRefresh?: (token: string) => void;
  /** AbortSignal forwarded to the underlying fetch call so callers
   *  can cancel in-flight requests on unmount or dependency churn. */
  signal?: AbortSignal;
}

function getBaseUrl(): string {
  const env = import.meta.env.VITE_API_BASE_URL as string | undefined;
  return env?.replace(/\/+$/, "") ?? "";
}

function getAuthToken(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return localStorage.getItem("access_token");
  } catch {
    return null;
  }
}

function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return localStorage.getItem("refresh_token");
  } catch {
    return null;
  }
}

function setTokens(accessToken: string, refreshToken: string) {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem("access_token", accessToken);
    localStorage.setItem("refresh_token", refreshToken);
  } catch {
    // ignore
  }
}

function clearTokens() {
  if (typeof window === "undefined") return;
  try {
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
  } catch {
    // ignore
  }
}

let refreshPromise: Promise<string> | null = null;

async function doRefresh(baseUrl: string): Promise<string> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) {
    throw new Error("No refresh token");
  }
  const res = await fetch(`${baseUrl}/api/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
  if (!res.ok) {
    throw new Error("Refresh failed");
  }
  const data = (await res.json()) as { access_token: string; refresh_token: string };
  setTokens(data.access_token, data.refresh_token);
  return data.access_token;
}

function refreshAccessToken(baseUrl: string): Promise<string> {
  if (!refreshPromise) {
    refreshPromise = doRefresh(baseUrl)
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
}

function redirectToLogin() {
  if (typeof window !== "undefined") {
    clearTokens();
    const pathname = window.location.pathname ?? "";
    const search = window.location.search ?? "";
    const returnPath = pathname + search;
    const loginUrl = returnPath !== "" && returnPath !== "/login" && returnPath !== "/"
      ? `/login?redirect=${encodeURIComponent(returnPath)}`
      : "/login";
    window.location.href = loginUrl;
  }
}

function getLanguage(): string {
  try {
    return i18next.language || getBrowserLanguage();
  } catch {
    return getBrowserLanguage();
  }
}

function getBrowserLanguage(): string {
  if (typeof navigator === "undefined") return "en";
  return navigator.language || "en";
}

function generateRequestId(): string {
  return crypto.randomUUID();
}

function isBaseResponse<T>(value: unknown): value is BaseResponse<T> {
  return (
    value !== null &&
    typeof value === "object" &&
    "code" in value &&
    value.code === "ok" &&
    "data" in value
  );
}

export async function request<T>(
  workspaceSlug: string | undefined,
  path: string,
  options: RequestOptions = {}
): Promise<T> {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  const prefix = workspaceSlug
    ? `/api/workspaces/${encodeURIComponent(workspaceSlug)}`
    : "/api";
  const url = `${getBaseUrl()}${prefix}${normalizedPath}`;

  const headers = new Headers(options.headers);
  const body = options.body;

  if (!(body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (!headers.has("Accept")) {
    headers.set("Accept", "application/json");
  }

  const token = options.token ?? getAuthToken();

  const requestId = generateRequestId();
  headers.set("X-Request-ID", requestId);
  headers.set("Accept-Language", getLanguage());
  if (options.idempotencyKey) {
    headers.set("X-Idempotency-Key", options.idempotencyKey);
  }

  const execute = async (authToken: string | null): Promise<Response> => {
    if (authToken && !options.skipAuth) {
      headers.set("Authorization", `Bearer ${authToken}`);
    }
    return fetch(url, { ...options, headers });
  };

  let response: Response;
  try {
    response = await execute(token);
  } catch (err) {
    throw new ApiError({
      status: 0,
      code: "network_error",
      message: err instanceof Error ? err.message : "Network error",
      requestId,
    });
  }

  // Attempt silent refresh on 401 (unless this is an auth endpoint).
  if (response.status === 401 && !options.skipAuth) {
    try {
      const newToken = await refreshAccessToken(getBaseUrl());
      response = await execute(newToken);
    } catch {
      redirectToLogin();
      throw new ApiError({
        status: 401,
        code: "unauthorized",
        message: "Session expired. Please sign in again.",
        requestId,
      });
    }
  }

  if (!response.ok) {
    let body: BaseResponse<unknown> | null = null;
    try {
      body = (await response.json()) as BaseResponse<unknown>;
    } catch {
      // ignore non-JSON error bodies
    }
    const gateFlags = body as GateErrorFlags | null;
    throw new ApiError({
      status: response.status,
      code: body?.code ?? "http_error",
      message: body?.message ?? response.statusText,
      requestId: body?.request_id ?? requestId,
      details: body?.details,
      requiresEmail: gateFlags?.requiresEmail,
      requiresEmailVerification: gateFlags?.requiresEmailVerification,
      requiresPassword: gateFlags?.requiresPassword,
      requiresNda: gateFlags?.requiresNda,
    });
  }

  if (response.status === 204) {
    handleSessionRefresh(response, options);
    return undefined as T;
  }

  const payload: unknown = await response.json();
  handleSessionRefresh(response, options);
  if (isBaseResponse<T>(payload)) {
    return payload.data as T;
  }
  return payload as T;
}

function handleSessionRefresh(response: Response, options: RequestOptions) {
  // Per-request callback takes priority.
  const refreshed = response.headers.get("X-Link-Session-Refresh");
  if (refreshed) {
    if (options.onSessionRefresh) {
      options.onSessionRefresh(refreshed);
    } else if (linkSessionRefreshHandler) {
      linkSessionRefreshHandler(refreshed);
    }
  }
}

// linkSessionRefreshHandler is a module-level callback set by the viewer
// page so that every API call (including image signed-URL fetches) can
// automatically update the stored session token when the backend returns
// a refreshed one.
let linkSessionRefreshHandler: ((token: string) => void) | null = null;

export function setLinkSessionRefreshHandler(handler: ((token: string) => void) | null) {
  linkSessionRefreshHandler = handler;
}

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

export class ApiError extends Error {
  status: number;
  code: string;
  requestId: string;
  details?: ApiErrorDetails[];

  constructor({
    status,
    code,
    message,
    requestId,
    details,
  }: {
    status: number;
    code: string;
    message: string;
    requestId: string;
    details?: ApiErrorDetails[];
  }) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
    this.details = details;
  }
}

export interface RequestOptions extends RequestInit {
  token?: string;
  idempotencyKey?: string;
  skipAuth?: boolean;
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

export function generateRequestId(): string {
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
  if (token && !options.skipAuth) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const requestId = generateRequestId();
  headers.set("X-Request-ID", requestId);
  headers.set("Accept-Language", getLanguage());
  if (options.idempotencyKey) {
    headers.set("Idempotency-Key", options.idempotencyKey);
  }

  let response: Response;
  try {
    response = await fetch(url, { ...options, headers });
  } catch (err) {
    throw new ApiError({
      status: 0,
      code: "network_error",
      message: err instanceof Error ? err.message : "Network error",
      requestId,
    });
  }

  if (!response.ok) {
    let body: BaseResponse<unknown> | null = null;
    try {
      body = (await response.json()) as BaseResponse<unknown>;
    } catch {
      // ignore non-JSON error bodies
    }
    throw new ApiError({
      status: response.status,
      code: body?.code ?? "http_error",
      message: body?.message ?? response.statusText,
      requestId: body?.request_id ?? requestId,
      details: body?.details,
    });
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const payload: unknown = await response.json();
  if (isBaseResponse<T>(payload)) {
    return payload.data as T;
  }
  return payload as T;
}

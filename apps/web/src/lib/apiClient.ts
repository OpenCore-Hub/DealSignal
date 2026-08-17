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
  isDealRoom?: boolean;
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
  isDealRoom?: boolean;

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
    isDealRoom,
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
    this.isDealRoom = isDealRoom;
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

export type UploadProgressHandler = (event: { loaded: number; total: number }) => void;

/** Multipart POST options. Isolated from `request()` so fetch callers stay untouched. */
export interface UploadRequestOptions extends RequestOptions {
  onUploadProgress?: UploadProgressHandler;
}

function getBaseUrl(): string {
  const env = import.meta.env.VITE_API_BASE_URL as string | undefined;
  return env?.replace(/\/+$/, "") ?? "";
}

let refreshPromise: Promise<void> | null = null;

async function doRefresh(baseUrl: string): Promise<void> {
  const res = await fetch(`${baseUrl}/api/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  });
  if (!res.ok) {
    throw new Error("Refresh failed");
  }
}

function refreshAccessToken(baseUrl: string): Promise<void> {
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

async function executeAuthorizedFetch(
  workspaceSlug: string | undefined,
  path: string,
  options: RequestOptions = {},
  defaultAccept: string,
): Promise<{ response: Response; requestId: string }> {
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
    headers.set("Accept", defaultAccept);
  }

  const requestId = generateRequestId();
  headers.set("X-Request-ID", requestId);
  headers.set("Accept-Language", getLanguage());
  if (options.idempotencyKey) {
    headers.set("X-Idempotency-Key", options.idempotencyKey);
  }

  const execute = async (): Promise<Response> => {
    if (options.token && !options.skipAuth) {
      headers.set("Authorization", `Bearer ${options.token}`);
    }
    return fetch(url, { ...options, headers, credentials: "include" });
  };

  let response: Response;
  try {
    response = await execute();
  } catch (err) {
    if (options.signal?.aborted || (err instanceof DOMException && err.name === "AbortError")) {
      throw err;
    }
    throw new ApiError({
      status: 0,
      code: "network_error",
      message: err instanceof Error ? err.message : "network_error",
      requestId,
    });
  }

  // Attempt silent refresh on 401 (unless this is an auth endpoint).
  if (response.status === 401 && !options.skipAuth) {
    try {
      await refreshAccessToken(getBaseUrl());
      response = await execute();
    } catch {
      redirectToLogin();
      throw new ApiError({
        status: 401,
        code: "unauthorized",
        message: "unauthorized",
        requestId,
      });
    }
  }

  return { response, requestId };
}

export async function request<T>(
  workspaceSlug: string | undefined,
  path: string,
  options: RequestOptions = {}
): Promise<T> {
  const { response, requestId } = await executeAuthorizedFetch(
    workspaceSlug,
    path,
    options,
    "application/json",
  );

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
      isDealRoom: gateFlags?.isDealRoom,
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

/**
 * Same contract as `request`, but reports real `xhr.upload` bytes.
 * Do not route other callers through this — `request` stays fetch-only (CRITICAL).
 */
export async function requestWithUploadProgress<T>(
  workspaceSlug: string | undefined,
  path: string,
  options: UploadRequestOptions = {},
): Promise<T> {
  if (!options.onUploadProgress || !(options.body instanceof FormData)) {
    const { onUploadProgress: _ignored, ...rest } = options;
    return request<T>(workspaceSlug, path, rest);
  }

  const send = () =>
    executeAuthorizedXhr<T>(workspaceSlug, path, options, /* retried */ false);
  try {
    return await send();
  } catch (err) {
    if (!(err instanceof ApiError) || err.status !== 401 || options.skipAuth) {
      throw err;
    }
    try {
      await refreshAccessToken(getBaseUrl());
    } catch {
      redirectToLogin();
      throw err;
    }
    return executeAuthorizedXhr<T>(workspaceSlug, path, options, /* retried */ true);
  }
}

function executeAuthorizedXhr<T>(
  workspaceSlug: string | undefined,
  path: string,
  options: UploadRequestOptions,
  retried: boolean,
): Promise<T> {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  const prefix = workspaceSlug
    ? `/api/workspaces/${encodeURIComponent(workspaceSlug)}`
    : "/api";
  const url = `${getBaseUrl()}${prefix}${normalizedPath}`;
  const requestId = generateRequestId();

  return new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open((options.method ?? "POST").toString().toUpperCase(), url);
    xhr.withCredentials = true;
    xhr.responseType = "text";

    const headers = new Headers(options.headers);
    if (!headers.has("Accept")) {
      headers.set("Accept", "application/json");
    }
    headers.set("X-Request-ID", requestId);
    headers.set("Accept-Language", getLanguage());
    if (options.idempotencyKey) {
      headers.set("X-Idempotency-Key", options.idempotencyKey);
    }
    if (options.token && !options.skipAuth) {
      headers.set("Authorization", `Bearer ${options.token}`);
    }
    headers.forEach((value, key) => {
      if (key.toLowerCase() === "content-type") return;
      xhr.setRequestHeader(key, value);
    });

    const onAbort = () => {
      xhr.abort();
    };
    if (options.signal?.aborted) {
      reject(new DOMException("Aborted", "AbortError"));
      return;
    }
    options.signal?.addEventListener("abort", onAbort);

    xhr.upload.onprogress = (event) => {
      if (!event.lengthComputable) return;
      options.onUploadProgress?.({ loaded: event.loaded, total: event.total });
    };

    const sessionHeaders = {
      headers: { get: (name: string) => xhr.getResponseHeader(name) },
    } as Response;

    xhr.onload = () => {
      options.signal?.removeEventListener("abort", onAbort);

      if (xhr.status === 401 && !options.skipAuth && !retried) {
        reject(
          new ApiError({
            status: 401,
            code: "unauthorized",
            message: "unauthorized",
            requestId,
          }),
        );
        return;
      }

      if (xhr.status === 204) {
        handleSessionRefresh(sessionHeaders, options);
        resolve(undefined as T);
        return;
      }

      let payload: unknown = null;
      if (xhr.responseText) {
        try {
          payload = JSON.parse(xhr.responseText) as unknown;
        } catch {
          payload = null;
        }
      }

      if (xhr.status < 200 || xhr.status >= 300) {
        const body = payload as BaseResponse<unknown> | null;
        const gateFlags = body as GateErrorFlags | null;
        reject(
          new ApiError({
            status: xhr.status,
            code: body?.code ?? "http_error",
            message: body?.message ?? xhr.statusText,
            requestId: body?.request_id ?? requestId,
            details: body?.details,
            requiresEmail: gateFlags?.requiresEmail,
            requiresEmailVerification: gateFlags?.requiresEmailVerification,
            requiresPassword: gateFlags?.requiresPassword,
            requiresNda: gateFlags?.requiresNda,
            isDealRoom: gateFlags?.isDealRoom,
          }),
        );
        return;
      }

      handleSessionRefresh(sessionHeaders, options);
      if (isBaseResponse<T>(payload)) {
        resolve(payload.data as T);
        return;
      }
      resolve(payload as T);
    };

    xhr.onerror = () => {
      options.signal?.removeEventListener("abort", onAbort);
      reject(
        new ApiError({
          status: 0,
          code: "network_error",
          message: "network_error",
          requestId,
        }),
      );
    };

    xhr.onabort = () => {
      options.signal?.removeEventListener("abort", onAbort);
      reject(new DOMException("Aborted", "AbortError"));
    };

    xhr.send(options.body instanceof FormData ? options.body : null);
  });
}

/**
 * Authenticated fetch that returns a live Response for streaming bodies (SSE).
 * Shares request-id / Accept-Language / 401 refresh with `request`.
 */
export async function openStream(
  workspaceSlug: string | undefined,
  path: string,
  options: RequestOptions = {},
): Promise<Response> {
  const { response, requestId } = await executeAuthorizedFetch(
    workspaceSlug,
    path,
    options,
    "text/event-stream",
  );

  if (!response.ok) {
    let body: BaseResponse<unknown> | null = null;
    try {
      body = (await response.json()) as BaseResponse<unknown>;
    } catch {
      /* non-JSON */
    }
    throw new ApiError({
      status: response.status,
      code: body?.code ?? "http_error",
      message: body?.message ?? body?.code ?? "http_error",
      requestId: body?.request_id ?? requestId,
      details: body?.details,
    });
  }

  handleSessionRefresh(response, options);
  return response;
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

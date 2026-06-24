// @vitest-environment jsdom
import {
  describe,
  it,
  expect,
  beforeAll,
  afterAll,
  afterEach,
  beforeEach,
  vi,
} from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "@/lib/mocks/server";
import { ApiError, request } from "@/lib/apiClient";

class MemoryStorage implements Storage {
  private store = new Map<string, string>();

  get length() {
    return this.store.size;
  }

  key(index: number): string | null {
    return Array.from(this.store.keys())[index] ?? null;
  }

  getItem(key: string): string | null {
    return this.store.get(key) ?? null;
  }

  setItem(key: string, value: string): void {
    this.store.set(key, String(value));
  }

  removeItem(key: string): void {
    this.store.delete(key);
  }

  clear(): void {
    this.store.clear();
  }
}

describe("apiClient", () => {
  beforeAll(() => {
    import.meta.env.VITE_API_BASE_URL = "http://localhost";
    server.listen({ onUnhandledRequest: "error" });
  });
  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());

  const storage = new MemoryStorage();

  beforeEach(() => {
    vi.stubGlobal("localStorage", storage);
    storage.clear();
    Object.defineProperty(navigator, "language", {
      configurable: true,
      value: "en-US",
      writable: true,
    });
  });

  it("unwraps BaseResponse data", async () => {
    server.use(
      http.get("*/api/workspaces/acme/client-test/unwrap", () =>
        HttpResponse.json({
          code: "ok",
          message: "success",
          request_id: "req_001",
          data: [{ id: "doc_1", title: "Pitch Deck" }],
        })
      )
    );

    const data = await request<{ id: string; title: string }[]>(
      "acme",
      "/client-test/unwrap"
    );
    expect(data).toHaveLength(1);
    expect(data[0].id).toBe("doc_1");
  });

  it("returns direct payload when no BaseResponse wrapper", async () => {
    server.use(
      http.get("*/api/workspaces/acme/client-test/direct", () =>
        HttpResponse.json({ id: "doc_1", title: "Pitch Deck" })
      )
    );

    const data = await request<{ id: string; title: string }>(
      "acme",
      "/client-test/direct"
    );
    expect(data.title).toBe("Pitch Deck");
  });

  it("returns undefined for 204 responses", async () => {
    server.use(
      http.delete("*/api/workspaces/acme/client-test/delete", () =>
        new HttpResponse(null, { status: 204 })
      )
    );

    const data = await request<unknown>("acme", "/client-test/delete", {
      method: "DELETE",
    });
    expect(data).toBeUndefined();
  });

  it("throws ApiError with backend error body", async () => {
    server.use(
      http.post("*/api/workspaces/acme/client-test/error", () =>
        HttpResponse.json(
          {
            code: "invalid_request",
            message: "Bad request",
            request_id: "req_002",
            details: [{ field: "document_id", issue: "required" }],
          },
          { status: 400 }
        )
      )
    );

    await expect(
      request("acme", "/client-test/error", {
        method: "POST",
        body: JSON.stringify({}),
      })
    ).rejects.toSatisfy((err: ApiError) => {
      return (
        err instanceof ApiError &&
        err.status === 400 &&
        err.code === "invalid_request" &&
        err.message === "Bad request" &&
        err.requestId === "req_002" &&
        err.details?.[0].field === "document_id"
      );
    });
  });

  it("throws ApiError for network failures", async () => {
    server.use(
      http.get("*/api/workspaces/acme/client-test/network", () =>
        HttpResponse.error()
      )
    );

    await expect(request("acme", "/client-test/network")).rejects.toSatisfy(
      (err: ApiError) => err instanceof ApiError && err.code === "network_error"
    );
  });

  it("injects required headers", async () => {
    let captured: Request | null = null;
    server.use(
      http.get("*/api/workspaces/acme/client-test/headers", ({ request }) => {
        captured = request;
        return HttpResponse.json({ ok: true });
      })
    );

    await request("acme", "/client-test/headers", {
      idempotencyKey: "idem-1",
      token: "fake-jwt",
    });

    expect(captured).not.toBeNull();
    expect(captured!.headers.get("Authorization")).toBe("Bearer fake-jwt");
    expect(captured!.headers.get("Content-Type")).toBe("application/json");
    expect(captured!.headers.get("Accept")).toBe("application/json");
    expect(captured!.headers.get("Accept-Language")).toBe("en-US");
    expect(captured!.headers.get("X-Idempotency-Key")).toBe("idem-1");
    expect(captured!.headers.get("X-Request-ID")).toBeTruthy();
  });

  it("builds URL with base URL when configured", async () => {
    import.meta.env.VITE_API_BASE_URL = "https://api.example.com";

    server.use(
      http.get("https://api.example.com/api/workspaces/acme/client-test/base", () =>
        HttpResponse.json({ ok: true })
      )
    );

    const data = await request<{ ok: boolean }>("acme", "/client-test/base");
    expect(data.ok).toBe(true);
  });

  it("uses the stored access token by default", async () => {
    localStorage.setItem("access_token", "stored-token");
    let captured: Request | null = null;
    server.use(
      http.get("*/api/workspaces/acme/client-test/token", ({ request }) => {
        captured = request;
        return HttpResponse.json({ ok: true });
      })
    );

    await request("acme", "/client-test/token");
    expect(captured!.headers.get("Authorization")).toBe("Bearer stored-token");
  });

  it("refreshes the token on 401 and retries the request", async () => {
    localStorage.setItem("access_token", "old-token");
    localStorage.setItem("refresh_token", "refresh-1");

    let originalCalls = 0;
    let refreshCaptured: Request | null = null;

    server.use(
      http.get("*/api/workspaces/acme/client-test/protected", ({ request }) => {
        originalCalls++;
        if (request.headers.get("Authorization") !== "Bearer new-token") {
          return new HttpResponse(null, { status: 401 });
        }
        return HttpResponse.json({ ok: true });
      }),
      http.post("*/api/auth/refresh", async ({ request }) => {
        refreshCaptured = request;
        const body = (await request.json()) as { refresh_token: string };
        if (body.refresh_token !== "refresh-1") {
          return new HttpResponse(null, { status: 401 });
        }
        return HttpResponse.json({
          access_token: "new-token",
          refresh_token: "refresh-2",
        });
      })
    );

    const data = await request<{ ok: boolean }>("acme", "/client-test/protected");
    expect(data.ok).toBe(true);
    expect(originalCalls).toBe(2);
    expect(refreshCaptured).not.toBeNull();
    expect(localStorage.getItem("access_token")).toBe("new-token");
    expect(localStorage.getItem("refresh_token")).toBe("refresh-2");
  });

  it("redirects to login when token refresh fails", async () => {
    localStorage.setItem("access_token", "old-token");
    localStorage.setItem("refresh_token", "refresh-1");

    const locationSpy = { href: "" };
    vi.stubGlobal("location", locationSpy);

    server.use(
      http.get("*/api/workspaces/acme/client-test/protected", () =>
        new HttpResponse(null, { status: 401 })
      ),
      http.post("*/api/auth/refresh", () => new HttpResponse(null, { status: 401 }))
    );

    await expect(
      request("acme", "/client-test/protected")
    ).rejects.toSatisfy((err: ApiError) => err instanceof ApiError && err.code === "unauthorized");

    expect(locationSpy.href).toBe("/login");
    expect(localStorage.getItem("access_token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();

    vi.unstubAllGlobals();
  });

  it("honours skipAuth and does not send the Authorization header", async () => {
    localStorage.setItem("access_token", "stored-token");
    let captured: Request | null = null;
    server.use(
      http.get("*/api/workspaces/acme/client-test/public", ({ request }) => {
        captured = request;
        return HttpResponse.json({ ok: true });
      })
    );

    await request("acme", "/client-test/public", { skipAuth: true });
    expect(captured!.headers.get("Authorization")).toBeNull();
  });

  it("does not override Content-Type for FormData bodies", async () => {
    const formData = new FormData();
    formData.append("file", new Blob(["x"]), "x.txt");

    let captured: Request | null = null;
    server.use(
      http.post("*/api/workspaces/acme/client-test/upload", ({ request }) => {
        captured = request;
        return HttpResponse.json({ ok: true });
      })
    );

    await request("acme", "/client-test/upload", {
      method: "POST",
      body: formData,
    });

    expect(captured).not.toBeNull();
    expect(captured!.headers.get("Content-Type")).not.toContain("application/json");
  });

  it("falls back to navigator.language for Accept-Language", async () => {
    Object.defineProperty(navigator, "language", {
      configurable: true,
      value: "fr-FR",
      writable: true,
    });

    let captured: Request | null = null;
    server.use(
      http.get("*/api/workspaces/acme/client-test/lang", ({ request }) => {
        captured = request;
        return HttpResponse.json({ ok: true });
      })
    );

    await request("acme", "/client-test/lang");
    expect(captured!.headers.get("Accept-Language")).toBe("fr-FR");
  });
});

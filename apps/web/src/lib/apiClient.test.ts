import {
  describe,
  it,
  expect,
  beforeAll,
  afterAll,
  afterEach,
  beforeEach,
} from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "@/lib/mocks/server";
import { ApiError, request } from "@/lib/apiClient";

describe("apiClient", () => {
  beforeAll(() => {
    import.meta.env.VITE_API_BASE_URL = "http://localhost";
    server.listen({ onUnhandledRequest: "error" });
  });
  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());

  beforeEach(() => {
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
    expect(captured!.headers.get("Idempotency-Key")).toBe("idem-1");
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
});

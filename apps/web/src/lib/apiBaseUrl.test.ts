import { describe, expect, it } from "vitest";
import {
  resolveBrowserApiBaseUrl,
  resolveDevApiProxyTarget,
} from "./apiBaseUrl";

describe("resolveBrowserApiBaseUrl", () => {
  it("keeps an absolute API origin in production builds", () => {
    expect(
      resolveBrowserApiBaseUrl("https://api.example.com/", {
        dev: false,
        vitest: false,
      }),
    ).toBe("https://api.example.com");
  });

  it("keeps an absolute API origin under Vitest so MSW URL matchers stay exact", () => {
    expect(
      resolveBrowserApiBaseUrl("http://localhost", {
        dev: true,
        vitest: true,
      }),
    ).toBe("http://localhost");
  });

  it("uses same-origin /api in Vite browser dev so session cookies stay first-party", () => {
    expect(
      resolveBrowserApiBaseUrl("http://127.0.0.1:8090", {
        dev: true,
        vitest: false,
      }),
    ).toBe("");
  });

  it("leaves relative mode empty for MSW / nginx", () => {
    expect(resolveBrowserApiBaseUrl("", { dev: true, vitest: false })).toBe("");
    expect(resolveBrowserApiBaseUrl(undefined, { dev: false, vitest: false })).toBe("");
  });
});

describe("resolveDevApiProxyTarget", () => {
  it("returns the origin for a configured backend", () => {
    expect(resolveDevApiProxyTarget("http://127.0.0.1:8090/")).toBe("http://127.0.0.1:8090");
  });

  it("ignores empty and relative values so MSW is not proxied", () => {
    expect(resolveDevApiProxyTarget("")).toBeUndefined();
    expect(resolveDevApiProxyTarget("/api")).toBeUndefined();
  });

  it("ignores unparseable absolute URLs", () => {
    expect(resolveDevApiProxyTarget("http://[")).toBeUndefined();
  });
});

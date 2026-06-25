import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { toCreateLinkPayload, toCreateDealRoomPayload } from "./apiAdapters";
import type { PermissionConfig } from "@/types";

describe("toCreateLinkPayload", () => {
  const baseConfig: PermissionConfig = {
    level: "low",
    requireEmail: false,
    whitelistEnabled: false,
    whitelist: [],
    passwordEnabled: false,
    allowDownload: false,
    watermarkEnabled: false,
    expiryDays: "custom",
    maxViews: "unlimited",
  };

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-21T12:00:00.000Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("maps permission levels correctly", () => {
    expect(toCreateLinkPayload("doc-1", { ...baseConfig, level: "low" }).permission_type).toBe("public");
    expect(toCreateLinkPayload("doc-1", { ...baseConfig, level: "medium" }).permission_type).toBe("email_required");
    expect(toCreateLinkPayload("doc-1", { ...baseConfig, level: "high" }).permission_type).toBe("whitelist");
  });

  it("maps high + password to password permission type", () => {
    const config: PermissionConfig = { ...baseConfig, level: "high", passwordEnabled: true, password: "secret" };
    expect(toCreateLinkPayload("doc-1", config).permission_type).toBe("password");
  });

  it("includes whitelist emails when enabled", () => {
    const config: PermissionConfig = { ...baseConfig, level: "high", whitelistEnabled: true, whitelist: ["a@example.test"] };
    const payload = toCreateLinkPayload("doc-1", config);
    expect(payload.allowed_emails).toEqual(["a@example.test"]);
  });

  it("omits whitelist emails when disabled", () => {
    const payload = toCreateLinkPayload("doc-1", { ...baseConfig, whitelist: ["a@example.test"] });
    expect(payload.allowed_emails).toBeUndefined();
  });

  it("splits whitelist emails and domains into separate fields", () => {
    const config: PermissionConfig = {
      ...baseConfig,
      level: "high",
      whitelistEnabled: true,
      whitelist: ["a@example.test", "example.org", " @example.io ", "b@example.test"],
    };
    const payload = toCreateLinkPayload("doc-1", config);
    expect(payload.allowed_emails).toEqual(["a@example.test", "b@example.test"]);
    expect(payload.allowed_domains).toEqual(["example.org", "@example.io"]);
  });

  it("includes password when enabled", () => {
    const payload = toCreateLinkPayload("doc-1", { ...baseConfig, passwordEnabled: true, password: "secret" });
    expect(payload.password).toBe("secret");
  });

  it("sets expiration from expiryDays", () => {
    const payload = toCreateLinkPayload("doc-1", { ...baseConfig, expiryDays: 7 });
    expect(payload.expires_at).toBe("2026-06-28T12:00:00.000Z");
  });

  it("sets max access count from maxViews", () => {
    const payload = toCreateLinkPayload("doc-1", { ...baseConfig, maxViews: 10 });
    expect(payload.max_access_count).toBe(10);
  });
});

describe("toCreateDealRoomPayload", () => {
  it("converts template slug to snake_case type", () => {
    const payload = toCreateDealRoomPayload({
      name: "Room",
      slug: "room-1",
      template: "fundraising-pitch",
      ndaEnabled: true,
      requiresApproval: true,
    });
    expect(payload.template_type).toBe("fundraising_pitch");
    expect(payload.requires_nda).toBe(true);
    expect(payload.requires_approval).toBe(true);
  });

  it("omits optional fields when not provided", () => {
    const payload = toCreateDealRoomPayload({ name: "Room", slug: "room-1" });
    expect(payload.template_type).toBeUndefined();
    expect(payload.requires_nda).toBeUndefined();
    expect(payload.requires_approval).toBeUndefined();
  });
});

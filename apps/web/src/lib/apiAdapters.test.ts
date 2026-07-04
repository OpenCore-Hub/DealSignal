import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { toCreateLinkPayload, toCreateDealRoomPayload } from "./apiAdapters";
import type { PermissionConfig } from "@/types";

describe("toCreateLinkPayload", () => {
  const baseConfig: PermissionConfig = {
    level: "public",
    isCustomized: false,
    requireEmailVerification: false,
    whitelistEnabled: false,
    whitelist: [],
    passwordEnabled: false,
    ndaEnabled: false,
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

  it("maps public preset to public permission type", () => {
    const payload = toCreateLinkPayload("doc-1", { ...baseConfig, level: "public" });
    expect(payload.permission_type).toBe("public");
  });

  it("maps standard preset with email verification to public permission type with boolean flag", () => {
    const payload = toCreateLinkPayload("doc-1", {
      ...baseConfig,
      level: "standard",
      isCustomized: false,
      requireEmailVerification: true,
      contactId: "contact-1",
    });
    expect(payload.permission_type).toBe("public");
    expect(payload.require_email_verification).toBe(true);
    expect(payload.contact_ids).toEqual(["contact-1"]);
  });

  it("maps confidential preset to nda permission type", () => {
    const payload = toCreateLinkPayload("doc-1", {
      ...baseConfig,
      level: "confidential",
      isCustomized: false,
      whitelistEnabled: true,
      whitelist: ["a@example.test"],
      ndaEnabled: true,
      passwordEnabled: true,
      password: "secret",
    });
    expect(payload.permission_type).toBe("nda");
    expect(payload.require_nda).toBe(true);
    expect(payload.require_password).toBe(true);
    expect(payload.allowed_emails).toEqual(["a@example.test"]);
  });

  it("maps password to password permission type", () => {
    const config: PermissionConfig = {
      ...baseConfig,
      level: "confidential",
      isCustomized: false,
      passwordEnabled: true,
      password: "secret",
    };
    expect(toCreateLinkPayload("doc-1", config).permission_type).toBe("password");
  });

  it("maps whitelist to whitelist permission type", () => {
    const config: PermissionConfig = {
      ...baseConfig,
      level: "standard",
      isCustomized: false,
      whitelistEnabled: true,
      whitelist: ["a@example.test"],
    };
    const payload = toCreateLinkPayload("doc-1", config);
    expect(payload.permission_type).toBe("whitelist");
    expect(payload.allowed_emails).toEqual(["a@example.test"]);
  });

  it("omits whitelist emails when disabled", () => {
    const payload = toCreateLinkPayload("doc-1", {
      ...baseConfig,
      whitelist: ["a@example.test"],
    });
    expect(payload.allowed_emails).toBeUndefined();
  });

  it("splits whitelist emails and domains into separate fields", () => {
    const config: PermissionConfig = {
      ...baseConfig,
      level: "standard",
      isCustomized: false,
      whitelistEnabled: true,
      whitelist: [
        "a@example.test",
        "example.org",
        " @example.io ",
        "b@example.test",
      ],
    };
    const payload = toCreateLinkPayload("doc-1", config);
    expect(payload.allowed_emails).toEqual(["a@example.test", "b@example.test"]);
    expect(payload.allowed_domains).toEqual(["example.org", "@example.io"]);
  });

  it("includes password when enabled", () => {
    const payload = toCreateLinkPayload("doc-1", {
      ...baseConfig,
      passwordEnabled: true,
      password: "secret",
    });
    expect(payload.password).toBe("secret");
  });

  it("sets expiration from expiryDays", () => {
    const payload = toCreateLinkPayload("doc-1", {
      ...baseConfig,
      expiryDays: 7,
    });
    expect(payload.expires_at).toBe("2026-06-28T12:00:00.000Z");
  });

  it("sets max access count from maxViews", () => {
    const payload = toCreateLinkPayload("doc-1", {
      ...baseConfig,
      maxViews: 10,
    });
    expect(payload.max_access_count).toBe(10);
  });

  it("sends require_password when password is enabled", () => {
    const payload = toCreateLinkPayload("doc-1", {
      ...baseConfig,
      passwordEnabled: true,
      password: "secret",
    });
    expect(payload.require_password).toBe(true);
    expect(payload.require_email_verification).toBe(false);
  });

  it("sends require_email_verification for whitelist and require_nda for NDA", () => {
    const payload = toCreateLinkPayload("doc-1", {
      ...baseConfig,
      whitelistEnabled: true,
      whitelist: ["a@example.test"],
      ndaEnabled: true,
    });
    expect(payload.require_email_verification).toBe(true);
    expect(payload.require_nda).toBe(true);
  });

  it("maps collaborative preset to public with download and watermark", () => {
    const payload = toCreateLinkPayload("doc-1", {
      ...baseConfig,
      level: "collaborative",
      isCustomized: false,
      requireEmailVerification: true,
      contactId: "contact-1",
      allowDownload: true,
      watermarkEnabled: true,
    });
    expect(payload.permission_type).toBe("public");
    expect(payload.require_email_verification).toBe(true);
    expect(payload.download_enabled).toBe(true);
    expect(payload.watermark_enabled).toBe(true);
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

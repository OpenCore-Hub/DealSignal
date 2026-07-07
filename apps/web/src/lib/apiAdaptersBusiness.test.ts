import { describe, it, expect } from "vitest";
import { toCreateLinkPayload } from "@/lib/apiAdapters";
import { buildConfigFromPreset } from "@/components/links/link-bundle/pipelineUtils";
import type { PermissionConfig } from "@/types";

/**
 * Edge case and business rule tests for toCreateLinkPayload.
 * Focuses on:
 * - Validation constraints (NDA → email verification enforced)
 * - Duplicate processing
 * - Edge value handling
 */

describe("toCreateLinkPayload — business rules", () => {
  it("enforces email verification when NDA is enabled", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      ndaEnabled: true,
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    // NDA should force email verification even on otherwise public config
    expect(payload.require_email_verification).toBe(true);
    expect(payload.require_nda).toBe(true);
    expect(payload.permission_type).toBe("nda");
  });

  it("maps permission_type to nda when nda is primary gate", () => {
    const config = buildConfigFromPreset("confidential");
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.permission_type).toBe("nda");
    expect(payload.require_nda).toBe(true);
  });

  it("maps permission_type=public for collaborative preset", () => {
    const config = buildConfigFromPreset("collaborative");
    const payload = toCreateLinkPayload(["doc-1"], config);
    // collaborative has email verification but no nda
    expect(payload.permission_type).toBe("public");
    expect(payload.require_email_verification).toBe(true);
    expect(payload.download_enabled).toBe(true);
  });

  it("maps permission_type=public for standard preset", () => {
    const config = buildConfigFromPreset("standard");
    const payload = toCreateLinkPayload(["doc-1"], config);
    // standard uses email verification only, so permission_type stays public
    expect(payload.permission_type).toBe("public");
    expect(payload.require_email_verification).toBe(true);
    expect(payload.allowed_emails).toBeUndefined();
    expect(payload.allowed_domains).toBeUndefined();
  });
});

describe("toCreateLinkPayload — edge cases", () => {
  it("handles empty document IDs array", () => {
    const config = buildConfigFromPreset("public");
    const payload = toCreateLinkPayload([], config);
    expect(payload.document_ids).toEqual([]);
  });

  it("handles single document consistently with multiple", () => {
    const config = buildConfigFromPreset("standard");
    const single = toCreateLinkPayload(["doc-1"], config);
    const multiple = toCreateLinkPayload(["doc-1", "doc-2"], config);
    // Security fields should be identical
    expect(single.permission_type).toBe(multiple.permission_type);
    expect(single.require_email_verification).toBe(multiple.require_email_verification);
    // Only document_ids differ
    expect(single.document_ids).toEqual(["doc-1"]);
    expect(multiple.document_ids).toEqual(["doc-1", "doc-2"]);
  });

  it("always disables password fields regardless of legacy config values", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      passwordEnabled: true,
      password: "should-be-ignored",
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_password).toBe(false);
    expect(payload.password).toBeUndefined();
  });

  it("ignores whitelist values and always omits allowed_emails/allowed_domains", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("standard"),
      whitelistEnabled: true,
      whitelist: ["user@test.com", "@example.com"],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.allowed_emails).toBeUndefined();
    expect(payload.allowed_domains).toBeUndefined();
  });

  it("does not include contact_ids when contactIds is empty", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("standard"),
      requireEmailVerification: true,
      contactIds: [],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.contact_ids).toBeUndefined();
  });

  it("handles expiryDays:custom in a non-custom scenario", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      expiryDays: "custom",
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    // Custom expiry means no expires_at is set — server uses default or no expiry
    expect(payload.expires_at).toBeUndefined();
  });

  it("handles maxViews:unlimited", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      maxViews: "unlimited",
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.max_access_count).toBeUndefined();
  });
});

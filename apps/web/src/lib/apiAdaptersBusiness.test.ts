import { describe, it, expect } from "vitest";
import { toCreateLinkPayload } from "@/lib/apiAdapters";
import { buildConfigFromPreset } from "@/components/links/link-bundle/pipelineUtils";
import type { PermissionConfig } from "@/types";

/**
 * Edge case and business rule tests for toCreateLinkPayload.
 * Focuses on:
 * - Validation constraints (NDA+whitelist → email verification enforced)
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
  });

  it("enforces email verification when whitelist is enabled with entries", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      requireEmailVerification: false,
      whitelistEnabled: true,
      whitelist: ["user@test.com"],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_email_verification).toBe(true);
    expect(payload.allowed_emails).toEqual(["user@test.com"]);
  });

  it("enforces email verification when whitelist is enabled (even if empty)", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      whitelistEnabled: true,
      whitelist: [],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    // whitelistEnabled=true implies email verification is needed
    // (the user may add entries later, or it may be a gate signal)
    expect(payload.require_email_verification).toBe(true);
  });

  it("maps permission_type to nda when nda is primary gate", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("confidential"),
      passwordEnabled: false, // override so nda is the strongest gate
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.permission_type).toBe("nda");
  });

  it("maps permission_type to password when both password and nda are enabled (password > nda priority)", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("confidential"),
      password: "test",
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    // Priority: password > nda (matches backend normalizeSecurityConfig)
    expect(payload.permission_type).toBe("password");
  });

  it("maps permission_type=public for collaborative preset", () => {
    const config = buildConfigFromPreset("collaborative");
    const payload = toCreateLinkPayload(["doc-1"], config);
    // collaborative has email verif but no whitelist/nda/password
    expect(payload.permission_type).toBe("public");
    expect(payload.require_email_verification).toBe(true);
    expect(payload.download_enabled).toBe(true);
  });

  it("maps permission_type=public for standard preset with empty whitelist", () => {
    const config = buildConfigFromPreset("standard");
    const payload = toCreateLinkPayload(["doc-1"], config);
    // standard has whitelistEnabled=true but empty whitelist
    // so no actual whitelist entries → maps to "public"
    expect(payload.permission_type).toBe("public");
    expect(payload.require_email_verification).toBe(true);
    // allowed_emails/domains should be undefined when empty
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

  it("handles password=undefined when passwordEnabled=false", () => {
    const config = buildConfigFromPreset("public");
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_password).toBe(false);
    expect(payload.password).toBeUndefined();
  });

  it("handles password=empty string when passwordEnabled=true", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("confidential"),
      password: "",
      passwordEnabled: true,
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    // Empty password is still truthy for passwordEnabled check
    expect(payload.require_password).toBe(true);
    expect(payload.password).toBe("");
  });

  it("handles whitelist with only whitespace entries", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("standard"),
      whitelistEnabled: true,
      whitelist: ["  ", "\t", "\n"],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    // All whitespace gets trimmed and filtered
    expect(payload.allowed_emails).toBeUndefined();
    expect(payload.allowed_domains).toBeUndefined();
  });

  it("handles mixed valid and whitespace whitelist entries", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("standard"),
      whitelistEnabled: true,
      whitelist: ["  user@test.com  ", "", "  "],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.allowed_emails).toEqual(["user@test.com"]);
  });

  it("correctly identifies domain entries with @ prefix", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("standard"),
      whitelistEnabled: true,
      whitelist: ["@acme.com", "@investor.vc"],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.allowed_domains).toEqual(["@acme.com", "@investor.vc"]);
    expect(payload.allowed_emails).toBeUndefined();
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

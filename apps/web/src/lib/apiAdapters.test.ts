import { describe, it, expect } from "vitest";
import {
  toCreateLinkPayload,
  toIntegrationStatus,
  toBackendIntegrationStatus,
} from "@/lib/apiAdapters";
import { buildConfigFromPreset } from "@/components/links/link-bundle/pipelineUtils";
import type { PermissionConfig } from "@/types";

describe("toCreateLinkPayload", () => {
  it("converts a public config with single document", () => {
    const config = buildConfigFromPreset("public");
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.document_ids).toEqual(["doc-1"]);
    expect(payload.permission_type).toBe("public");
    expect(payload.require_email_verification).toBe(false);
    expect(payload.require_password).toBe(false);
    expect(payload.require_nda).toBe(false);
    expect(payload.download_enabled).toBe(false);
    expect(payload.watermark_enabled).toBe(true);
  });

  it("converts a standard config with multiple documents", () => {
    const config = buildConfigFromPreset("standard");
    const payload = toCreateLinkPayload(["doc-1", "doc-2", "doc-3"], config);
    expect(payload.document_ids).toEqual(["doc-1", "doc-2", "doc-3"]);
    // Standard preset uses email verification only, so permission_type stays "public"
    expect(payload.permission_type).toBe("public");
    expect(payload.require_email_verification).toBe(true);
  });

  it("maps confidential config (NDA) correctly", () => {
    const config = buildConfigFromPreset("confidential");
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_nda).toBe(true);
    expect(payload.nda_document_id).toBe("doc-1");
    expect(payload.require_email_verification).toBe(true);
    expect(payload.permission_type).toBe("nda");
  });

  it("NDA forces require_email_verification even when explicitly false", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      ndaEnabled: true,
      requireEmailVerification: false,
      contactIds: ["contact-nda"],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_nda).toBe(true);
    expect(payload.require_email_verification).toBe(true);
    expect(payload.permission_type).toBe("nda");
    expect(payload.contact_ids).toEqual(["contact-nda"]);
  });

  it("includes contact_ids when email verification is enabled", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("standard"),
      requireEmailVerification: true,
      contactIds: ["contact-abc"],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.contact_ids).toEqual(["contact-abc"]);
  });

  it("uses explicit ndaDocumentId when NDA is enabled", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      ndaEnabled: true,
      ndaDocumentId: "nda-doc-id",
      contactIds: ["contact-nda"],
    };
    const payload = toCreateLinkPayload(["doc-1", "doc-2"], config);
    expect(payload.require_nda).toBe(true);
    expect(payload.nda_document_id).toBe("nda-doc-id");
  });


  it("omits contact_ids when email verification is disabled", () => {
    const config = buildConfigFromPreset("public");
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.contact_ids).toBeUndefined();
  });

  it("sets expires_at from expiryDays", () => {
    const config = buildConfigFromPreset("public");
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.expires_at).toBeDefined();
    // Should be ~30 days from now
    const expiresAt = new Date(payload.expires_at!);
    const expected = new Date();
    expected.setDate(expected.getDate() + 30);
    expect(Math.abs(expiresAt.getTime() - expected.getTime())).toBeLessThan(60000); // within 1 minute
  });

  it("does not set expires_at for custom expiryDays", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      expiryDays: "custom",
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.expires_at).toBeUndefined();
  });

  it("sets max_access_count from maxViews", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      maxViews: 50,
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.max_access_count).toBe(50);
  });

  it("does not set max_access_count for unlimited views", () => {
    const config = buildConfigFromPreset("public");
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.max_access_count).toBeUndefined();
  });

  it("includes name in payload", () => {
    const config = buildConfigFromPreset("public");
    const payload = toCreateLinkPayload(["doc-1"], config, "My Bundle");
    expect(payload.name).toBe("My Bundle");
  });

  it("always sends password and whitelist fields as disabled/undefined", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("standard"),
      whitelistEnabled: true,
      whitelist: ["user@company.com", "@company.io"],
      passwordEnabled: true,
      password: "secret123",
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_password).toBe(false);
    expect(payload.password).toBeUndefined();
    expect(payload.allowed_emails).toBeUndefined();
    expect(payload.permission_type).toBe("public");
  });
});



describe("integration status adapters", () => {
  it("maps backend integration status to frontend shape", () => {
    const backend = {
      workspace_id: "ws-1",
      email_enabled: false,
      slack_connected: true,
      hubspot_connected: false,
      salesforce_connected: true,
      updated_at: "2026-07-05T00:00:00Z",
    };
    expect(toIntegrationStatus(backend)).toEqual({
      emailEnabled: false,
      slack: true,
      hubspot: false,
      zapier: false,
    });
  });

  it("defaults emailEnabled to true when backend field is missing", () => {
    expect(toIntegrationStatus({})).toEqual({
      emailEnabled: true,
      slack: false,
      hubspot: false,
      zapier: false,
    });
  });

  it("maps frontend integration status to backend shape", () => {
    const frontend = {
      emailEnabled: true,
      slack: false,
      hubspot: true,
      zapier: false,
    };
    expect(toBackendIntegrationStatus(frontend)).toEqual({
      email_enabled: true,
      slack_connected: false,
      hubspot_connected: true,
    });
  });
});

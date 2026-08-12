import { describe, it, expect } from "vitest";
import {
  toCreateLinkPayload,
  toIntegrationStatus,
  toBackendIntegrationStatus,
  toWorkspaceMember,
  toWorkspaceMembers,
  toWorkspaceSettings,
  toUpdateWorkspaceSettingsPayload,
  toWorkspaceViewerDomain,
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
    const config = buildConfigFromPreset("confidential", {
      ndaDocumentId: "nda-doc-1",
    });
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_nda).toBe(true);
    expect(payload.nda_document_id).toBe("nda-doc-1");
    expect(payload.nda_template_id).toBeUndefined();
    expect(payload.require_email_verification).toBe(true);
    expect(payload.permission_type).toBe("nda");
  });

  it("does not fall back to shared documentIds for NDA binding", () => {
    const config = buildConfigFromPreset("confidential");
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_nda).toBe(true);
    expect(payload.nda_document_id).toBeUndefined();
    expect(payload.nda_template_id).toBeUndefined();
  });

  it("NDA forces require_email_verification even when explicitly false", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      ndaEnabled: true,
      ndaDocumentId: "nda-doc-id",
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

  it("prefers nda_template_id when both template and document are set", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      ndaEnabled: true,
      ndaTemplateId: "tpl-1",
      ndaDocumentId: "nda-doc-id",
      contactIds: ["contact-nda"],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.nda_template_id).toBe("tpl-1");
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

  it("does not set expires_at for custom expiryDays without a datetime", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      expiryDays: "custom",
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.expires_at).toBeUndefined();
  });

  it("sets expires_at from _editExpiresAt for custom expiry", () => {
    const customAt = new Date();
    customAt.setDate(customAt.getDate() + 12);
    const iso = customAt.toISOString();
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      expiryDays: "custom",
      _editExpiresAt: iso,
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.expires_at).toBe(iso);
  });

  it("sets expires_at for 15-day preset", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      expiryDays: 15,
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.expires_at).toBeDefined();
    const expiresAt = new Date(payload.expires_at!);
    const expected = new Date();
    expected.setDate(expected.getDate() + 15);
    expect(Math.abs(expiresAt.getTime() - expected.getTime())).toBeLessThan(60000);
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
      dailyDigestEnabled: false,
      keyPageSlackEnabled: false,
      slack: true,
      hubspot: false,
      canManage: false,
    });
  });

  it("defaults emailEnabled to true when backend field is missing", () => {
    expect(toIntegrationStatus({})).toEqual({
      emailEnabled: true,
      dailyDigestEnabled: false,
      keyPageSlackEnabled: false,
      slack: false,
      hubspot: false,
      canManage: false,
    });
  });

  it("maps can_manage onto IntegrationStatus", () => {
    expect(toIntegrationStatus({ can_manage: true })).toEqual({
      emailEnabled: true,
      dailyDigestEnabled: false,
      keyPageSlackEnabled: false,
      slack: false,
      hubspot: false,
      canManage: true,
    });
  });

  it("maps frontend integration status to backend shape", () => {
    const frontend = {
      emailEnabled: true,
      dailyDigestEnabled: true,
      keyPageSlackEnabled: true,
      slack: false,
      hubspot: true,
      canManage: true,
    };
    expect(toBackendIntegrationStatus(frontend)).toEqual({
      email_enabled: true,
      daily_digest_enabled: true,
      key_page_slack_enabled: true,
      slack_connected: false,
      hubspot_connected: true,
    });
  });
});

describe("workspace settings adapters", () => {
  it("maps snake_case brand_color from create/get settings onto the Brand UI", () => {
    expect(
      toWorkspaceSettings({
        name: "Acme",
        slug: "acme",
        brand_color: "#0055ff",
        viewer_domain: "invest.acme.com",
        logo_url: "https://cdn.example.com/logo.png",
      }),
    ).toEqual({
      name: "Acme",
      slug: "acme",
      brandColor: "#0055ff",
      viewerDomain: "invest.acme.com",
      logoUrl: "https://cdn.example.com/logo.png",
    });
  });

  it("accepts camelCase or { data } wrappers from mocks", () => {
    expect(
      toWorkspaceSettings({
        data: { name: "Demo", slug: "demo", brandColor: "#0f172a" },
      }),
    ).toEqual({
      name: "Demo",
      slug: "demo",
      brandColor: "#0f172a",
      viewerDomain: "",
      logoUrl: "",
    });
  });

  it("sends brand_color on update so the DB column is persisted", () => {
    expect(
      toUpdateWorkspaceSettingsPayload({
        name: "Acme",
        slug: "acme",
        brandColor: "#3366ff",
        viewerDomain: "",
        logoUrl: "",
      }),
    ).toEqual({
      name: "Acme",
      slug: "acme",
      brand_color: "#3366ff",
      logo_url: undefined,
    });
  });

  it("maps viewer-domain snake_case including pending CNAME fields", () => {
    expect(
      toWorkspaceViewerDomain({
        hostname: "invest.acme.com",
        status: "pending",
        cname_host: "invest.acme.com",
        cname_target: "cname.dealsignal.com",
      }),
    ).toEqual({
      hostname: "invest.acme.com",
      status: "pending",
      cnameHost: "invest.acme.com",
      cnameTarget: "cname.dealsignal.com",
      verifiedAt: undefined,
    });
  });
});

describe("workspace member adapters", () => {
  it("maps snake_case members and pending invites", () => {
    expect(
      toWorkspaceMembers({
        data: [
          {
            id: "u_1",
            user_id: "u_1",
            email: "owner@acme.com",
            name: "",
            role: "owner",
            joined_at: "2026-01-01T00:00:00Z",
            status: "active",
          },
          {
            id: "tok_1",
            user_id: "",
            email: "pending@acme.com",
            role: "member",
            joined_at: "2026-01-02T00:00:00Z",
            status: "pending",
          },
        ],
      }),
    ).toEqual([
      {
        id: "u_1",
        userId: "u_1",
        email: "owner@acme.com",
        name: "owner@acme.com",
        role: "owner",
        joinedAt: "2026-01-01T00:00:00Z",
        status: "active",
        avatarUrl: undefined,
      },
      {
        id: "tok_1",
        userId: "",
        email: "pending@acme.com",
        name: "pending@acme.com",
        role: "member",
        joinedAt: "2026-01-02T00:00:00Z",
        status: "pending",
        avatarUrl: undefined,
      },
    ]);
  });

  it("keeps explicit names when present", () => {
    expect(
      toWorkspaceMember({
        id: "u_2",
        userId: "u_2",
        email: "jane@acme.com",
        name: "Jane Smith",
        role: "admin",
        joinedAt: "2026-01-01T00:00:00Z",
        status: "active",
      }),
    ).toMatchObject({
      name: "Jane Smith",
      email: "jane@acme.com",
      role: "admin",
    });
  });
});

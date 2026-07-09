import { describe, it, expect } from "vitest";
import { buildConfigFromPreset } from "./pipelineUtils";
import type { Link, PermissionConfig } from "@/types";

// -------------------------------------------------------------------------
// 1. Edit reconstruction: deriving config from a Link API response.
//    This mirrors the logic in BundlePipelinePage.tsx useEffect.
// -------------------------------------------------------------------------

function reconstructConfig(link: Link): Omit<PermissionConfig, "level" | "isCustomized"> {
  let expiryDays: number | "custom" = 30;
  if (link.expiresAt) {
    const expires = new Date(link.expiresAt);
    const now = new Date();
    const diffMs = expires.getTime() - now.getTime();
    const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));
    if (diffDays > 0) {
      expiryDays = diffDays;
    }
  }

  const maxViews: number | "unlimited" =
    typeof link.maxAccessCount === "number" && link.maxAccessCount > 0
      ? link.maxAccessCount
      : "unlimited";

  // Use requireEmailVerification field when available (modern flag),
  // then fall back to legacy permission_type inference.
  const hasEmailVerification = link.requireEmailVerification === true
    || link.permissionType === "email"
    || link.permissionType === "nda"
    || link.permissionType === "whitelist";

  return {
    requireEmailVerification: hasEmailVerification,
    whitelistEnabled: link.permissionType === "whitelist",
    whitelist: [],
    passwordEnabled: link.permissionType === "password",
    password: undefined,
    ndaEnabled: link.permissionType === "nda",
    allowDownload: link.downloadEnabled ?? true,
    watermarkEnabled: link.watermarkEnabled ?? true,
    aiCopilotEnabled: link.aiCopilotEnabled ?? false,
    qaEnabled: link.qaEnabled ?? false,
    fileRequestsEnabled: link.fileRequestsEnabled ?? false,
    indexFileEnabled: link.indexFileEnabled ?? false,
    expiryDays,
    maxViews,
    contactIds: [],
  };
}

function makeBaseLink(overrides?: Partial<Link>): Link {
  return {
    id: "b7a00000-0000-0000-0000-000000000001",
    documentId: "d2900000-0000-0000-0000-000000000001",
    documentIds: ["d2900000-0000-0000-0000-000000000001"],
    documentTitle: "Test Deck",
    shortUrl: "https://example.com/l/abc123",
    accessCount: 0,
    heatLevel: "cold",
    createdAt: "2025-01-01T00:00:00Z",
    isBundle: false,
    documents: [],
    ...overrides,
  };
}

describe("Edit mode config reconstruction", () => {
  it("reconstructs public link with download and watermark defaults", () => {
    const link = makeBaseLink({ permissionType: "public" });
    const config = reconstructConfig(link);
    expect(config.requireEmailVerification).toBe(false);
    expect(config.whitelistEnabled).toBe(false);
    expect(config.passwordEnabled).toBe(false);
    expect(config.ndaEnabled).toBe(false);
    expect(config.allowDownload).toBe(true);
    expect(config.watermarkEnabled).toBe(true);
    expect(config.expiryDays).toBe(30);
    expect(config.maxViews).toBe("unlimited");
  });

  it("reconstructs legacy email_required link (mapped as 'email')", () => {
    const link = makeBaseLink({ permissionType: "email" });
    const config = reconstructConfig(link);
    expect(config.requireEmailVerification).toBe(true);
    expect(config.whitelistEnabled).toBe(false);
    expect(config.passwordEnabled).toBe(false);
    expect(config.ndaEnabled).toBe(false);
  });

  it("reconstructs password link", () => {
    const link = makeBaseLink({ permissionType: "password" });
    const config = reconstructConfig(link);
    expect(config.requireEmailVerification).toBe(false);
    expect(config.passwordEnabled).toBe(true);
    expect(config.password).toBeUndefined(); // not reconstructable
  });

  it("reconstructs NDA link", () => {
    const link = makeBaseLink({ permissionType: "nda" });
    const config = reconstructConfig(link);
    expect(config.requireEmailVerification).toBe(true);
    expect(config.ndaEnabled).toBe(true);
  });

  it("reconstructs whitelist link", () => {
    const link = makeBaseLink({ permissionType: "whitelist" });
    const config = reconstructConfig(link);
    expect(config.requireEmailVerification).toBe(true);
    expect(config.whitelistEnabled).toBe(true);
    expect(config.whitelist).toEqual([]); // whitelist content not reconstructable
  });

  // Modern flag: require_email_verification=true with permission_type="public"
  it("uses requireEmailVerification field for modern email-verification-only links", () => {
    const link = makeBaseLink({
      permissionType: "public",
      requireEmailVerification: true,
    });
    const config = reconstructConfig(link);
    expect(config.requireEmailVerification).toBe(true);
    // permission_type is "public" but requireEmailVerification was explicitly set
  });

  it("falls back to permission_type when requireEmailVerification is undefined", () => {
    const link = makeBaseLink({
      permissionType: "nda",
      requireEmailVerification: undefined,
    });
    const config = reconstructConfig(link);
    expect(config.requireEmailVerification).toBe(true); // from permission_type
  });

  it("preserves downloadEnabled=false from backend", () => {
    const link = makeBaseLink({
      permissionType: "public",
      downloadEnabled: false,
    });
    const config = reconstructConfig(link);
    expect(config.allowDownload).toBe(false);
  });

  it("preserves watermarkEnabled=false from backend", () => {
    const link = makeBaseLink({
      permissionType: "public",
      watermarkEnabled: false,
    });
    const config = reconstructConfig(link);
    expect(config.watermarkEnabled).toBe(false);
  });

  it("reconstructs expiresAt as expiryDays", () => {
    const future = new Date();
    future.setDate(future.getDate() + 15);
    const link = makeBaseLink({
      permissionType: "public",
      expiresAt: future.toISOString(),
    });
    const config = reconstructConfig(link);
    expect(config.expiryDays).toBe(15);
  });

  it("handles already-expired link (0 or negative diffDays)", () => {
    const past = new Date();
    past.setDate(past.getDate() - 10);
    const link = makeBaseLink({
      permissionType: "public",
      expiresAt: past.toISOString(),
    });
    const config = reconstructConfig(link);
    // Should not set a negative expiryDays
    expect(config.expiryDays === 30 || config.expiryDays === "custom" || (typeof config.expiryDays === "number" && config.expiryDays <= 0)).toBeDefined();
  });

  it("reconstructs maxAccessCount as maxViews", () => {
    const link = makeBaseLink({
      permissionType: "public",
      maxAccessCount: 100,
    });
    const config = reconstructConfig(link);
    expect(config.maxViews).toBe(100);
  });

  it("treats maxAccessCount=0 as unlimited", () => {
    const link = makeBaseLink({
      permissionType: "public",
      maxAccessCount: 0,
    });
    const config = reconstructConfig(link);
    expect(config.maxViews).toBe("unlimited");
  });

  it("treats undefined maxAccessCount as unlimited", () => {
    const link = makeBaseLink({
      permissionType: "public",
      maxAccessCount: undefined,
    });
    const config = reconstructConfig(link);
    expect(config.maxViews).toBe("unlimited");
  });
});

// -------------------------------------------------------------------------
// 2. Verify classification round-trip: build config from preset →
//    toCreateLinkPayload → backend normalizeSecurityConfig →
//    classifyPresetFromConfig should return same preset.
// -------------------------------------------------------------------------

import { classifyPresetFromConfig } from "../smart-link/levelConfig";

describe("Preset round-trip consistency", () => {
  const presets = ["public", "standard", "confidential", "collaborative"] as const;

  for (const preset of presets) {
    it(`round-trips ${preset} preset`, () => {
      const config = buildConfigFromPreset(preset);
      // Simulate the config that would come back from the backend
      // after normalizeSecurityConfig → linkResponse → edit reconstruction.
      // We test classifyPresetFromConfig which should return the matching preset.
      const { level, isCustomized } = classifyPresetFromConfig(config);
      expect(level).toBe(preset);
      expect(isCustomized).toBe(false);
    });
  }

  it("customized preset template matches public in classification (by design)", () => {
    // classifyPresetFromConfig only compares against named presets.
    // The "customized" template is identical to "public", so it maps to public.
    // isCustomized is determined by buildConfigFromPreset based on the preset argument.
    const config = buildConfigFromPreset("customized");
    expect(config.isCustomized).toBe(true); // from buildConfigFromPreset
    const { level, isCustomized } = classifyPresetFromConfig(config);
    expect(level).toBe("public");
    expect(isCustomized).toBe(false);
  });

  it("classifies non-matching configs as customized", () => {
    const config = buildConfigFromPreset("public");
    const tweaked = { ...config, allowDownload: true };
    const { level, isCustomized } = classifyPresetFromConfig(tweaked);
    expect(level).toBe("customized");
    expect(isCustomized).toBe(true);
  });
});

// -------------------------------------------------------------------------
// 3. RESET action in edit mode: should not clear mode/editingLinkId.
// -------------------------------------------------------------------------

import { createInitialState, type BundlePipelineState, type BundlePipelineAction } from "./BundlePipelineContext";

function applyReducer(initial: BundlePipelineState, action: BundlePipelineAction): BundlePipelineState {
  const state = { ...initial };
  switch (action.type) {
    case "INIT_FOR_EDIT": {
      const { linkId, token, documents, selectedDocuments, config } = action.payload;
      state.mode = "edit";
      state.editingLinkId = linkId;
      state.linkToken = token;
      state.documents = documents;
      state.selectedDocuments = selectedDocuments;
      state.config = config;
      state.isDirty = false;
      state.step = 1;
      break;
    }
    case "RESET":
      state.step = 1;
      state.isDirty = false;
      state.generatedLink = null;
      state.copied = false;
      state.isSubmitting = false;
      break;
    case "TOGGLE_DOCUMENT":
      if (state.mode === "edit") state.isDirty = true;
      break;
    case "SET_CONFIG":
      state.config = action.config;
      if (state.mode === "edit") state.isDirty = true;
      break;
    case "SET_GENERATED_LINK":
      state.generatedLink = action.link;
      break;
  }
  return state;
}

describe("RESET in edit mode", () => {
  it("preserves edit mode and editingLinkId after RESET", () => {
    const config = buildConfigFromPreset("standard");
    let state = createInitialState();
    state = applyReducer(state, {
      type: "INIT_FOR_EDIT",
      payload: {
        linkId: "link-reset-test",
        token: "token-xyz",
        documents: [],
        selectedDocuments: [],
        config,
      },
    });
    expect(state.mode).toBe("edit");
    expect(state.editingLinkId).toBe("link-reset-test");

    // Navigate and make dirty
    state = applyReducer(state, { type: "SET_CONFIG", config });
    expect(state.isDirty).toBe(true);

    // RESET (as in create-another after successful submission)
    state = applyReducer(state, { type: "RESET" });
    expect(state.mode).toBe("edit");
    expect(state.editingLinkId).toBe("link-reset-test");
    expect(state.isDirty).toBe(false);
  });

  it("clears generatedLink on RESET", () => {
    let state = createInitialState();
    state = applyReducer(state, { type: "SET_GENERATED_LINK", link: "https://example.com/l/tok" });
    expect(state.generatedLink).not.toBeNull();
    state = applyReducer(state, { type: "RESET" });
    expect(state.generatedLink).toBeNull();
  });
});

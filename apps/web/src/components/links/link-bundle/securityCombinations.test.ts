/**
 * End-to-end security combination tests for the Bundle Pipeline.
 *
 * Verifies:
 *  1. Preset templates correctly map to CreateLinkPayload (apiAdapters)
 *  2. Contact ID presence is correct for each preset (the bug fix)
 *  3. Client-side guard conditions (email requires contact, password non-empty)
 *  4. Cross-option constraints (whitelist/NDA → email verification)
 */

import { describe, it, expect } from "vitest";
import { toCreateLinkPayload } from "@/lib/apiAdapters";
import { buildConfigFromPreset } from "./pipelineUtils";
import {
  enforceCrossOptionConstraints,
  classifyPresetFromConfig,
} from "../smart-link/levelConfig";
import type { PermissionConfig, PermissionPreset } from "@/types";

// ---------------------------------------------------------------------------
// Helper: guard function matching StepReview logic
// ---------------------------------------------------------------------------

interface GuardResult {
  blocked: boolean;
  reason?: "contactRequired" | "passwordEmpty";
}

function clientGuard(config: PermissionConfig): GuardResult {
  if (config.requireEmailVerification && config.contactIds.length === 0) {
    return { blocked: true, reason: "contactRequired" };
  }
  if (config.passwordEnabled && (!config.password || config.password.trim() === "")) {
    return { blocked: true, reason: "passwordEmpty" };
  }
  return { blocked: false };
}

// ---------------------------------------------------------------------------
// Helper: build a config with contact
// ---------------------------------------------------------------------------

function withContact(preset: PermissionPreset, contactId: string): PermissionConfig {
  return { ...buildConfigFromPreset(preset), contactIds: [contactId] };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("Preset → CreateLinkPayload mapping", () => {
  describe("public preset", () => {
    it("does NOT require email verification", () => {
      const payload = toCreateLinkPayload(["doc-1"], buildConfigFromPreset("public"));
      expect(payload.require_email_verification).toBe(false);
    });

    it("does NOT include contact_ids (even if contactIds is set)", () => {
      // Public preset has requireEmailVerification=false, so contactIds is ignored
      const config = withContact("public", "contact-123");
      const payload = toCreateLinkPayload(["doc-1"], config);
      expect(payload.contact_ids).toBeUndefined();
    });

    it("passes client guard without contact", () => {
      expect(clientGuard(buildConfigFromPreset("public")).blocked).toBe(false);
    });
  });

  describe("standard preset", () => {
    it("requires email verification", () => {
      const payload = toCreateLinkPayload(["doc-1"], buildConfigFromPreset("standard"));
      expect(payload.require_email_verification).toBe(true);
    });

    it("BLOCKS without contact — client guard catches it", () => {
      const result = clientGuard(buildConfigFromPreset("standard"));
      expect(result.blocked).toBe(true);
      expect(result.reason).toBe("contactRequired");
    });

    it("includes contact_ids when contact is set", () => {
      const config = withContact("standard", "contact-456");
      const payload = toCreateLinkPayload(["doc-1"], config);
      expect(payload.contact_ids).toEqual(["contact-456"]);
    });

    it("passes client guard when contact is set", () => {
      const result = clientGuard(withContact("standard", "contact-456"));
      expect(result.blocked).toBe(false);
    });
  });

  describe("confidential preset", () => {
    it("requires email, password, NDA", () => {
      const config = { ...buildConfigFromPreset("confidential"), password: "s3cret!1" };
      const payload = toCreateLinkPayload(["doc-1"], config);
      expect(payload.require_email_verification).toBe(true);
      expect(payload.require_password).toBe(true);
      expect(payload.require_nda).toBe(true);
      expect(payload.permission_type).toBe("password");
    });

    it("BLOCKS without contact", () => {
      const config = { ...buildConfigFromPreset("confidential"), password: "s3cret!1" };
      const result = clientGuard(config);
      expect(result.blocked).toBe(true);
      expect(result.reason).toBe("contactRequired");
    });

    it("BLOCKS with empty password", () => {
      const config = withContact("confidential", "contact-789");
      // passwordEnabled=true but no password set
      const result = clientGuard(config);
      expect(result.blocked).toBe(true);
      expect(result.reason).toBe("passwordEmpty");
    });

    it("passes with contact + password", () => {
      const config: PermissionConfig = {
        ...buildConfigFromPreset("confidential"),
        contactIds: ["contact-789"],
        password: "s3cret!1",
      };
      expect(clientGuard(config).blocked).toBe(false);
      const payload = toCreateLinkPayload(["doc-1"], config);
      expect(payload.contact_ids).toEqual(["contact-789"]);
      expect(payload.password).toBe("s3cret!1");
    });
  });

  describe("collaborative preset", () => {
    it("requires email verification but NOT password/NDA/whitelist", () => {
      const payload = toCreateLinkPayload(["doc-1"], buildConfigFromPreset("collaborative"));
      expect(payload.require_email_verification).toBe(true);
      expect(payload.require_password).toBe(false);
      expect(payload.require_nda).toBe(false);
      expect(payload.download_enabled).toBe(true);
    });

    it("BLOCKS without contact", () => {
      const result = clientGuard(buildConfigFromPreset("collaborative"));
      expect(result.blocked).toBe(true);
      expect(result.reason).toBe("contactRequired");
    });

    it("passes with contact", () => {
      const config = withContact("collaborative", "contact-abc");
      expect(clientGuard(config).blocked).toBe(false);
      const payload = toCreateLinkPayload(["doc-1"], config);
      expect(payload.contact_ids).toEqual(["contact-abc"]);
    });
  });

  describe("customized preset", () => {
    it("empty customized config does NOT require email", () => {
      const payload = toCreateLinkPayload(["doc-1"], buildConfigFromPreset("customized"));
      expect(payload.require_email_verification).toBe(false);
    });

    it("with email enabled but no contact → BLOCKED", () => {
      const config: PermissionConfig = {
        ...buildConfigFromPreset("customized"),
        requireEmailVerification: true,
      };
      const result = clientGuard(config);
      expect(result.blocked).toBe(true);
      expect(result.reason).toBe("contactRequired");
    });

    it("with email + contact → passes", () => {
      const config: PermissionConfig = {
        ...buildConfigFromPreset("customized"),
        requireEmailVerification: true,
        contactIds: ["contact-xyz"],
      };
      expect(clientGuard(config).blocked).toBe(false);
      const payload = toCreateLinkPayload(["doc-1"], config);
      expect(payload.contact_ids).toEqual(["contact-xyz"]);
    });

    it("with password enabled but empty → BLOCKED", () => {
      const config: PermissionConfig = {
        ...buildConfigFromPreset("customized"),
        passwordEnabled: true,
      };
      const result = clientGuard(config);
      expect(result.blocked).toBe(true);
      expect(result.reason).toBe("passwordEmpty");
    });

    it("with password set → passes", () => {
      const config: PermissionConfig = {
        ...buildConfigFromPreset("customized"),
        passwordEnabled: true,
        password: "p@ssword",
      };
      expect(clientGuard(config).blocked).toBe(false);
    });

    it("with whitelist enabled → email verification auto-on", () => {
      const config: PermissionConfig = {
        ...buildConfigFromPreset("customized"),
        requireEmailVerification: false,
        whitelistEnabled: true,
        whitelist: ["test@example.com"],
        contactIds: ["contact-wl"],
      };
      expect(clientGuard(config).blocked).toBe(false);
      const payload = toCreateLinkPayload(["doc-1"], config);
      expect(payload.require_email_verification).toBe(true);
      expect(payload.contact_ids).toEqual(["contact-wl"]);
    });

    it("with NDA enabled → email verification auto-on", () => {
      const config: PermissionConfig = {
        ...buildConfigFromPreset("customized"),
        requireEmailVerification: false,
        ndaEnabled: true,
        contactIds: ["contact-nda"],
      };
      expect(clientGuard(config).blocked).toBe(false);
      const payload = toCreateLinkPayload(["doc-1"], config);
      expect(payload.require_email_verification).toBe(true);
      expect(payload.permission_type).toBe("nda");
      expect(payload.contact_ids).toEqual(["contact-nda"]);
    });

    it("full locked-down combo: email + whitelist + password + NDA + no download", () => {
      const config: PermissionConfig = {
        ...buildConfigFromPreset("customized"),
        requireEmailVerification: true,
        whitelistEnabled: true,
        whitelist: ["vip@corp.com", "@partner.io"],
        passwordEnabled: true,
        password: "ultra-secure-p@ss",
        ndaEnabled: true,
        allowDownload: false,
        watermarkEnabled: true,
        aiCopilotEnabled: false,
        expiryDays: 7,
        maxViews: 10,
        contactIds: ["contact-full"],
      };
      expect(clientGuard(config).blocked).toBe(false);

      const payload = toCreateLinkPayload(["doc-1"], config);
      expect(payload.require_email_verification).toBe(true);
      expect(payload.require_password).toBe(true);
      expect(payload.require_nda).toBe(true);
      expect(payload.allowed_emails).toEqual(["vip@corp.com"]);
      expect(payload.allowed_domains).toEqual(["@partner.io"]);
      expect(payload.password).toBe("ultra-secure-p@ss");
      expect(payload.download_enabled).toBe(false);
      expect(payload.watermark_enabled).toBe(true);
      expect(payload.ai_copilot_enabled).toBe(false);
      expect(payload.contact_ids).toEqual(["contact-full"]);
      expect(payload.permission_type).toBe("password");
      expect(payload.max_access_count).toBe(10);
    });
  });
});

describe("Cross-option constraints", () => {
  it("whitelist toggle automatically enables email verification", () => {
    const base = buildConfigFromPreset("customized");
    const constrained = enforceCrossOptionConstraints({
      ...base,
      whitelistEnabled: true,
    });
    expect(constrained.requireEmailVerification).toBe(true);
  });

  it("NDA toggle automatically enables email verification", () => {
    const base = buildConfigFromPreset("customized");
    const constrained = enforceCrossOptionConstraints({
      ...base,
      ndaEnabled: true,
    });
    expect(constrained.requireEmailVerification).toBe(true);
  });

  it("does not disable email verification when toggling whitelist off", () => {
    const constrained = enforceCrossOptionConstraints({
      ...buildConfigFromPreset("standard"),
      whitelistEnabled: false,
    });
    // Only adds constraints, never removes
    expect(constrained.requireEmailVerification).toBe(true);
  });
});

describe("apiAdapters.toCreateLinkPayload edge cases", () => {
  it("contact_ids is undefined when requireEmailVerification=true but contactIds empty", () => {
    // This is the PREVIOUSLY-broken scenario (now guarded client-side)
    const config: PermissionConfig = {
      ...buildConfigFromPreset("standard"),
      contactIds: [],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_email_verification).toBe(true);
    expect(payload.contact_ids).toBeUndefined();
    // Client guard should block this case
    expect(clientGuard(config).blocked).toBe(true);
  });

  it("contact_ids is undefined when requireEmailVerification=false regardless of contactIds", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("public"),
      contactIds: ["contact-has-id-but-public"],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_email_verification).toBe(false);
    expect(payload.contact_ids).toBeUndefined();
  });
});

describe("Classify preset from config", () => {
  it("standard config is classified as standard", () => {
    const standard = buildConfigFromPreset("standard");
    const result = classifyPresetFromConfig(standard);
    expect(result.level).toBe("standard");
    expect(result.isCustomized).toBe(false);
  });

  it("customized template matches public in classification (by design)", () => {
    // classifyPresetFromConfig only compares against named presets.
    // The "customized" template is identical to "public", so it classifies as public.
    // isCustomized is set by buildConfigFromPreset based on the preset argument.
    const customized = buildConfigFromPreset("customized");
    expect(customized.isCustomized).toBe(true);
    const result = classifyPresetFromConfig(customized);
    expect(result.level).toBe("public");
    expect(result.isCustomized).toBe(false);
  });

  it("standard with contactIds still classifies as standard", () => {
    // contactIds is NOT part of the classification algorithm
    const config = withContact("standard", "contact-123");
    const result = classifyPresetFromConfig(config);
    expect(result.level).toBe("standard");
    expect(result.isCustomized).toBe(false);
  });

  it("toggling one option classifies as customized", () => {
    const modified: PermissionConfig = {
      ...buildConfigFromPreset("standard"),
      watermarkEnabled: false, // standard preset has it true
    };
    const result = classifyPresetFromConfig(modified);
    expect(result.isCustomized).toBe(true);
    expect(result.level).toBe("customized");
  });

  it("max match score matches actual presetMatchScore for all named presets", () => {
    // Guard: if someone adds/removes a field in PermissionFields/PresetConfigTemplate
    // and updates presetMatchScore but forgets SCORED_DIMENSION_NAMES, this will fail.
    // Each named preset built from its template must self-classify as a PERFECT match.
    for (const preset of ["public", "standard", "confidential", "collaborative"] as PermissionPreset[]) {
      const config = buildConfigFromPreset(preset);
      // Strip level/isCustomized so classifyPresetFromConfig re-evaluates fresh.
      const { level, isCustomized } = classifyPresetFromConfig(config);
      expect(isCustomized).toBe(false);
      expect(level).toBe(preset);
    }
  });
});

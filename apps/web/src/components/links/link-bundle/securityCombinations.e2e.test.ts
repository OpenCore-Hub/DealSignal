/**
 * End-to-end security option combination tests.
 *
 * Verifies the real end-to-end chain:
 *   frontend PermissionConfig → toCreateLinkPayload (exact JSON) →
 *   backend-normalized storage → access gate simulation
 *
 * Covered switches:
 *   - requireEmailVerification
 *   - ndaEnabled
 *   - allowDownload
 *   - watermarkEnabled
 *
 * Constraints:
 *   1. ndaEnabled → requireEmailVerification = true
 *   2. requireEmailVerification=true AND contactIds=[] → client guard blocks submission
 */

import { describe, it, expect } from "vitest";
import { toCreateLinkPayload } from "@/lib/apiAdapters";
import { buildConfigFromPreset } from "./pipelineUtils";
import {
  enforceCrossOptionConstraints,
  classifyPresetFromConfig,
} from "../smart-link/levelConfig";
import type { PermissionConfig, PermissionPreset } from "@/types";

// ============================================================================
// Constants
// ============================================================================

const BOOL_FIELDS = [
  "requireEmailVerification",
  "ndaEnabled",
  "allowDownload",
  "watermarkEnabled",
] as const;

const EXPIRY_VALUES = [7, 30, 90, "custom"] as const;
const MAX_VIEWS_VALUES = ["unlimited", 10, 50, 100] as const;

const NAMED_PRESETS: PermissionPreset[] = [
  "public",
  "standard",
  "confidential",
  "collaborative",
];

// ============================================================================
// Backend normalization simulation
// ============================================================================

interface StoreResult {
  requireEmailVerification: boolean;
  requireNda: boolean;
  downloadEnabled: boolean;
  watermarkEnabled: boolean;
  aiCopilotEnabled: boolean;
  expiresAt: string | null;
  maxAccessCount: number | null;
  contactIds: string[];
  permissionType: string;
}

function normalizeAndStoreConfig(
  payload: ReturnType<typeof toCreateLinkPayload>,
  contactIds: string[],
): StoreResult {
  const requireNda = payload.require_nda ?? false;
  let requireEmail = payload.require_email_verification ?? false;

  // NDA forces email verification so the signer identity is recorded.
  if (!requireEmail && requireNda) {
    requireEmail = true;
  }

  return {
    requireEmailVerification: requireEmail,
    requireNda,
    downloadEnabled: payload.download_enabled ?? false,
    watermarkEnabled: payload.watermark_enabled ?? true,
    aiCopilotEnabled: payload.ai_copilot_enabled ?? false,
    expiresAt: payload.expires_at ?? null,
    maxAccessCount: payload.max_access_count ?? null,
    contactIds: requireEmail && contactIds.length > 0 ? [...contactIds] : [],
    permissionType: payload.permission_type ?? "public",
  };
}

// ============================================================================
// Backend access gate simulation
// ============================================================================

interface GateResult {
  granted: boolean;
  errorCode: string | null;
  requiresEmailVerification: boolean;
  requiresNda: boolean;
}

interface AccessRequest {
  email: string;
  emailCode: string;
  ndaAgreed: boolean;
}

function accessGate(store: StoreResult, req: AccessRequest): GateResult {
  const requiresEmailVerification = store.requireEmailVerification;
  const requiresNda = store.requireNda;

  if (requiresEmailVerification && req.emailCode.trim() === "") {
    return {
      granted: false,
      errorCode: "requires_email_code",
      requiresEmailVerification,
      requiresNda,
    };
  }

  if (requiresNda && !req.ndaAgreed) {
    return {
      granted: false,
      errorCode: "nda_required",
      requiresEmailVerification,
      requiresNda,
    };
  }

  return {
    granted: true,
    errorCode: null,
    requiresEmailVerification,
    requiresNda,
  };
}

// ============================================================================
// Client guard simulation
// ============================================================================

interface GuardResult {
  blocked: boolean;
  reason?: "contactRequired";
}

function clientGuard(config: PermissionConfig): GuardResult {
  const requireEmail = config.requireEmailVerification || config.ndaEnabled;
  if (requireEmail && config.contactIds.length === 0) {
    return { blocked: true, reason: "contactRequired" };
  }
  return { blocked: false };
}

// ============================================================================
// Helpers
// ============================================================================

function withContact(
  src: PermissionPreset | PermissionConfig,
  contactId: string = "test-contact",
): PermissionConfig {
  const base = typeof src === "string" ? buildConfigFromPreset(src) : src;
  return { ...base, contactIds: [contactId] };
}

// ============================================================================
// Part 1: Exact JSON output verification
// ============================================================================

describe("Exact JSON output (toCreateLinkPayload → JSON → backend parse)", () => {
  it("image-state config → exact JSON match", () => {
    const config: PermissionConfig = {
      level: "customized",
      isCustomized: true,
      requireEmailVerification: false,
      whitelistEnabled: false,
      whitelist: [],
      passwordEnabled: false,
      ndaEnabled: false,
      allowDownload: true,
      watermarkEnabled: true,
      aiCopilotEnabled: false,
      qaEnabled: false,
      fileRequestsEnabled: false,
      indexFileEnabled: false,
      expiryDays: 30,
      maxViews: "unlimited",
      contactIds: [],
    };

    const payload = toCreateLinkPayload(["doc-1", "doc-2"], config);
    expect(payload.document_ids).toEqual(["doc-1", "doc-2"]);

    expect(payload.require_email_verification).toBe(false);
    expect(payload.require_password).toBe(false);
    expect(payload.require_nda).toBe(false);
    expect(payload.allowed_emails).toBeUndefined();
    expect(payload.allowed_domains).toBeUndefined();
    expect(payload.password).toBeUndefined();
    expect(payload.contact_ids).toBeUndefined();
    expect(payload.download_enabled).toBe(true);
    expect(payload.watermark_enabled).toBe(true);
    expect(payload.ai_copilot_enabled).toBe(false);
    expect(payload.permission_type).toBe("public");

    const json = JSON.stringify(payload);
    expect(json).toContain('"require_email_verification":false');
    expect(json).toContain('"require_password":false');
    expect(json).toContain('"download_enabled":true');
    expect(json).not.toContain('"require_email_verification":true');
  });

  it("standard + contact → correct serialization", () => {
    const config = withContact("standard");
    const payload = toCreateLinkPayload(["doc-1"], config);
    const json = JSON.stringify(payload);

    expect(json).toContain('"require_email_verification":true');
    expect(json).toContain('"contact_ids":["test-contact"]');
    expect(payload.permission_type).toBe("public");
  });

  it("confidential with NDA → NDA fields serialized", () => {
    const config = withContact("confidential");
    const payload = toCreateLinkPayload(["doc-1"], config);
    const json = JSON.stringify(payload);

    expect(json).toContain('"require_email_verification":true');
    expect(json).toContain('"require_nda":true');
    expect(json).toContain('"permission_type":"nda"');
    expect(payload.require_password).toBe(false);
    expect(payload.password).toBeUndefined();
  });
});

// ============================================================================
// Part 2: Full chain normalization → access gate
// ============================================================================

describe("normalizeAndStore → accessGate full chain", () => {
  function fullFlow(
    config: PermissionConfig,
    accessReq: AccessRequest = { email: "", emailCode: "", ndaAgreed: false },
  ): { store: StoreResult; gate: GateResult } {
    const payload = toCreateLinkPayload(["doc-1"], config);
    const store = normalizeAndStoreConfig(payload, config.contactIds);
    const gate = accessGate(store, accessReq);
    return { store, gate };
  }

  it("image-state: direct access, no gates", () => {
    const config: PermissionConfig = {
      level: "customized",
      isCustomized: true,
      requireEmailVerification: false,
      whitelistEnabled: false,
      whitelist: [],
      passwordEnabled: false,
      ndaEnabled: false,
      allowDownload: true,
      watermarkEnabled: true,
      aiCopilotEnabled: false,
      qaEnabled: false,
      fileRequestsEnabled: false,
      indexFileEnabled: false,
      expiryDays: 30,
      maxViews: "unlimited",
      contactIds: [],
    };

    const { store, gate } = fullFlow(config);

    expect(store.requireEmailVerification).toBe(false);
    expect(store.requireNda).toBe(false);
    expect(store.permissionType).toBe("public");

    expect(gate.granted).toBe(true);
    expect(gate.errorCode).toBeNull();
    expect(gate.requiresEmailVerification).toBe(false);
    expect(gate.requiresNda).toBe(false);
  });

  it("standard: requires email code, blocked without code", () => {
    const config = withContact("standard");

    const { gate: blocked } = fullFlow(config, {
      email: "user@corp.com",
      emailCode: "",
      ndaAgreed: false,
    });
    expect(blocked.granted).toBe(false);
    expect(blocked.errorCode).toBe("requires_email_code");

    const { gate: passed } = fullFlow(config, {
      email: "user@corp.com",
      emailCode: "123456",
      ndaAgreed: false,
    });
    expect(passed.granted).toBe(true);
  });

  it("confidential: requires email code + NDA", () => {
    const config = withContact("confidential");

    const { gate: noCode } = fullFlow(config, {
      email: "legal@firm.com",
      emailCode: "",
      ndaAgreed: true,
    });
    expect(noCode.granted).toBe(false);
    expect(noCode.errorCode).toBe("requires_email_code");

    const { gate: noNda } = fullFlow(config, {
      email: "legal@firm.com",
      emailCode: "123456",
      ndaAgreed: false,
    });
    expect(noNda.granted).toBe(false);
    expect(noNda.errorCode).toBe("nda_required");

    const { gate: allOk } = fullFlow(config, {
      email: "legal@firm.com",
      emailCode: "123456",
      ndaAgreed: true,
    });
    expect(allOk.granted).toBe(true);
  });
});

// ============================================================================
// Part 3: Cartesian product of the 4 boolean switches
// ============================================================================

describe("Boolean switch cartesian product → backend storage correctness", () => {
  function boolCombinations(): Record<(typeof BOOL_FIELDS)[number], boolean>[] {
    const results: Record<(typeof BOOL_FIELDS)[number], boolean>[] = [];
    const total = 1 << BOOL_FIELDS.length;
    for (let i = 0; i < total; i++) {
      const combo = {} as Record<(typeof BOOL_FIELDS)[number], boolean>;
      for (let j = 0; j < BOOL_FIELDS.length; j++) {
        combo[BOOL_FIELDS[j]] = ((i >> j) & 1) === 1;
      }
      results.push(combo);
    }
    return results;
  }

  it.each(boolCombinations())(
    "combo: email=%s nda=%s dl=%s wm=%s → storage consistent",
    (combo) => {
      const config: PermissionConfig = {
        level: "customized",
        isCustomized: true,
        requireEmailVerification: combo.requireEmailVerification,
        whitelistEnabled: false,
        whitelist: [],
        passwordEnabled: false,
        ndaEnabled: combo.ndaEnabled,
        allowDownload: combo.allowDownload,
        watermarkEnabled: combo.watermarkEnabled,
        aiCopilotEnabled: false,
        qaEnabled: false,
        fileRequestsEnabled: false,
        indexFileEnabled: false,
        expiryDays: 30,
        maxViews: "unlimited",
        contactIds:
          combo.requireEmailVerification || combo.ndaEnabled
            ? ["test-contact"]
            : [],
      };

      const guard = clientGuard(config);
      if (guard.blocked) {
        return;
      }

      const payload = toCreateLinkPayload(["doc-1"], config);
      const store = normalizeAndStoreConfig(payload, config.contactIds);

      const expectedRequireEmail =
        combo.requireEmailVerification || combo.ndaEnabled;
      expect(store.requireEmailVerification).toBe(expectedRequireEmail);

      if (combo.ndaEnabled) {
        expect(store.requireEmailVerification).toBe(true);
      }

      expect(store.requireNda).toBe(combo.ndaEnabled);
      expect(store.downloadEnabled).toBe(combo.allowDownload);
      expect(store.watermarkEnabled).toBe(combo.watermarkEnabled);

      if (store.requireEmailVerification) {
        expect(store.contactIds.length).toBeGreaterThanOrEqual(1);
      }

      expect(store.permissionType).toBe(combo.ndaEnabled ? "nda" : "public");
    },
  );
});

// ============================================================================
// Part 4: Advanced settings Expiry × MaxViews
// ============================================================================

describe("Advanced settings Expiry × MaxViews storage", () => {
  it.each(EXPIRY_VALUES)("expiryDays=%s → expiresAt correct", (expiry) => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("customized"),
      expiryDays: expiry,
    };
    const payload = toCreateLinkPayload(["doc-1"], config);

    if (typeof expiry === "number") {
      expect(payload.expires_at).toBeDefined();
      expect(() => new Date(payload.expires_at!)).not.toThrow();
    } else {
      expect(payload.expires_at).toBeUndefined();
    }
  });

  it.each(MAX_VIEWS_VALUES)("maxViews=%s → max_access_count correct", (maxV) => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("customized"),
      maxViews: maxV,
    };
    const payload = toCreateLinkPayload(["doc-1"], config);

    if (typeof maxV === "number") {
      expect(payload.max_access_count).toBe(maxV);
    } else {
      expect(payload.max_access_count).toBeUndefined();
    }
  });
});

// ============================================================================
// Part 5: permission_type derivation
// ============================================================================

describe("permission_type derivation (nda > public)", () => {
  function getPermType(config: PermissionConfig): string {
    const payload = toCreateLinkPayload(["doc-1"], config);
    const store = normalizeAndStoreConfig(payload, config.contactIds);
    return store.permissionType;
  }

  it("NDA enabled → nda", () => {
    expect(
      getPermType({ ...withContact(buildConfigFromPreset("customized")), ndaEnabled: true }),
    ).toBe("nda");
  });

  it("email verification only → public", () => {
    expect(
      getPermType({ ...withContact(buildConfigFromPreset("customized")), requireEmailVerification: true }),
    ).toBe("public");
  });

  it("all off → public", () => {
    expect(getPermType(buildConfigFromPreset("public"))).toBe("public");
  });
});

// ============================================================================
// Part 6: Client guard block conditions
// ============================================================================

describe("Client guard block conditions", () => {
  it("email enabled but no contact → contactRequired", () => {
    expect(
      clientGuard({ ...buildConfigFromPreset("standard"), contactIds: [] }).reason,
    ).toBe("contactRequired");
  });

  it("NDA enabled but no contact → contactRequired", () => {
    expect(
      clientGuard({
        ...buildConfigFromPreset("customized"),
        ndaEnabled: true,
      }).reason,
    ).toBe("contactRequired");
  });
});

// ============================================================================
// Part 7: Cross-option constraints
// ============================================================================

describe("Cross-option constraints", () => {
  it("NDA ON → email ON", () => {
    const result = enforceCrossOptionConstraints({
      ...buildConfigFromPreset("customized"),
      requireEmailVerification: false,
      ndaEnabled: true,
    });
    expect(result.requireEmailVerification).toBe(true);
  });

  it("email already ON → constraints have no side effects", () => {
    const result = enforceCrossOptionConstraints({
      ...buildConfigFromPreset("standard"),
      ndaEnabled: true,
    });
    expect(result.requireEmailVerification).toBe(true);
  });
});

// ============================================================================
// Part 8: Preset templates & classifier
// ============================================================================

describe("Preset templates & classifier", () => {
  it.each(NAMED_PRESETS)("%s self-classifies correctly", (preset) => {
    const config = buildConfigFromPreset(preset);
    const { level, isCustomized } = classifyPresetFromConfig(config);
    expect(level).toBe(preset);
    expect(isCustomized).toBe(false);
  });

  it("modifying any field classifies as customized", () => {
    const standard = buildConfigFromPreset("standard");
    const modified = { ...standard, watermarkEnabled: false };
    const { isCustomized } = classifyPresetFromConfig(modified);
    expect(isCustomized).toBe(true);
  });
});

// ============================================================================
// Part 9: Regression scenarios
// ============================================================================

describe("Regression — key scenarios", () => {
  it("scenario A: public — zero gates, direct access", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("customized"),
      requireEmailVerification: false,
      ndaEnabled: false,
      allowDownload: true,
      watermarkEnabled: true,
      contactIds: [],
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    const store = normalizeAndStoreConfig(payload, []);
    const gate = accessGate(store, { email: "", emailCode: "", ndaAgreed: false });

    expect(gate.granted).toBe(true);
    expect(gate.requiresEmailVerification).toBe(false);
    expect(gate.requiresNda).toBe(false);

    const json = JSON.stringify(payload);
    expect(json).toContain('"require_email_verification":false');
    expect(json).not.toContain('"require_email_verification":true');
  });

  it("scenario C: NDA only — forces email verification + NDA gate", () => {
    const config = withContact(buildConfigFromPreset("customized"));
    config.ndaEnabled = true;

    const payload = toCreateLinkPayload(["doc-1"], config);
    const store = normalizeAndStoreConfig(payload, config.contactIds);

    expect(store.requireEmailVerification).toBe(true);
    expect(store.requireNda).toBe(true);

    const noCode = accessGate(store, {
      email: "test@x.com",
      emailCode: "",
      ndaAgreed: true,
    });
    expect(noCode.errorCode).toBe("requires_email_code");

    const noNda = accessGate(store, {
      email: "test@x.com",
      emailCode: "123456",
      ndaAgreed: false,
    });
    expect(noNda.errorCode).toBe("nda_required");

    const ok = accessGate(store, {
      email: "test@x.com",
      emailCode: "123456",
      ndaAgreed: true,
    });
    expect(ok.granted).toBe(true);
  });

  it("scenario E: download + watermark only — free distribution", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("customized"),
      allowDownload: true,
      watermarkEnabled: true,
    };
    const payload = toCreateLinkPayload(["doc-1"], config);
    const store = normalizeAndStoreConfig(payload, []);

    expect(store.requireEmailVerification).toBe(false);
    expect(store.permissionType).toBe("public");
    expect(store.downloadEnabled).toBe(true);

    const gate = accessGate(store, { email: "", emailCode: "", ndaAgreed: false });
    expect(gate.granted).toBe(true);
  });
});

// ============================================================================
// Part 10: Edit mode round-trip
// ============================================================================

describe("Edit mode round-trip", () => {
  it("public link stays public after edit (no accidental email enable)", () => {
    const config: PermissionConfig = {
      ...buildConfigFromPreset("customized"),
      requireEmailVerification: false,
      ndaEnabled: false,
      allowDownload: true,
      watermarkEnabled: true,
      contactIds: [],
    };

    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_email_verification).toBe(false);
    expect(JSON.stringify(payload)).toContain('"require_email_verification":false');
  });

  it("standard link keeps email verification after edit", () => {
    const config = withContact("standard");
    const payload = toCreateLinkPayload(["doc-1"], config);
    expect(payload.require_email_verification).toBe(true);
  });
});

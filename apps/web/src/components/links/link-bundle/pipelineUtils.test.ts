import { describe, it, expect } from "vitest";
import {
  buildConfigFromPreset,
  buildEditModeDocumentLists,
  validateBundleSecurityConfig,
  resolveDraftDocumentRestore,
  resolveExpiryDaysFromExpiresAt,
  resolveMaxViewsFromAccessCount,
  resolveShareDocumentReadiness,
} from "./pipelineUtils";
import { PRESET_TEMPLATES } from "../smart-link/levelConfig";
import type { Document, DocumentSummary, PermissionPreset } from "@/types";

const libraryDoc: Document = {
  id: "doc-lib",
  title: "Library Deck",
  sourceType: "pdf",
  fileName: "Library Deck.pdf",
  fileType: "pdf",
  fileSize: 100,
  pageCount: 1,
  status: "ready",
  category: "general",
  createdAt: "2026-08-01T00:00:00Z",
  updatedAt: "2026-08-01T00:00:00Z",
};

describe("buildConfigFromPreset", () => {
  it("builds a public preset config correctly", () => {
    const config = buildConfigFromPreset("public");
    expect(config.level).toBe("public");
    expect(config.isCustomized).toBe(false);
    expect(config.requireEmailVerification).toBe(false);
    expect(config.whitelistEnabled).toBe(false);
    expect(config.passwordEnabled).toBe(false);
    expect(config.ndaEnabled).toBe(false);
    expect(config.allowDownload).toBe(false);
    expect(config.watermarkEnabled).toBe(true);
    expect(config.expiryDays).toBe(30);
    expect(config.maxViews).toBe("unlimited");
  });

  it("builds a standard preset config correctly", () => {
    const config = buildConfigFromPreset("standard");
    expect(config.level).toBe("standard");
    expect(config.requireEmailVerification).toBe(true);
    expect(config.whitelistEnabled).toBe(false);
    expect(config.passwordEnabled).toBe(false);
    expect(config.ndaEnabled).toBe(false);
    expect(config.allowDownload).toBe(false);
    expect(config.watermarkEnabled).toBe(true);
  });

  it("builds a confidential preset config correctly", () => {
    const config = buildConfigFromPreset("confidential");
    expect(config.level).toBe("confidential");
    expect(config.requireEmailVerification).toBe(true);
    expect(config.passwordEnabled).toBe(false);
    expect(config.ndaEnabled).toBe(true);
  });

  it("builds a collaborative preset config correctly", () => {
    const config = buildConfigFromPreset("collaborative");
    expect(config.level).toBe("collaborative");
    expect(config.requireEmailVerification).toBe(true);
    expect(config.allowDownload).toBe(true);
    expect(config.ndaEnabled).toBe(false);
  });

  it("applies overrides to template values", () => {
    const config = buildConfigFromPreset("public", {
      requireEmailVerification: true,
      expiryDays: 15,
    });
    expect(config.level).toBe("public");
    expect(config.requireEmailVerification).toBe(true);
    expect(config.expiryDays).toBe(15);
    // Unchanged template values
    expect(config.passwordEnabled).toBe(false);
    expect(config.watermarkEnabled).toBe(true);
  });

  it("preserves contactIds from overrides", () => {
    const config = buildConfigFromPreset("standard", {
      contactIds: ["contact-123"],
    });
    expect(config.contactIds).toEqual(["contact-123"]);
    expect(config.level).toBe("standard");
  });

  it("matches template exactly without overrides", () => {
    for (const preset of ["public", "standard", "confidential", "collaborative", "customized"] as PermissionPreset[]) {
      const config = buildConfigFromPreset(preset);
      const template = PRESET_TEMPLATES[preset];
      expect(config.requireEmailVerification).toBe(template.requireEmailVerification);
      expect(config.whitelistEnabled).toBe(template.whitelistEnabled);
      expect(config.passwordEnabled).toBe(template.passwordEnabled);
      expect(config.ndaEnabled).toBe(template.ndaEnabled);
      expect(config.allowDownload).toBe(template.allowDownload);
      expect(config.watermarkEnabled).toBe(template.watermarkEnabled);
      expect(config.expiryDays).toBe(template.expiryDays);
      expect(config.maxViews).toBe(template.maxViews);
    }
  });

  it("builds a customized preset config with isCustomized=true", () => {
    const config = buildConfigFromPreset("customized");
    expect(config.level).toBe("customized");
    expect(config.isCustomized).toBe(true);
    expect(config.requireEmailVerification).toBe(false);
    expect(config.allowDownload).toBe(false);
    expect(config.watermarkEnabled).toBe(true);
  });

  it("initializes empty NDA selection fields", () => {
    const config = buildConfigFromPreset("confidential");
    expect(config.ndaDocumentId).toBe("");
    expect(config.ndaTemplateId).toBe("");
  });
});

describe("validateBundleSecurityConfig", () => {
  it("blocks email/NDA without contact", () => {
    expect(validateBundleSecurityConfig(buildConfigFromPreset("standard"))).toEqual({
      ok: false,
      reason: "contactRequired",
    });
  });

  it("blocks NDA without template/document even with contact", () => {
    const config = buildConfigFromPreset("confidential", {
      contactIds: ["c-1"],
    });
    expect(validateBundleSecurityConfig(config)).toEqual({
      ok: false,
      reason: "ndaDocumentRequired",
    });
  });

  it("passes with contact and ndaDocumentId", () => {
    const config = buildConfigFromPreset("confidential", {
      contactIds: ["c-1"],
      ndaDocumentId: "nda-doc-1",
    });
    expect(validateBundleSecurityConfig(config)).toEqual({ ok: true });
  });

  it("passes with contact and ndaTemplateId", () => {
    const config = buildConfigFromPreset("confidential", {
      contactIds: ["c-1"],
      ndaTemplateId: "tpl-1",
    });
    expect(validateBundleSecurityConfig(config)).toEqual({ ok: true });
  });

  it("blocks custom expiry without datetime", () => {
    const config = buildConfigFromPreset("public", { expiryDays: "custom" });
    expect(validateBundleSecurityConfig(config)).toEqual({
      ok: false,
      reason: "customExpiresAtRequired",
    });
  });

  it("blocks custom expiry in the past", () => {
    const past = new Date();
    past.setDate(past.getDate() - 1);
    const config = buildConfigFromPreset("public", {
      expiryDays: "custom",
      _editExpiresAt: past.toISOString(),
    });
    expect(validateBundleSecurityConfig(config)).toEqual({
      ok: false,
      reason: "customExpiresAtFuture",
    });
  });

  it("passes custom expiry in the future", () => {
    const future = new Date();
    future.setDate(future.getDate() + 12);
    const config = buildConfigFromPreset("public", {
      expiryDays: "custom",
      _editExpiresAt: future.toISOString(),
    });
    expect(validateBundleSecurityConfig(config)).toEqual({ ok: true });
  });

  it("blocks custom max views without a count", () => {
    const config = buildConfigFromPreset("public", { maxViews: "custom" });
    expect(validateBundleSecurityConfig(config)).toEqual({
      ok: false,
      reason: "customMaxViewsRequired",
    });
  });

  it("blocks custom max views that are not a positive integer", () => {
    const config = buildConfigFromPreset("public", {
      maxViews: "custom",
      _editMaxViews: 0,
    });
    expect(validateBundleSecurityConfig(config)).toEqual({
      ok: false,
      reason: "customMaxViewsInvalid",
    });
  });

  it("passes custom max views with a valid count", () => {
    const config = buildConfigFromPreset("public", {
      maxViews: "custom",
      _editMaxViews: 25,
    });
    expect(validateBundleSecurityConfig(config)).toEqual({ ok: true });
  });
});

describe("resolveExpiryDaysFromExpiresAt", () => {
  it("defaults to 30 when missing", () => {
    expect(resolveExpiryDaysFromExpiresAt(undefined).expiryDays).toBe(30);
  });

  it("snaps presets within ±1 day", () => {
    const now = new Date("2026-08-11T12:00:00.000Z");
    const in15 = new Date(now);
    in15.setDate(in15.getDate() + 15);
    expect(resolveExpiryDaysFromExpiresAt(in15.toISOString(), now).expiryDays).toBe(15);
  });

  it("uses custom for non-preset day counts", () => {
    const now = new Date("2026-08-11T12:00:00.000Z");
    const in12 = new Date(now);
    in12.setDate(in12.getDate() + 12);
    const resolved = resolveExpiryDaysFromExpiresAt(in12.toISOString(), now);
    expect(resolved.expiryDays).toBe("custom");
    expect(resolved._editExpiresAt).toBe(in12.toISOString());
  });
});

describe("resolveMaxViewsFromAccessCount", () => {
  it("defaults to unlimited when missing or zero", () => {
    expect(resolveMaxViewsFromAccessCount(undefined).maxViews).toBe("unlimited");
    expect(resolveMaxViewsFromAccessCount(0).maxViews).toBe("unlimited");
  });

  it("snaps 10 / 50 / 100 to presets", () => {
    expect(resolveMaxViewsFromAccessCount(10)).toEqual({ maxViews: 10 });
    expect(resolveMaxViewsFromAccessCount(50)).toEqual({ maxViews: 50 });
    expect(resolveMaxViewsFromAccessCount(100)).toEqual({ maxViews: 100 });
  });

  it("uses custom for non-preset counts", () => {
    expect(resolveMaxViewsFromAccessCount(25)).toEqual({
      maxViews: "custom",
      _editMaxViews: 25,
    });
  });
});

describe("buildEditModeDocumentLists", () => {
  it("keeps the available picker free of selected orphans (agreement / room docs)", () => {
    const agreementOnLink: DocumentSummary = {
      id: "doc-nda",
      title: "Standard NDA",
      sourceType: "pdf",
      pageCount: 2,
      status: "ready",
      fileSize: 50,
    };
    const { pickerDocuments, selectedDocuments } = buildEditModeDocumentLists(
      [libraryDoc],
      [libraryDoc, agreementOnLink],
    );

    expect(pickerDocuments.map((d) => d.id)).toEqual(["doc-lib"]);
    expect(selectedDocuments.map((d) => d.id)).toEqual(["doc-lib", "doc-nda"]);
    expect(selectedDocuments[1]?.title).toBe("Standard NDA");
  });
});

describe("resolveDraftDocumentRestore", () => {
  it("starts fresh without a warning when every draft document is gone", () => {
    expect(
      resolveDraftDocumentRestore({
        draftIds: ["gone-1", "gone-2"],
        availableIds: ["doc-lib"],
      }),
    ).toEqual({
      restoreIds: [],
      warnMissing: false,
      missing: 2,
      total: 2,
      clearDraft: true,
    });
  });

  it("warns only when a partial draft can still be restored", () => {
    expect(
      resolveDraftDocumentRestore({
        draftIds: ["gone-1", "doc-lib"],
        availableIds: ["doc-lib"],
      }),
    ).toEqual({
      restoreIds: ["doc-lib"],
      warnMissing: true,
      missing: 1,
      total: 2,
      clearDraft: false,
    });
  });

  it("discards the draft when the URL already picks documents", () => {
    expect(
      resolveDraftDocumentRestore({
        draftIds: ["gone-1", "gone-2"],
        availableIds: ["doc-lib"],
        explicitDocumentIds: ["doc-lib"],
      }),
    ).toEqual({
      restoreIds: [],
      warnMissing: false,
      missing: 0,
      total: 2,
      clearDraft: true,
    });
  });
});

describe("resolveShareDocumentReadiness", () => {
  it("is ready only when every document is ready", () => {
    expect(resolveShareDocumentReadiness([{ status: "ready" }])).toEqual({
      ready: true,
      processingCount: 0,
      failedCount: 0,
      reason: "ok",
    });
  });

  it("blocks create while any document is still processing", () => {
    expect(
      resolveShareDocumentReadiness([{ status: "ready" }, { status: "processing" }]),
    ).toEqual({
      ready: false,
      processingCount: 1,
      failedCount: 0,
      reason: "processing",
    });
  });

  it("blocks create when any document failed", () => {
    expect(
      resolveShareDocumentReadiness([{ status: "processing" }, { status: "failed" }]),
    ).toEqual({
      ready: false,
      processingCount: 1,
      failedCount: 1,
      reason: "failed",
    });
  });
});

import { describe, it, expect } from "vitest";
import { applyPreset, isPresetMatch, PRESETS } from "./presets";
import type { DraftLink } from "./types";
import type { PresetValues } from "./presets";

type DraftWithViewers = PresetValues & {
  allowedViewers: string[];
  blockedViewers: string[];
  password?: string;
};

function makeDraft(overrides: Partial<DraftWithViewers> = {}): DraftWithViewers {
  return {
    ...PRESETS.standard,
    allowedViewers: ["alice@vc.com"],
    blockedViewers: ["leaker@bad.com"],
    password: "old-password-123",
    ...overrides,
  } as DraftWithViewers;
}

describe("applyPreset", () => {
  it("public preset clears allowed/blocked viewers, password, and protections", () => {
    const draft = makeDraft({
      allowedViewers: ["alice@vc.com"],
      blockedViewers: ["leaker@bad.com"],
      requireEmail: true,
      requireEmailVerification: true,
      requirePassword: true,
      password: "old-password-123",
      requireNda: true,
      watermarkEnabled: true,
    });
    const { patch, changedFields } = applyPreset("public", draft);

    expect(patch.allowedViewers).toEqual([]);
    expect(patch.blockedViewers).toEqual([]);
    expect(patch.password).toBe("");
    expect(patch.requireEmail).toBe(false);
    expect(patch.requireEmailVerification).toBe(false);
    expect(patch.requirePassword).toBe(false);
    expect(patch.requireNda).toBe(false);
    expect(patch.watermarkEnabled).toBe(false);

    expect(changedFields).toContain("allowedViewers");
    expect(changedFields).toContain("blockedViewers");
    expect(changedFields).toContain("password");
  });

  it("public preset does not clear password when no password was set", () => {
    const draft = makeDraft({ requirePassword: false, password: "" });
    const { patch, changedFields } = applyPreset("public", draft);

    expect(patch.password).toBeUndefined();
    expect(changedFields).not.toContain("password");
  });

  it("confidential preset clears an existing password", () => {
    const draft = makeDraft({
      requirePassword: true,
      password: "old-password-123",
    });
    const { patch, changedFields } = applyPreset("confidential", draft);

    expect(patch.password).toBe("");
    expect(changedFields).toContain("password");
  });

  it("standard preset does not clear existing viewers", () => {
    const draft = makeDraft({
      allowedViewers: ["alice@vc.com"],
      blockedViewers: ["leaker@bad.com"],
    });
    const { patch } = applyPreset("standard", draft);

    expect(patch.allowedViewers).toBeUndefined();
    expect(patch.blockedViewers).toBeUndefined();
  });

  it("changedFields includes all fields that differ from the preset", () => {
    const draft = makeDraft({
      requireEmail: false,
      watermarkEnabled: false,
      requireNda: true,
    });
    const { changedFields } = applyPreset("standard", draft);

    expect(changedFields).toContain("requireEmail");
    expect(changedFields).toContain("watermarkEnabled");
    expect(changedFields).toContain("requireNda");
  });
});

describe("isPresetMatch", () => {
  it("matches when all preset fields equal the draft", () => {
    const draft: DraftLink = {
      name: "Test",
      ...PRESETS.public,
      password: "",
      allowedViewers: [],
      blockedViewers: [],
      customDomain: "",
      notifyOnAccess: false,
      folderPaths: [],
      contactIds: [],
    };
    expect(isPresetMatch("public", draft)).toBe(true);
  });
});

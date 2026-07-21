import type { LinkPreset } from "./types";

export const PRESET_NAMES: LinkPreset[] = ["public", "standard", "confidential"];

function daysFromNow(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
  return d.toISOString().slice(0, 16);
}

export interface PresetValues {
  requireEmail: boolean;
  requireEmailVerification: boolean;
  requirePassword: boolean;
  watermarkEnabled: boolean;
  requireNda: boolean;
  ndaDocumentId: string;
  ndaTemplateId: string;
  allowDownloading: boolean;
  enableScreenshotProtection: boolean;
  aiCopilotEnabled: boolean;
  enableFileRequests: boolean;
  enableIndexFileGeneration: boolean;
  enableQaConversations: boolean;
  expiresAt: string;
}

export const PRESETS: Record<Exclude<LinkPreset, "custom">, PresetValues> = {
  public: {
    requireEmail: false,
    requireEmailVerification: false,
    requirePassword: false,
    watermarkEnabled: false,
    requireNda: false,
    ndaDocumentId: "",
    ndaTemplateId: "",
    allowDownloading: false,
    enableScreenshotProtection: false,
    aiCopilotEnabled: false,
    enableFileRequests: false,
    enableIndexFileGeneration: false,
    enableQaConversations: false,
    expiresAt: "",
  },
  standard: {
    requireEmail: false,
    requireEmailVerification: true,
    requirePassword: false,
    watermarkEnabled: true,
    requireNda: false,
    ndaDocumentId: "",
    ndaTemplateId: "",
    allowDownloading: false,
    enableScreenshotProtection: false,
    aiCopilotEnabled: false,
    enableFileRequests: false,
    enableIndexFileGeneration: false,
    enableQaConversations: false,
    expiresAt: daysFromNow(30),
  },
  confidential: {
    requireEmail: false,
    requireEmailVerification: true,
    requirePassword: true,
    watermarkEnabled: true,
    requireNda: false,
    ndaDocumentId: "",
    ndaTemplateId: "",
    allowDownloading: false,
    enableScreenshotProtection: false,
    aiCopilotEnabled: false,
    enableFileRequests: false,
    enableIndexFileGeneration: false,
    enableQaConversations: false,
    expiresAt: daysFromNow(7),
  },
};

export function applyPreset(
  name: Exclude<LinkPreset, "custom">,
  draft: PresetValues & { allowedViewers: string[]; blockedViewers: string[] }
): { patch: Partial<PresetValues & { allowedViewers: string[]; blockedViewers: string[]; password: string }>; changedFields: string[] } {
  const base = PRESETS[name];
  const patch: Partial<PresetValues & { allowedViewers: string[]; blockedViewers: string[]; password: string }> = {};
  const changedFields: string[] = [];

  const fields: Array<keyof PresetValues> = [
    "requireEmail",
    "requireEmailVerification",
    "requirePassword",
    "watermarkEnabled",
    "requireNda",
    "ndaDocumentId",
    "allowDownloading",
    "enableScreenshotProtection",
    "aiCopilotEnabled",
    "enableFileRequests",
    "enableIndexFileGeneration",
    "enableQaConversations",
    "expiresAt",
  ];

  for (const key of fields) {
    if (base[key] !== (draft as PresetValues)[key]) {
      patch[key] = base[key] as never;
      changedFields.push(key);
    }
  }

  if (name === "public") {
    if (draft.allowedViewers.length > 0) {
      patch.allowedViewers = [];
      changedFields.push("allowedViewers");
    }
    if (draft.blockedViewers.length > 0) {
      patch.blockedViewers = [];
      changedFields.push("blockedViewers");
    }
    if (draft.requirePassword && (draft as { password?: string }).password) {
      patch.password = "";
      changedFields.push("password");
    }
  } else {
    if (name === "confidential" && (draft as { password?: string }).password) {
      patch.password = "";
      changedFields.push("password");
    }
  }

  return { patch, changedFields };
}

export function isPresetMatch(
  preset: Exclude<LinkPreset, "custom">,
  draft: PresetValues
): boolean {
  const expected = PRESETS[preset];
  return (
    expected.requireEmail === draft.requireEmail &&
    expected.requireEmailVerification === draft.requireEmailVerification &&
    expected.requirePassword === draft.requirePassword &&
    expected.watermarkEnabled === draft.watermarkEnabled &&
    expected.requireNda === draft.requireNda &&
    expected.allowDownloading === draft.allowDownloading &&
    expected.enableScreenshotProtection === draft.enableScreenshotProtection &&
    expected.aiCopilotEnabled === draft.aiCopilotEnabled &&
    expected.enableFileRequests === draft.enableFileRequests &&
    expected.enableIndexFileGeneration === draft.enableIndexFileGeneration &&
    expected.enableQaConversations === draft.enableQaConversations &&
    expected.expiresAt === draft.expiresAt
  );
}

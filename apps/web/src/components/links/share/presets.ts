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
    allowDownloading: false,
    enableScreenshotProtection: false,
    aiCopilotEnabled: false,
    enableFileRequests: false,
    enableIndexFileGeneration: false,
    enableQaConversations: false,
    expiresAt: "",
  },
  standard: {
    requireEmail: true,
    requireEmailVerification: false,
    requirePassword: false,
    watermarkEnabled: true,
    requireNda: false,
    allowDownloading: false,
    enableScreenshotProtection: false,
    aiCopilotEnabled: false,
    enableFileRequests: false,
    enableIndexFileGeneration: false,
    enableQaConversations: false,
    expiresAt: daysFromNow(30),
  },
  confidential: {
    requireEmail: true,
    requireEmailVerification: true,
    requirePassword: true,
    watermarkEnabled: true,
    requireNda: false,
    allowDownloading: false,
    enableScreenshotProtection: false,
    aiCopilotEnabled: false,
    enableFileRequests: false,
    enableIndexFileGeneration: false,
    enableQaConversations: false,
    expiresAt: daysFromNow(7),
  },
};

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

import {
  GlobeHemisphereWest,
  LockKey,
  ShieldCheck,
  UsersThree,
} from "@phosphor-icons/react";
import type { PermissionConfig, PermissionPreset } from "@/types";
import type { PresetConfigTemplate, PresetDef } from "./types";

export const PRESET_ORDER: PermissionPreset[] = [
  "public",
  "standard",
  "confidential",
  "collaborative",
];

// ---------------------------------------------------------------------------
// Preset base templates – every preset has opinionated defaults that
// represent the best-practice combination from the product/security analysis.
// Individual options can be toggled on top, which flips isCustomized=true.
// ---------------------------------------------------------------------------

export const PRESET_TEMPLATES: Record<PermissionPreset, PresetConfigTemplate> = {
  public: {
    requireEmailVerification: false,
    whitelistEnabled: false,
    whitelist: [],
    passwordEnabled: false,
    ndaEnabled: false,
    allowDownload: false,
    watermarkEnabled: true,
    expiryDays: 30,
    maxViews: "unlimited",
  },
  standard: {
    requireEmailVerification: true,
    whitelistEnabled: true,
    whitelist: [],
    passwordEnabled: false,
    ndaEnabled: false,
    allowDownload: false,
    watermarkEnabled: true,
    expiryDays: 30,
    maxViews: "unlimited",
  },
  confidential: {
    requireEmailVerification: true,
    whitelistEnabled: true,
    whitelist: [],
    passwordEnabled: true,
    ndaEnabled: true,
    allowDownload: false,
    watermarkEnabled: true,
    expiryDays: 30,
    maxViews: "unlimited",
  },
  collaborative: {
    requireEmailVerification: true,
    whitelistEnabled: false,
    whitelist: [],
    passwordEnabled: false,
    ndaEnabled: false,
    allowDownload: true,
    watermarkEnabled: true,
    expiryDays: 30,
    maxViews: "unlimited",
  },
};

export const presetDef: Record<PermissionPreset, PresetDef> = {
  public: {
    label: "preset.public.label",
    description: "preset.public.description",
    icon: GlobeHemisphereWest,
    color:
      "text-success-600 bg-success-50 border-success-200 dark:bg-success-950 dark:border-success-800",
    friction: "preset.public.friction",
    usage: "preset.public.usage",
    gates: [],
  },
  standard: {
    label: "preset.standard.label",
    description: "preset.standard.description",
    icon: LockKey,
    color:
      "text-warm-600 bg-warm-50 border-warm-200 dark:bg-warm-950 dark:border-warm-800",
    friction: "preset.standard.friction",
    usage: "preset.standard.usage",
    gates: [
      "featureEmailVerification",
      "featureWhitelist",
      "featureWatermark",
      "featureNoDownload",
    ],
  },
  confidential: {
    label: "preset.confidential.label",
    description: "preset.confidential.description",
    icon: ShieldCheck,
    color:
      "text-hot-600 bg-hot-50 border-hot-200 dark:bg-hot-950 dark:border-hot-800",
    friction: "preset.confidential.friction",
    usage: "preset.confidential.usage",
    gates: [
      "featureEmailVerification",
      "featureWhitelist",
      "featurePassword",
      "featureNDA",
      "featureWatermark",
      "featureNoDownload",
    ],
  },
  collaborative: {
    label: "preset.collaborative.label",
    description: "preset.collaborative.description",
    icon: UsersThree,
    color:
      "text-primary-600 bg-primary-50 border-primary-200 dark:bg-primary-950 dark:border-primary-800",
    friction: "preset.collaborative.friction",
    usage: "preset.collaborative.usage",
    gates: [
      "featureEmailVerification",
      "featureDownload",
      "featureWatermark",
    ],
  },
};

// ---------------------------------------------------------------------------
// Derive which preset a config most closely matches. Returns the preset
// PLUS a boolean indicating whether the config is a clean match (isCustomized=false)
// or has been manually tweaked (isCustomized=true).
// ---------------------------------------------------------------------------

export function classifyPresetFromConfig(
  config: Omit<PermissionConfig, "level" | "isCustomized">,
): { level: PermissionPreset; isCustomized: boolean } {
  let bestMatch: PermissionPreset = "public";
  let bestScore = -1;

  for (const level of PRESET_ORDER) {
    const template = PRESET_TEMPLATES[level];
    const score = presetMatchScore(config, template);
    if (score > bestScore) {
      bestScore = score;
      bestMatch = level;
    }
  }

  return {
    level: bestMatch,
    isCustomized: bestScore < 10, // 10 = perfect match (6 booleans + expiryDays + maxViews + whitelist present + password present)
  };
}

function presetMatchScore(
  config: Omit<PermissionConfig, "level" | "isCustomized">,
  template: PresetConfigTemplate,
): number {
  let score = 0;
  if (config.requireEmailVerification === template.requireEmailVerification) score++;
  if (config.whitelistEnabled === template.whitelistEnabled) score++;
  if (config.passwordEnabled === template.passwordEnabled) score++;
  if (config.ndaEnabled === template.ndaEnabled) score++;
  if (config.allowDownload === template.allowDownload) score++;
  if (config.watermarkEnabled === template.watermarkEnabled) score++;
  if (config.expiryDays === template.expiryDays) score++;
  if (config.maxViews === template.maxViews) score++;

  // Whitelist contents must match (empty or not)
  const configHasWhitelist =
    config.whitelistEnabled && config.whitelist.length > 0;
  const templateHasWhitelist =
    template.whitelistEnabled && template.whitelist.length > 0;
  if (configHasWhitelist === templateHasWhitelist) score++;

  // Password presence must match
  const configHasPassword = config.passwordEnabled && !!config.password;
  const templateHasPassword = template.passwordEnabled && !!template.password;
  if (configHasPassword === templateHasPassword) score++;

  return score;
}

// ---------------------------------------------------------------------------
// Cross-option constraints – when a "parent" option is toggled, dependent
// options are automatically adjusted. Classification is left to the caller.
// ---------------------------------------------------------------------------

export function enforceCrossOptionConstraints(
  next: PermissionConfig,
): PermissionConfig {
  // Whitelist requires email verification to identify the visitor.
  if (next.whitelistEnabled && !next.requireEmailVerification) {
    return { ...next, requireEmailVerification: true };
  }

  // NDA requires email verification so the signer identity is recorded.
  if (next.ndaEnabled && !next.requireEmailVerification) {
    return { ...next, requireEmailVerification: true };
  }

  return next;
}

// ---------------------------------------------------------------------------
// Scoring functions – unchanged logic, adjusted weights for new preset system.
// ---------------------------------------------------------------------------

export function calculateFrictionScore(
  config: Omit<PermissionConfig, "level" | "isCustomized">,
): number {
  let score = 0;
  if (config.requireEmailVerification) score += 1;
  if (config.whitelistEnabled) score += 3;
  if (config.passwordEnabled) score += 3;
  if (config.ndaEnabled) score += 2;
  if (config.watermarkEnabled) score += 1;
  if (!config.allowDownload) score += 1;
  if (
    config.expiryDays !== "custom" &&
    typeof config.expiryDays === "number" &&
    config.expiryDays <= 7
  )
    score += 1;
  if (config.maxViews !== "unlimited" && typeof config.maxViews === "number")
    score += 2;
  return Math.min(10, score);
}

export function calculateSecurityScore(
  config: Omit<PermissionConfig, "level" | "isCustomized">,
): number {
  let score = 0;
  if (config.requireEmailVerification) score += 2;
  if (config.whitelistEnabled) score += 3;
  if (config.passwordEnabled) score += 3;
  if (config.ndaEnabled) score += 2;
  if (config.watermarkEnabled) score += 2;
  if (!config.allowDownload) score += 1;
  if (config.maxViews !== "unlimited" && typeof config.maxViews === "number")
    score += 1;
  if (
    config.expiryDays !== "custom" &&
    typeof config.expiryDays === "number" &&
    config.expiryDays <= 30
  )
    score += 1;
  return Math.min(10, score);
}

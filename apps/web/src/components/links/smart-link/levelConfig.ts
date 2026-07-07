import {
  GlobeHemisphereWestIcon,
  LockKeyIcon,
  ShieldCheckIcon,
  SlidersIcon,
  UsersThreeIcon,
} from "@phosphor-icons/react";
import type { PermissionConfig, PermissionPreset } from "@/types";
import type { PresetConfigTemplate, PresetDef } from "./types";

const PRESET_ORDER: PermissionPreset[] = [
  "public",
  "standard",
  "confidential",
  "collaborative",
  "customized",
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
    aiCopilotEnabled: false,
    expiryDays: 30,
    maxViews: "unlimited",
  },
  standard: {
    requireEmailVerification: true,
    whitelistEnabled: false,
    whitelist: [],
    passwordEnabled: false,
    ndaEnabled: false,
    allowDownload: false,
    watermarkEnabled: true,
    aiCopilotEnabled: false,
    expiryDays: 30,
    maxViews: "unlimited",
  },
  confidential: {
    requireEmailVerification: true,
    whitelistEnabled: false,
    whitelist: [],
    passwordEnabled: false,
    ndaEnabled: true,
    allowDownload: false,
    watermarkEnabled: true,
    aiCopilotEnabled: false,
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
    aiCopilotEnabled: false,
    expiryDays: 30,
    maxViews: "unlimited",
  },
  customized: {
    requireEmailVerification: false,
    whitelistEnabled: false,
    whitelist: [],
    passwordEnabled: false,
    ndaEnabled: false,
    allowDownload: false,
    watermarkEnabled: true,
    aiCopilotEnabled: false,
    expiryDays: 30,
    maxViews: "unlimited",
  },
};

export const presetDef: Record<PermissionPreset, PresetDef> = {
  public: {
    label: "preset.public.label",
    description: "preset.public.description",
    icon: GlobeHemisphereWestIcon,
    color:
      "text-success-600 bg-success-50 border-success-200 dark:bg-success-950 dark:border-success-800",
    friction: "preset.public.friction",
    usage: "preset.public.usage",
    gates: [],
  },
  standard: {
    label: "preset.standard.label",
    description: "preset.standard.description",
    icon: LockKeyIcon,
    color:
      "text-warm-600 bg-warm-50 border-warm-200 dark:bg-warm-950 dark:border-warm-800",
    friction: "preset.standard.friction",
    usage: "preset.standard.usage",
    gates: [
      "featureEmailVerification",
      "featureWatermark",
      "featureNoDownload",
    ],
  },
  confidential: {
    label: "preset.confidential.label",
    description: "preset.confidential.description",
    icon: ShieldCheckIcon,
    color:
      "text-hot-600 bg-hot-50 border-hot-200 dark:bg-hot-950 dark:border-hot-800",
    friction: "preset.confidential.friction",
    usage: "preset.confidential.usage",
    gates: [
      "featureEmailVerification",
      "featureNDA",
      "featureWatermark",
      "featureNoDownload",
    ],
  },
  collaborative: {
    label: "preset.collaborative.label",
    description: "preset.collaborative.description",
    icon: UsersThreeIcon,
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
  customized: {
    label: "preset.customized.label",
    description: "preset.customized.description",
    icon: SlidersIcon,
    color:
      "text-muted-foreground bg-muted border-border dark:bg-muted/20 dark:border-border",
    friction: "preset.customized.friction",
    usage: "preset.customized.usage",
    gates: [],
  },
};

// ---------------------------------------------------------------------------
// Derive which named preset a config matches exactly. If it does not match
// any named preset, treat it as a customized configuration.
// ---------------------------------------------------------------------------

const NAMED_PRESETS: PermissionPreset[] = PRESET_ORDER.filter(
  (p) => p !== "customized",
);

/**
 * List of checked dimensions in presetMatchScore. Must stay in sync
 * with the score counter inside that function. Changing these will
 * automatically re-derive MAX_SCORE and PERFECT_MATCH_THRESHOLD.
 *
 *   - 5 boolean flags
 *   - 2 mixed-value fields (expiryDays, maxViews)
 *
 * Total = 7 (computed as MAX_SCORE below — do NOT hardcode elsewhere).
 */
const SCORED_DIMENSION_NAMES = [
  "requireEmailVerification",
  "ndaEnabled",
  "allowDownload",
  "watermarkEnabled",
  "aiCopilotEnabled",
  "expiryDays",
  "maxViews",
] as const;

/** Maximum possible score for presetMatchScore — computed from scored dimensions. */
const MAX_SCORE = SCORED_DIMENSION_NAMES.length;

export function classifyPresetFromConfig(
  config: Omit<PermissionConfig, "level" | "isCustomized">,
): { level: PermissionPreset; isCustomized: boolean } {
  // bestMatch is only valid when bestScore === MAX_SCORE (perfect match).
  // The initial "public" value is never returned because any non-perfect
  // match goes to "customized" regardless of bestMatch.
  let bestMatch: PermissionPreset = "public";
  let bestScore = -1;

  for (const level of NAMED_PRESETS) {
    const template = PRESET_TEMPLATES[level];
    const score = presetMatchScore(config, template);
    if (score > bestScore) {
      bestScore = score;
      bestMatch = level;
    }
  }

  if (bestScore === MAX_SCORE) {
    return { level: bestMatch, isCustomized: false };
  }

  return { level: "customized", isCustomized: true };
}

function presetMatchScore(
  config: Omit<PermissionConfig, "level" | "isCustomized">,
  template: PresetConfigTemplate,
): number {
  let score = 0;
  // These 7 lines correspond to the first 7 entries in SCORED_DIMENSION_NAMES.
  // When adding/removing a field here, update SCORED_DIMENSION_NAMES above.
  if (config.requireEmailVerification === template.requireEmailVerification) score++;
  if (config.ndaEnabled === template.ndaEnabled) score++;
  if (config.allowDownload === template.allowDownload) score++;
  if (config.watermarkEnabled === template.watermarkEnabled) score++;
  if (config.aiCopilotEnabled === template.aiCopilotEnabled) score++;
  if (config.expiryDays === template.expiryDays) score++;
  if (config.maxViews === template.maxViews) score++;

  return score;
}

// ---------------------------------------------------------------------------
// Cross-option constraints – when a "parent" option is toggled, dependent
// options are automatically adjusted. Classification is left to the caller.
//
// NOTE: These constraints are additive only (turn ON dependencies). They
// never turn OFF options that were manually enabled. This is intentional
// to avoid silent security degradation. There is no "reset" mechanism —
// users must manually disable derivative options.
// ---------------------------------------------------------------------------

export function enforceCrossOptionConstraints(
  next: PermissionConfig,
): PermissionConfig {
  // NDA requires email verification so the signer identity is recorded.
  if (next.ndaEnabled && !next.requireEmailVerification) {
    return { ...next, requireEmailVerification: true };
  }

  return next;
}

// ---------------------------------------------------------------------------
// Scoring functions – unchanged logic, adjusted weights for new preset system.
//
// NOTE: Scores are clamped at 100 for display. Raw scores vary from 0
// (public) to ~14 (confidential), providing granular differentiation across
// all preset tiers without artificial capping.
// ---------------------------------------------------------------------------

export function calculateFrictionScore(
  config: Omit<PermissionConfig, "level" | "isCustomized">,
): number {
  let score = 0;
  if (config.requireEmailVerification) score += 1;
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
  return Math.min(100, score);
}

export function calculateSecurityScore(
  config: Omit<PermissionConfig, "level" | "isCustomized">,
): number {
  let score = 0;
  if (config.requireEmailVerification) score += 2;
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
  return Math.min(100, score);
}

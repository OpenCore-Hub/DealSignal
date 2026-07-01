import { LockKeyOpen, Lock, Shield } from "@phosphor-icons/react";
import type { PermissionConfig } from "@/types";
import type { LevelInfo, PermissionLevel } from "./types";

export const LEVEL_ORDER: PermissionLevel[] = ["low", "medium", "high"];

export const levelConfig: Record<PermissionLevel, LevelInfo> = {
  low: {
    label: "level.low.label",
    description: "level.low.description",
    icon: LockKeyOpen,
    color: "text-success-500 bg-success-500/10 border-success-500/20",
    friction: "level.low.friction",
  },
  medium: {
    label: "level.medium.label",
    description: "level.medium.description",
    icon: Lock,
    color: "text-warm-500 bg-warm-500/10 border-warm-500/20",
    friction: "level.medium.friction",
  },
  high: {
    label: "level.high.label",
    description: "level.high.description",
    icon: Shield,
    color: "text-hot-500 bg-hot-500/10 border-hot-500/20",
    friction: "level.high.friction",
  },
};

export function calculateFrictionScore(config: PermissionConfig): number {
  let score = 0;
  if (config.requireEmailVerification) score += 1;
  if (config.whitelistEnabled) score += 3;
  if (config.passwordEnabled) score += 3;
  if (config.watermarkEnabled) score += 1;
  if (!config.allowDownload) score += 1;
  if (config.expiryDays !== "custom" && config.expiryDays <= 7) score += 1;
  if (config.maxViews !== "unlimited") score += 2;
  return Math.min(10, score);
}

export function calculateSecurityScore(config: PermissionConfig): number {
  let score = 0;
  if (config.requireEmailVerification) score += 2;
  if (config.whitelistEnabled) score += 3;
  if (config.passwordEnabled) score += 3;
  if (config.watermarkEnabled) score += 2;
  if (!config.allowDownload) score += 1;
  if (config.maxViews !== "unlimited") score += 1;
  if (config.expiryDays !== "custom" && config.expiryDays <= 30) score += 1;
  return Math.min(10, score);
}

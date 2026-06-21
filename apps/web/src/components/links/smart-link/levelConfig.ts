import { LockKeyOpen, Lock, Shield } from "@phosphor-icons/react";
import type { PermissionConfig } from "@/types";
import type { LevelInfo, PermissionLevel } from "./types";

export const LEVEL_ORDER: PermissionLevel[] = ["low", "medium", "high"];

export const levelConfig: Record<PermissionLevel, LevelInfo> = {
  low: {
    label: "低摩擦",
    description: "公开或邮箱验证即可访问",
    icon: LockKeyOpen,
    color: "text-success-500 bg-success-500/10 border-success-500/20",
    friction: "接收方无需额外步骤，打开率最高。",
  },
  medium: {
    label: "中强度",
    description: "白名单或密码保护",
    icon: Lock,
    color: "text-warm-500 bg-warm-500/10 border-warm-500/20",
    friction: "需要邮箱/密码/白名单验证，适合敏感材料。",
  },
  high: {
    label: "高强度",
    description: "NDA + 白名单 + 密码组合",
    icon: Shield,
    color: "text-hot-500 bg-hot-500/10 border-hot-500/20",
    friction: "NDA 签署 + 多重验证，适合机密尽调资料。",
  },
};

export function calculateFrictionScore(config: PermissionConfig): number {
  let score = 0;
  if (config.requireEmail) score += 1;
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
  if (config.requireEmail) score += 1;
  if (config.whitelistEnabled) score += 3;
  if (config.passwordEnabled) score += 3;
  if (config.watermarkEnabled) score += 2;
  if (!config.allowDownload) score += 1;
  if (config.maxViews !== "unlimited") score += 1;
  if (config.expiryDays !== "custom" && config.expiryDays <= 30) score += 1;
  return Math.min(10, score);
}

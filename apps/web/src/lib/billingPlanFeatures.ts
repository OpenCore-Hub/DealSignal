import type { BillingInfo, BillingPlanOffer } from "@/types";

type PlanCode = "free" | "pro" | "business" | "enterprise";

const TIER_RANK: Record<PlanCode, number> = {
  free: 0,
  pro: 1,
  business: 2,
  enterprise: 3,
};

/**
 * docs/billing.md qualitative rows in source order (after seats / links / docs).
 * Each row is inherited by higher tiers — comparison cards must not drop lower-tier items.
 */
const BILLING_MD_FEATURES: {
  key: string;
  min: PlanCode;
  flag?: keyof Pick<
    BillingPlanOffer,
    "branding" | "watermark" | "customDomain" | "accessControls" | "nda"
  >;
}[] = [
  { key: "unlimitedVisitors", min: "free" },
  { key: "pageAnalytics", min: "free" },
  { key: "sharingControls", min: "free" },
  { key: "largeFiles", min: "pro" },
  { key: "branding", min: "pro", flag: "branding" },
  { key: "videos", min: "pro" },
  { key: "folders", min: "free" },
  { key: "customDomain", min: "business", flag: "customDomain" },
  { key: "multiFileSharing", min: "pro" },
  { key: "emailVerification", min: "business", flag: "accessControls" },
  { key: "allowBlockList", min: "business", flag: "accessControls" },
  { key: "screenshotProtection", min: "pro", flag: "watermark" },
  { key: "nda", min: "business", flag: "nda" },
  { key: "watermark", min: "pro", flag: "watermark" },
  { key: "webhooks", min: "business" },
  { key: "roomAnalytics", min: "pro" },
  { key: "roomInsights", min: "business" },
  { key: "emailSupport", min: "enterprise" },
  { key: "securityEvents", min: "free" },
  { key: "emailNotifications", min: "free" },
  { key: "dailyDigest", min: "business" },
  { key: "slackAlerts", min: "business" },
  { key: "hubspot", min: "business" },
];

function planRank(code: string): number {
  return TIER_RANK[code as PlanCode] ?? -1;
}

/** i18n keys under settings:billing.planFeatures.* — catalog caps + docs/billing.md rows. */
export function billingPlanFeatureKeys(offer: BillingPlanOffer): string[] {
  const keys: string[] = [];
  if (offer.internalSeats <= 0) {
    keys.push("seatsUnlimited");
  } else {
    keys.push(`seats_${offer.internalSeats}`);
  }
  const gib = 1024 * 1024 * 1024;
  if (offer.storageBytes <= 0) {
    keys.push("storageUnlimited");
  } else if (offer.storageBytes === 2 * gib) {
    keys.push("storage2GiB");
  } else if (offer.storageBytes === 50 * gib) {
    keys.push("storage50GiB");
  } else if (offer.storageBytes === 500 * gib) {
    keys.push("storage500GiB");
  } else {
    keys.push("storageCustom");
  }
  if (offer.documents != null) {
    if (offer.documents <= 0) {
      keys.push("docsUnlimited");
    } else {
      keys.push(`docs_${offer.documents}`);
    }
  }
  if (offer.links <= 0) {
    keys.push("linksUnlimited");
  } else {
    keys.push(`links_${offer.links}`);
  }
  if (offer.rooms <= 0) {
    keys.push("roomsUnlimited");
  } else {
    keys.push(`rooms_${offer.rooms}`);
  }
  if (offer.visitorAskAi) {
    if ((offer.visitorAskAiMonthly ?? 0) <= 0) {
      keys.push("askUnlimited");
    } else {
      keys.push(`ask_${offer.visitorAskAiMonthly}`);
    }
  }

  const rank = planRank(offer.code);
  for (const row of BILLING_MD_FEATURES) {
    if (rank < TIER_RANK[row.min]) continue;
    if (row.flag && !offer[row.flag]) continue;
    keys.push(row.key);
  }
  if (offer.formalAsk) {
    keys.push("formalAsk");
  }
  return keys;
}

type CurrentPlanIncludedRow = {
  key: string;
  min?: PlanCode;
  flag?: keyof Pick<
    BillingInfo,
    | "brandingEnabled"
    | "watermarkEnabled"
    | "visitorAskAiEnabled"
    | "customDomainEnabled"
    | "ndaEnabled"
    | "accessControlsEnabled"
    | "knowledgeDeskEnabled"
    | "webhooksEnabled"
    | "hubspotEnabled"
    | "dailyDigestEnabled"
    | "slackAlertsEnabled"
    | "roomAnalyticsEnabled"
    | "roomInsightsEnabled"
    | "formalAskEnabled"
  >;
};

/** Qualitative entitlements for GET /billing — capacity meters stay in Usage. */
const CURRENT_PLAN_INCLUDED: CurrentPlanIncludedRow[] = [
  { key: "unlimitedVisitors" },
  { key: "pageAnalytics" },
  { key: "sharingControls" },
  { key: "folders" },
  { key: "emailNotifications" },
  { key: "securityEvents" },
  { key: "largeFiles", min: "pro" },
  { key: "videos", min: "pro" },
  { key: "multiFileSharing", min: "pro" },
  { key: "branding", flag: "brandingEnabled" },
  { key: "visitorAskAi", flag: "visitorAskAiEnabled" },
  { key: "watermark", flag: "watermarkEnabled" },
  { key: "screenshotProtection", flag: "watermarkEnabled" },
  { key: "knowledgeDesk", flag: "knowledgeDeskEnabled" },
  { key: "roomAnalytics", flag: "roomAnalyticsEnabled" },
  { key: "customDomain", flag: "customDomainEnabled" },
  { key: "nda", flag: "ndaEnabled" },
  { key: "emailVerification", flag: "accessControlsEnabled" },
  { key: "allowBlockList", flag: "accessControlsEnabled" },
  { key: "webhooks", flag: "webhooksEnabled" },
  { key: "dailyDigest", flag: "dailyDigestEnabled" },
  { key: "slackAlerts", flag: "slackAlertsEnabled" },
  { key: "hubspot", flag: "hubspotEnabled" },
  { key: "roomInsights", flag: "roomInsightsEnabled" },
  { key: "formalAsk", flag: "formalAskEnabled" },
  { key: "emailSupport", min: "enterprise" },
];

function effectiveDisplayRank(plan: string, trialExpired?: boolean): number {
  const code = plan.trim().toLowerCase();
  if (code === "trial") {
    return trialExpired ? TIER_RANK.free : TIER_RANK.business;
  }
  return TIER_RANK[code as PlanCode] ?? TIER_RANK.free;
}

/** i18n keys under settings:billing.planFeatures.* for the current-plan checklist. */
export function currentPlanIncludedFeatureKeys(info: BillingInfo): string[] {
  const rank = effectiveDisplayRank(info.plan, info.trialExpired);
  const keys: string[] = [];
  for (const row of CURRENT_PLAN_INCLUDED) {
    if (row.min && rank < TIER_RANK[row.min]) continue;
    if (row.flag && !info[row.flag]) continue;
    keys.push(row.key);
  }
  return keys;
}

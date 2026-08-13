import { describe, expect, it } from "vitest";
import { billingPlanFeatureKeys, currentPlanIncludedFeatureKeys } from "./billingPlanFeatures";
import type { BillingPlanOffer } from "@/types";

const gib = 1024 * 1024 * 1024;

function offer(partial: Partial<BillingPlanOffer>): BillingPlanOffer {
  return {
    code: "free",
    internalSeats: 1,
    storageBytes: 2 * gib,
    documents: 50,
    links: 20,
    rooms: 1,
    maxUploadBytes: 25 * 1024 * 1024,
    visitorAskAiMonthly: 0,
    customDomain: false,
    watermark: false,
    nda: false,
    visitorAskAi: false,
    branding: false,
    accessControls: false,
    priceMonthlyUsd: 0,
    customPricing: false,
    highlighted: false,
    ...partial,
  };
}

describe("billingPlanFeatureKeys", () => {
  it("lists every Free billing.md row on the Free card", () => {
    expect(billingPlanFeatureKeys(offer({}))).toEqual([
      "seats_1",
      "storage2GiB",
      "docs_50",
      "links_20",
      "rooms_1",
      "unlimitedVisitors",
      "pageAnalytics",
      "sharingControls",
      "folders",
      "securityEvents",
      "emailNotifications",
    ]);
  });

  it("inherits Free rows on Pro and adds professional-share rows", () => {
    expect(
      billingPlanFeatureKeys(
        offer({
          code: "pro",
          internalSeats: 3,
          storageBytes: 50 * gib,
          documents: 200,
          links: 0,
          rooms: 5,
          visitorAskAiMonthly: 200,
          customDomain: false,
          watermark: true,
          nda: false,
          visitorAskAi: true,
          branding: true,
          accessControls: false,
          priceMonthlyUsd: 49,
        }),
      ),
    ).toEqual([
      "seats_3",
      "storage50GiB",
      "docs_200",
      "linksUnlimited",
      "rooms_5",
      "ask_200",
      "unlimitedVisitors",
      "pageAnalytics",
      "sharingControls",
      "largeFiles",
      "branding",
      "videos",
      "folders",
      "multiFileSharing",
      "screenshotProtection",
      "watermark",
      "roomAnalytics",
      "securityEvents",
      "emailNotifications",
    ]);
  });

  it("inherits Free+Pro rows on Business and adds diligence rows", () => {
    expect(
      billingPlanFeatureKeys(
        offer({
          code: "business",
          internalSeats: 10,
          storageBytes: 500 * gib,
          documents: 1000,
          links: 0,
          rooms: 0,
          visitorAskAiMonthly: 1000,
          customDomain: true,
          watermark: true,
          nda: true,
          visitorAskAi: true,
          branding: true,
          accessControls: true,
          priceMonthlyUsd: 99,
          highlighted: true,
        }),
      ),
    ).toEqual([
      "seats_10",
      "storage500GiB",
      "docs_1000",
      "linksUnlimited",
      "roomsUnlimited",
      "ask_1000",
      "unlimitedVisitors",
      "pageAnalytics",
      "sharingControls",
      "largeFiles",
      "branding",
      "videos",
      "folders",
      "customDomain",
      "multiFileSharing",
      "emailVerification",
      "allowBlockList",
      "screenshotProtection",
      "nda",
      "watermark",
      "webhooks",
      "roomAnalytics",
      "roomInsights",
      "securityEvents",
      "emailNotifications",
      "dailyDigest",
      "slackAlerts",
      "hubspot",
    ]);
  });

  it("lists the full billing.md set on Enterprise plus Formal Ask", () => {
    expect(
      billingPlanFeatureKeys(
        offer({
          code: "enterprise",
          internalSeats: 0,
          storageBytes: 0,
          documents: 0,
          links: 0,
          rooms: 0,
          visitorAskAiMonthly: 0,
          customDomain: true,
          watermark: true,
          nda: true,
          visitorAskAi: true,
          branding: true,
          accessControls: true,
          formalAsk: true,
          customPricing: true,
        }),
      ),
    ).toEqual([
      "seatsUnlimited",
      "storageUnlimited",
      "docsUnlimited",
      "linksUnlimited",
      "roomsUnlimited",
      "askUnlimited",
      "unlimitedVisitors",
      "pageAnalytics",
      "sharingControls",
      "largeFiles",
      "branding",
      "videos",
      "folders",
      "customDomain",
      "multiFileSharing",
      "emailVerification",
      "allowBlockList",
      "screenshotProtection",
      "nda",
      "watermark",
      "webhooks",
      "roomAnalytics",
      "roomInsights",
      "emailSupport",
      "securityEvents",
      "emailNotifications",
      "dailyDigest",
      "slackAlerts",
      "hubspot",
      "formalAsk",
    ]);
  });

  it("lists Formal Ask only from the catalog flag", () => {
    expect(billingPlanFeatureKeys(offer({ code: "enterprise" }))).not.toContain("formalAsk");
    expect(billingPlanFeatureKeys(offer({ code: "pro", formalAsk: true }))).toContain("formalAsk");
  });
});

describe("currentPlanIncludedFeatureKeys", () => {
  const trialInfo = {
    plan: "trial",
    period: "monthly",
    trialExpired: false,
    storageUsed: 0,
    storageLimit: 0,
    linksUsed: 0,
    linksLimit: 0,
    roomsUsed: 0,
    roomsLimit: 0,
    seatsUsed: 1,
    seatsLimit: 10,
    customDomainEnabled: true,
    watermarkEnabled: true,
    ndaEnabled: true,
    visitorAskAiEnabled: true,
    brandingEnabled: true,
    accessControlsEnabled: true,
    knowledgeDeskEnabled: true,
    webhooksEnabled: true,
    hubspotEnabled: true,
    dailyDigestEnabled: true,
    slackAlertsEnabled: true,
    roomAnalyticsEnabled: true,
    roomInsightsEnabled: true,
    formalAskEnabled: true,
  };

  it("lists Business entitlements plus Formal Ask on an active trial", () => {
    const keys = currentPlanIncludedFeatureKeys(trialInfo);
    expect(keys).toContain("webhooks");
    expect(keys).toContain("hubspot");
    expect(keys).toContain("knowledgeDesk");
    expect(keys).toContain("roomInsights");
    expect(keys).toContain("formalAsk");
    expect(keys).not.toContain("emailSupport");
  });

  it("drops paid entitlements when the trial has expired", () => {
    const keys = currentPlanIncludedFeatureKeys({
      ...trialInfo,
      trialExpired: true,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
      brandingEnabled: false,
      accessControlsEnabled: false,
      knowledgeDeskEnabled: false,
      webhooksEnabled: false,
      hubspotEnabled: false,
      dailyDigestEnabled: false,
      slackAlertsEnabled: false,
      roomAnalyticsEnabled: false,
      roomInsightsEnabled: false,
      formalAskEnabled: false,
    });
    expect(keys).toContain("unlimitedVisitors");
    expect(keys).not.toContain("webhooks");
    expect(keys).not.toContain("formalAsk");
    expect(keys).not.toContain("largeFiles");
  });
});

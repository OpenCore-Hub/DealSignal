import { useEffect, type ReactNode } from "react";
import { Check, CreditCard, ShareNetwork, ShieldCheck, Sparkle } from "@phosphor-icons/react";
import type { Icon } from "@phosphor-icons/react";
import { Link, useParams, useSearchParams } from "react-router";
import { Button } from "@/components/ui/button";
import { UsageBar } from "@/components/common/UsageBar";
import { UpgradeCrownIcon } from "@/components/common/UpgradeCrownIcon";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { currentPlanIncludedFeatureKeys } from "@/lib/billingPlanFeatures";
import { formatDate, formatFileSize } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";
import { toast } from "sonner";
import type { BillingInfo } from "@/types";

const CHECKOUT_POLL_MS = 1000;
const CHECKOUT_WAIT_MS = 15_000;

const ENTITLEMENT_GROUPS: {
  id: "sharing" | "trust" | "intelligence";
  icon: Icon;
  keys: string[];
}[] = [
  {
    id: "sharing",
    icon: ShareNetwork,
    keys: [
      "unlimitedVisitors",
      "pageAnalytics",
      "sharingControls",
      "folders",
      "largeFiles",
      "videos",
      "multiFileSharing",
    ],
  },
  {
    id: "trust",
    icon: ShieldCheck,
    keys: [
      "emailNotifications",
      "securityEvents",
      "branding",
      "watermark",
      "screenshotProtection",
      "customDomain",
      "nda",
      "emailVerification",
      "allowBlockList",
    ],
  },
  {
    id: "intelligence",
    icon: Sparkle,
    keys: [
      "visitorAskAi",
      "knowledgeDesk",
      "formalAsk",
      "roomAnalytics",
      "roomInsights",
      "webhooks",
      "dailyDigest",
      "slackAlerts",
      "hubspot",
      "emailSupport",
    ],
  },
];

function checkoutPlanApplied(info: Pick<BillingInfo, "plan" | "hasStripeSubscription">): boolean {
  if (info.hasStripeSubscription) {
    return true;
  }
  const plan = info.plan.trim().toLowerCase();
  return plan === "pro" || plan === "business" || plan === "enterprise";
}

function billingLabel(
  t: (key: string) => string,
  prefix: "plans" | "periods",
  code: string,
): string {
  const normalized = code.trim().toLowerCase();
  const key = `billing.${prefix}.${normalized}`;
  const label = t(key);
  return label === key ? code : label;
}

function groupedEntitlements(keys: string[]) {
  const included = new Set(keys);
  const grouped = new Set(ENTITLEMENT_GROUPS.flatMap((group) => group.keys));
  const leftovers = keys.filter((key) => !grouped.has(key));
  return ENTITLEMENT_GROUPS.map((group, index) => ({
    ...group,
    index,
    items:
      group.id === "intelligence"
        ? [...group.keys.filter((key) => included.has(key)), ...leftovers]
        : group.keys.filter((key) => included.has(key)),
  })).filter((group) => group.items.length > 0);
}

function IslandLink({
  to,
  children,
  primary,
  testId,
}: {
  to: string;
  children: ReactNode;
  primary?: boolean;
  testId: string;
}) {
  return (
    <Link
      to={to}
      data-testid={testId}
      className={cn(
        "group inline-flex items-center gap-3 rounded-full py-2 pr-1.5 pl-5 text-sm tracking-tight",
        "transition-all duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] active:scale-[0.98]",
        primary
          ? "bg-foreground text-background hover:bg-foreground/90"
          : "bg-foreground/[0.04] text-foreground ring-1 ring-foreground/[0.08] hover:bg-foreground/[0.07]",
      )}
    >
      {children}
      <span
        className={cn(
          "flex size-8 items-center justify-center rounded-full transition-transform duration-500 ease-[cubic-bezier(0.32,0.72,0,1)]",
          "group-hover:scale-110",
          primary ? "bg-background/15" : "bg-foreground/[0.06]",
        )}
      >
        <UpgradeCrownIcon />
      </span>
    </Link>
  );
}

function BillingSkeleton() {
  return (
    <div className="space-y-10">
      <div className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
        <div className="space-y-3">
          <Skeleton className="h-3 w-28" />
          <Skeleton className="h-10 w-36" />
          <Skeleton className="h-4 w-64" />
        </div>
        <Skeleton className="h-11 w-32 rounded-full" />
      </div>
      <div className="space-y-5">
        <Skeleton className="h-8" />
        <Skeleton className="h-8" />
        <Skeleton className="h-8" />
        <Skeleton className="h-8" />
      </div>
    </div>
  );
}

export function SettingsBillingPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const { workspaceSlug } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const checkout = searchParams.get("checkout");
  const { data: billing, loading, error, refetch } = useAsyncData(() => api.getBillingInfo(), []);

  useEffect(() => {
    if (checkout !== "success" && checkout !== "cancel") {
      return;
    }

    let cancelled = false;
    const stripCheckout = () => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.delete("checkout");
          return next;
        },
        { replace: true },
      );
    };

    if (checkout === "cancel") {
      toast.info(t("billing.checkoutCanceled"));
      stripCheckout();
      return;
    }

    toast.info(t("billing.checkoutProcessing"));
    const started = Date.now();

    const poll = async () => {
      while (!cancelled && Date.now() - started < CHECKOUT_WAIT_MS) {
        try {
          const info = await api.getBillingInfo();
          if (cancelled) {
            return;
          }
          if (checkoutPlanApplied(info)) {
            toast.success(t("billing.checkoutSuccess"));
            await refetch();
            if (!cancelled) {
              stripCheckout();
            }
            return;
          }
        } catch {
          // Webhook may still be in flight; keep polling until the deadline.
        }
        if (cancelled) {
          return;
        }
        const remaining = CHECKOUT_WAIT_MS - (Date.now() - started);
        if (remaining <= 0) {
          break;
        }
        await new Promise((resolve) => {
          setTimeout(resolve, Math.min(CHECKOUT_POLL_MS, remaining));
        });
      }
      if (!cancelled) {
        toast.info(t("billing.checkoutPending"));
        await refetch();
        if (!cancelled) {
          stripCheckout();
        }
      }
    };
    void poll();
    return () => {
      cancelled = true;
    };
  }, [checkout, refetch, setSearchParams, t]);

  if (loading) {
    return <BillingSkeleton />;
  }

  if (error) {
    return (
      <div className="max-w-xl space-y-6">
        <p className="text-[10px] font-medium uppercase tracking-[0.22em] text-muted-foreground">
          {t("billing.title")}
        </p>
        <h2 className="text-3xl tracking-tighter text-foreground">{t("billing.loadFailed")}</h2>
        <p className="text-sm leading-relaxed text-muted-foreground">{error}</p>
        <Button
          variant="outline"
          size="sm"
          className="rounded-full px-4"
          onClick={refetch}
        >
          {tc("retry")}
        </Button>
      </div>
    );
  }

  if (!billing) {
    return null;
  }

  const planName = billingLabel(t, "plans", billing.plan);
  const planLine = [
    planName,
    billing.trialExpired ? t("billing.trialExpired") : null,
    !billing.trialExpired && billing.trialEndsAt
      ? t("billing.trialEnds", { date: formatDate(billing.trialEndsAt) })
      : null,
    billingLabel(t, "periods", billing.period),
  ]
    .filter(Boolean)
    .join(" · ");

  const entitlementGroups = groupedEntitlements(currentPlanIncludedFeatureKeys(billing));

  const usageMeters: {
    key: string;
    label: string;
    current: number;
    max: number;
    formatCurrent?: string;
    formatMax?: string;
    included?: boolean;
  }[] = [
    {
      key: "documents",
      label: t("billing.documents"),
      current: billing.documentsUsed ?? 0,
      max: billing.documentsLimit ?? 0,
    },
    {
      key: "storage",
      label: t("billing.storage"),
      current: billing.storageUsed,
      max: billing.storageLimit,
      formatCurrent: formatFileSize(billing.storageUsed),
      formatMax: billing.storageLimit > 0 ? formatFileSize(billing.storageLimit) : undefined,
    },
    {
      key: "askAi",
      label: t("billing.askAi"),
      current: billing.askAiUsed ?? 0,
      max: billing.askAiLimit ?? 0,
      included: billing.visitorAskAiEnabled,
    },
    {
      key: "knowledge",
      label: t("billing.knowledgeAnswers"),
      current: billing.knowledgeAnswersUsed ?? 0,
      max: billing.knowledgeAnswersLimit ?? 0,
      included: Boolean(billing.knowledgeDeskEnabled),
    },
    {
      key: "links",
      label: t("billing.links"),
      current: billing.linksUsed,
      max: billing.linksLimit,
    },
    {
      key: "rooms",
      label: t("billing.rooms"),
      current: billing.roomsUsed,
      max: billing.roomsLimit,
    },
    {
      key: "seats",
      label: t("billing.seats"),
      current: billing.seatsUsed,
      max: billing.seatsLimit,
    },
  ];

  return (
    <div className="space-y-12 pb-4">
      <section className="animate-fade-up">
        <div className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div className="min-w-0">
            <h2 className="text-[10px] font-medium uppercase tracking-[0.22em] text-muted-foreground">
              {t("billing.title")}
            </h2>
            <div
              aria-hidden
              data-plan={planName}
              className="mt-3 text-[2.5rem] leading-none tracking-tighter text-foreground before:content-[attr(data-plan)] sm:text-[2.75rem]"
            />
            <p
              className="mt-3 max-w-xl text-sm leading-relaxed text-muted-foreground"
              data-testid="billing-plan-summary"
            >
              {planLine}
            </p>
          </div>
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            {billing.hasStripeSubscription ? (
              <button
                type="button"
                data-testid="billing-manage"
                className={cn(
                  "group inline-flex items-center gap-3 rounded-full py-2 pr-1.5 pl-5 text-sm tracking-tight",
                  "bg-foreground/[0.04] text-foreground ring-1 ring-foreground/[0.08]",
                  "transition-all duration-500 ease-[cubic-bezier(0.32,0.72,0,1)]",
                  "hover:bg-foreground/[0.07] active:scale-[0.98]",
                )}
                onClick={() => {
                  void (async () => {
                    try {
                      const { url } = await api.createBillingPortal();
                      window.location.assign(url);
                    } catch (err) {
                      toast.error(apiErrorMessage(err, { fallback: "saveFailed" }));
                    }
                  })();
                }}
              >
                {t("billing.manageBilling")}
                <span className="flex size-8 items-center justify-center rounded-full bg-foreground/[0.06] transition-transform duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:translate-x-0.5 group-hover:-translate-y-px">
                  <CreditCard size={14} weight="light" aria-hidden />
                </span>
              </button>
            ) : null}
            <IslandLink
              to={`/${workspaceSlug}/settings/billing/plans`}
              testId="billing-upgrade"
              primary
            >
              {t("billing.upgrade")}
            </IslandLink>
          </div>
        </div>
      </section>

      <section
        className="animate-fade-up space-y-5 [animation-delay:80ms]"
        data-testid="billing-usage"
      >
        <p className="text-[10px] font-medium uppercase tracking-[0.22em] text-muted-foreground">
          {t("billing.usageHeading")}
        </p>
        <div className="space-y-5">
          {usageMeters.map((meter) => (
            <UsageBar
              key={meter.key}
              variant="ledger"
              label={meter.label}
              current={meter.current}
              max={meter.max}
              formatCurrent={meter.formatCurrent}
              formatMax={meter.formatMax}
              included={meter.included}
            />
          ))}
        </div>
      </section>

      <section
        className="animate-fade-up space-y-6 [animation-delay:160ms]"
        data-testid="billing-features"
      >
        <p className="text-[10px] font-medium uppercase tracking-[0.22em] text-muted-foreground">
          {t("billing.includedFeatures")}
        </p>
        <div className="grid grid-cols-1 gap-10 md:grid-cols-3 md:gap-0 md:divide-x md:divide-foreground/[0.08]">
          {entitlementGroups.map((group) => {
            const Icon = group.icon;
            return (
              <div key={group.id} className="min-w-0 md:px-8 md:first:pl-0 md:last:pr-0">
                <div className="flex size-9 items-center justify-center rounded-full bg-foreground/[0.05] ring-1 ring-foreground/[0.06]">
                  <Icon size={18} weight="light" aria-hidden />
                </div>
                <div className="mt-4 flex items-baseline justify-between gap-3">
                  <h3 className="text-[15px] tracking-tight text-foreground">
                    {t(`billing.entitlementGroups.${group.id}`)}
                  </h3>
                  <span className="shrink-0 font-mono text-[11px] tabular-nums text-muted-foreground">
                    {t("billing.entitlementGroups.includedCount", { count: group.items.length })}
                  </span>
                </div>
                <p className="mt-1.5 max-w-[28ch] text-[13px] leading-relaxed text-muted-foreground">
                  {t(`billing.entitlementGroups.${group.id}Lead`)}
                </p>
                <ul className="mt-5 space-y-2.5">
                  {group.items.map((key) => (
                    <li
                      key={key}
                      className="flex items-start gap-2 text-[13px] leading-snug text-foreground"
                    >
                      <Check
                        size={14}
                        weight="light"
                        className="mt-0.5 shrink-0 text-foreground/45"
                        aria-hidden
                      />
                      {t(`billing.planFeatures.${key}`)}
                    </li>
                  ))}
                </ul>
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}

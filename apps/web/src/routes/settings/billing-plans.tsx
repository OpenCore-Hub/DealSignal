import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Check } from "@phosphor-icons/react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { UpgradeCrownIcon } from "@/components/common/UpgradeCrownIcon";
import { Skeleton } from "@/components/ui/skeleton";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { billingPlanFeatureKeys } from "@/lib/billingPlanFeatures";
import { cn } from "@/lib/utils";
import type { BillingPlanOffer } from "@/types";

function planName(t: (key: string) => string, code: string): string {
  const key = `billing.plans.${code}`;
  const label = t(key);
  return label === key ? code : label;
}

function priceLabel(
  t: (key: string, opts?: Record<string, unknown>) => string,
  offer: BillingPlanOffer,
  period: "monthly" | "yearly",
): string {
  if (offer.customPricing) {
    return t("billing.plansPage.customPrice");
  }
  if (offer.priceMonthlyUsd <= 0) {
    return t("billing.plansPage.priceFree");
  }
  const monthly = offer.priceMonthlyUsd;
  if (period === "yearly") {
    return t("billing.plansPage.priceYearly", {
      amount: Math.round(monthly * 12 * 0.85),
    });
  }
  return t("billing.plansPage.priceMonthly", { amount: monthly });
}

function isCapacityKey(key: string): boolean {
  return /^(seats|storage|docs|links|rooms|ask)/.test(key);
}

function partitionFeatures(keys: string[], previousKeys: string[]) {
  const prev = new Set(previousKeys);
  const capacity = keys.filter(isCapacityKey);
  const qualitative = keys.filter((key) => !isCapacityKey(key));
  return {
    capacity,
    unlocks: qualitative.filter((key) => !prev.has(key)),
    inherited: qualitative.filter((key) => prev.has(key)),
  };
}

function FeatureList({
  keys,
  t,
  inverted,
  quiet,
}: {
  keys: string[];
  t: (key: string) => string;
  inverted?: boolean;
  quiet?: boolean;
}) {
  if (keys.length === 0) return null;
  return (
    <ul className={cn("space-y-2", quiet && "space-y-1.5")}>
      {keys.map((key) => (
        <li
          key={key}
          className={cn(
            "flex items-start gap-2 leading-snug",
            quiet ? "text-[12px]" : "text-[13px]",
            inverted
              ? quiet
                ? "text-background/55"
                : "text-background"
              : quiet
                ? "text-muted-foreground"
                : "text-foreground",
          )}
        >
          <Check
            size={14}
            weight="light"
            className={cn(
              "mt-0.5 shrink-0",
              inverted ? "text-background/50" : "text-foreground/40",
            )}
            aria-hidden
          />
          <span>{t(`billing.planFeatures.${key}`)}</span>
        </li>
      ))}
    </ul>
  );
}

export function SettingsBillingPlansPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const { workspaceSlug } = useParams();
  const navigate = useNavigate();
  const { canManage } = useWorkspaceAccess();
  const [period, setPeriod] = useState<"monthly" | "yearly">("monthly");
  const [selecting, setSelecting] = useState<string | null>(null);

  const { data, loading, error, refetch } = useAsyncData(() => api.getBillingPlans(), []);

  const onChoose = async (code: string) => {
    if (!canManage || selecting) return;
    if (data?.currentPlan === code && data.currentPeriod === period) {
      toast.success(t("billing.plansPage.alreadyOnPlan"));
      return;
    }
    setSelecting(code);
    try {
      if (code === "free") {
        if (data?.hasStripeSubscription) {
          const { url } = await api.createBillingPortal();
          window.location.assign(url);
          return;
        }
        await api.changeBillingPlan(code, period);
        toast.success(t("billing.plansPage.changeSuccess", { plan: planName(t, code) }));
        await refetch();
        navigate(`/${workspaceSlug}/settings/billing`);
        return;
      }
      if (data?.hasStripeSubscription) {
        const { url } = await api.createBillingPortal();
        window.location.assign(url);
        return;
      }
      const { url } = await api.createBillingCheckout(code, period);
      toast.message(t("billing.plansPage.checkoutRedirect"));
      window.location.assign(url);
    } catch (err) {
      toast.error(apiErrorMessage(err, { fallback: "saveFailed" }));
    } finally {
      setSelecting(null);
    }
  };

  if (loading) {
    return (
      <div className="space-y-10">
        <div className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div className="space-y-3">
            <Skeleton className="h-3 w-24" />
            <Skeleton className="h-10 w-56" />
          </div>
          <Skeleton className="h-10 w-40 rounded-full" />
        </div>
        <Skeleton className="h-[28rem] rounded-[1.75rem]" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="max-w-xl space-y-5">
        <h1 className="text-3xl tracking-tighter">{t("billing.plansPage.title")}</h1>
        <p className="text-sm text-error-500">{t("billing.loadFailed")}</p>
        <Button variant="outline" size="sm" className="rounded-full px-4" onClick={refetch}>
          {tc("retry")}
        </Button>
      </div>
    );
  }

  const currentLabel = planName(t, data.currentPlan);

  return (
    <div className="space-y-10 pb-4" data-testid="billing-plans-page">
      <section className="animate-fade-up flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
        <div className="min-w-0">
          <Link
            to={`/${workspaceSlug}/settings/billing`}
            className="inline-flex items-center gap-1.5 text-[13px] text-muted-foreground transition-colors duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] hover:text-foreground"
          >
            <ArrowLeft size={14} weight="light" aria-hidden />
            {t("billing.plansPage.back")}
          </Link>
          <div className="mt-4 flex items-center gap-3">
            <UpgradeCrownIcon size={22} />
            <h1 className="text-[2.5rem] leading-none tracking-tighter text-foreground">
              {t("billing.plansPage.title")}
            </h1>
          </div>
          <p className="mt-3 text-sm text-muted-foreground">
            {t("billing.plansPage.subtitle", { plan: currentLabel })}
            {data.currentPlan === "trial" && data.trialEndsAt
              ? ` · ${t("billing.trialEnds", { date: data.trialEndsAt.slice(0, 10) })}`
              : null}
          </p>
        </div>
        <div className="flex flex-col items-start gap-2 sm:items-end">
          <p className="text-[10px] font-medium uppercase tracking-[0.18em] text-muted-foreground">
            {t("billing.plansPage.billedAs")}
          </p>
          <div
            className="inline-flex rounded-full bg-foreground/[0.04] p-1 ring-1 ring-foreground/[0.06]"
            data-testid="billing-period-toggle"
            role="group"
            aria-label={t("billing.plansPage.billedAs")}
          >
            {(["monthly", "yearly"] as const).map((p) => (
              <button
                key={p}
                type="button"
                aria-pressed={period === p}
                className={cn(
                  "inline-flex items-center gap-2 rounded-full px-4 py-1.5 text-[13px] tracking-tight",
                  "transition-all duration-500 ease-[cubic-bezier(0.32,0.72,0,1)]",
                  period === p
                    ? "bg-foreground text-background"
                    : "text-muted-foreground hover:text-foreground",
                )}
                onClick={() => setPeriod(p)}
              >
                {t(`billing.periods.${p === "yearly" ? "yearly" : "monthly"}`)}
                {p === "yearly" ? (
                  <span className="rounded-full bg-[#F4C430] px-1.5 py-px text-[10px] font-medium tracking-[0.02em] text-[#0f172a]">
                    {t("billing.plansPage.saveAnnual")}
                  </span>
                ) : null}
              </button>
            ))}
          </div>
          <p className="text-[12px] text-muted-foreground">
            {period === "yearly"
              ? t("billing.plansPage.billedYearlyHint")
              : t("billing.plansPage.billedMonthlyHint")}
          </p>
        </div>
      </section>

      <div className="animate-fade-up rounded-[1.75rem] bg-foreground/[0.03] p-1.5 ring-1 ring-foreground/[0.06] [animation-delay:80ms]">
        <div className="grid grid-cols-1 overflow-hidden rounded-[calc(1.75rem-0.375rem)] bg-background md:grid-cols-2 xl:grid-cols-4">
          {data.plans.map((offer, index) => {
            const isCurrent = data.currentPlan === offer.code;
            const inverted = offer.highlighted;
            const previous = data.plans[index - 1];
            const { capacity, unlocks, inherited } = partitionFeatures(
              billingPlanFeatureKeys(offer),
              previous ? billingPlanFeatureKeys(previous) : [],
            );
            const leadKey = `billing.plansPage.leads.${offer.code}`;
            const lead = t(leadKey);
            const ctaLabel = isCurrent
              ? t("billing.plansPage.currentPlan")
              : selecting === offer.code
                ? tc("saving")
                : offer.customPricing
                  ? t("billing.plansPage.contactSales")
                  : t("billing.plansPage.choose", { plan: planName(t, offer.code) });

            return (
              <article
                key={offer.code}
                data-testid={`billing-plan-card-${offer.code}`}
                className={cn(
                  "flex flex-col px-6 py-7",
                  inverted
                    ? "bg-foreground text-background"
                    : "ring-1 ring-foreground/[0.06] xl:ring-0 xl:[&:not(:first-child)]:border-l xl:[&:not(:first-child)]:border-foreground/[0.08]",
                )}
              >
                <div className="flex items-start justify-between gap-3">
                  <h2 className="text-[13px] font-medium uppercase tracking-[0.16em]">
                    {planName(t, offer.code)}
                  </h2>
                  {offer.highlighted ? (
                    <span
                      className={cn(
                        "rounded-full px-2 py-0.5 text-[10px] font-medium tracking-[0.08em] uppercase",
                        inverted ? "bg-background/15 text-background" : "bg-foreground/10",
                      )}
                    >
                      {t("billing.plansPage.mostPopular")}
                    </span>
                  ) : null}
                </div>
                <p className="mt-5 font-mono text-[2rem] leading-none tracking-tight">
                  {priceLabel(t, offer, period)}
                </p>
                {lead !== leadKey ? (
                  <p
                    className={cn(
                      "mt-3 max-w-[22ch] text-[13px] leading-relaxed",
                      inverted ? "text-background/70" : "text-muted-foreground",
                    )}
                  >
                    {lead}
                  </p>
                ) : null}

                <ul className="mt-6 space-y-2">
                  {capacity.map((key) => (
                    <li
                      key={key}
                      className="font-mono text-[13px] tracking-tight tabular-nums"
                    >
                      {t(`billing.planFeatures.${key}`)}
                    </li>
                  ))}
                </ul>

                <div className="mt-6 flex-1 space-y-5">
                  <div>
                    <p
                      className={cn(
                        "mb-3 text-[10px] font-medium uppercase tracking-[0.18em]",
                        inverted ? "text-background/50" : "text-muted-foreground",
                      )}
                    >
                      {offer.code === "free"
                        ? t("billing.plansPage.includesBase")
                        : t("billing.plansPage.includesPrevious")}
                    </p>
                    <FeatureList keys={unlocks} t={t} inverted={inverted} />
                  </div>
                  {inherited.length > 0 ? (
                    <div>
                      <p
                        className={cn(
                          "mb-3 text-[10px] font-medium uppercase tracking-[0.18em]",
                          inverted ? "text-background/50" : "text-muted-foreground",
                        )}
                      >
                        {t("billing.plansPage.alsoIncludes")}
                      </p>
                      <FeatureList keys={inherited} t={t} inverted={inverted} quiet />
                    </div>
                  ) : null}
                </div>

                <button
                  type="button"
                  disabled={!canManage || selecting !== null || isCurrent}
                  data-testid={`billing-choose-${offer.code}`}
                  className={cn(
                    "mt-7 inline-flex w-full items-center justify-center rounded-full py-2.5 text-sm tracking-tight",
                    "transition-all duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] active:scale-[0.98]",
                    "disabled:pointer-events-none disabled:opacity-40",
                    inverted
                      ? "bg-background text-foreground hover:bg-background/90"
                      : "bg-foreground text-background hover:bg-foreground/90",
                  )}
                  onClick={() => {
                    if (offer.customPricing) {
                      toast.message(t("billing.plansPage.contactSalesHint"));
                      return;
                    }
                    void onChoose(offer.code);
                  }}
                >
                  {ctaLabel}
                </button>
                {!canManage ? (
                  <p
                    className={cn(
                      "mt-2 text-center text-[12px]",
                      inverted ? "text-background/55" : "text-muted-foreground",
                    )}
                  >
                    {t("billing.plansPage.managersOnly")}
                  </p>
                ) : null}
              </article>
            );
          })}
        </div>
      </div>
    </div>
  );
}

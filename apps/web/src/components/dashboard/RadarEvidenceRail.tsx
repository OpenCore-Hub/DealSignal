import { type ReactNode } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import { ArrowRight, SpinnerGap } from "@phosphor-icons/react";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import {
  coalesceSecurityEvents,
  evidenceEmptyPrimaryKey,
  gateTimelineSummary,
  type CoalescedSecurityEvent,
  type GateTimelineSummary,
} from "@/lib/radarEvidencePresentation";
import {
  evidenceMetricsHaveActivity,
  type RadarEvidencePack,
  type RadarWorkItem,
} from "@/lib/radarQueue";
import { cn } from "@/lib/utils";

/** Shared tile for Evidence list rows (parity with metric tiles across all 6 products). */
const EMBEDDED_ITEM =
  "rounded-md border border-border bg-card px-2.5 py-2 text-sm";

interface RadarEvidenceRailProps {
  item: RadarWorkItem | null;
  workspaceSlug: string;
}

export function RadarEvidenceRail({
  item,
  workspaceSlug,
}: RadarEvidenceRailProps) {
  const { t } = useTranslation("dashboard");

  const { data, loading, error } = useAsyncData<RadarEvidencePack | null>(
    async () => {
      if (!item) return null;
      return api.getRadarEvidence(item.id);
    },
    [item?.id],
  );

  if (!item) {
    return (
      <section
        data-testid="radar-evidence-rail"
        className="rounded-xl border border-border px-4 py-4"
      >
        <h3 className="text-sm font-semibold">{t("radar.evidenceRail.title")}</h3>
        <p className="text-caption mt-2 text-muted-foreground">
          {t("radar.evidenceRail.empty")}
        </p>
      </section>
    );
  }

  const isDiligenceGate = item.product === "diligence_gate";
  const accessRequest = data?.accessRequest;
  const metricsActive = evidenceMetricsHaveActivity(data?.metrics);
  const coalescedEvents = coalesceSecurityEvents(data?.securityEvents);
  const emptyPrimaryKey = evidenceEmptyPrimaryKey(item.product, {
    metricsActive,
    hasSecurityEvents: coalescedEvents.length > 0,
  });
  // Timeline narrative is for Diligence gate (request ± gate hits), not leak/abuse.
  const timeline =
    isDiligenceGate
      ? gateTimelineSummary(data?.securityEvents, accessRequest?.requestedAt)
      : null;
  // Selected radar row already shows deal / headline / actor / why-now — Evidence leads with facets only.
  const cardActor = item.actor?.trim().toLowerCase() ?? "";

  return (
    <section
      data-testid="radar-evidence-rail"
      className="rounded-xl border border-border px-4 py-4"
    >
      <h3 className="text-sm font-semibold">{t("radar.evidenceRail.title")}</h3>

      {loading ? (
        <div className="mt-4 flex items-center gap-2 text-caption text-muted-foreground">
          <SpinnerGap size={14} className="animate-spin" />
          {t("radar.evidenceRail.loading")}
        </div>
      ) : null}

      {error ? (
        <p className="text-caption mt-4 text-error-500">
          {t("radar.evidenceRail.error")}
        </p>
      ) : null}

      {data ? (
        <div className="mt-4 space-y-4">
          {data.degradedSections && data.degradedSections.length > 0 ? (
            <p
              className="rounded-md border border-warning-500/30 bg-warning-500/10 px-2.5 py-2 text-caption text-warning-500"
              data-testid="radar-evidence-degraded"
            >
              {t("radar.evidenceRail.degraded")}{" "}
              {data.degradedSections
                .map((section) =>
                  t(`radar.evidenceRail.degradedSections.${section}`, {
                    defaultValue: section,
                  }),
                )
                .join(" · ")}
            </p>
          ) : null}

          {accessRequest ? (
            <Block title={t("radar.evidenceRail.accessRequest.title")}>
              <dl
                className="space-y-1.5"
                data-testid="radar-evidence-access-request"
              >
                {accessRequest.reason ? (
                  <div className={EMBEDDED_ITEM}>
                    <dt className="text-caption text-muted-foreground">
                      {t("radar.evidenceRail.accessRequest.reason")}
                    </dt>
                    <dd className="text-foreground">{accessRequest.reason}</dd>
                  </div>
                ) : (
                  <div className={EMBEDDED_ITEM}>
                    <dt className="text-caption text-muted-foreground">
                      {t("radar.evidenceRail.accessRequest.reason")}
                    </dt>
                    <dd className="text-muted-foreground">
                      {t("radar.evidenceRail.accessRequest.noReason")}
                    </dd>
                  </div>
                )}
                <div className={EMBEDDED_ITEM}>
                  <dt className="text-caption text-muted-foreground">
                    {t("radar.evidenceRail.accessRequest.surface")}
                  </dt>
                  <dd className="text-muted-foreground">
                    {t(`radar.evidenceRail.accessRequest.surfaces.${accessRequest.surface}`, {
                      defaultValue: accessRequest.surface,
                    })}
                  </dd>
                </div>
              </dl>
            </Block>
          ) : null}

          {timeline ? (
            <p
              className="text-sm text-muted-foreground"
              data-testid="radar-evidence-gate-timeline"
            >
              {formatGateTimeline(t, timeline)}
            </p>
          ) : null}

          {coalescedEvents.length > 0 ? (
            <Block
              title={
                isDiligenceGate
                  ? t("radar.evidenceRail.gateEvents")
                  : t("radar.evidenceRail.securityEvents")
              }
            >
              <ul className="space-y-1.5" data-testid="radar-evidence-security-events">
                {coalescedEvents.map((group) => (
                  <CoalescedGateRow
                    key={group.key}
                    group={group}
                    label={eventLabel(t, group)}
                    countLabel={t("radar.evidenceRail.coalesced.count", {
                      count: group.count,
                    })}
                    hideEmail={
                      Boolean(cardActor) &&
                      group.email?.trim().toLowerCase() === cardActor
                    }
                  />
                ))}
              </ul>
            </Block>
          ) : null}

          {data.metrics && metricsActive ? (
            <dl className="grid grid-cols-2 gap-2" data-testid="radar-evidence-metrics">
              <Metric
                label={t("radar.evidenceRail.metrics.opens24h")}
                value={data.metrics.opens24h}
              />
              <Metric
                label={t("radar.evidenceRail.metrics.visitors24h")}
                value={data.metrics.uniqueVisitors24h}
              />
              <Metric
                label={t("radar.evidenceRail.metrics.forwards24h")}
                value={data.metrics.forwardSignals24h}
              />
              <Metric
                label={t("radar.evidenceRail.metrics.downloads24h")}
                value={data.metrics.downloads24h}
              />
              {(data.metrics.captureAttempts24h ?? 0) > 0 ? (
                <Metric
                  label={t("radar.evidenceRail.metrics.captures24h")}
                  value={data.metrics.captureAttempts24h ?? 0}
                />
              ) : null}
            </dl>
          ) : null}

          {/* Product-primary empty copy — never lead with four zero tiles. */}
          {emptyPrimaryKey ? (
            <p
              className="text-caption text-muted-foreground"
              data-testid={
                isDiligenceGate
                  ? "radar-evidence-gate-no-opens"
                  : "radar-evidence-empty-primary"
              }
            >
              {t(emptyPrimaryKey)}
            </p>
          ) : null}

          {data.keyPageTitles && data.keyPageTitles.length > 0 ? (
            <Block title={t("radar.evidenceRail.keyPages")}>
              <ul className="space-y-1.5" data-testid="radar-evidence-key-pages">
                {data.keyPageTitles.map((title) => (
                  <li key={title} className={cn(EMBEDDED_ITEM, "text-foreground")}>
                    {title}
                  </li>
                ))}
              </ul>
            </Block>
          ) : null}

          {data.topPages && data.topPages.length > 0 ? (
            <Block title={t("radar.evidenceRail.topPages")}>
              <ul className="space-y-1.5" data-testid="radar-evidence-top-pages">
                {data.topPages.map((p) => (
                  <li
                    key={p.pageNumber}
                    className={cn(EMBEDDED_ITEM, "flex justify-between gap-2")}
                  >
                    <span>
                      {t("radar.evidenceRail.page", { page: p.pageNumber })}
                    </span>
                    <span className="tabular-nums text-muted-foreground">
                      {t("radar.evidenceRail.views", { count: p.views })}
                    </span>
                  </li>
                ))}
              </ul>
            </Block>
          ) : null}

          {data.recentVisitors && data.recentVisitors.length > 0 ? (
            <Block title={t("radar.evidenceRail.recentVisitors")}>
              <ul
                className="space-y-1.5"
                data-testid="radar-evidence-recent-visitors"
              >
                {data.recentVisitors.map((v) => (
                  <li
                    key={v.visitorId}
                    className={cn(
                      EMBEDDED_ITEM,
                      "flex flex-wrap items-baseline justify-between gap-x-2 gap-y-0.5",
                    )}
                  >
                    <span className="font-medium">
                      {v.email || t("radar.evidenceRail.anonymousVisitor")}
                    </span>
                    <span className="text-caption text-muted-foreground">
                      {t("radar.evidenceRail.views", { count: v.totalViews })}
                    </span>
                  </li>
                ))}
              </ul>
            </Block>
          ) : null}

          {(data.insightsPath || data.evidencePath || data.navigatePath) && (
            <Link
              to={
                data.navigatePath ||
                data.insightsPath ||
                data.evidencePath ||
                `/${workspaceSlug}/insights/overview`
              }
              className="inline-flex items-center gap-1 text-sm font-medium text-foreground hover:underline"
              data-testid="radar-evidence-open"
            >
              {isDiligenceGate && accessRequest
                ? t("radar.evidenceRail.openShareInbox")
                : t("radar.evidenceRail.openFull")}
              <ArrowRight size={14} />
            </Link>
          )}
        </div>
      ) : null}
    </section>
  );
}

function eventLabel(
  t: (key: string, opts?: Record<string, unknown>) => string,
  group: CoalescedSecurityEvent,
): string {
  if (group.reason) {
    return t(`radar.evidenceRail.reasons.${group.reason}`, {
      defaultValue: t(`radar.evidenceRail.eventTypes.${group.eventType}`, {
        defaultValue: t("radar.evidenceRail.eventTypes.unknown"),
      }),
    });
  }
  return t(`radar.evidenceRail.eventTypes.${group.eventType}`, {
    defaultValue: t("radar.evidenceRail.eventTypes.unknown"),
  });
}

function formatGateTimeline(
  t: (key: string, opts?: Record<string, unknown>) => string,
  summary: GateTimelineSummary,
): string {
  switch (summary.kind) {
    case "before_and_after":
      return t("radar.evidenceRail.gateTimeline.beforeAndAfter", {
        before: summary.before,
        after: summary.after,
      });
    case "before_only":
      return t("radar.evidenceRail.gateTimeline.beforeOnly", {
        before: summary.before,
      });
    case "after_only":
      return t("radar.evidenceRail.gateTimeline.afterOnly", {
        after: summary.after,
      });
    case "events_only":
      return t("radar.evidenceRail.gateTimeline.eventsOnly", {
        total: summary.total,
      });
  }
}

function CoalescedGateRow({
  group,
  label,
  countLabel,
  hideEmail,
}: {
  group: CoalescedSecurityEvent;
  label: string;
  countLabel: string;
  hideEmail?: boolean;
}) {
  return (
    <li className={EMBEDDED_ITEM}>
      <div className="flex flex-wrap items-baseline gap-x-1.5 gap-y-0.5">
        <span className="font-medium">{label}</span>
        {group.count > 1 ? (
          <span className="text-caption text-muted-foreground">· {countLabel}</span>
        ) : null}
        {!hideEmail && group.email ? (
          <span className="text-caption text-muted-foreground">· {group.email}</span>
        ) : null}
      </div>
    </li>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div className={EMBEDDED_ITEM}>
      <dt className="text-caption text-muted-foreground">{label}</dt>
      <dd className="font-semibold tabular-nums">{value}</dd>
    </div>
  );
}

function Block({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div>
      <h4 className="text-caption font-medium text-muted-foreground">{title}</h4>
      <div className="mt-1.5">{children}</div>
    </div>
  );
}

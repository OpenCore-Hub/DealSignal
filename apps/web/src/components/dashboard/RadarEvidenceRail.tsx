import { type ReactNode } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import { ArrowRight, SpinnerGap } from "@phosphor-icons/react";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { documentDetailPath } from "@/lib/documentDetailNav";
import { displayablePageTitles } from "@/lib/insights/pageTitleDisplay";
import {
  coalesceSecurityEvents,
  evidenceEmptyPrimaryKey,
  gateTimelineI18nKey,
  gateTimelineSummary,
  isRadarGateHoldItem,
  radarEvidenceOpenLabelKey,
  radarEvidenceOpenPath,
  topPagesSpanMultipleDocuments,
  type CoalescedSecurityEvent,
  type GateTimelineSummary,
} from "@/lib/radarEvidencePresentation";
import {
  evidenceMetricsHaveActivity,
  type RadarEvidencePack,
  type RadarWorkItem,
} from "@/lib/radarQueue";
import { cn } from "@/lib/utils";
import { isAccessGatePromptReason } from "@/lib/accessEventLabels";

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
  const foldShareActivity = isRadarGateHoldItem(item);
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
  const labelTopPagesWithDocument = topPagesSpanMultipleDocuments(data?.topPages ?? []);
  const keyPageTitles = displayablePageTitles(data?.keyPageTitles);
  const openHref = data
    ? radarEvidenceOpenPath({
        product: item.product,
        workspaceSlug,
        dealRoomId: item.dealRoomId,
        linkId: data.linkId || item.linkId,
        navigatePath: data.navigatePath || item.navigatePath,
        insightsPath: data.insightsPath,
        evidencePath: data.evidencePath || item.evidencePath,
      })
    : null;

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
              {formatGateTimeline(t, timeline, accessRequest?.status)}
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
                    prompt={isAccessGatePromptReason(group.reason)}
                    promptLabel={t("radar.evidenceRail.gatePromptBadge")}
                    hideEmail={
                      Boolean(cardActor) &&
                      group.email?.trim().toLowerCase() === cardActor
                    }
                  />
                ))}
              </ul>
            </Block>
          ) : null}

          <ShareActivitySection
            fold={foldShareActivity}
            metrics={data.metrics}
            visitors={data.recentVisitors}
            t={t}
          />

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

          {keyPageTitles.length > 0 ? (
            <Block title={t("radar.evidenceRail.keyPages")}>
              <ul className="space-y-1.5" data-testid="radar-evidence-key-pages">
                {keyPageTitles.map((title) => (
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
                {data.topPages.map((p) => {
                  const documentId = p.documentId?.trim();
                  const title =
                    p.documentTitle?.trim() || documentId || "";
                  const label =
                    labelTopPagesWithDocument && title
                      ? t("radar.evidenceRail.pageOnDocument", {
                          title,
                          page: p.pageNumber,
                        })
                      : t("radar.evidenceRail.page", { page: p.pageNumber });
                  return (
                    <li
                      key={`${documentId ?? ""}-${p.pageNumber}`}
                      className={cn(EMBEDDED_ITEM, "flex justify-between gap-2")}
                    >
                      {documentId ? (
                        <Link
                          to={documentDetailPath(workspaceSlug, documentId, {
                            tab: "content",
                            page: p.pageNumber,
                          })}
                          className="hover:underline"
                        >
                          {label}
                        </Link>
                      ) : (
                        <span>{label}</span>
                      )}
                      <span className="tabular-nums text-muted-foreground">
                        {t("radar.evidenceRail.views", { count: p.views })}
                      </span>
                    </li>
                  );
                })}
              </ul>
            </Block>
          ) : null}

          {!foldShareActivity && data.recentVisitors && data.recentVisitors.length > 0 ? (
            <RecentVisitorsBlock visitors={data.recentVisitors} t={t} />
          ) : null}

          {openHref ? (
            <Link
              to={openHref}
              className="inline-flex items-center gap-1 text-sm font-medium text-foreground hover:underline"
              data-testid="radar-evidence-open"
            >
              {t(radarEvidenceOpenLabelKey(item.product, openHref))}
              <ArrowRight size={14} />
            </Link>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

type EvidenceTranslate = (key: string, opts?: Record<string, unknown>) => string;

function ShareActivitySection({
  fold,
  metrics,
  visitors,
  t,
}: {
  fold: boolean;
  metrics?: RadarEvidencePack["metrics"];
  visitors?: RadarEvidencePack["recentVisitors"];
  t: EvidenceTranslate;
}) {
  const showMetrics = Boolean(metrics && evidenceMetricsHaveActivity(metrics));
  const showVisitors = Boolean(visitors?.length);
  if (!showMetrics && !(fold && showVisitors)) return null;

  const body = (
    <>
      {showMetrics && metrics ? (
        <div data-testid="radar-evidence-metrics">
          {fold ? (
            <p
              className="mb-2 text-caption text-muted-foreground"
              data-testid="radar-evidence-metrics-link-level"
            >
              {t("radar.evidenceRail.metrics.linkLevel")}
            </p>
          ) : null}
          <EvidenceMetricsGrid metrics={metrics} t={t} />
        </div>
      ) : null}
      {fold && showVisitors ? (
        <RecentVisitorsBlock visitors={visitors ?? []} t={t} />
      ) : null}
    </>
  );

  if (!fold) return body;

  return (
    <details data-testid="radar-evidence-share-activity">
      <summary
        className="cursor-pointer text-caption font-medium text-muted-foreground"
        data-testid="radar-evidence-share-activity-summary"
      >
        {t("radar.evidenceRail.shareActivity.summary")}
      </summary>
      <div className="mt-2 space-y-3">{body}</div>
    </details>
  );
}

function EvidenceMetricsGrid({
  metrics,
  t,
}: {
  metrics: NonNullable<RadarEvidencePack["metrics"]>;
  t: EvidenceTranslate;
}) {
  return (
    <dl className="grid grid-cols-2 gap-2">
      <Metric
        label={t("radar.evidenceRail.metrics.opens24h")}
        value={metrics.opens24h}
      />
      <Metric
        label={t("radar.evidenceRail.metrics.visitors24h")}
        value={metrics.uniqueVisitors24h}
      />
      <Metric
        label={t("radar.evidenceRail.metrics.forwards24h")}
        value={metrics.forwardSignals24h}
      />
      <Metric
        label={t("radar.evidenceRail.metrics.downloads24h")}
        value={metrics.downloads24h}
      />
      {(metrics.captureAttempts24h ?? 0) > 0 ? (
        <Metric
          label={t("radar.evidenceRail.metrics.captures24h")}
          value={metrics.captureAttempts24h ?? 0}
        />
      ) : null}
    </dl>
  );
}

function RecentVisitorsBlock({
  visitors,
  t,
}: {
  visitors: NonNullable<RadarEvidencePack["recentVisitors"]>;
  t: EvidenceTranslate;
}) {
  return (
    <Block title={t("radar.evidenceRail.recentVisitors")}>
      <ul className="space-y-1.5" data-testid="radar-evidence-recent-visitors">
        {visitors.map((v) => (
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
  requestStatus?: string | null,
): string {
  const key = gateTimelineI18nKey(summary, requestStatus);
  switch (summary.kind) {
    case "before_and_after":
      return t(key, {
        before: summary.before,
        after: summary.after,
      });
    case "before_only":
      return t(key, {
        before: summary.before,
      });
    case "after_only":
      return t(key, {
        after: summary.after,
      });
    case "events_only":
      return t(key, {
        total: summary.total,
      });
  }
}

function CoalescedGateRow({
  group,
  label,
  countLabel,
  hideEmail,
  prompt,
  promptLabel,
}: {
  group: CoalescedSecurityEvent;
  label: string;
  countLabel: string;
  hideEmail?: boolean;
  prompt?: boolean;
  promptLabel?: string;
}) {
  return (
    <li className={EMBEDDED_ITEM} data-gate-prompt={prompt ? "true" : undefined}>
      <div className="flex flex-wrap items-baseline gap-x-1.5 gap-y-0.5">
        <span className={prompt ? "font-medium text-muted-foreground" : "font-medium"}>
          {label}
        </span>
        {group.count > 1 ? (
          <span className="text-caption text-muted-foreground">· {countLabel}</span>
        ) : null}
        {!hideEmail && group.email ? (
          <span className="text-caption text-muted-foreground">· {group.email}</span>
        ) : null}
      </div>
      {prompt && promptLabel ? (
        <p className="text-caption mt-0.5 text-muted-foreground">{promptLabel}</p>
      ) : null}
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

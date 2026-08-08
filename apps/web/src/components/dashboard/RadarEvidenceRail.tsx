import type { ReactNode } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import { ArrowRight, SpinnerGap } from "@phosphor-icons/react";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import {
  radarWhyNowFallbackKey,
  radarWhyNowKey,
  type RadarEvidencePack,
  type RadarWorkItem,
} from "@/lib/radarQueue";

interface RadarEvidenceRailProps {
  item: RadarWorkItem | null;
  workspaceSlug: string;
}

export function RadarEvidenceRail({
  item,
  workspaceSlug,
}: RadarEvidenceRailProps) {
  const { t, i18n } = useTranslation("dashboard");

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

  return (
    <section
      data-testid="radar-evidence-rail"
      className="rounded-xl border border-border px-4 py-4"
    >
      <h3 className="text-sm font-semibold">{t("radar.evidenceRail.title")}</h3>
      <p className="text-caption mt-1 text-muted-foreground">
        {t(`radar.products.${item.product}`)}
        {item.dealName ? ` · ${item.dealName}` : ""}
      </p>
      <p className="mt-2 text-sm font-medium text-foreground">{item.headline}</p>
      {item.actor ? (
        <p className="text-caption mt-0.5 text-muted-foreground">{item.actor}</p>
      ) : null}

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
          {data.whyNowCode || item.whyNowCode ? (
            <p className="text-sm text-muted-foreground" data-testid="radar-evidence-why-now">
              {(() => {
                const why = {
                  scenario: item.scenario,
                  whyNowCode: data.whyNowCode || item.whyNowCode,
                };
                const hours = data.whyNowHours ?? item.whyNowHours ?? 1;
                const count =
                  data.whyNowHours ??
                  item.whyNowHours ??
                  item.coalescedFrom?.length ??
                  1;
                return t(radarWhyNowKey(why), {
                  hours,
                  count,
                  defaultValue: t(radarWhyNowFallbackKey(why), {
                    hours,
                    count,
                  }),
                });
              })()}
            </p>
          ) : null}

          {data.metrics ? (
            <dl className="grid grid-cols-2 gap-2">
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

          {data.keyPageTitles && data.keyPageTitles.length > 0 ? (
            <Block title={t("radar.evidenceRail.keyPages")}>
              <ul className="space-y-1">
                {data.keyPageTitles.map((title) => (
                  <li key={title} className="text-sm text-foreground">
                    {title}
                  </li>
                ))}
              </ul>
            </Block>
          ) : null}

          {data.topPages && data.topPages.length > 0 ? (
            <Block title={t("radar.evidenceRail.topPages")}>
              <ul className="space-y-1">
                {data.topPages.map((p) => (
                  <li
                    key={p.pageNumber}
                    className="flex justify-between gap-2 text-sm"
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
              <ul className="space-y-1.5">
                {data.recentVisitors.map((v) => (
                  <li key={v.visitorId} className="text-sm">
                    <span className="font-medium">
                      {v.email || t("radar.evidenceRail.anonymousVisitor")}
                    </span>
                    <span className="ml-2 text-caption text-muted-foreground">
                      {t("radar.evidenceRail.views", { count: v.totalViews })}
                      {v.lastAccessAt
                        ? ` · ${new Date(v.lastAccessAt).toLocaleString(i18n.language)}`
                        : ""}
                    </span>
                  </li>
                ))}
              </ul>
            </Block>
          ) : null}

          {data.securityEvents && data.securityEvents.length > 0 ? (
            <Block title={t("radar.evidenceRail.securityEvents")}>
              <ul className="space-y-1.5">
                {data.securityEvents.map((e, i) => (
                  <li key={`${e.eventType}-${e.createdAt}-${i}`} className="text-sm">
                    <span className="font-medium">
                      {t(`radar.evidenceRail.eventTypes.${e.eventType}`, {
                        defaultValue: t("radar.evidenceRail.eventTypes.unknown"),
                      })}
                    </span>
                    {e.reason ? (
                      <span className="ml-1 text-muted-foreground">
                        —{" "}
                        {t(`radar.evidenceRail.reasons.${e.reason}`, {
                          defaultValue: t("radar.evidenceRail.reasons.unknown"),
                        })}
                      </span>
                    ) : null}
                  </li>
                ))}
              </ul>
            </Block>
          ) : null}

          {(data.insightsPath || data.evidencePath) && (
            <Link
              to={data.insightsPath || data.evidencePath || `/${workspaceSlug}/insights/overview`}
              className="inline-flex items-center gap-1 text-sm font-medium text-foreground hover:underline"
              data-testid="radar-evidence-open"
            >
              {t("radar.evidenceRail.openFull")}
              <ArrowRight size={14} />
            </Link>
          )}
        </div>
      ) : null}
    </section>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-md border border-border px-2.5 py-2">
      <dt className="text-caption text-muted-foreground">{label}</dt>
      <dd className="text-sm font-semibold tabular-nums">{value}</dd>
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

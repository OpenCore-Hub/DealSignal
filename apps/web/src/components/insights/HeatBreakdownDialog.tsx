import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { HeatBadge } from "@/components/common/HeatBadge";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api, type DocumentHeatScore, type LinkHeatScore } from "@/lib/api";
import { displayablePageTitle } from "@/lib/insights/pageTitleDisplay";
import { shareKindFromLink } from "@/lib/shareKind";

const FACTOR_ORDER = [
  "opens",
  "revisits",
  "avgDurationMinutes",
  "keyPageViews",
  "forwardSignals",
  "downloads",
  "bouncePenalty",
] as const;

const OVERLAY_ORDER = ["readingDepth", "qaCitations", "crossDomain"] as const;

type FactorKey = (typeof FACTOR_ORDER)[number];
type OverlayKey = (typeof OVERLAY_ORDER)[number];
type HeatKind = "link" | "document";
type HeatScorePayload = LinkHeatScore | DocumentHeatScore;

interface HeatBreakdownDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  kind?: HeatKind;
  entityId?: string | null;
  label?: string;
  /** Existing callers / tests — treated as kind=link. */
  linkId?: string | null;
  linkLabel?: string;
}

function factorRows(score: HeatScorePayload): { key: FactorKey; value: number }[] {
  return FACTOR_ORDER.map((key) => ({
    key,
    value: Number(score.breakdown[key] ?? 0),
  }));
}

function overlayRows(score: DocumentHeatScore): { key: OverlayKey; value: number }[] {
  const rows = OVERLAY_ORDER.map((key) => ({
    key,
    value: Number(score.overlay?.[key] ?? score.breakdown[key] ?? 0),
  }));
  if (rows.every((row) => row.value === 0)) {
    return [];
  }
  return rows;
}

function resolveTarget(props: HeatBreakdownDialogProps): {
  kind: HeatKind;
  id: string | null;
  label: string;
} {
  if (props.kind === "document") {
    return {
      kind: "document",
      id: props.entityId ?? null,
      label: props.label ?? "",
    };
  }
  return {
    kind: "link",
    id: props.entityId ?? props.linkId ?? null,
    label: props.label ?? props.linkLabel ?? "",
  };
}

function isDocumentScore(data: HeatScorePayload): data is DocumentHeatScore {
  return "documentId" in data && "contributingLinks" in data;
}

function keyPageEvidenceLabel(
  t: (key: string, opts?: Record<string, unknown>) => string,
  pageNumber: number,
  rawTitle: string,
): string {
  const title = displayablePageTitle(rawTitle);
  return title
    ? t("heatBreakdown.keyPageRow", { page: pageNumber, title })
    : t("heatBreakdown.keyPageRowPageOnly", { page: pageNumber });
}

function heatCircleLabel(
  data: HeatScorePayload | null,
  t: (key: string) => string,
): string {
  const circle = data && "circle" in data ? data.circle : undefined;
  if (circle === "founder" || circle === "investor_ir" || circle === "sales") {
    return t(`keyPages.circles.${circle}`);
  }
  return t("heatBreakdown.circleFallback");
}

export function HeatBreakdownDialog({
  open,
  onOpenChange,
  kind,
  entityId,
  label,
  linkId,
  linkLabel,
}: HeatBreakdownDialogProps) {
  const { t } = useTranslation("insights");
  const { t: tc } = useTranslation("common");
  const target = resolveTarget({ open, onOpenChange, kind, entityId, label, linkId, linkLabel });

  const { data, loading, error, refetch } = useAsyncData(async () => {
    if (!open || !target.id) return null;
    if (target.kind === "document") {
      return api.getDocumentHeatScore(target.id);
    }
    return api.getLinkHeatScore(target.id);
  }, [open, target.kind, target.id]);

  const overlay = data && isDocumentScore(data) ? overlayRows(data) : [];
  const maxAbs = data
    ? Math.max(
        1,
        ...factorRows(data).map((r) => Math.abs(r.value)),
        ...overlay.map((r) => Math.abs(r.value)),
      )
    : 1;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="min-w-0 overflow-x-hidden sm:max-w-md"
        showCloseButton
        data-testid="heat-breakdown-dialog"
      >
        <DialogHeader className="min-w-0">
          <DialogTitle>{t("heatBreakdown.title")}</DialogTitle>
          <DialogDescription className="flex min-w-0 items-baseline gap-1">
            <span
              data-testid="heat-breakdown-label"
              className="min-w-0 truncate"
              title={target.label || undefined}
            >
              {target.label}
            </span>
            <span className="shrink-0">
              ·{" "}
              {target.kind === "document"
                ? t("heatBreakdown.subtitleDocument", { circle: heatCircleLabel(data, t) })
                : t("heatBreakdown.subtitle", { circle: heatCircleLabel(data, t) })}
            </span>
          </DialogDescription>
        </DialogHeader>

        {loading ? (
          <div className="space-y-3">
            <Skeleton className="h-8 w-40" />
            <Skeleton className="h-24 w-full" />
          </div>
        ) : error ? (
          <div className="flex flex-col items-center gap-3 py-4 text-center">
            <p className="text-body text-muted-foreground">{error}</p>
            <Button variant="outline" onClick={() => void refetch()}>
              {tc("retry")}
            </Button>
          </div>
        ) : data ? (
          <div className="min-w-0 space-y-4">
            <div className="flex flex-wrap items-center gap-3">
              <HeatBadge level={data.level} />
              <span className="text-h3 tabular-nums">
                {t("heatBreakdown.score", { score: data.score })}
              </span>
              <span className="text-caption text-muted-foreground">
                {t(`heatBreakdown.trend.${data.trend}`)}
              </span>
            </div>
            <ul className="space-y-2">
              {factorRows(data).map(({ key, value }) => {
                const widthPct = Math.round((Math.abs(value) / maxAbs) * 100);
                const negative = value < 0;
                const hintKey =
                  key === "forwardSignals"
                    ? target.kind === "document"
                      ? "heatBreakdown.factorHints.forwardSignalsDocument"
                      : "heatBreakdown.factorHints.forwardSignals"
                    : key === "bouncePenalty"
                      ? target.kind === "document"
                        ? "heatBreakdown.factorHints.bouncePenaltyDocument"
                        : "heatBreakdown.factorHints.bouncePenalty"
                      : key === "keyPageViews"
                        ? target.kind === "document"
                          ? "heatBreakdown.factorHints.keyPageViewsDocument"
                          : "heatBreakdown.factorHints.keyPageViewsLink"
                        : undefined;
                return (
                  <li key={key} className="space-y-1">
                    <div className="flex items-center justify-between gap-2 text-caption">
                      <span title={hintKey ? t(hintKey) : undefined}>
                        {t(`heatBreakdown.factors.${key}`)}
                      </span>
                      <span className="tabular-nums text-muted-foreground">
                        {value.toFixed(1)}
                      </span>
                    </div>
                    <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                      <div
                        className={
                          negative
                            ? "h-full rounded-full bg-risk-500/70"
                            : "h-full rounded-full bg-primary/70"
                        }
                        style={{ width: `${widthPct}%` }}
                      />
                    </div>
                  </li>
                );
              })}
            </ul>
            {overlay.length > 0 ? (
              <div className="space-y-2 border-t border-border pt-3">
                <p className="text-caption text-muted-foreground">
                  {t("heatBreakdown.overlayTitle")}
                </p>
                <ul className="space-y-2">
                  {overlay.map(({ key, value }) => {
                    const widthPct = Math.round((Math.abs(value) / maxAbs) * 100);
                    return (
                      <li key={key} className="space-y-1">
                        <div className="flex items-center justify-between gap-2 text-caption">
                          <span title={t(`heatBreakdown.factorHints.${key}`)}>
                            {t(`heatBreakdown.factors.${key}`)}
                          </span>
                          <span className="tabular-nums text-muted-foreground">
                            {value.toFixed(1)}
                          </span>
                        </div>
                        <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                          <div
                            className="h-full rounded-full bg-primary/70"
                            style={{ width: `${widthPct}%` }}
                          />
                        </div>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ) : null}
            {(data.keyPages?.total ?? 0) > 0 ? (
              <div className="space-y-2 border-t border-border pt-3">
                <p className="text-caption text-muted-foreground">
                  {t("heatBreakdown.keyPagesTitle")}
                </p>
                <p className="text-caption text-muted-foreground">
                  {data.keyPages && data.keyPages.engaged > 0
                    ? t("heatBreakdown.keyPagesEngaged", {
                        engaged: data.keyPages.engaged,
                        total: data.keyPages.total,
                        seconds: data.keyPages.minSeconds,
                      })
                    : t("heatBreakdown.keyPagesSkim", {
                        seconds: data.keyPages?.minSeconds ?? 3,
                      })}
                </p>
                {data.keyPages && data.keyPages.pages.length > 0 ? (
                  <ul className="space-y-1">
                    {data.keyPages.pages.map((page) => {
                      const label = keyPageEvidenceLabel(
                        t,
                        page.pageNumber,
                        page.title,
                      );
                      return (
                      <li
                        key={`${page.pageNumber}-${label}`}
                        className="flex items-center justify-between gap-2 text-caption"
                      >
                        <span className="min-w-0 truncate" title={label}>
                          {label}
                        </span>
                        <span className="shrink-0 tabular-nums text-muted-foreground">
                          {t("heatBreakdown.keyPageRowViews", {
                            engaged: page.engagedViews,
                            total: page.totalViews,
                          })}
                        </span>
                      </li>
                      );
                    })}
                  </ul>
                ) : null}
              </div>
            ) : null}
            {isDocumentScore(data) && data.contributingLinks.length > 0 ? (
              <div className="space-y-2 border-t border-border pt-3">
                <p className="text-caption text-muted-foreground">
                  {t("heatBreakdown.contributingLinks")}
                </p>
                <ul className="space-y-1">
                  {data.contributingLinks.map((link) => (
                    <li
                      key={link.id}
                      className="flex items-center justify-between gap-2 text-caption"
                    >
                      <span className="min-w-0 truncate">
                        {link.name.trim() || t("overview.untitledLink")}
                        <span className="ml-2 text-muted-foreground">
                          {t(`overview.shareKind.${shareKindFromLink(link)}`)}
                        </span>
                      </span>
                      <span className="shrink-0 tabular-nums text-muted-foreground">
                        {t("heatBreakdown.contributingLinkViews", { count: link.pageViews })}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

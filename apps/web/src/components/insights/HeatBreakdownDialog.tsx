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
import { api, type LinkHeatScore } from "@/lib/api";

const FACTOR_ORDER = [
  "opens",
  "revisits",
  "avgDurationMinutes",
  "keyPageViews",
  "forwardSignals",
  "downloads",
  "bouncePenalty",
] as const;

type FactorKey = (typeof FACTOR_ORDER)[number];

interface HeatBreakdownDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  linkId: string | null;
  linkLabel: string;
}

function factorRows(score: LinkHeatScore): { key: FactorKey; value: number }[] {
  return FACTOR_ORDER.map((key) => ({
    key,
    value: Number(score.breakdown[key] ?? 0),
  }));
}

export function HeatBreakdownDialog({
  open,
  onOpenChange,
  linkId,
  linkLabel,
}: HeatBreakdownDialogProps) {
  const { t } = useTranslation("insights");
  const { t: tc } = useTranslation("common");

  const { data, loading, error, refetch } = useAsyncData(async () => {
    if (!open || !linkId) return null;
    return api.getLinkHeatScore(linkId);
  }, [open, linkId]);

  const maxAbs = data
    ? Math.max(1, ...factorRows(data).map((r) => Math.abs(r.value)))
    : 1;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md" showCloseButton>
        <DialogHeader>
          <DialogTitle>{t("heatBreakdown.title")}</DialogTitle>
          <DialogDescription>
            {linkLabel} · {t("heatBreakdown.subtitle")}
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
          <div className="space-y-4">
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
                return (
                  <li key={key} className="space-y-1">
                    <div className="flex items-center justify-between gap-2 text-caption">
                      <span
                        title={
                          key === "forwardSignals" || key === "bouncePenalty"
                            ? t(`heatBreakdown.factorHints.${key}`)
                            : undefined
                        }
                      >
                        {t(`heatBreakdown.factors.${key}`)}
                      </span>
                      <span className="tabular-nums text-muted-foreground">
                        {negative ? value.toFixed(1) : value.toFixed(1)}
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
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

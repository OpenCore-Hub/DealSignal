import { Clock, Flag, Path, Users } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { DocumentReadingSessions } from "@/lib/api";
import { formatRelativeTime } from "@/lib/formatters";
import { cn } from "@/lib/utils";

interface ReadingSessionsCardProps {
  data: DocumentReadingSessions | null;
  loading?: boolean;
  onOpenPage?: (pageNumber: number) => void;
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return s > 0 ? `${m}m ${s}s` : `${m}m`;
}

export function ReadingSessionsCard({ data, loading, onOpenPage }: ReadingSessionsCardProps) {
  const { t } = useTranslation("insights");
  const { i18n } = useTranslation();
  const locale = i18n.language;

  if (loading) {
    return <Skeleton className="h-64 w-full" data-testid="reading-sessions-loading" />;
  }

  const sessions = data?.sessions ?? [];
  if (!data || sessions.length === 0) {
    return (
      <Card data-testid="reading-sessions-empty">
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Path size={20} />
            {t("sessions.title")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-body text-muted-foreground">{t("sessions.empty")}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card data-testid="reading-sessions">
      <CardHeader className="space-y-1">
        <CardTitle className="text-h2 flex items-center gap-2">
          <Path size={20} />
          {t("sessions.title")}
        </CardTitle>
        <p className="text-caption text-muted-foreground">{t("sessions.subtitle")}</p>
      </CardHeader>
      <CardContent>
        <ul className="space-y-3">
          {sessions.map((s) => {
            const when = new Date(s.lastActivityAt).toLocaleString(locale, {
              month: "short",
              day: "numeric",
              hour: "2-digit",
              minute: "2-digit",
            });
            return (
              <li
                key={s.id}
                className="rounded-md border border-border px-3 py-3"
                data-testid="reading-session-row"
              >
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div className="min-w-0 space-y-1">
                    <p className="truncate text-sm font-medium">
                      {s.visitorEmail?.trim() || t("pages.anonymousVisitor")}
                    </p>
                    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-caption text-muted-foreground">
                      <span className="inline-flex items-center gap-1">
                        <Clock size={12} />
                        {when}
                        <span aria-hidden>·</span>
                        {formatRelativeTime(s.lastActivityAt)}
                      </span>
                      <span className="inline-flex items-center gap-1">
                        <Users size={12} />
                        {t("sessions.depth", {
                          page: s.maxPage,
                          pages: data.pageCount || s.maxPage,
                        })}
                      </span>
                      <span>
                        {t("sessions.duration", {
                          duration: formatDuration(s.totalDurationSeconds),
                        })}
                      </span>
                      {s.completed ? (
                        <span className="inline-flex items-center gap-1 text-foreground">
                          <Flag size={12} />
                          {t("sessions.completed")}
                        </span>
                      ) : null}
                    </div>
                  </div>
                  <span className="text-caption text-muted-foreground">
                    {t("sessions.pagesTouched", { count: s.distinctPageCount })}
                  </span>
                </div>
                {s.pages.length > 0 ? (
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {s.pages.map((p) => (
                      <button
                        key={`${s.id}-${p.pageNumber}`}
                        type="button"
                        disabled={!onOpenPage}
                        onClick={() => onOpenPage?.(p.pageNumber)}
                        className={cn(
                          "rounded border border-border px-2 py-0.5 text-caption tabular-nums transition-colors",
                          onOpenPage
                            ? "hover:border-foreground/30 hover:bg-muted/60"
                            : "cursor-default",
                          p.pageNumber === s.maxPage && "border-foreground/40 font-medium",
                        )}
                        title={t("sessions.pageChipTitle", {
                          page: p.pageNumber,
                          duration: formatDuration(p.durationSeconds),
                        })}
                      >
                        {t("funnel.pageLabel", { page: p.pageNumber })}
                      </button>
                    ))}
                  </div>
                ) : null}
              </li>
            );
          })}
        </ul>
      </CardContent>
    </Card>
  );
}

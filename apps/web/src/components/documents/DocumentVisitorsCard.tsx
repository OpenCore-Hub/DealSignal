import { Users } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router";
import { VisitorList } from "@/components/common/VisitorList";
import { EmptyState } from "@/components/common/EmptyState";
import { formatDuration, formatRelativeTime } from "@/lib/formatters";
import type { VisitorSummary } from "@/types";

interface VisitorListItem {
  id: string;
  email: string;
  organization?: string;
  visitCount: number;
  avgDurationSeconds: number;
  lastSeenAt: string;
}

function toVisitorListItems(visitors: VisitorSummary[]): VisitorListItem[] {
  return visitors
    .map((v) => ({
      id: v.visitorId || v.visitorEmail || "unknown",
      email: v.visitorEmail || v.visitorId || "unknown",
      organization: undefined,
      visitCount: v.pageViewCount,
      avgDurationSeconds: Math.round(v.avgDurationSeconds),
      lastSeenAt: v.lastSeenAt,
    }))
    .sort((a, b) => new Date(b.lastSeenAt).getTime() - new Date(a.lastSeenAt).getTime())
    .slice(0, 10);
}

interface DocumentVisitorsCardProps {
  visitors: VisitorSummary[];
  /** Override card title (defaults to documents detail copy). */
  title?: string;
  /** When false, render a non-linking analysis list (visitor ids are not contact ids). */
  linkToContacts?: boolean;
  emptyTitle?: string;
  emptyDescription?: string;
  anonymousLabel?: string;
  metaLabel?: (v: { pages: number; duration: number }) => string;
}

export function DocumentVisitorsCard({
  visitors,
  title,
  linkToContacts = true,
  emptyTitle,
  emptyDescription,
  anonymousLabel,
  metaLabel,
}: DocumentVisitorsCardProps) {
  const { t } = useTranslation("documents");
  const { t: tc } = useTranslation("common");
  const location = useLocation();
  const visitorList = toVisitorListItems(visitors);
  const heading = title ?? t("documents:detail.recentVisitors");

  return (
    <section className="rounded-2xl border border-border/70 bg-background px-5 py-5">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Users size={16} className="text-muted-foreground" weight="duotone" />
          <h2 className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/80">
            {heading}
          </h2>
        </div>
        {visitorList.length > 0 ? (
          <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
            {visitorList.length}
          </span>
        ) : null}
      </div>
      {linkToContacts ? (
        <VisitorList
          visitors={visitorList}
          returnTo={location.pathname + location.search}
          returnLabel={t("documents:detail.back")}
        />
      ) : visitorList.length === 0 ? (
        <EmptyState
          icon={<Users size={32} />}
          title={emptyTitle ?? tc("visitor.empty.title")}
          description={emptyDescription ?? tc("visitor.empty.description")}
          size="large"
        />
      ) : (
        <ul className="space-y-2">
          {visitorList.map((visitor) => {
            const label =
              visitor.email && visitor.email !== "unknown"
                ? visitor.email
                : (anonymousLabel ?? visitor.id);
            const meta = metaLabel
              ? metaLabel({ pages: visitor.visitCount, duration: visitor.avgDurationSeconds })
              : `${tc("visitor.visitCount", { count: visitor.visitCount })} · ${tc("visitor.avgDuration", { duration: formatDuration(visitor.avgDurationSeconds) })} · ${tc("visitor.lastSeen", { time: formatRelativeTime(visitor.lastSeenAt) })}`;
            return (
              <li
                key={visitor.id}
                className="flex items-center gap-3 rounded-lg border border-border bg-card p-3"
              >
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground text-xs font-medium">
                  {label.slice(0, 2).toUpperCase()}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{label}</p>
                  <p className="text-caption text-muted-foreground">{meta}</p>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

import { Clock, Eye, Link as LinkIcon, User } from "@phosphor-icons/react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatRelativeTime } from "@/lib/formatters";
import type { HeatLevel, Link } from "@/types";

export interface ActivityVisitor {
  email: string;
  name?: string;
  heatLevel: HeatLevel;
  lastSeenAt: string;
}

interface DealRoomActivityTabProps {
  recentVisitors?: ActivityVisitor[];
  links?: Link[];
  onOpenShare?: () => void;
  onOpenAnalytics?: () => void;
}

type ActivityEvent = {
  id: string;
  kind: "visitor" | "link";
  title: string;
  subtitle?: string;
  at: string;
};

export function DealRoomActivityTab({
  recentVisitors = [],
  links = [],
  onOpenShare,
  onOpenAnalytics,
}: DealRoomActivityTabProps) {
  const { t } = useTranslation("dealRooms");

  const events = useMemo(() => {
    const items: ActivityEvent[] = [];

    for (const v of recentVisitors) {
      items.push({
        id: `visitor-${v.email}-${v.lastSeenAt}`,
        kind: "visitor",
        title: v.name?.trim() || v.email,
        subtitle: v.name ? v.email : undefined,
        at: v.lastSeenAt,
      });
    }

    for (const link of links) {
      if (!link.lastViewedAt) continue;
      items.push({
        id: `link-${link.id}-${link.lastViewedAt}`,
        kind: "link",
        title: link.name?.trim() || link.documentTitle || t("activity.linkFallback"),
        subtitle: t("activity.linkViews", { count: link.accessCount ?? 0 }),
        at: link.lastViewedAt,
      });
    }

    return items.sort((a, b) => new Date(b.at).getTime() - new Date(a.at).getTime()).slice(0, 40);
  }, [recentVisitors, links, t]);

  return (
    <Card data-testid="deal-room-activity-tab">
      <CardHeader className="pb-2">
        <CardTitle className="text-h3">{t("activity.title")}</CardTitle>
        <p className="text-body text-muted-foreground">{t("activity.description")}</p>
      </CardHeader>
      <CardContent className="space-y-4">
        {events.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border px-4 py-8 text-center">
            <Clock size={28} className="mx-auto text-muted-foreground" />
            <p className="mt-3 text-sm font-medium">{t("activity.emptyTitle")}</p>
            <p className="mt-1 text-body text-muted-foreground">{t("activity.emptyDescription")}</p>
            <div className="mt-4 flex flex-wrap justify-center gap-2">
              {onOpenShare && (
                <button
                  type="button"
                  className="text-sm text-primary underline-offset-2 hover:underline"
                  onClick={onOpenShare}
                >
                  {t("activity.goShare")}
                </button>
              )}
              {onOpenAnalytics && (
                <button
                  type="button"
                  className="text-sm text-primary underline-offset-2 hover:underline"
                  onClick={onOpenAnalytics}
                >
                  {t("activity.goAnalytics")}
                </button>
              )}
            </div>
          </div>
        ) : (
          <ul className="divide-y divide-border rounded-lg border border-border">
            {events.map((event) => (
              <li key={event.id} className="flex items-start gap-3 px-3 py-3">
                <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                  {event.kind === "visitor" ? <User size={16} /> : event.kind === "link" ? <LinkIcon size={16} /> : <Eye size={16} />}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{event.title}</p>
                  {event.subtitle && (
                    <p className="truncate text-caption text-muted-foreground">{event.subtitle}</p>
                  )}
                </div>
                <time className="shrink-0 text-caption text-muted-foreground" dateTime={event.at}>
                  {formatRelativeTime(event.at)}
                </time>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

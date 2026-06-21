import { Link, useParams } from "react-router";
import { Users } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { HeatBadge } from "./HeatBadge";
import { EmptyState } from "./EmptyState";
import { formatDuration, formatRelativeTime, getInitials } from "@/lib/formatters";
import type { HeatLevel } from "@/types";

interface Visitor {
  id: string;
  email: string;
  name?: string;
  organization?: string;
  heatLevel: HeatLevel;
  visitCount: number;
  avgDurationSeconds: number;
  lastSeenAt: string;
}

interface VisitorListProps {
  visitors: Visitor[];
  className?: string;
}

export function VisitorList({ visitors, className }: VisitorListProps) {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { t } = useTranslation("common");

  if (visitors.length === 0) {
    return (
      <EmptyState
        icon={<Users size={32} />}
        title={t("visitor.empty.title")}
        description={t("visitor.empty.description")}
        size="large"
      />
    );
  }

  return (
    <ul className={cn("space-y-2", className)}>
      {visitors.map((visitor) => (
        <li key={visitor.id}>
          <Link
            to={`/${workspaceSlug}/contacts/${visitor.id}`}
            className="flex items-center gap-3 rounded-lg border border-border bg-card p-3 transition-colors hover:bg-muted/50 focus-ring"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground text-xs font-medium">
              {visitor.name ? getInitials(visitor.name) : getInitials(visitor.email)}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{visitor.name ?? visitor.email}</p>
              <p className="text-caption text-muted-foreground">
                {visitor.organization ?? t("visitor.unknownOrganization")} · {t("visitor.visitCount", { count: visitor.visitCount })} · {t("visitor.avgDuration", { duration: formatDuration(visitor.avgDurationSeconds) })} · {t("visitor.lastSeen", { time: formatRelativeTime(visitor.lastSeenAt) })}
              </p>
            </div>
            <HeatBadge level={visitor.heatLevel} />
          </Link>
        </li>
      ))}
    </ul>
  );
}

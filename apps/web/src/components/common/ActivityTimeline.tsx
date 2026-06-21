import { cn } from "@/lib/utils";
import { EmptyState } from "./EmptyState";
import { Clock, DownloadSimple, Eye, ArrowClockwise, ShareNetwork } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";

export type ActivityType = "open" | "page_view" | "revisit" | "download" | "share";

interface Activity {
  id: string;
  time: string;
  title: string;
  description?: string;
  type?: ActivityType;
}

interface ActivityTimelineProps {
  activities: Activity[];
  className?: string;
}

export function ActivityTimeline({ activities, className }: ActivityTimelineProps) {
  const { t } = useTranslation("common");
  const typeConfig: Record<ActivityType, { icon: typeof Eye; color: string; label: string }> = {
    open: { icon: Eye, color: "bg-info-500", label: t("activity.open") },
    page_view: { icon: Eye, color: "bg-info-500", label: t("activity.pageView") },
    revisit: { icon: ArrowClockwise, color: "bg-warm-500", label: t("activity.revisit") },
    download: { icon: DownloadSimple, color: "bg-success-500", label: t("activity.download") },
    share: { icon: ShareNetwork, color: "bg-risk-500", label: t("activity.share") },
  };

  if (activities.length === 0) {
    return (
      <EmptyState
        icon={<Clock size={32} />}
        title={t("activity.emptyTitle")}
        description={t("activity.emptyDescription")}
        size="large"
      />
    );
  }

  return (
    <div className={cn("space-y-4", className)}>
      {activities.map((activity, index) => {
        const config = activity.type ? typeConfig[activity.type] : null;
        const Icon = config?.icon ?? Eye;
        return (
          <div key={activity.id} className="flex gap-3">
            <div className="flex flex-col items-center">
              <div className={cn("flex h-6 w-6 items-center justify-center rounded-full text-background", config?.color ?? "bg-primary")}>
                <Icon size={12} weight="bold" />
              </div>
              {index < activities.length - 1 && (
                <div className="mt-2 h-full w-px bg-border" />
              )}
            </div>
            <div className="min-w-0 flex-1 pb-4">
              <div className="flex items-center justify-between gap-2">
                <p className="text-sm font-medium">{activity.title}</p>
                <p className="text-caption shrink-0 text-muted-foreground" title={activity.time}>
                  {activity.time}
                </p>
              </div>
              {activity.description && (
                <p className="text-caption text-muted-foreground">{activity.description}</p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

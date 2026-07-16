import { useMemo, useState } from "react";
import { Link, useLocation } from "react-router";
import { useTranslation } from "react-i18next";
import {
  Clock,
  Download,
  Question,
  Upload,
  Eye,
  ArrowRight,
} from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/common/EmptyState";
import { formatRelativeTime } from "@/lib/formatters";
import {
  buildActivityGroups,
  countDisplayItems,
  type DisplayActivity,
} from "@/lib/activityFeed";
import type { RecentActivityItem } from "@/lib/api";

interface RecentActivityFeedProps {
  activities: RecentActivityItem[];
  workspaceSlug: string;
}

const OLDER_DISPLAY_LIMIT = 10;

const eventConfig = {
  visit: {
    icon: Eye,
    labelKey: "activity.events.visit" as const,
    style: "bg-primary/10 text-primary",
  },
  download: {
    icon: Download,
    labelKey: "activity.events.download" as const,
    style: "bg-success-100 text-success-500",
  },
  question: {
    icon: Question,
    labelKey: "activity.events.question" as const,
    style: "bg-warning-100 text-warning-500",
  },
  upload: {
    icon: Upload,
    labelKey: "activity.events.upload" as const,
    style: "bg-info-100 text-info-500",
  },
};

function isEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
}

function ActorName({
  actor,
  t,
}: {
  actor: string;
  t: (key: string, options?: Record<string, unknown>) => string;
}) {
  const display = isEmail(actor) ? actor : t("activity.anonymousUser");
  return (
    <span
      className={`font-medium ${
        isEmail(actor) ? "text-foreground" : "text-muted-foreground"
      }`}
      title={actor}
    >
      {display}
    </span>
  );
}

function ActivityText({
  item,
  locale,
  t,
}: {
  item: DisplayActivity;
  locale: string;
  t: (key: string, options?: Record<string, unknown>) => string;
}) {
  const activities = item.kind === "combined" ? item.activities : [item.activity];
  const first = activities[0];
  const config = eventConfig[first.eventType] ?? eventConfig.visit;

  const names = activities.map((a) => a.objectName).filter(Boolean);
  const displayedNames = names.slice(0, 2);
  const restCount = Math.max(0, names.length - displayedNames.length);
  const list = new Intl.ListFormat(locale, {
    type: "conjunction",
    style: "narrow",
  }).format(displayedNames);
  const objectText = restCount
    ? `${list}${t("activity.combined.more", { count: restCount })}`
    : list;

  return (
    <p className="text-sm leading-snug">
      <ActorName actor={first.actor} t={t} />{" "}
      <span className="text-muted-foreground">{t(config.labelKey)}</span>{" "}
      <span className="font-medium text-foreground">{objectText}</span>
    </p>
  );
}

function ActivityRow({
  item,
  workspaceSlug,
  returnState,
  locale,
  t,
}: {
  item: DisplayActivity;
  workspaceSlug: string;
  returnState: { returnTo: string; returnLabel: string };
  locale: string;
  t: (key: string, options?: Record<string, unknown>) => string;
}) {
  const isCombined = item.kind === "combined";
  const first = isCombined ? item.activities[0] : item.activity;
  const activities = item.kind === "combined" ? item.activities : [item.activity];
  const config = eventConfig[first.eventType] ?? eventConfig.visit;
  const Icon = config.icon;

  const isClickable =
    !isCombined &&
    (first.objectType === "room" || first.objectType === "document");
  const to = isClickable
    ? `/${workspaceSlug}/${
        first.objectType === "room" ? "deal-rooms" : "documents"
      }/${first.objectId}`
    : undefined;

  const time = activities[activities.length - 1].createdAt;

  const className =
    "group relative flex items-start gap-3 rounded-lg py-2 pr-2 transition-colors hover:bg-muted/50 focus-ring";

  const body = (
    <>
      <div className="relative z-10 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border-2 border-card bg-card">
        <div
          className={`flex h-6 w-6 items-center justify-center rounded-full ${config.style}`}
        >
          <Icon size={12} weight="fill" />
        </div>
      </div>
      <div className="min-w-0 flex-1 pt-0.5">
        <ActivityText item={item} locale={locale} t={t} />
        <p className="text-caption mt-0.5 text-muted-foreground">
          {formatRelativeTime(time)}
        </p>
      </div>
      {isClickable && (
        <ArrowRight
          size={16}
          className="mt-2 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100"
        />
      )}
    </>
  );

  return isClickable ? (
    <Link key={first.id} to={to!} state={returnState} className={className}>
      {body}
    </Link>
  ) : (
    <div key={first.id} className={`${className} cursor-default`}>
      {body}
    </div>
  );
}

export function RecentActivityFeed({
  activities,
  workspaceSlug,
}: RecentActivityFeedProps) {
  const { t, i18n } = useTranslation("dashboard");
  const { t: tCommon } = useTranslation("common");
  const location = useLocation();
  const [expanded, setExpanded] = useState(false);

  const returnState = {
    returnTo: location.pathname + location.search,
    returnLabel: tCommon("back"),
  };

  const groups = useMemo(() => buildActivityGroups(activities), [activities]);
  const total = useMemo(() => countDisplayItems(groups), [groups]);
  const visibleGroups = useMemo(() => {
    if (expanded) return groups;
    return groups
      .map((group) =>
        group.key === "older"
          ? { ...group, items: group.items.slice(0, OLDER_DISPLAY_LIMIT) }
          : group
      )
      .filter((group) => group.items.length > 0);
  }, [expanded, groups]);
  const visibleTotal = useMemo(
    () => countDisplayItems(visibleGroups),
    [visibleGroups]
  );

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-body flex items-center gap-2 font-medium text-muted-foreground">
          <Clock size={16} />
          {t("sections.activityFeed")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {activities.length === 0 ? (
          <EmptyState
            size="compact"
            icon={<Clock size={32} />}
            title={t("empty.activity.title")}
            description={t("empty.activity.description")}
          />
        ) : (
          <>
            <div className="relative space-y-6 pl-2">
              <div className="absolute bottom-2 left-[18px] top-2 w-px bg-border" />
              {visibleGroups.map((group) => (
                <div key={group.key} className="relative">
                  <div className="sticky top-0 z-10 mb-2 flex items-center">
                    <span className="bg-card px-2 text-xs font-medium text-muted-foreground">
                      {t(`activity.groups.${group.key}`)}
                    </span>
                  </div>
                  <div className="space-y-0">
                    {group.items.map((item, index) => (
                      <ActivityRow
                        key={`${group.key}-${index}`}
                        item={item}
                        workspaceSlug={workspaceSlug}
                        returnState={returnState}
                        locale={i18n.language}
                        t={t}
                      />
                    ))}
                  </div>
                </div>
              ))}
            </div>
            {visibleTotal < total && (
              <div className="mt-4 flex justify-end">
                <button
                  type="button"
                  onClick={() => setExpanded((prev) => !prev)}
                  className="text-caption font-medium text-primary hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                >
                  {expanded
                    ? t("activity.viewLess")
                    : t("activity.viewAllWithCount", { count: total })}
                </button>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}

import { FolderOpen, Users, ChatTeardropText, Fire } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router";
import { Card, CardContent } from "@/components/ui/card";
import { formatCompactNumber } from "@/lib/formatters";

interface MetricsCardsProps {
  workspaceSlug: string;
  activeRooms: number;
  weeklyVisitors: number;
  pendingQuestions: number;
  highIntentContacts: number;
}

export function MetricsCards({
  workspaceSlug,
  activeRooms,
  weeklyVisitors,
  pendingQuestions,
  highIntentContacts,
}: MetricsCardsProps) {
  const { t, i18n } = useTranslation("dashboard");
  const { t: tCommon } = useTranslation("common");
  const location = useLocation();

  const items = [
    {
      label: t("metrics.activeRooms"),
      count: activeRooms,
      icon: FolderOpen,
      color: "text-primary border border-primary/30 bg-transparent",
      to: `/${workspaceSlug}/deal-rooms`,
      ariaLabel: t("metrics.aria.activeRooms", { count: activeRooms }),

    },
    {
      label: t("metrics.weeklyVisitors"),
      count: weeklyVisitors,
      icon: Users,
      color: "text-info-500 border border-info-500/30 bg-transparent",
      to: `/${workspaceSlug}/contacts`,
      ariaLabel: t("metrics.aria.weeklyVisitors", { count: weeklyVisitors }),

    },
    {
      label: t("metrics.pendingQuestions"),
      count: pendingQuestions,
      icon: ChatTeardropText,
      color: "text-warning-500 border border-warning-500/30 bg-transparent",
      to: `/${workspaceSlug}/links`,
      ariaLabel: t("metrics.aria.pendingQuestions", { count: pendingQuestions }),

    },
    {
      label: t("metrics.highIntentContacts"),
      count: highIntentContacts,
      icon: Fire,
      color: "text-hot-500 border border-hot-500/30 bg-transparent",
      to: `/${workspaceSlug}/contacts`,
      ariaLabel: t("metrics.aria.highIntentContacts", { count: highIntentContacts }),

    },
  ];

  return (
    <div
      className="grid grid-cols-2 gap-4 lg:grid-cols-4"
      role="region"
      aria-label={t("metrics.title")}
    >
      {items.map((item) => {
        const Icon = item.icon;
        return (
          <Link
            key={item.label}
            to={item.to}
            state={{
              returnTo: location.pathname + location.search,
              returnLabel: tCommon("back"),
            }}
            aria-label={item.ariaLabel}
            className="block rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
          >
            <Card className="relative h-full min-h-[120px] overflow-visible transition-shadow pressable">
              <span className="absolute -top-3 left-4 inline-flex items-center gap-1 bg-card px-2 text-caption text-muted-foreground">
                {item.label}
              </span>
              <div
                className={`absolute right-3 top-3 flex h-8 w-8 items-center justify-center rounded-lg ${item.color}`}
              >
                <Icon size={16} weight="fill" />
              </div>
              <CardContent className="relative z-10 flex h-full items-center justify-center p-4 pt-5">
                <p className="text-stat tabular-nums">
                  {formatCompactNumber(item.count, i18n.language)}
                </p>
              </CardContent>
            </Card>
          </Link>
        );
      })}
    </div>
  );
}

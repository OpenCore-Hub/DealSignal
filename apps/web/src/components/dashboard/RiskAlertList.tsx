import { Link, useLocation } from "react-router";
import { useTranslation } from "react-i18next";
import { ArrowRight, Warning } from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
import type { RiskAlert } from "@/types";

interface RiskAlertListProps {
  alerts: RiskAlert[];
  workspaceSlug?: string;
}

const severityOrder = { high: 3, medium: 2, low: 1 };

const severityConfig = {
  high: {
    badge: "bg-error-500/10 text-error-500 border-error-500/20",
    strip: "bg-error-500",
    icon: "text-error-500",
  },
  medium: {
    badge: "bg-warning-500/10 text-warning-500 border-warning-500/20",
    strip: "bg-warning-500",
    icon: "text-warning-500",
  },
  low: {
    badge: "bg-muted text-muted-foreground",
    strip: "bg-muted-foreground",
    icon: "text-muted-foreground",
  },
};

export function RiskAlertList({ alerts, workspaceSlug }: RiskAlertListProps) {
  const { t: tCommon } = useTranslation("common");
  const location = useLocation();

  if (alerts.length === 0) return null;

  const sorted = [...alerts].sort(
    (a, b) => severityOrder[b.priority] - severityOrder[a.priority]
  );

  const returnState = {
    returnTo: location.pathname + location.search,
    returnLabel: tCommon("back"),
  };

  return (
    <div className="max-h-[340px] overflow-y-auto pr-1">
      <ul className="space-y-3">
        {sorted.map((alert) => {
        const severity = severityConfig[alert.priority] ?? severityConfig.medium;
        const to = alert.documentId
          ? `/${workspaceSlug}/documents/${alert.documentId}`
          : alert.linkId
            ? `/${workspaceSlug}/links/${alert.linkId}`
            : undefined;

        const content = (
          <>
            <div className={`absolute left-0 top-0 h-full w-[3px] ${severity.strip}`} />
            <div className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-risk-500" />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <Warning size={14} className={severity.icon} />
                <p className="text-sm font-medium">{alert.title}</p>
                <Badge variant="outline" className={`text-xs ${severity.badge}`}>
                  {tCommon(`priority.${alert.priority}`)}
                </Badge>
              </div>
              <p className="text-caption text-muted-foreground">
                {alert.description}
              </p>
            </div>
            <ArrowRight
              size={16}
              className="mt-1 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover/risk:opacity-100"
            />
          </>
        );

        const className =
          "group/risk spotlight relative flex items-start gap-3 overflow-hidden rounded-xl border border-risk-500/20 bg-risk-500/5 p-3 shadow-card transition-all hover:bg-risk-500/10 hover:shadow-card-hover focus-ring pressable";

        return to ? (
          <li key={alert.id}>
            <Link to={to} state={returnState} className={className}>
              {content}
            </Link>
          </li>
        ) : (
          <li key={alert.id} className={className}>
            {content}
          </li>
        );
      })}
      </ul>
    </div>
  );
}

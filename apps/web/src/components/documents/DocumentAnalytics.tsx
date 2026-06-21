import { ChartBar } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/common/EmptyState";
import type { PageAnalytics } from "@/types";

interface DocumentAnalyticsProps {
  analytics: PageAnalytics[];
  className?: string;
}

export function DocumentAnalytics({ analytics, className }: DocumentAnalyticsProps) {
  const { t } = useTranslation("documents");

  if (analytics.length === 0) {
    return (
      <Card className={cn("overflow-hidden", className)}>
        <CardHeader className="pb-2">
          <CardTitle className="text-h3">{t("documents:analytics.title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <EmptyState
            icon={<ChartBar size={48} />}
            title={t("documents:analytics.emptyTitle")}
            description={t("documents:analytics.emptyDescription")}
          />
        </CardContent>
      </Card>
    );
  }

  const maxDuration = Math.max(...analytics.map((a) => a.avgDurationSeconds), 1);

  return (
    <Card className={cn("overflow-hidden", className)}>
      <CardHeader className="pb-2">
        <CardTitle className="text-h3">{t("documents:analytics.title")}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex h-48 items-end gap-1">
          {analytics.map((page) => {
            const height = Math.max(4, (page.avgDurationSeconds / maxDuration) * 100);
            return (
              <div
                key={page.pageNumber}
                className="group relative flex flex-1 flex-col items-center"
              >
                <div
                  className="w-full rounded-sm bg-primary/10 transition-colors hover:bg-primary/20"
                  style={{ height: `${height}%` }}
                  aria-hidden="true"
                />
                <span className="mt-1 text-caption text-muted-foreground">
                  {page.pageNumber}
                </span>
                <div className="pointer-events-none absolute -top-8 left-1/2 z-10 hidden -translate-x-1/2 whitespace-nowrap rounded-md bg-foreground px-2 py-1 text-xs text-background group-hover:block">
                  {t("documents:analytics.pageTooltip", {
                    pageNumber: page.pageNumber,
                    seconds: page.avgDurationSeconds,
                  })}
                </div>
              </div>
            );
          })}
        </div>
        <div className="mt-2 flex justify-between text-caption text-muted-foreground">
          <span>{t("documents:analytics.firstPage")}</span>
          <span>{t("documents:analytics.lastPage", { count: analytics.length })}</span>
        </div>
      </CardContent>
    </Card>
  );
}

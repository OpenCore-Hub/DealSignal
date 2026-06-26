import { useState } from "react";
import { cn } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "./EmptyState";
import { ChartLineUp } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";

interface TrendChartProps {
  title: string;
  data?: number[];
  labels?: string[];
  className?: string;
  emptyDescription?: string;
  formatValue?: (value: number) => string;
}

export function TrendChart({
  title,
  data,
  labels,
  className,
  emptyDescription,
  formatValue,
}: TrendChartProps) {
  const { t } = useTranslation("common");
  const [hovered, setHovered] = useState<number | null>(null);

  if (!data || data.length === 0) {
    return (
      <Card className={cn(className)}>
        <CardHeader className="pb-2">
          <CardTitle className="text-h3">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <EmptyState
            icon={<ChartLineUp size={32} />}
            title={t("trendEmptyTitle")}
            description={emptyDescription ?? t("empty.description")}
            size="large"
          />
        </CardContent>
      </Card>
    );
  }

  const max = Math.max(...data, 1);

  return (
    <Card className={cn(className)}>
      <CardHeader className="pb-2">
        <CardTitle className="text-h3">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex h-48 items-end gap-1">
          {data.map((h, i) => (
            <div
              key={i}
              className="group relative flex h-full flex-1 flex-col justify-end"
              onMouseEnter={() => setHovered(i)}
              onMouseLeave={() => setHovered(null)}
            >
              <div
                className="w-full min-w-2 rounded-t-sm bg-primary transition-colors group-hover:bg-primary/80"
                style={{ height: `${(h / max) * 100}%` }}
                aria-hidden="true"
              />
              {hovered === i && (
                <div className="absolute bottom-full left-1/2 z-10 mb-1 -translate-x-1/2 rounded-md bg-foreground px-2 py-1 text-xs text-background shadow-dropdown">
                  {formatValue ? formatValue(h) : h}
                </div>
              )}
            </div>
          ))}
        </div>
        <div className="mt-3 flex justify-between text-caption text-muted-foreground">
          <span>{labels?.[0] ?? t("start")}</span>
          <span>{labels?.[labels.length - 1] ?? t("now")}</span>
        </div>
      </CardContent>
    </Card>
  );
}

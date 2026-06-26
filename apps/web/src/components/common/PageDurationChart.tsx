import { useEffect, useRef } from "react";
import { Column } from "@antv/g2plot";
import { cn } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "./EmptyState";
import { ChartLineUp } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";

export interface PageDurationPoint {
  page: number;
  duration: number;
}

interface PageDurationChartProps {
  title: string;
  data?: PageDurationPoint[];
  className?: string;
  emptyDescription?: string;
  formatValue?: (value: number) => string;
  xAxisTitle?: string;
  yAxisTitle?: string;
  tooltipName?: string;
  pageLabel?: (page: number) => string;
}

export function PageDurationChart({
  title,
  data,
  className,
  emptyDescription,
  formatValue,
  xAxisTitle,
  yAxisTitle,
  tooltipName,
  pageLabel,
}: PageDurationChartProps) {
  const { t } = useTranslation("common");
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!containerRef.current || !data || data.length === 0) return;

    const plot = new Column(containerRef.current, {
      data,
      xField: "page",
      yField: "duration",
      autoFit: true,
      padding: [16, 16, 48, 48],
      columnWidthRatio: 0.7,
      color: "#0f172a",
      xAxis: {
        title: xAxisTitle
          ? {
              text: xAxisTitle,
              style: { fontSize: 12, fill: "#64748b" },
            }
          : null,
        label: {
          autoHide: true,
          autoRotate: true,
          style: { fontSize: 11, fill: "#64748b" },
        },
        line: null,
        tickLine: null,
        grid: null,
      },
      yAxis: {
        title: yAxisTitle
          ? {
              text: yAxisTitle,
              style: { fontSize: 12, fill: "#64748b" },
            }
          : null,
        label: {
          style: { fontSize: 11, fill: "#64748b" },
        },
        grid: {
          line: { style: { stroke: "#e2e8f0" } },
        },
      },
      tooltip: {
        title: (_title: string, datum: Record<string, unknown>) =>
          pageLabel ? pageLabel(datum.page as number) : `Page ${datum.page}`,
        formatter: (datum: Record<string, unknown>) => ({
          name: tooltipName ?? "Avg. duration",
          value: formatValue ? formatValue(datum.duration as number) : `${datum.duration}s`,
        }),
      },
      state: {
        active: {
          style: {
            fillOpacity: 0.8,
          },
        },
      },
      interactions: [{ type: "element-active" }],
    });

    plot.render();

    return () => {
      plot.destroy();
    };
  }, [data, formatValue, t, xAxisTitle, yAxisTitle, tooltipName, pageLabel]);

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

  return (
    <Card className={cn(className)}>
      <CardHeader className="pb-2">
        <CardTitle className="text-h3">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div ref={containerRef} className="h-64 w-full" />
      </CardContent>
    </Card>
  );
}

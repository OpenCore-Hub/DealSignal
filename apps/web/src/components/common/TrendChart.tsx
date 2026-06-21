import { cn } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

interface TrendChartProps {
  title: string;
  className?: string;
}

export function TrendChart({ title, className }: TrendChartProps) {
  return (
    <Card className={cn("overflow-hidden", className)}>
      <CardHeader className="pb-2">
        <CardTitle className="text-h3">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex h-48 items-end gap-2">
          {[35, 52, 28, 65, 48, 72, 58, 80, 45, 60, 55, 75].map((h, i) => (
            <div
              key={i}
              className="flex-1 rounded-sm bg-primary/10 transition-all hover:bg-primary/20"
              style={{ height: `${h}%` }}
              aria-hidden="true"
            />
          ))}
        </div>
        <div className="mt-3 flex justify-between text-caption text-muted-foreground">
          <span>Week 1</span>
          <span>Week 12</span>
        </div>
      </CardContent>
    </Card>
  );
}

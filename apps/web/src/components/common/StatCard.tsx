import { cn } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";

interface StatCardProps {
  label: string;
  value: string | number;
  subtext?: string;
  icon?: React.ReactNode;
  className?: string;
  size?: "default" | "sm";
}

export function StatCard({ label, value, subtext, icon, className, size = "default" }: StatCardProps) {
  return (
    <Card className={cn(className)}>
      <CardContent>
        <div className="flex items-start justify-between gap-2">
          <div>
            <p className="text-caption text-muted-foreground">{label}</p>
            <p className={cn("mt-1 tabular-nums", size === "sm" ? "text-h3" : "text-stat")}>{value}</p>
            {subtext && <p className="text-caption mt-1 text-muted-foreground">{subtext}</p>}
          </div>
          {icon && (
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
              {icon}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

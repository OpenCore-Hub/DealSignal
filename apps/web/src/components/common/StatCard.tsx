import { cn } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";

interface StatCardProps {
  label: string;
  value: string | number;
  subtext?: string;
  icon?: React.ReactNode;
  className?: string;
}

export function StatCard({ label, value, subtext, icon, className }: StatCardProps) {
  return (
    <Card className={cn("transition-shadow hover:shadow-sm", className)}>
      <CardContent className="p-4">
        <div className="flex items-start justify-between">
          <div>
            <p className="text-caption text-muted-foreground">{label}</p>
            <p className="text-h2 mt-1 tabular-nums">{value}</p>
            {subtext && <p className="text-caption text-muted-foreground mt-1">{subtext}</p>}
          </div>
          {icon && <div className="text-muted-foreground">{icon}</div>}
        </div>
      </CardContent>
    </Card>
  );
}

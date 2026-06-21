import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { HeatLevel } from "@/types";
import { useTranslation } from "react-i18next";

const dotStyles: Record<HeatLevel, string> = {
  hot: "bg-hot-500",
  warm: "bg-warm-500",
  cold: "bg-cold-500",
};

interface HeatBadgeProps {
  level: HeatLevel;
  className?: string;
}

export function HeatBadge({ level, className }: HeatBadgeProps) {
  const { t } = useTranslation("common");
  return (
    <Badge variant={level} className={cn("gap-1", className)}>
      <span className={cn("h-1.5 w-1.5 rounded-full", dotStyles[level])} />
      {t(`heat.${level}`)}
    </Badge>
  );
}

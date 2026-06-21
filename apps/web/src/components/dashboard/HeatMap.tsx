import { Fire, Clock, Snowflake, Link as LinkIcon } from "@phosphor-icons/react";
import type { HeatLevel, Link as LinkType } from "@/types";

interface HeatMapProps {
  links: LinkType[];
}

export function HeatMap({ links }: HeatMapProps) {
  const grouped: Record<HeatLevel, LinkType[]> = {
    hot: links.filter((l) => l.heatLevel === "hot"),
    warm: links.filter((l) => l.heatLevel === "warm"),
    cold: links.filter((l) => l.heatLevel === "cold"),
  };

  const tiers: { level: HeatLevel; label: string; icon: typeof Fire; color: string }[] = [
    { level: "hot", label: "高热度", icon: Fire, color: "bg-hot-500" },
    { level: "warm", label: "中热度", icon: Clock, color: "bg-warm-500" },
    { level: "cold", label: "低热度", icon: Snowflake, color: "bg-cold-500" },
  ];

  return (
    <div className="space-y-4">
      {tiers.map((tier) => {
        const Icon = tier.icon;
        const items = grouped[tier.level];
        return (
          <div key={tier.level}>
            <div className="mb-2 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Icon size={16} weight="fill" className={tier.color.replace("bg-", "text-")} />
                <span className="text-sm font-medium">{tier.label}</span>
              </div>
              <span className="text-caption text-muted-foreground">{items.length} 个链接</span>
            </div>
            <div className="space-y-2">
              {items.slice(0, 5).map((link) => (
                <div
                  key={link.id}
                  className="flex items-center gap-2 rounded-md border border-border bg-card p-2"
                >
                  <LinkIcon size={14} className="shrink-0 text-muted-foreground" />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm">{link.documentTitle}</p>
                    <p className="text-caption text-muted-foreground">{link.accessCount} 次访问</p>
                  </div>
                  <div className={`h-1.5 w-1.5 rounded-full ${tier.color}`} />
                </div>
              ))}
              {items.length === 0 && (
                <p className="text-caption text-muted-foreground">暂无 {tier.label} 链接</p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

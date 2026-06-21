import { useNavigate, useParams } from "react-router";
import { Fire, Clock, Snowflake, Link as LinkIcon } from "@phosphor-icons/react";
import type { HeatLevel, Link as LinkType } from "@/types";
import { useTranslation } from "react-i18next";

interface HeatMapProps {
  links: LinkType[];
}

export function HeatMap({ links }: HeatMapProps) {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { t } = useTranslation("dashboard");
  const { t: tCommon } = useTranslation("common");

  const grouped: Record<HeatLevel, LinkType[]> = {
    hot: links.filter((l) => l.heatLevel === "hot"),
    warm: links.filter((l) => l.heatLevel === "warm"),
    cold: links.filter((l) => l.heatLevel === "cold"),
  };

  const tiers: { level: HeatLevel; icon: typeof Fire; color: string }[] = [
    { level: "hot", icon: Fire, color: "bg-hot-500" },
    { level: "warm", icon: Clock, color: "bg-warm-500" },
    { level: "cold", icon: Snowflake, color: "bg-cold-500" },
  ];

  return (
    <div className="space-y-4">
      {tiers.map((tier) => {
        const Icon = tier.icon;
        const items = grouped[tier.level];
        const tierLabel = tCommon(`heat.${tier.level}`);
        return (
          <div key={tier.level}>
            <div className="mb-2 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Icon size={16} weight="fill" className={tier.color.replace("bg-", "text-")} />
                <span className="text-sm font-medium">{tierLabel}</span>
              </div>
              <span className="text-caption text-muted-foreground">{t("heatMap.linkCount", { count: items.length })}</span>
            </div>
            <div className="space-y-2">
              {items.slice(0, 5).map((link) => {
                const handleClick = () => navigate(`/${workspaceSlug}/links/${link.id}`);
                return (
                  <div
                    key={link.id}
                    role="link"
                    tabIndex={0}
                    className="flex cursor-pointer items-center gap-2 rounded-md border border-border bg-card p-2 transition-colors hover:bg-muted"
                    onClick={handleClick}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        handleClick();
                      }
                    }}
                  >
                    <LinkIcon size={14} className="shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm">{link.documentTitle}</p>
                      <p className="text-caption text-muted-foreground">{t("heatMap.accessCount", { count: link.accessCount })}</p>
                    </div>
                    <div className={`h-1.5 w-1.5 rounded-full ${tier.color}`} />
                  </div>
                );
              })}
              {items.length === 0 && (
                <p className="text-caption text-muted-foreground">{t("heatMap.noLinks", { tier: tierLabel })}</p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

import { Link, useLocation, useParams } from "react-router";
import { Fire, Clock, Snowflake, Link as LinkIcon, ArrowRight } from "@phosphor-icons/react";
import type { HeatLevel, Link as LinkType } from "@/types";
import { useTranslation } from "react-i18next";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { formatCountWithPlural } from "@/lib/formatters";

interface HeatMapProps {
  links: LinkType[];
}

export function HeatMap({ links }: HeatMapProps) {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const location = useLocation();
  const { t, i18n } = useTranslation("dashboard");
  const { t: tCommon } = useTranslation("common");

  const returnState = {
    returnTo: location.pathname + location.search,
    returnLabel: tCommon("back"),
  };

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
    <div className="space-y-4" role="list">
      {tiers.map((tier) => {
        const Icon = tier.icon;
        const items = grouped[tier.level];
        const tierLabel = tCommon(`heat.${tier.level}`);
        const topItems = [...items]
          .sort((a, b) => b.accessCount - a.accessCount)
          .slice(0, 3);
        const maxCount = Math.max(...topItems.map((l) => l.accessCount), 1);

        return (
          <div key={tier.level} role="listitem" className="space-y-2">
            <div className="mb-2 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Icon size={16} weight="fill" className={tier.color.replace("bg-", "text-")} />
                <span className="text-sm font-medium">{tierLabel}</span>
              </div>
              <span className="text-caption text-muted-foreground">
                {formatCountWithPlural(t, "heatMap.linkCount", items.length, i18n.language)}
              </span>
            </div>

            {topItems.length > 0 && (
              <div className="space-y-2">
                {topItems.map((link) => (
                  <Link
                    key={link.id}
                    to={`/${workspaceSlug}/links/${link.id}`}
                    state={returnState}
                    aria-label={`${link.documentTitle}: ${formatCountWithPlural(
                      t,
                      "heatMap.accessCount",
                      link.accessCount,
                      i18n.language
                    )}`}
                    className="flex items-center gap-2 rounded-xl border border-border bg-card p-2 shadow-card transition-all hover:bg-muted/50 hover:border-muted-foreground/20 hover:shadow-card-hover focus-ring"
                  >
                    <LinkIcon size={14} className="shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm">{link.documentTitle}</p>
                      <p className="text-caption text-muted-foreground">
                        {formatCountWithPlural(t, "heatMap.accessCount", link.accessCount, i18n.language)}
                      </p>
                    </div>
                    <div className="h-1.5 w-16 overflow-hidden rounded-full bg-muted">
                      <div
                        className={`h-full rounded-full ${tier.color}`}
                        style={{ width: `${(link.accessCount / maxCount) * 100}%` }}
                      />
                    </div>
                  </Link>
                ))}

                {items.length > 3 && (
                  <Link
                    to={`/${workspaceSlug}/links`}
                    state={returnState}
                    className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "w-full")}
                  >
                    {t("heatMap.viewAll")}
                    <ArrowRight size={14} className="ml-1" />
                  </Link>
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

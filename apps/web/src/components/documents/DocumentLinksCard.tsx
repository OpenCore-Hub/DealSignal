import { useLocation, useNavigate } from "react-router";
import { Copy, Link as LinkIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/common/EmptyState";
import { HeatBadge } from "@/components/common/HeatBadge";
import { copyToClipboard } from "@/lib/clipboard";
import { formatRelativeTime } from "@/lib/formatters";
import type { Document, Link } from "@/types";

interface DocumentLinksCardProps {
  doc: Document;
  links: Link[];
  workspaceSlug: string;
}

export function DocumentLinksCard({ links, workspaceSlug }: DocumentLinksCardProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useTranslation(["documents", "common"]);

  return (
    <section className="rounded-2xl border border-border/70 bg-background px-5 py-5">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <LinkIcon size={16} className="text-muted-foreground" weight="duotone" />
          <h2 className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/80">
            {t("documents:detail.documentLinks")}
          </h2>
        </div>
        {links.length > 0 ? (
          <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
            {links.length}
          </span>
        ) : null}
      </div>

      {links.length === 0 ? (
        <EmptyState
          icon={<LinkIcon size={40} />}
          title={t("documents:detail.linksEmptyTitle")}
          description={t("documents:detail.linksEmptyDescription")}
        />
      ) : (
        <ul className="divide-y divide-border/60">
          {links.map((link) => (
            <li
              key={link.id}
              className="flex flex-col gap-2.5 py-3.5 first:pt-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between"
            >
              <div className="min-w-0 space-y-1">
                <p className="truncate font-mono text-[13px] tracking-tight text-foreground" title={link.shortUrl}>
                  {link.shortUrl}
                </p>
                <p className="text-caption text-muted-foreground">
                  {t("documents:detail.linkViews", {
                    count: link.accessCount,
                    createdAt: formatRelativeTime(link.createdAt),
                  })}
                </p>
              </div>
              <div className="flex shrink-0 items-center gap-1">
                <HeatBadge level={link.heatLevel} />
                <Button
                  size="icon-sm"
                  variant="ghost"
                  aria-label={t("common:copy")}
                  onClick={() => {
                    void copyToClipboard(link.shortUrl, t("common:linkCopied"));
                  }}
                >
                  <Copy size={14} />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  className="h-8 px-2 text-muted-foreground hover:text-foreground"
                  onClick={() =>
                    navigate(`/${workspaceSlug}/links/${link.id}`, {
                      state: {
                        returnTo: location.pathname + location.search,
                        returnLabel: t("documents:detail.back"),
                      },
                    })
                  }
                >
                  {t("common:logs")}
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

import { useLocation, useNavigate } from "react-router";
import { Copy, Link as LinkIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/common/EmptyState";
import { HeatBadge } from "@/components/common/HeatBadge";
import { FileTypeIcon } from "@/components/common/FileTypeIcon";
import { copyToClipboard } from "@/lib/clipboard";
import { formatRelativeTime } from "@/lib/formatters";
import type { Document, Link } from "@/types";

interface DocumentLinksCardProps {
  doc: Document;
  links: Link[];
  workspaceSlug: string;
}

export function DocumentLinksCard({ doc, links, workspaceSlug }: DocumentLinksCardProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useTranslation(["documents", "common"]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2 flex items-center gap-2">
          <LinkIcon size={20} />
          {t("documents:detail.documentLinks")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {links.length === 0 ? (
          <EmptyState
            icon={<LinkIcon size={48} />}
            title={t("documents:detail.linksEmptyTitle")}
            description={t("documents:detail.linksEmptyDescription")}
          />
        ) : (
          <ul className="space-y-2">
            {links.map((link) => (
              <li
                key={link.id}
                className="flex items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
              >
                <div className="flex min-w-0 items-center gap-3">
                  <FileTypeIcon type={doc.fileType} />
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{link.shortUrl}</p>
                    <p className="text-caption text-muted-foreground">
                      {t("documents:detail.linkViews", {
                        count: link.accessCount,
                        createdAt: formatRelativeTime(link.createdAt),
                      })}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <HeatBadge level={link.heatLevel} />
                  <Button
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => {
                      void copyToClipboard(link.shortUrl, t("common:linkCopied"));
                    }}
                  >
                    <Copy size={14} />
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
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
      </CardContent>
    </Card>
  );
}

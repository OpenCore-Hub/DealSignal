import { Link, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { Fire, ArrowRight } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { InsightsOverview } from "@/lib/api";

interface RecentVisitorsFeedProps {
  insights: InsightsOverview | null;
  workspaceSlug: string;
}

function isValidContactId(id: string): boolean {
  // Contact IDs are UUIDs. The backend may return an email fallback for
  // visitors that are not yet persisted as contacts; those should not navigate.
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id);
}

export function RecentVisitorsFeed({
  insights,
  workspaceSlug,
}: RecentVisitorsFeedProps) {
  const { t } = useTranslation("dashboard");
  const { t: tCommon } = useTranslation("common");
  const navigate = useNavigate();

  const visitors = insights?.topContacts ?? [];

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-body flex items-center gap-2 font-medium text-muted-foreground">
          <Fire size={16} className="text-hot-500" />
          {t("sections.recentVisitors")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {visitors.length === 0 ? (
          <p className="text-body text-muted-foreground">
            {t("empty.visitors.description")}
          </p>
        ) : (
          <div className="space-y-3">
            {visitors.slice(0, 5).map((contact) => {
              const hasContactId = isValidContactId(contact.id);
              return (
                <div
                  key={contact.id}
                  role={hasContactId ? "link" : undefined}
                  tabIndex={hasContactId ? 0 : undefined}
                  aria-label={
                    hasContactId
                      ? t("visitor.viewProfile", { email: contact.email })
                      : undefined
                  }
                  className={`flex items-center gap-3 rounded-lg border border-border p-3 transition-colors ${
                    hasContactId
                      ? "cursor-pointer hover:bg-muted"
                      : "cursor-default"
                  }`}
                  onClick={() => {
                    if (hasContactId) {
                      navigate(`/${workspaceSlug}/contacts/${contact.id}`);
                    }
                  }}
                  onKeyDown={(e) => {
                    if (
                      hasContactId &&
                      (e.key === "Enter" || e.key === " ")
                    ) {
                      e.preventDefault();
                      navigate(`/${workspaceSlug}/contacts/${contact.id}`);
                    }
                  }}
                >
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-medium text-primary">
                    {contact.email.charAt(0).toUpperCase()}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">
                      {contact.email}
                    </p>
                    <p className="text-caption text-muted-foreground">
                      {t("visitor.score", { score: contact.score })}
                      {" · "}
                      {tCommon(`heat.${contact.heatLevel}`)}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {visitors.length > 5 && (
          <Link
            to={`/${workspaceSlug}/contacts`}
            className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "mt-3 w-full")}
          >
            {tCommon("viewAll")}
            <ArrowRight size={14} className="ml-1" />
          </Link>
        )}
      </CardContent>
    </Card>
  );
}

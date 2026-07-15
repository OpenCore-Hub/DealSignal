import { useLocation, useNavigate, useParams } from "react-router";
import { Lightning, Envelope, Lightbulb } from "@phosphor-icons/react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { HeatBadge } from "@/components/common/HeatBadge";
import { EmptyState } from "@/components/common/EmptyState";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useTranslation } from "react-i18next";

export function InsightsSuggestionsPage() {
  const { t } = useTranslation("insights");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const location = useLocation();

  const {
    data: suggestions,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    const res = await api.getSuggestions();
    return res.data;
  }, [tc]);

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{tc("retry")}</Button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-32" />
        <Skeleton className="h-32" />
        <Skeleton className="h-32" />
      </div>
    );
  }

  if (!suggestions || suggestions.length === 0) {
    return (
      <EmptyState
        icon={<Lightbulb size={48} />}
        title={t("suggestions.emptyTitle")}
        description={t("suggestions.emptyDescription")}
        size="large"
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-4">
        {suggestions.map((s) => (
          <Card key={s.id} className="transition-colors hover:bg-muted/50">
            <CardContent className="p-5">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Lightning size={18} className="text-warning-500" />
                    <h3 className="text-h3">{s.action}</h3>
                  </div>
                  <p className="text-body text-muted-foreground">{s.reason}</p>
                  <div className="flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
                    <span>{s.contactEmail}</span>
                    <span>·</span>
                    <span>{s.documentTitle}</span>
                    <span>·</span>
                    <HeatBadge level={s.heatLevel} />
                    <span className="tabular-nums">{t("suggestions.score", { score: s.score })}</span>
                  </div>
                </div>
                <div className="flex shrink-0 gap-2">
                  <Button
                    variant="outline"
                    onClick={() =>
                      navigate(`/${workspaceSlug}/contacts/${s.contactId}`, {
                        state: {
                          returnTo: location.pathname + location.search,
                          returnLabel: tc("back"),
                        },
                      })
                    }
                  >
                    {t("suggestions.viewContact")}
                  </Button>
                  <Button className="gap-1.5" disabled title={t("suggestions.emailDisabled")}>
                    <Envelope size={16} />
                    {t("suggestions.writeEmail")}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}

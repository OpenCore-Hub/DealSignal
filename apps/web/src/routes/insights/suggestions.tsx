import { useNavigate, useParams } from "react-router";
import { Crosshair } from "@phosphor-icons/react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useTranslation } from "react-i18next";

/**
 * Follow-ups live on Deal Radar. This tab is a thin redirect surface so Insights
 * stays the analysis center and Radar stays the only action inbox.
 */
export function InsightsSuggestionsPage() {
  const { t } = useTranslation("insights");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

  return (
    <Card data-testid="suggestions-radar-redirect">
      <CardContent className="flex flex-col items-start gap-4 p-6 sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-1">
          <h2 className="text-h3 flex items-center gap-2">
            <Crosshair size={18} />
            {t("overview.radarCtaTitle")}
          </h2>
          <p className="text-body text-muted-foreground">
            {t("suggestions.radarRedirectDescription")}
          </p>
        </div>
        <Button
          className="gap-1.5"
          onClick={() =>
            navigate(`/${workspaceSlug}/dashboard?filter=buying_window`)
          }
        >
          {t("overview.openDealRadar")}
        </Button>
      </CardContent>
    </Card>
  );
}

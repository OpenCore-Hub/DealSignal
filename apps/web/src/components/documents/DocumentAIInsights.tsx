import { Sparkle, FileText, TrendUp, Warning } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/common/EmptyState";
import { useAIStore } from "@/stores/aiStore";
import type { PageAnalytics } from "@/types";

interface DocumentAIInsightsProps {
  documentId: string;
  analytics: PageAnalytics[];
}

export function DocumentAIInsights({ documentId, analytics }: DocumentAIInsightsProps) {
  const { t } = useTranslation("documents");
  const { setOpen, sendMessage } = useAIStore();

  const askAI = (question: string) => {
    setOpen(true);
    sendMessage(question, { documentId });
  };

  if (analytics.length === 0) {
    return (
      <EmptyState
        icon={<Sparkle size={48} />}
        title={t("documents:aiInsights.emptyTitle")}
        description={t("documents:aiInsights.emptyDescription")}
      />
    );
  }

  const topPage = analytics.reduce((top, current) =>
    current.avgDurationSeconds > top.avgDurationSeconds ? current : top
  );
  const exitRisk = analytics.filter((a) => a.exitRate > 0.08).sort((a, b) => b.exitRate - a.exitRate)[0];

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Sparkle size={20} className="text-warning-500" />
            {t("documents:aiInsights.title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-start gap-3 rounded-lg bg-muted/50 p-3">
            <TrendUp size={18} className="mt-0.5 text-success-500" />
            <div>
              <p className="text-sm font-medium">
                {t("documents:aiInsights.topPage", { pageNumber: topPage?.pageNumber ?? 1 })}
              </p>
              <p className="text-caption text-muted-foreground">
                {t("documents:aiInsights.topPageDescription", {
                  seconds: topPage?.avgDurationSeconds ?? 0,
                })}
              </p>
            </div>
          </div>
          {exitRisk && (
            <div className="flex items-start gap-3 rounded-lg bg-muted/50 p-3">
              <Warning size={18} className="mt-0.5 text-hot-500" />
              <div>
                <p className="text-sm font-medium">
                  {t("documents:aiInsights.exitRisk", { pageNumber: exitRisk.pageNumber })}
                </p>
                <p className="text-caption text-muted-foreground">
                  {t("documents:aiInsights.exitRiskDescription", {
                    percent: Math.round(exitRisk.exitRate * 100),
                  })}
                </p>
              </div>
            </div>
          )}
          <div className="flex items-start gap-3 rounded-lg bg-muted/50 p-3">
            <FileText size={18} className="mt-0.5 text-primary" />
            <div>
              <p className="text-sm font-medium">{t("documents:aiInsights.evidenceDriven")}</p>
              <p className="text-caption text-muted-foreground">
                {t("documents:aiInsights.evidenceDrivenDescription")}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-h2">{t("documents:aiInsights.followUpTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => askAI(t("documents:aiInsights.questions.risks"))}
            >
              {t("documents:aiInsights.questionLabels.risks")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => askAI(t("documents:aiInsights.questions.focus"))}
            >
              {t("documents:aiInsights.questionLabels.focus")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => askAI(t("documents:aiInsights.questions.optimize"))}
            >
              {t("documents:aiInsights.questionLabels.optimize")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

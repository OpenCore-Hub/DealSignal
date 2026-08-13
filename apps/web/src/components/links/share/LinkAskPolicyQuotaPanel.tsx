import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import {
  experienceUsesAiLane,
  type VisitorAskExperience,
} from "./visitorAskExperience";

interface LinkAskPolicyQuotaPanelProps {
  linkId: string;
  experience: VisitorAskExperience;
}

/** Read-only AI quota + entitlement for existing deal-room share links (Access tab). */
export function LinkAskPolicyQuotaPanel({
  linkId,
  experience,
}: LinkAskPolicyQuotaPanelProps) {
  const { t } = useTranslation("linkShare");
  const [loading, setLoading] = useState(true);
  const [monthlyUsed, setMonthlyUsed] = useState<number | null>(null);
  const [monthlyLimit, setMonthlyLimit] = useState<number | null>(null);
  const [quotaExceeded, setQuotaExceeded] = useState(false);
  const [entitled, setEntitled] = useState(true);

  useEffect(() => {
    if (!experienceUsesAiLane(experience)) {
      setLoading(false);
      return;
    }

    let cancelled = false;
    setLoading(true);
    void api
      .getLinkAskPolicy(linkId)
      .then((res) => {
        if (cancelled) return;
        setMonthlyUsed(res.data.askAiMonthlyUsed ?? null);
        setMonthlyLimit(res.data.askAiMonthlyLimit ?? null);
        setQuotaExceeded(Boolean(res.data.askAiQuotaExceeded));
        setEntitled(res.data.askAiEntitled !== false);
      })
      .catch(() => {
        if (!cancelled) {
          setMonthlyUsed(null);
          setMonthlyLimit(null);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [linkId, experience]);

  if (!experienceUsesAiLane(experience)) {
    return null;
  }

  const showQuota =
    !loading &&
    monthlyUsed != null &&
    monthlyLimit != null &&
    monthlyLimit > 0;

  return (
    <div className="space-y-2" data-testid="link-ask-policy-quota">
      {!entitled ? (
        <p className="text-xs text-muted-foreground">{t("management.askPolicyNotEntitled")}</p>
      ) : null}

      {quotaExceeded ? (
        <p className="text-xs font-medium text-amber-700 dark:text-amber-300">
          {t("management.askPolicyQuotaPaywall")}
        </p>
      ) : null}

      {showQuota ? (
        <div className="rounded-lg border border-border/60 bg-muted/20 px-3 py-2.5 text-sm">
          <p className="text-muted-foreground">
            {t("management.askPolicyQuotaUsage", {
              used: monthlyUsed,
              limit: monthlyLimit,
            })}
          </p>
        </div>
      ) : null}
    </div>
  );
}

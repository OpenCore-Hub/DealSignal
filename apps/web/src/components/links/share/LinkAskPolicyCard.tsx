import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Sparkle } from "@phosphor-icons/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { VisitorAskExperienceField } from "./VisitorAskExperienceField";
import {
  DEFAULT_VISITOR_ASK_EXPERIENCE,
  experienceUsesAiLane,
  resolveAskPolicyFromExperience,
  resolveExperienceFromAskPolicy,
  type VisitorAskExperience,
} from "./visitorAskExperience";

interface LinkAskPolicyCardProps {
  linkId: string;
  initialAskAiEnabled: boolean;
  initialAskMode?: string;
  onAskAiEnabledChange?: (enabled: boolean) => void;
}

function askPolicyErrorMessage(
  err: unknown,
  t: (key: string) => string,
): string {
  if (err instanceof ApiError) {
    if (err.code === "ask_ai_not_entitled") return t("management.askPolicyNotEntitled");
    if (err.code === "link_not_found") return t("management.askPolicyLinkNotFound");
    if (err.code === "invalid_input") return t("management.askPolicyInvalidInput");
  }
  return t("management.askPolicyUpdateFailed");
}

export function LinkAskPolicyCard({
  linkId,
  initialAskAiEnabled,
  initialAskMode,
  onAskAiEnabledChange,
}: LinkAskPolicyCardProps) {
  const { t } = useTranslation("linkShare");
  const [experience, setExperience] = useState<VisitorAskExperience>(() =>
    resolveExperienceFromAskPolicy(initialAskAiEnabled, initialAskMode),
  );
  const [saving, setSaving] = useState(false);
  const [loadingQuota, setLoadingQuota] = useState(true);
  const [monthlyUsed, setMonthlyUsed] = useState<number | null>(null);
  const [monthlyLimit, setMonthlyLimit] = useState<number | null>(null);
  const [quotaExceeded, setQuotaExceeded] = useState(false);
  const [entitled, setEntitled] = useState(true);

  useEffect(() => {
    setExperience(resolveExperienceFromAskPolicy(initialAskAiEnabled, initialAskMode));
  }, [initialAskAiEnabled, initialAskMode, linkId]);

  useEffect(() => {
    let cancelled = false;
    setLoadingQuota(true);
    void api
      .getLinkAskPolicy(linkId)
      .then((res) => {
        if (cancelled) return;
        setMonthlyUsed(res.data.askAiMonthlyUsed ?? null);
        setMonthlyLimit(res.data.askAiMonthlyLimit ?? null);
        setQuotaExceeded(Boolean(res.data.askAiQuotaExceeded));
        setEntitled(res.data.askAiEntitled !== false);
        setExperience(
          resolveExperienceFromAskPolicy(res.data.askAiEnabled, res.data.askMode),
        );
      })
      .catch(() => {
        if (!cancelled) {
          setMonthlyUsed(null);
          setMonthlyLimit(null);
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingQuota(false);
      });
    return () => {
      cancelled = true;
    };
  }, [linkId]);

  const onExperienceChange = async (next: VisitorAskExperience) => {
    const previous = experience;
    setExperience(next);
    setSaving(true);
    const policy = resolveAskPolicyFromExperience(next);
    try {
      const res = await api.updateLinkAskPolicy(linkId, {
        askAiEnabled: policy.askAiEnabled,
        askMode: policy.askMode,
      });
      setExperience(
        resolveExperienceFromAskPolicy(res.data.askAiEnabled, res.data.askMode),
      );
      if (res.data.askAiMonthlyUsed != null) setMonthlyUsed(res.data.askAiMonthlyUsed);
      if (res.data.askAiMonthlyLimit != null) setMonthlyLimit(res.data.askAiMonthlyLimit);
      if (res.data.askAiQuotaExceeded != null) setQuotaExceeded(res.data.askAiQuotaExceeded);
      if (res.data.askAiEntitled != null) setEntitled(res.data.askAiEntitled);
      onAskAiEnabledChange?.(res.data.askAiEnabled);
      toast.success(t("management.askPolicyExperienceUpdated"));
    } catch (err) {
      setExperience(previous);
      toast.error(askPolicyErrorMessage(err, t));
    } finally {
      setSaving(false);
    }
  };

  const showQuota =
    experienceUsesAiLane(experience) &&
    monthlyUsed != null &&
    monthlyLimit != null &&
    monthlyLimit > 0;

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-h3">
          <Sparkle size={20} />
          {t("management.askPolicyTitle")}
        </CardTitle>
        <CardDescription>{t("management.askPolicyDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <VisitorAskExperienceField
          value={experience || DEFAULT_VISITOR_ASK_EXPERIENCE}
          onChange={(next) => {
            void onExperienceChange(next);
          }}
          disabled={saving || loadingQuota}
          labelKey="management.askPolicyExperienceLabel"
          testId="link-ask-experience"
        />

        {!entitled && experienceUsesAiLane(experience) ? (
          <p className="text-xs text-muted-foreground">{t("management.askPolicyNotEntitled")}</p>
        ) : null}

        {showQuota ? (
          <div
            className="rounded-lg border border-border/60 bg-muted/20 px-3 py-2.5 text-sm"
            data-testid="link-ask-ai-quota"
          >
            <p className="text-muted-foreground">
              {t("management.askPolicyQuotaUsage", {
                used: monthlyUsed,
                limit: monthlyLimit,
              })}
            </p>
            {quotaExceeded ? (
              <p className="mt-1 text-xs font-medium text-amber-700 dark:text-amber-300">
                {t("management.askPolicyQuotaPaywall")}
              </p>
            ) : null}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

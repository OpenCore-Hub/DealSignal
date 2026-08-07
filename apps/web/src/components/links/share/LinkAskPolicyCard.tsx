import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Sparkle } from "@phosphor-icons/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

type AskRoutingMode = "supervised" | "self_serve" | "formal";

interface LinkAskPolicyCardProps {
  linkId: string;
  initialAskAiEnabled: boolean;
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
  onAskAiEnabledChange,
}: LinkAskPolicyCardProps) {
  const { t } = useTranslation("linkShare");
  const [enabled, setEnabled] = useState(initialAskAiEnabled);
  const [askMode, setAskMode] = useState<AskRoutingMode>("supervised");
  const [saving, setSaving] = useState(false);
  const [savingMode, setSavingMode] = useState(false);
  const [loadingQuota, setLoadingQuota] = useState(true);
  const [monthlyUsed, setMonthlyUsed] = useState<number | null>(null);
  const [monthlyLimit, setMonthlyLimit] = useState<number | null>(null);
  const [quotaExceeded, setQuotaExceeded] = useState(false);
  const [entitled, setEntitled] = useState(true);

  useEffect(() => {
    setEnabled(initialAskAiEnabled);
  }, [initialAskAiEnabled, linkId]);

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
        setEnabled(res.data.askAiEnabled);
        const mode = res.data.askMode;
        if (mode === "self_serve" || mode === "supervised" || mode === "formal") {
          setAskMode(mode);
        }
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

  const applyPolicyResponse = (data: {
    askAiEnabled: boolean;
    askMode?: string;
    askAiMonthlyUsed?: number;
    askAiMonthlyLimit?: number;
    askAiQuotaExceeded?: boolean;
    askAiEntitled?: boolean;
  }) => {
    setEnabled(data.askAiEnabled);
    if (data.askMode === "self_serve" || data.askMode === "supervised" || data.askMode === "formal") {
      setAskMode(data.askMode);
    }
    if (data.askAiMonthlyUsed != null) setMonthlyUsed(data.askAiMonthlyUsed);
    if (data.askAiMonthlyLimit != null) setMonthlyLimit(data.askAiMonthlyLimit);
    if (data.askAiQuotaExceeded != null) setQuotaExceeded(data.askAiQuotaExceeded);
    if (data.askAiEntitled != null) setEntitled(data.askAiEntitled);
  };

  const onToggle = async (checked: boolean) => {
    const previous = enabled;
    setEnabled(checked);
    setSaving(true);
    try {
      const res = await api.updateLinkAskPolicy(linkId, { askAiEnabled: checked });
      applyPolicyResponse(res.data);
      onAskAiEnabledChange?.(res.data.askAiEnabled);
      toast.success(
        checked
          ? t("management.askPolicyEnabledSuccess")
          : t("management.askPolicyDisabledSuccess"),
      );
    } catch (err) {
      setEnabled(previous);
      toast.error(askPolicyErrorMessage(err, t));
    } finally {
      setSaving(false);
    }
  };

  const onModeChange = async (value: AskRoutingMode) => {
    const previous = askMode;
    setAskMode(value);
    setSavingMode(true);
    try {
      const res = await api.updateLinkAskPolicy(linkId, { askMode: value });
      applyPolicyResponse(res.data);
    } catch (err) {
      setAskMode(previous);
      toast.error(
        err instanceof ApiError && err.code === "invalid_input"
          ? t("management.askPolicyInvalidInput")
          : t("management.askPolicyModeUpdateFailed"),
      );
    } finally {
      setSavingMode(false);
    }
  };

  const showQuota =
    enabled && monthlyUsed != null && monthlyLimit != null && monthlyLimit > 0;

  const modeHintKey =
    askMode === "self_serve"
      ? "management.askPolicyModeSelfServeHint"
      : askMode === "formal"
        ? "management.askPolicyModeFormalHint"
        : "management.askPolicyModeSupervisedHint";

  const aiToggleDisabled = saving || loadingQuota || (!entitled && !enabled) || askMode === "formal";

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
        <div className="space-y-2">
          <Label className="text-sm font-medium">{t("management.askPolicyModeLabel")}</Label>
          <Select
            value={askMode}
            disabled={loadingQuota || savingMode}
            onValueChange={(value) => void onModeChange(value as AskRoutingMode)}
          >
            <SelectTrigger data-testid="link-ask-mode">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="supervised">
                {t("management.askPolicyModeSupervised")}
              </SelectItem>
              <SelectItem value="self_serve">
                {t("management.askPolicyModeSelfServe")}
              </SelectItem>
              <SelectItem value="formal">
                {t("management.askPolicyModeFormal")}
              </SelectItem>
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">{t(modeHintKey)}</p>
        </div>

        <div
          className="flex items-center justify-between gap-4 rounded-md p-1"
          data-testid="link-ask-ai-enabled"
        >
          <Label className="leading-none font-normal text-foreground">
            {t("management.askPolicyToggleLabel")}
          </Label>
          <Switch
            checked={enabled}
            disabled={aiToggleDisabled}
            onCheckedChange={onToggle}
            aria-label={t("management.askPolicyToggleLabel")}
          />
        </div>

        {!entitled ? (
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

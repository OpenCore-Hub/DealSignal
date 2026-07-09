import { useState } from "react";
import { Question, Eye, EyeSlash } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { EmailTagInput } from "./EmailTagInput";
import { CollapsibleSection } from "./CollapsibleSection";
import type { DraftLink } from "./types";

interface AccessTabProps {
  draft: DraftLink;
  updateDraft: (patch: Partial<DraftLink>) => void;
  errors: Record<string, string>;
}

function OptionSwitch({
  label,
  description,
  checked,
  onCheckedChange,
  disabled,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className={cn("flex items-start justify-between gap-4", disabled && "opacity-50")}>
      <div className="space-y-0.5">
        <Label className="flex items-center gap-1.5 font-normal text-foreground">
          {label}
          {description && (
            <span title={description}>
              <Question size={14} className="text-muted-foreground" />
            </span>
          )}
        </Label>
      </div>
      <Switch
        checked={checked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
      />
    </div>
  );
}

const ADVANCED_KEYS: Array<keyof DraftLink> = [
  "aiCopilotEnabled",
  "enableFileRequests",
  "enableIndexFileGeneration",
  "enableQaConversations",
];

const ADVANCED_LABELS: Record<string, string> = {
  aiCopilotEnabled: "accessRules.advanced.aiAgents",
  enableFileRequests: "accessRules.advanced.fileRequests",
  enableIndexFileGeneration: "accessRules.advanced.indexFile",
  enableQaConversations: "accessRules.advanced.qaConversations",
};

const ADVANCED_DESCRIPTIONS: Record<string, string> = {
  aiCopilotEnabled: "accessRules.advanced.aiAgentsDescription",
  enableFileRequests: "accessRules.advanced.fileRequestsDescription",
  enableIndexFileGeneration: "accessRules.advanced.indexFileDescription",
  enableQaConversations: "accessRules.advanced.qaConversationsDescription",
};

export function AccessTab({ draft, updateDraft, errors }: AccessTabProps) {
  const { t } = useTranslation("linkShare");
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  const advancedCount = ADVANCED_KEYS.filter((key) => draft[key]).length;

  const handleRequireEmailChange = (checked: boolean) => {
    updateDraft({ requireEmail: checked });
    if (!checked) {
      updateDraft({ requireEmailVerification: false });
    }
  };

  const handleRequireVerificationChange = (checked: boolean) => {
    if (checked) {
      updateDraft({ requireEmail: true, requireEmailVerification: true });
    } else {
      updateDraft({ requireEmailVerification: false });
    }
  };

  return (
    <div className="space-y-6 py-2">
      <div className="space-y-4">
        <h4 className="text-sm font-medium">{t("accessRules.authentication.title")}</h4>
        <OptionSwitch
          label={t("accessRules.authentication.requireEmail")}
          description={t("accessRules.authentication.requireEmailDescription")}
          checked={draft.requireEmail}
          onCheckedChange={handleRequireEmailChange}
        />
        <OptionSwitch
          label={t("accessRules.authentication.requireVerification")}
          description={t("accessRules.authentication.requireVerificationDescription")}
          checked={draft.requireEmailVerification}
          onCheckedChange={handleRequireVerificationChange}
          disabled={!draft.requireEmail}
        />
        <div className="space-y-2">
          <OptionSwitch
            label={t("accessRules.authentication.requirePassword")}
            description={t("accessRules.authentication.requirePasswordDescription")}
            checked={draft.requirePassword}
            onCheckedChange={(checked) => updateDraft({ requirePassword: checked })}
          />
          {draft.requirePassword && (
            <div className="relative">
              <Input
                type={showPassword ? "text" : "password"}
                value={draft.password}
                onChange={(e) => updateDraft({ password: e.target.value })}
                placeholder={t("accessRules.authentication.passwordPlaceholder")}
                className="pr-10"
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                className="absolute inset-y-0 right-0 flex items-center px-3 text-muted-foreground hover:text-foreground"
                aria-label={showPassword ? "Hide password" : "Show password"}
              >
                {showPassword ? <EyeSlash size={16} /> : <Eye size={16} />}
              </button>
            </div>
          )}
          {errors.password && (
            <p className="text-xs text-destructive">{errors.password}</p>
          )}
        </div>
      </div>

      <div className="space-y-3">
        <h4 className="text-sm font-medium">{t("accessRules.allowedViewers.title")}</h4>
        <EmailTagInput
          values={draft.allowedViewers}
          onChange={(values) => updateDraft({ allowedViewers: values })}
          placeholder={t("accessRules.allowedViewers.placeholder")}
          hint={t("accessRules.allowedViewers.hint")}
        />
        <OptionSwitch
          label={t("accessRules.allowedViewers.autoAddInvited")}
          description={t("accessRules.allowedViewers.autoAddInvitedDescription")}
          checked={draft.autoAddInvited}
          onCheckedChange={(checked) => updateDraft({ autoAddInvited: checked })}
        />
        {errors.allowedViewers && (
          <p className="text-xs text-destructive">{errors.allowedViewers}</p>
        )}
      </div>

      <div className="space-y-3">
        <h4 className="text-sm font-medium">{t("accessRules.blockedViewers.title")}</h4>
        <EmailTagInput
          values={draft.blockedViewers}
          onChange={(values) => updateDraft({ blockedViewers: values })}
          placeholder={t("accessRules.blockedViewers.placeholder")}
          hint={t("accessRules.blockedViewers.hint")}
        />
        {errors.blockedViewers && (
          <p className="text-xs text-destructive">{errors.blockedViewers}</p>
        )}
      </div>

      <div className="space-y-4">
        <h4 className="text-sm font-medium">{t("accessRules.additionalProtections.title")}</h4>
        <OptionSwitch
          label={t("accessRules.additionalProtections.watermark")}
          description={t("accessRules.additionalProtections.watermarkDescription")}
          checked={draft.watermarkEnabled}
          onCheckedChange={(checked) => updateDraft({ watermarkEnabled: checked })}
        />
        <OptionSwitch
          label={t("accessRules.additionalProtections.requireNda")}
          description={t("accessRules.additionalProtections.requireNdaDescription")}
          checked={draft.requireNda}
          onCheckedChange={(checked) => updateDraft({ requireNda: checked })}
        />
        <OptionSwitch
          label={t("accessRules.additionalProtections.allowDownloading")}
          description={t("accessRules.additionalProtections.allowDownloadingDescription")}
          checked={draft.allowDownloading}
          onCheckedChange={(checked) => updateDraft({ allowDownloading: checked })}
        />
        <OptionSwitch
          label={t("accessRules.additionalProtections.screenshotProtection")}
          description={t("accessRules.additionalProtections.screenshotProtectionDescription")}
          checked={draft.enableScreenshotProtection}
          onCheckedChange={(checked) => updateDraft({ enableScreenshotProtection: checked })}
        />
      </div>

      <CollapsibleSection
        title={t("accessRules.advanced.title")}
        badge={
          advancedCount > 0 ? (
            <Badge variant="secondary" className="text-xs">
              {t("accessRules.advanced.enabledCount", { count: advancedCount })}
            </Badge>
          ) : undefined
        }
        open={advancedOpen}
        onToggle={() => setAdvancedOpen((v) => !v)}
      >
        {ADVANCED_KEYS.map((key) => (
          <OptionSwitch
            key={key}
            label={t(ADVANCED_LABELS[key])}
            description={t(ADVANCED_DESCRIPTIONS[key])}
            checked={draft[key] as boolean}
            onCheckedChange={(checked) => updateDraft({ [key]: checked } as Partial<DraftLink>)}
          />
        ))}
      </CollapsibleSection>

      {(errors.submit || errors.conflict) && (
        <p className="text-xs text-destructive">{errors.submit || errors.conflict}</p>
      )}
    </div>
  );
}

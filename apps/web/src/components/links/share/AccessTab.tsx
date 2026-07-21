import { useState, useMemo } from "react";
import { Question, Eye, EyeSlash } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { ContactEmailTagInput } from "./ContactEmailTagInput";
import { CollapsibleSection } from "./CollapsibleSection";
import type { DraftLink } from "./types";

interface AccessTabProps {
  draft: DraftLink;
  updateDraft: (patch: Partial<DraftLink>) => void;
  errors: Record<string, string>;
  highlightedFields?: string[];
  isDealRoomLink?: boolean;
  documents?: { id: string; title: string }[];
  ndaTemplates?: { id: string; name: string; sourceDocumentId: string }[];
}

function OptionSwitch({
  label,
  description,
  checked,
  onCheckedChange,
  disabled,
  highlighted,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  highlighted?: boolean;
}) {
  return (
    <div className={cn(
      "flex items-start justify-between gap-4 rounded-md p-1",
      disabled && "opacity-50",
      highlighted && "bg-primary/10 motion-safe:transition-colors motion-safe:duration-200"
    )}>
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
        aria-label={label}
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

const FUNCTIONAL_ADVANCED_KEYS: Array<keyof DraftLink> = [...ADVANCED_KEYS];

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

type PasswordStrengthLevel = 0 | 1 | 2 | 3 | 4;

function getPasswordStrength(password: string): {
  level: PasswordStrengthLevel;
  variety: number;
} {
  if (password.length === 0) return { level: 0, variety: 0 };
  const hasLower = /[a-z]/.test(password);
  const hasUpper = /[A-Z]/.test(password);
  const hasDigit = /[0-9]/.test(password);
  const hasSymbol = /[^a-zA-Z0-9]/.test(password);
  const variety = [hasLower, hasUpper, hasDigit, hasSymbol].filter(Boolean).length;
  if (password.length < 8 || variety < 2) return { level: 1, variety };
  if (password.length < 10 || variety < 3) return { level: 2, variety };
  if (password.length < 12 || variety < 4) return { level: 3, variety };
  return { level: 4, variety };
}

function strengthBarColor(level: PasswordStrengthLevel): string {
  switch (level) {
    case 1:
      return "bg-destructive";
    case 2:
      return "bg-amber-500";
    case 3:
      return "bg-blue-500";
    case 4:
      return "bg-emerald-500";
    default:
      return "bg-muted";
  }
}

export function AccessTab({
  draft,
  updateDraft,
  errors,
  highlightedFields = [],
  isDealRoomLink,
  documents = [],
  ndaTemplates = [],
}: AccessTabProps) {
  const { t } = useTranslation("linkShare");

  const isHighlighted = (field: string) => highlightedFields.includes(field);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  const advancedCount = FUNCTIONAL_ADVANCED_KEYS.filter((key) => draft[key]).length;
  const verificationDisabled = !isDealRoomLink;

  const handleRequireEmailChange = (checked: boolean) => {
    updateDraft({
      requireEmail: checked,
      requireEmailVerification: checked ? draft.requireEmailVerification : false,
    });
  };

  const handleRequireVerificationChange = (checked: boolean) => {
    updateDraft({
      requireEmailVerification: checked,
      requireEmail: checked ? true : draft.requireEmail,
    });
  };

  const handleAllowedViewersChange = (values: string[]) => {
    const patch: Partial<DraftLink> = { allowedViewers: values };
    if (values.length > 0 && !draft.requireEmail && !draft.requireEmailVerification) {
      patch.requireEmail = true;
    }
    updateDraft(patch);
  };

  const handleRequireNdaChange = (checked: boolean) => {
    updateDraft({
      requireNda: checked,
      ndaDocumentId: checked ? draft.ndaDocumentId : "",
      ndaTemplateId: checked ? draft.ndaTemplateId : "",
    });
  };

  const ndaOptions =
    ndaTemplates.length > 0
      ? ndaTemplates.map((tpl) => ({
          id: tpl.id,
          title: tpl.name,
          templateId: tpl.id,
          documentId: tpl.sourceDocumentId,
        }))
      : documents.map((doc) => ({
          id: doc.id,
          title: doc.title,
          templateId: "",
          documentId: doc.id,
        }));

  // Always controlled: empty selection is null (not undefined) for Base UI Select.
  const selectedNdaValue = draft.ndaTemplateId || draft.ndaDocumentId || "";

  const conflicts = useMemo(
    () => draft.allowedViewers.filter((v) => draft.blockedViewers.includes(v)),
    [draft.allowedViewers, draft.blockedViewers]
  );

  const passwordStrength = useMemo(
    () => getPasswordStrength(draft.password),
    [draft.password]
  );

  const allowedViewersNeedEmail =
    draft.allowedViewers.length > 0 && !draft.requireEmail && !draft.requireEmailVerification;

  return (
    <div className="space-y-6 py-2">
      <div className="space-y-4">
        <h4 className="text-sm font-medium">{t("accessRules.authentication.title")}</h4>
        <OptionSwitch
          label={t("accessRules.authentication.requireEmail")}
          description={t("accessRules.authentication.requireEmailDescription")}
          checked={draft.requireEmail}
          onCheckedChange={handleRequireEmailChange}
          highlighted={isHighlighted("requireEmail")}
        />
        <OptionSwitch
          label={t("accessRules.authentication.requireVerification")}
          description={
            verificationDisabled
              ? t("accessRules.authentication.verificationDisabledForDocuments")
              : t("accessRules.authentication.requireVerificationDescription")
          }
          checked={draft.requireEmailVerification}
          onCheckedChange={handleRequireVerificationChange}
          disabled={verificationDisabled}
          highlighted={isHighlighted("requireEmailVerification")}
        />
        {errors.requireVerificationContacts && (
          <p className="text-xs text-destructive">{errors.requireVerificationContacts}</p>
        )}
        <div className={cn("space-y-2 rounded-md p-1", isHighlighted("requirePassword") && "bg-primary/10 motion-safe:transition-colors motion-safe:duration-200")}>
          <OptionSwitch
            label={t("accessRules.authentication.requirePassword")}
            description={t("accessRules.authentication.requirePasswordDescription")}
            checked={draft.requirePassword}
            onCheckedChange={(checked) => updateDraft({ requirePassword: checked })}
          />
          {draft.requirePassword && (
            <div className="space-y-2">
              <div className="relative">
                <Input
                  type={showPassword ? "text" : "password"}
                  value={draft.password}
                  onChange={(e) => updateDraft({ password: e.target.value })}
                  placeholder={t("accessRules.authentication.passwordPlaceholder")}
                  className="pr-10"
                  aria-describedby="password-strength-hint"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((v) => !v)}
                  className="absolute inset-y-0 right-0 flex items-center px-3 text-muted-foreground hover:text-foreground"
                  aria-label={showPassword ? t("accessRules.authentication.hidePassword") : t("accessRules.authentication.showPassword")}
                >
                  {showPassword ? <EyeSlash size={16} /> : <Eye size={16} />}
                </button>
              </div>
              {draft.password.length > 0 && (
                <div className="space-y-1">
                  <div className="flex h-1.5 overflow-hidden rounded-full bg-muted">
                    <div
                      className={cn(
                        "motion-safe:transition-all motion-safe:duration-300",
                        strengthBarColor(passwordStrength.level)
                      )}
                      style={{ width: `${(passwordStrength.level / 4) * 100}%` }}
                      aria-hidden="true"
                    />
                  </div>
                  <p id="password-strength-hint" className="text-xs text-muted-foreground">
                    {t("accessRules.passwordStrength.label", {
                      level: t(`accessRules.passwordStrength.level${passwordStrength.level}`),
                    })}
                  </p>
                </div>
              )}
              {draft.password.length > 0 && draft.password.length < 8 && !errors.password && (
                <p className="text-xs text-destructive">
                  {t("accessRules.errors.passwordMinLength")}
                </p>
              )}
              {errors.password && (
                <p className="text-xs text-destructive">{errors.password}</p>
              )}
            </div>
          )}
        </div>
      </div>

      <div className="space-y-3">
        <h4 className="text-sm font-medium">{t("accessRules.allowedViewers.title")}</h4>
        <ContactEmailTagInput
          values={draft.allowedViewers}
          onChange={handleAllowedViewersChange}
          placeholder={t("accessRules.allowedViewers.placeholder")}
          hint={t("accessRules.allowedViewers.hint")}
          conflictValues={conflicts}
          allowDomains={false}
        />
        {allowedViewersNeedEmail && (
          <p className="text-xs text-muted-foreground">
            {t("accessRules.errors.allowRequiresEmail")}
          </p>
        )}
        {errors.allowedViewers && (
          <p className="text-xs text-destructive">{errors.allowedViewers}</p>
        )}
      </div>

      <div className="space-y-3">
        <h4 className="text-sm font-medium">{t("accessRules.blockedViewers.title")}</h4>
        <ContactEmailTagInput
          values={draft.blockedViewers}
          onChange={(values) => updateDraft({ blockedViewers: values })}
          placeholder={t("accessRules.blockedViewers.placeholder")}
          hint={t("accessRules.blockedViewers.hint")}
          conflictValues={conflicts}
          allowDomains={false}
        />
        {errors.blockedViewers && (
          <p className="text-xs text-destructive">{errors.blockedViewers}</p>
        )}
      </div>

      {conflicts.length > 0 && (
        <p className="text-xs text-destructive">
          {t("accessRules.errors.conflict", { value: conflicts.join(", ") })}
        </p>
      )}

      <div className="space-y-4">
        <h4 className="text-sm font-medium">{t("accessRules.additionalProtections.title")}</h4>
        <OptionSwitch
          label={t("accessRules.additionalProtections.watermark")}
          description={t("accessRules.additionalProtections.watermarkDescription")}
          checked={draft.watermarkEnabled}
          onCheckedChange={(checked) => updateDraft({ watermarkEnabled: checked })}
          highlighted={isHighlighted("watermarkEnabled")}
        />
        <div className={cn("space-y-3", isHighlighted("requireNda") && "bg-primary/10 motion-safe:transition-colors motion-safe:duration-200 rounded-md p-1")}>
          <OptionSwitch
            label={t("accessRules.additionalProtections.requireNda")}
            description={t("accessRules.additionalProtections.requireNdaDescription")}
            checked={draft.requireNda}
            onCheckedChange={handleRequireNdaChange}
            highlighted={isHighlighted("requireNda")}
          />
          {draft.requireNda && (
            <div className="space-y-2 pl-0 sm:pl-6">
              <Label className="text-xs font-normal text-muted-foreground">
                {t("accessRules.additionalProtections.ndaDocument")}
              </Label>
              <Select
                value={selectedNdaValue || null}
                onValueChange={(value) => {
                  const selected = value ?? "";
                  if (!selected || selected === "__empty__") return;
                  const opt = ndaOptions.find(
                    (o) => o.id === selected || o.templateId === selected || o.documentId === selected
                  );
                  const nextTemplateId =
                    opt?.templateId && opt.templateId.length > 0
                      ? opt.templateId
                      : ndaTemplates.some((t) => t.id === selected)
                        ? selected
                        : "";
                  const nextDocumentId =
                    opt?.documentId && opt.documentId.length > 0 ? opt.documentId : selected;
                  updateDraft({
                    ndaTemplateId: nextTemplateId,
                    ndaDocumentId: nextDocumentId,
                  });
                }}
              >
                <SelectTrigger aria-label={t("accessRules.additionalProtections.ndaDocument")} className="w-full">
                  <SelectValue placeholder={t("accessRules.additionalProtections.ndaDocumentPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {ndaOptions.length === 0 ? (
                    <SelectItem value="__empty__" disabled>
                      {t("accessRules.additionalProtections.ndaDocumentPlaceholder")}
                    </SelectItem>
                  ) : (
                    ndaOptions.map((opt) => (
                      <SelectItem key={opt.id} value={opt.id}>
                        {opt.title}
                      </SelectItem>
                    ))
                  )}
                </SelectContent>
              </Select>
              {errors.ndaDocumentId && (
                <p className="text-xs text-destructive">{errors.ndaDocumentId}</p>
              )}
            </div>
          )}
        </div>

        <OptionSwitch
          label={t("accessRules.additionalProtections.allowDownloading")}
          description={t("accessRules.additionalProtections.allowDownloadingDescription")}
          checked={draft.allowDownloading}
          onCheckedChange={(checked) => updateDraft({ allowDownloading: checked })}
          highlighted={isHighlighted("allowDownloading")}
        />
        <OptionSwitch
          label={t("accessRules.additionalProtections.screenshotProtection")}
          description={t("accessRules.additionalProtections.screenshotProtectionDescription")}
          checked={draft.enableScreenshotProtection}
          onCheckedChange={(checked) => updateDraft({ enableScreenshotProtection: checked })}
          highlighted={isHighlighted("enableScreenshotProtection")}
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
            highlighted={isHighlighted(key)}
          />
        ))}
      </CollapsibleSection>

      {errors.submit && (
        <p className="text-xs text-destructive">{errors.submit}</p>
      )}
    </div>
  );
}

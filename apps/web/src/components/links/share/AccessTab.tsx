import { useState, useMemo, useEffect } from "react";
import { Question } from "@phosphor-icons/react";
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { ContactEmailTagInput } from "./ContactEmailTagInput";
import { CollapsibleSection } from "./CollapsibleSection";
import type { DraftLink } from "./types";
import {
  STANDALONE_ADVANCED_KEYS,
  countAdvancedEnabled,
  visitorAskMasterEnabled,
  visitorAskMasterPatch,
} from "./visitorAskAdvanced";
import { shouldBlockAskDocsForKnowledgeBase } from "./visitorAskKbGate";
import type { DealRoomKnowledgeBaseStatus } from "@/types";

interface AccessTabProps {
  draft: DraftLink;
  updateDraft: (patch: Partial<DraftLink>) => void;
  errors: Record<string, string>;
  highlightedFields?: string[];
  isDealRoomLink?: boolean;
  /** True when the link already has a password hash server-side (plaintext is never returned). */
  passwordAlreadySet?: boolean;
  documents?: { id: string; title: string }[];
  ndaTemplates?: { id: string; name: string; sourceDocumentId: string }[];
  /** Deal-room KB status for Q4 Ask Docs pre-gate. */
  knowledgeBaseStatus?: DealRoomKnowledgeBaseStatus | null;
  /** Link to the room documents / knowledge base panel. */
  knowledgeBaseHref?: string;
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
      <div className="flex min-w-0 items-center gap-1.5">
        <Label className="font-normal text-foreground">{label}</Label>
        {description && (
          <TooltipProvider delay={150}>
            <Tooltip>
              <TooltipTrigger
                type="button"
                delay={150}
                className="inline-flex size-5 shrink-0 items-center justify-center rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
                aria-label={description}
              >
                <Question size={14} weight="regular" aria-hidden />
              </TooltipTrigger>
              <TooltipContent side="top">{description}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )}
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

const STANDALONE_ADVANCED_LABELS: Record<(typeof STANDALONE_ADVANCED_KEYS)[number], string> = {
  enableFileRequests: "accessRules.advanced.fileRequests",
  enableIndexFileGeneration: "accessRules.advanced.indexFile",
};

const STANDALONE_ADVANCED_DESCRIPTIONS: Record<(typeof STANDALONE_ADVANCED_KEYS)[number], string> = {
  enableFileRequests: "accessRules.advanced.fileRequestsDescription",
  enableIndexFileGeneration: "accessRules.advanced.indexFileDescription",
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

const STORED_PASSWORD_MASK = "••••••••";

export function AccessTab({
  draft,
  updateDraft,
  errors,
  highlightedFields = [],
  isDealRoomLink,
  passwordAlreadySet = false,
  documents = [],
  ndaTemplates = [],
  knowledgeBaseStatus = null,
  knowledgeBaseHref,
}: AccessTabProps) {
  const { t } = useTranslation("linkShare");

  const isHighlighted = (field: string) => highlightedFields.includes(field);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  /** When a stored password is masked, focus clears the mask so the owner can type a replacement. */
  const [editingStoredPassword, setEditingStoredPassword] = useState(false);

  useEffect(() => {
    if (!draft.requirePassword || !passwordAlreadySet) {
      setEditingStoredPassword(false);
    }
  }, [draft.requirePassword, passwordAlreadySet]);

  const advancedCount = countAdvancedEnabled(draft);
  const visitorAskOn = visitorAskMasterEnabled(draft);
  const verificationDisabled = !isDealRoomLink;
  const askDocsBlocked = shouldBlockAskDocsForKnowledgeBase(
    Boolean(isDealRoomLink),
    knowledgeBaseStatus,
  );

  const handleRequireEmailChange = (checked: boolean) => {
    updateDraft({
      requireEmail: checked,
      // Mutually exclusive with verification — identity is either self-reported or code-proven.
      requireEmailVerification: false,
    });
  };

  const handleRequireVerificationChange = (checked: boolean) => {
    updateDraft({
      requireEmailVerification: checked,
      // Mutually exclusive with email self-report; code resolves the visitor email.
      requireEmail: false,
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

  const showStoredPasswordMask =
    passwordAlreadySet && draft.password.length === 0 && !editingStoredPassword;
  const passwordFieldValue = showStoredPasswordMask ? STORED_PASSWORD_MASK : draft.password;
  const passwordFieldPlaceholder = passwordAlreadySet
    ? t("accessRules.authentication.passwordReplacePlaceholder")
    : t("accessRules.authentication.passwordPlaceholder");

  const handlePasswordChange = (next: string) => {
    if (showStoredPasswordMask) {
      setEditingStoredPassword(true);
      // Replace the mask wholesale — ignore residual mask characters from some browsers.
      const stripped = next.split(STORED_PASSWORD_MASK).join("");
      updateDraft({ password: stripped });
      return;
    }
    updateDraft({ password: next });
  };

  const handlePasswordFocus = () => {
    if (showStoredPasswordMask) {
      setEditingStoredPassword(true);
    }
  };

  const handlePasswordBlur = () => {
    if (draft.password.length === 0) {
      setEditingStoredPassword(false);
    }
  };

  const allowedViewersNeedEmail =
    draft.allowedViewers.length > 0 && !draft.requireEmail && !draft.requireEmailVerification;

  return (
    <div className="space-y-6 py-2">
      <div className="space-y-4">
        <h4 className="text-sm font-medium">{t("accessRules.authentication.title")}</h4>
        <p className="text-xs text-muted-foreground">
          {t("accessRules.authentication.emailIdentityHint")}
        </p>
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
              <Input
                type="password"
                value={passwordFieldValue}
                onChange={(e) => handlePasswordChange(e.target.value)}
                onFocus={handlePasswordFocus}
                onBlur={handlePasswordBlur}
                placeholder={passwordFieldPlaceholder}
                autoComplete="new-password"
                aria-describedby={
                  passwordAlreadySet && draft.password.length === 0
                    ? "password-set-hint"
                    : "password-strength-hint"
                }
              />
              {passwordAlreadySet && draft.password.length === 0 && (
                <p id="password-set-hint" className="text-xs text-muted-foreground">
                  {t("accessRules.authentication.passwordSetHint")}
                </p>
              )}
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
        <div
          className={cn(
            "space-y-3 rounded-md border border-border/60 p-3",
            (isHighlighted("aiCopilotEnabled") || isHighlighted("enableQaConversations")) &&
              "ring-2 ring-primary/40"
          )}
        >
          <OptionSwitch
            label={t("accessRules.advanced.visitorAsk")}
            description={t("accessRules.advanced.visitorAskDescription")}
            checked={visitorAskOn}
            onCheckedChange={(checked) => {
              if (!checked) {
                updateDraft(visitorAskMasterPatch(false));
                return;
              }
              // Q4: default to Ask Host when Ask Docs cannot be enabled yet.
              if (askDocsBlocked) {
                updateDraft({ aiCopilotEnabled: false, enableQaConversations: true });
                return;
              }
              updateDraft(visitorAskMasterPatch(true));
            }}
            highlighted={isHighlighted("aiCopilotEnabled") || isHighlighted("enableQaConversations")}
          />
          {visitorAskOn && (
            <div className="ml-1 space-y-2 border-l border-border/60 pl-3">
              <OptionSwitch
                label={t("accessRules.advanced.askDocs")}
                description={t("accessRules.advanced.askDocsDescription")}
                checked={draft.aiCopilotEnabled}
                disabled={askDocsBlocked}
                onCheckedChange={(checked) => {
                  if (askDocsBlocked && checked) return;
                  const nextQa = draft.enableQaConversations;
                  if (!checked && !nextQa) {
                    updateDraft(visitorAskMasterPatch(false));
                    return;
                  }
                  updateDraft({ aiCopilotEnabled: checked });
                }}
                highlighted={isHighlighted("aiCopilotEnabled")}
              />
              {askDocsBlocked && (
                <p className="text-xs text-muted-foreground">
                  {t("accessRules.advanced.knowledgeBaseRequired")}{" "}
                  {knowledgeBaseHref ? (
                    <a
                      href={knowledgeBaseHref}
                      className="font-medium text-primary underline-offset-2 hover:underline"
                    >
                      {t("accessRules.advanced.openKnowledgeBase")}
                    </a>
                  ) : null}
                </p>
              )}
              <OptionSwitch
                label={t("accessRules.advanced.askHost")}
                description={t("accessRules.advanced.askHostDescription")}
                checked={draft.enableQaConversations}
                onCheckedChange={(checked) => {
                  const nextDocs = draft.aiCopilotEnabled;
                  if (!checked && !nextDocs) {
                    updateDraft(visitorAskMasterPatch(false));
                    return;
                  }
                  updateDraft({ enableQaConversations: checked });
                }}
                highlighted={isHighlighted("enableQaConversations")}
              />
            </div>
          )}
        </div>
        {STANDALONE_ADVANCED_KEYS.map((key) => (
          <OptionSwitch
            key={key}
            label={t(STANDALONE_ADVANCED_LABELS[key])}
            description={t(STANDALONE_ADVANCED_DESCRIPTIONS[key])}
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

import { useState, useMemo, useEffect, type ReactNode } from "react";
import { LockSimple, Question } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
} from "./shareAdvanced";
import { VisitorAskExperienceField } from "./VisitorAskExperienceField";
import { LinkAskPolicyQuotaPanel } from "./LinkAskPolicyQuotaPanel";
import type { VisitorAskExperience } from "./visitorAskExperience";

export type AccessTabLayout = "compact" | "sections";

/**
 * Who-can-access UI scope:
 * - `full` — allow + block (share-link create/edit)
 * - `block-only` — room-wide blocklist only (Access Control page)
 * - `none` — hide audience section
 */
export type AccessAudienceMode = "full" | "block-only" | "none";

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
  /**
   * `compact` — stacked blocks for dialogs.
   * `sections` — numbered cards for the deal-room Access Control page.
   */
  layout?: AccessTabLayout;
  /** Defaults to `full`. Room Access Control uses `block-only`. */
  audienceMode?: AccessAudienceMode;
  /** Room-mandated floors: owner cannot turn these gates off on the link. */
  roomSecurityFloors?: {
    requireEmailVerification?: boolean;
    requireNda?: boolean;
  };
  /** Room-wide blocklist emails — read-only on deal-room share links. */
  roomBlockedEmails?: string[];
  /** Existing link id — enables read-only AI quota on edit. */
  linkId?: string;
}

function OptionSwitch({
  label,
  description,
  checked,
  onCheckedChange,
  disabled,
  locked,
  highlighted,
  testId,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  /** Room-security floor: forced ON and not user-toggleable. */
  locked?: boolean;
  highlighted?: boolean;
  testId?: string;
}) {
  const inert = Boolean(disabled || locked);
  const shownChecked = locked ? true : checked;
  return (
    <div
      className={cn(
        "flex items-center justify-between gap-4 rounded-md p-1",
        inert && "opacity-60",
        locked && "cursor-not-allowed",
        highlighted && "bg-primary/10 motion-safe:transition-colors motion-safe:duration-200"
      )}
      data-testid={testId}
      data-locked={locked ? "true" : "false"}
    >
      <div className="flex min-w-0 items-center gap-1.5">
        <Label className="leading-none font-normal text-foreground">{label}</Label>
        {locked ? (
          <LockSimple
            size={14}
            weight="fill"
            className="shrink-0 text-muted-foreground"
            aria-hidden
          />
        ) : null}
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
      <div className={cn(inert && "pointer-events-none")}>
        <Switch
          aria-label={label}
          aria-disabled={inert || undefined}
          checked={shownChecked}
          onCheckedChange={inert ? undefined : onCheckedChange}
          disabled={inert}
          readOnly={locked || undefined}
        />
      </div>
    </div>
  );
}

function SettingsSection({
  step,
  title,
  description,
  children,
  id,
}: {
  step?: string;
  title: string;
  description?: string;
  children: ReactNode;
  id: string;
}) {
  return (
    <Card data-testid={`access-section-${id}`}>
      <CardHeader className="gap-0 pb-3">
        <div className="flex items-center gap-3">
          {step ? (
            <span
              className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted text-xs font-semibold leading-none text-muted-foreground"
              aria-hidden
            >
              {step}
            </span>
          ) : null}
          <CardTitle id={`${id}-heading`} className="min-w-0 text-base leading-none">
            {title}
          </CardTitle>
        </div>
        {description ? (
          <CardDescription className="mt-1.5">{description}</CardDescription>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-4" aria-labelledby={`${id}-heading`}>
        {children}
      </CardContent>
    </Card>
  );
}

function CompactSection({
  title,
  description,
  children,
  id,
}: {
  step?: string;
  title: string;
  description?: string;
  children: ReactNode;
  id: string;
}) {
  return (
    <div className="space-y-4" data-testid={`access-section-${id}`}>
      <div className="space-y-1">
        <h4 id={`${id}-heading`} className="text-sm font-medium">
          {title}
        </h4>
        {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
      </div>
      {children}
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
  layout = "compact",
  audienceMode = "full",
  roomSecurityFloors,
  roomBlockedEmails = [],
  linkId,
}: AccessTabProps) {
  const { t } = useTranslation("linkShare");
  const sections = layout === "sections";
  const showAllow = audienceMode === "full";
  const showBlock = audienceMode === "full" || audienceMode === "block-only";
  const showViewersSection = showAllow || showBlock;
  const verifyFloor = Boolean(roomSecurityFloors?.requireEmailVerification);
  const ndaFloor = Boolean(roomSecurityFloors?.requireNda);
  const roomBlockedSet = useMemo(
    () => new Set(roomBlockedEmails.map((v) => v.trim().toLowerCase()).filter(Boolean)),
    [roomBlockedEmails],
  );
  const showRoomLockedBlocks = isDealRoomLink && roomBlockedSet.size > 0 && audienceMode === "full";

  const isHighlighted = (field: string) => highlightedFields.includes(field);
  const [advancedOpen, setAdvancedOpen] = useState(sections);
  /** When a stored password is masked, focus clears the mask so the owner can type a replacement. */
  const [editingStoredPassword, setEditingStoredPassword] = useState(false);

  useEffect(() => {
    if (!draft.requirePassword || !passwordAlreadySet) {
      setEditingStoredPassword(false);
    }
  }, [draft.requirePassword, passwordAlreadySet]);

  // Keep draft compliant when room floors turn on after dialog open.
  useEffect(() => {
    if (!verifyFloor && !ndaFloor) return;
    const patch: Partial<DraftLink> = {};
    if (verifyFloor && !draft.requireEmailVerification) {
      patch.requireEmailVerification = true;
      patch.requireEmail = false;
    }
    if (ndaFloor && !draft.requireNda) {
      patch.requireNda = true;
    }
    if (Object.keys(patch).length > 0) updateDraft(patch);
  }, [
    verifyFloor,
    ndaFloor,
    draft.requireEmailVerification,
    draft.requireNda,
    updateDraft,
  ]);

  const advancedCount = countAdvancedEnabled(draft, { countVisitorAsk: isDealRoomLink });
  /** Document links cannot use email verification; room floor locks it ON. */
  const verificationDisabledForDocuments = !isDealRoomLink;
  const emailSelfReportDisabled = verifyFloor;

  const handleRequireEmailChange = (checked: boolean) => {
    if (verifyFloor) return;
    updateDraft({
      requireEmail: checked,
      // Mutually exclusive with verification — identity is either self-reported or code-proven.
      requireEmailVerification: false,
    });
  };

  const handleRequireVerificationChange = (checked: boolean) => {
    // Room floor: never allow turning verification off.
    if (verifyFloor) return;
    updateDraft({
      requireEmailVerification: checked,
      // Mutually exclusive with email self-report; code resolves the visitor email.
      requireEmail: false,
    });
  };

  const handleAllowedViewersChange = (values: string[]) => {
    const filtered = showRoomLockedBlocks
      ? values.filter((v) => !roomBlockedSet.has(v.trim().toLowerCase()))
      : values;
    const patch: Partial<DraftLink> = { allowedViewers: filtered };
    if (filtered.length > 0 && !draft.requireEmail && !draft.requireEmailVerification) {
      patch.requireEmail = true;
    }
    updateDraft(patch);
  };

  const handleBlockedViewersChange = (values: string[]) => {
    const linkOnly = values.filter((v) => !roomBlockedSet.has(v.trim().toLowerCase()));
    updateDraft({
      blockedViewers: showRoomLockedBlocks
        ? [...roomBlockedEmails, ...linkOnly]
        : linkOnly,
    });
  };

  const blockedHint = (() => {
    if (audienceMode === "block-only") {
      return t("accessRules.blockedViewers.roomHint");
    }
    if (showRoomLockedBlocks) {
      return t("accessRules.blockedViewers.dealRoomLinkHint");
    }
    return t("accessRules.blockedViewers.hint");
  })();

  const handleRequireNdaChange = (checked: boolean) => {
    // Room floor: never allow turning NDA off.
    if (ndaFloor) return;
    updateDraft({
      requireNda: checked,
      ndaDocumentId: checked ? draft.ndaDocumentId : "",
      ndaTemplateId: checked ? draft.ndaTemplateId : "",
    });
  };

  // Templates first, then agreement-library docs not already covered by a template.
  // Never mix in deal-room / share-content docs — callers must pass agreement docs only.
  const ndaOptions = (() => {
    const fromTemplates = ndaTemplates.map((tpl) => ({
      id: tpl.id,
      title: tpl.name,
      templateId: tpl.id,
      documentId: tpl.sourceDocumentId,
    }));
    const coveredDocIds = new Set(
      fromTemplates.map((opt) => opt.documentId).filter(Boolean),
    );
    const fromDocs = documents
      .filter((doc) => !coveredDocIds.has(doc.id))
      .map((doc) => ({
        id: doc.id,
        title: doc.title,
        templateId: "",
        documentId: doc.id,
      }));
    return [...fromTemplates, ...fromDocs];
  })();

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

  const Section = sections ? SettingsSection : CompactSection;

  const authBody = (
    <>
      {!sections ? (
        <p className="text-xs text-muted-foreground">
          {t("accessRules.authentication.emailIdentityHint")}
        </p>
      ) : null}
      <div className="space-y-1 rounded-lg border border-border/70 bg-muted/20 p-3">
        <OptionSwitch
          label={t("accessRules.authentication.requireEmail")}
          description={
            emailSelfReportDisabled
              ? t("accessRules.authentication.verificationFloorLocked")
              : t("accessRules.authentication.requireEmailDescription")
          }
          checked={draft.requireEmail}
          onCheckedChange={handleRequireEmailChange}
          disabled={emailSelfReportDisabled}
          highlighted={isHighlighted("requireEmail")}
          testId="access-switch-require-email"
        />
        <OptionSwitch
          label={t("accessRules.authentication.requireVerification")}
          description={
            verificationDisabledForDocuments
              ? t("accessRules.authentication.verificationDisabledForDocuments")
              : verifyFloor
                ? t("accessRules.authentication.verificationFloorLocked")
                : t("accessRules.authentication.requireVerificationDescription")
          }
          checked={verifyFloor ? true : draft.requireEmailVerification}
          onCheckedChange={handleRequireVerificationChange}
          disabled={verificationDisabledForDocuments}
          locked={verifyFloor}
          highlighted={isHighlighted("requireEmailVerification")}
          testId="access-switch-require-verification"
        />
        {errors.requireVerificationContacts && (
          <p className="px-1 text-xs text-destructive">{errors.requireVerificationContacts}</p>
        )}
      </div>
      <div
        className={cn(
          "space-y-2 rounded-lg border border-border/70 p-3",
          isHighlighted("requirePassword") &&
            "bg-primary/10 motion-safe:transition-colors motion-safe:duration-200"
        )}
      >
        <OptionSwitch
          label={t("accessRules.authentication.requirePassword")}
          description={t("accessRules.authentication.requirePasswordDescription")}
          checked={draft.requirePassword}
          onCheckedChange={(checked) => updateDraft({ requirePassword: checked })}
        />
        {draft.requirePassword && (
          <div className="space-y-2 pt-1">
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
            {errors.password && <p className="text-xs text-destructive">{errors.password}</p>}
          </div>
        )}
      </div>
    </>
  );

  const viewersBody = (
    <>
      {showAllow ? (
        <div className="space-y-3">
          <div className="space-y-1">
            <Label className="text-sm font-medium">{t("accessRules.allowedViewers.title")}</Label>
            <p className="text-xs text-muted-foreground">{t("accessRules.allowedViewers.hint")}</p>
          </div>
          <ContactEmailTagInput
            values={draft.allowedViewers}
            onChange={handleAllowedViewersChange}
            placeholder={t("accessRules.allowedViewers.placeholder")}
            hint={undefined}
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
      ) : null}

      {showAllow && showBlock ? <div className="border-t border-border/60" /> : null}

      {showBlock ? (
        <div className="space-y-3">
          <div className="space-y-1">
            <Label className="text-sm font-medium">{t("accessRules.blockedViewers.title")}</Label>
            <p className="text-xs text-muted-foreground">
              {blockedHint}
            </p>
          </div>
          <ContactEmailTagInput
            values={draft.blockedViewers}
            onChange={handleBlockedViewersChange}
            placeholder={t("accessRules.blockedViewers.placeholder")}
            hint={undefined}
            conflictValues={conflicts}
            lockedValues={showRoomLockedBlocks ? roomBlockedEmails : undefined}
            allowDomains={false}
          />
          {errors.blockedViewers && (
            <p className="text-xs text-destructive">{errors.blockedViewers}</p>
          )}
        </div>
      ) : null}

      {showAllow && conflicts.length > 0 && (
        <p className="text-xs text-destructive">
          {t("accessRules.errors.conflict", { value: conflicts.join(", ") })}
        </p>
      )}
    </>
  );

  const protectionsBody = (
    <>
      <div className="space-y-1 rounded-lg border border-border/70 bg-muted/20 p-3">
        <OptionSwitch
          label={t("accessRules.additionalProtections.watermark")}
          description={t("accessRules.additionalProtections.watermarkDescription")}
          checked={draft.watermarkEnabled}
          onCheckedChange={(checked) => updateDraft({ watermarkEnabled: checked })}
          highlighted={isHighlighted("watermarkEnabled")}
        />
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

      <div
        className={cn(
          "space-y-3 rounded-lg border border-border/70 p-3",
          isHighlighted("requireNda") &&
            "bg-primary/10 motion-safe:transition-colors motion-safe:duration-200"
        )}
      >
        <OptionSwitch
          label={t("accessRules.additionalProtections.requireNda")}
          description={
            ndaFloor
              ? t("accessRules.additionalProtections.ndaFloorLocked")
              : t("accessRules.additionalProtections.requireNdaDescription")
          }
          checked={ndaFloor ? true : draft.requireNda}
          onCheckedChange={handleRequireNdaChange}
          locked={ndaFloor}
          highlighted={isHighlighted("requireNda")}
          testId="access-switch-require-nda"
        />
        {(ndaFloor || draft.requireNda) && (
          <div className="space-y-2">
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
                    : ndaTemplates.some((tpl) => tpl.id === selected)
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
              <SelectTrigger
                aria-label={t("accessRules.additionalProtections.ndaDocument")}
                className="w-full"
              >
                <SelectValue
                  placeholder={t("accessRules.additionalProtections.ndaDocumentPlaceholder")}
                />
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
    </>
  );

  const advancedBody = (
    <>
      {isDealRoomLink ? (
        <>
          <VisitorAskExperienceField
            value={draft.visitorAskExperience}
            onChange={(visitorAskExperience: VisitorAskExperience) =>
              updateDraft({ visitorAskExperience })
            }
            highlighted={isHighlighted("visitorAskExperience")}
          />
          {linkId ? (
            <LinkAskPolicyQuotaPanel
              linkId={linkId}
              experience={draft.visitorAskExperience}
            />
          ) : null}
        </>
      ) : null}
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
    </>
  );

  if (sections) {
    return (
      <div className="space-y-4" data-testid="access-tab-sections">
        <Section
          id="authentication"
          step="1"
          title={t("accessRules.authentication.title")}
        >
          {authBody}
        </Section>

        {showViewersSection ? (
          <Section
            id="viewers"
            step="2"
            title={
              audienceMode === "block-only"
                ? t("accessRules.blockedViewers.title")
                : t("accessRules.viewers.title")
            }
          >
            {viewersBody}
          </Section>
        ) : null}

        <Section
          id="protections"
          step={showViewersSection ? "3" : "2"}
          title={t("accessRules.additionalProtections.title")}
        >
          {protectionsBody}
        </Section>

        <Section
          id="advanced"
          step={showViewersSection ? "4" : "3"}
          title={t("accessRules.advanced.title")}
        >
          {advancedCount > 0 ? (
            <div className="mb-1 flex items-center gap-2">
              <Badge variant="secondary" className="text-xs">
                {t("accessRules.advanced.enabledCount", { count: advancedCount })}
              </Badge>
            </div>
          ) : null}
          <div className="space-y-3">{advancedBody}</div>
        </Section>

        {errors.submit && <p className="text-xs text-destructive">{errors.submit}</p>}
      </div>
    );
  }

  return (
    <div className="space-y-6 py-2" data-testid="access-tab-compact">
      <Section
        id="authentication"
        title={t("accessRules.authentication.title")}
        description={undefined}
      >
        {authBody}
      </Section>

      {showAllow ? (
        <Section id="allowed" title={t("accessRules.allowedViewers.title")}>
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
        </Section>
      ) : null}

      {showBlock ? (
        <Section id="blocked" title={t("accessRules.blockedViewers.title")}>
          <ContactEmailTagInput
            values={draft.blockedViewers}
            onChange={handleBlockedViewersChange}
            placeholder={t("accessRules.blockedViewers.placeholder")}
            hint={blockedHint}
            conflictValues={conflicts}
            lockedValues={showRoomLockedBlocks ? roomBlockedEmails : undefined}
            allowDomains={false}
          />
          {errors.blockedViewers && (
            <p className="text-xs text-destructive">{errors.blockedViewers}</p>
          )}
        </Section>
      ) : null}

      {showAllow && conflicts.length > 0 && (
        <p className="text-xs text-destructive">
          {t("accessRules.errors.conflict", { value: conflicts.join(", ") })}
        </p>
      )}

      <Section id="protections" title={t("accessRules.additionalProtections.title")}>
        {protectionsBody}
      </Section>

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
        {advancedBody}
      </CollapsibleSection>

      {errors.submit && <p className="text-xs text-destructive">{errors.submit}</p>}
    </div>
  );
}

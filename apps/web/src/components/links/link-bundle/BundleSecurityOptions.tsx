import { useMemo, useState, type ComponentType, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  EnvelopeIcon,
  ScrollIcon,
  DownloadIcon,
  DropIcon,
  ClockIcon,
  EyeIcon,
  CaretDownIcon,
} from "@phosphor-icons/react";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import type { PermissionConfig } from "@/types";
import { useNdaPickerSources } from "@/components/links/share/hooks";
import { useSecurityOptions } from "../smart-link/useSecurityOptions";

interface NdaOption {
  id: string;
  title: string;
  templateId: string;
  documentId: string;
}

interface BundleSecurityOptionsProps {
  config: PermissionConfig;
  onChange: (config: PermissionConfig) => void;
  contactSelector?: ReactNode;
  /** Shared content docs — excluded from NDA agreement picker. */
  excludeNdaDocumentIds?: string[];
}

interface OptionRowProps {
  icon: ComponentType<{ size?: number; className?: string }>;
  label: string;
  checked: boolean;
  disabled?: boolean;
  onCheckedChange: (checked: boolean) => void;
  "data-testid"?: string;
  children?: ReactNode;
}

function OptionRow({
  icon: Icon,
  label,
  checked,
  disabled,
  onCheckedChange,
  "data-testid": testId,
  children,
}: OptionRowProps) {
  return (
    <div className="px-1 py-2">
      {/* Icon stays on the title row so expanded children do not pull it down. */}
      <div className="flex items-center gap-3">
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-foreground text-background">
          <Icon size={16} />
        </div>
        <p className="min-w-0 flex-1 text-[13.5px] font-medium tracking-[-0.01em] text-foreground">
          {label}
        </p>
        <Switch
          checked={checked}
          disabled={disabled}
          onCheckedChange={onCheckedChange}
          className="shrink-0"
          data-testid={testId}
        />
      </div>
      {children ? (
        <div className="mt-2.5 animate-in fade-in-0 slide-in-from-top-1 duration-200 pl-11">
          {children}
        </div>
      ) : null}
    </div>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <section className="space-y-1">
      <h3 className="px-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/75">
        {title}
      </h3>
      <div className="divide-y divide-border/40">{children}</div>
    </section>
  );
}

export function BundleSecurityOptions({
  config,
  onChange,
  contactSelector,
  excludeNdaDocumentIds = [],
}: BundleSecurityOptionsProps) {
  const { t } = useTranslation("links");
  const { t: tShare } = useTranslation("linkShare");
  const [advancedOpen, setAdvancedOpen] = useState(true);
  // Templates + agreement-library docs (协议页); shared with AccessTab loaders.
  const { ndaTemplates, agreementDocs: ndaDocuments } = useNdaPickerSources();

  const { update } = useSecurityOptions(config, onChange);

  const excludeSet = useMemo(
    () => new Set(excludeNdaDocumentIds),
    [excludeNdaDocumentIds],
  );

  const ndaOptions = useMemo((): NdaOption[] => {
    const fromTemplates = ndaTemplates.map((tpl) => ({
      id: tpl.id,
      title: tpl.name,
      templateId: tpl.id,
      documentId: tpl.sourceDocumentId,
    }));
    const coveredDocIds = new Set(
      fromTemplates.map((opt) => opt.documentId).filter(Boolean),
    );
    const fromDocs = ndaDocuments
      .filter((doc) => !excludeSet.has(doc.id) && !coveredDocIds.has(doc.id))
      .map((doc) => ({
        id: doc.id,
        title: doc.title,
        templateId: "",
        documentId: doc.id,
      }));
    return [...fromTemplates, ...fromDocs];
  }, [ndaTemplates, ndaDocuments, excludeSet]);

  const selectedNdaValue = useMemo(() => {
    if (config.ndaTemplateId) {
      const byTpl = ndaOptions.find(
        (o) => o.templateId === config.ndaTemplateId || o.id === config.ndaTemplateId,
      );
      if (byTpl) return byTpl.id;
    }
    if (config.ndaDocumentId) {
      const byDoc = ndaOptions.find(
        (o) => o.documentId === config.ndaDocumentId || o.id === config.ndaDocumentId,
      );
      if (byDoc) return byDoc.id;
    }
    return null;
  }, [config.ndaTemplateId, config.ndaDocumentId, ndaOptions]);

  const ndaSelectionMissing =
    config.ndaEnabled && !config.ndaTemplateId && !config.ndaDocumentId;

  return (
    <div className="space-y-4">
      <Section title={t("creator.sectionAccessControl")}>
        <OptionRow
          icon={EnvelopeIcon}
          label={t("creator.requireEmailVerification")}
          // NDA forces email verification (same as Create link / toCreateLinkPayload).
          checked={config.requireEmailVerification || config.ndaEnabled}
          disabled={config.ndaEnabled}
          onCheckedChange={(checked) =>
            update({
              requireEmailVerification: checked,
              contactIds: checked ? config.contactIds : [],
            })
          }
          data-testid="security-switch-requireEmailVerification"
        >
          {config.requireEmailVerification || config.ndaEnabled
            ? contactSelector
            : null}
        </OptionRow>
      </Section>

      <Section title={t("creator.sectionContentProtection")}>
        <OptionRow
          icon={ScrollIcon}
          label={tShare("accessRules.additionalProtections.requireNda")}
          checked={config.ndaEnabled}
          onCheckedChange={(checked) =>
            update({
              ndaEnabled: checked,
              ndaDocumentId: checked ? config.ndaDocumentId : "",
              ndaTemplateId: checked ? config.ndaTemplateId : "",
            })
          }
          data-testid="security-switch-ndaEnabled"
        >
          {config.ndaEnabled ? (
            <div className="space-y-2">
              <Label className="text-[11px] font-medium tracking-wide text-muted-foreground">
                {tShare("accessRules.additionalProtections.ndaDocument")}
              </Label>
              <Select
                value={selectedNdaValue}
                onValueChange={(value) => {
                  const selected = value ?? "";
                  if (!selected || selected === "__empty__") return;
                  const opt = ndaOptions.find(
                    (o) =>
                      o.id === selected ||
                      o.templateId === selected ||
                      o.documentId === selected,
                  );
                  const nextTemplateId =
                    opt?.templateId && opt.templateId.length > 0
                      ? opt.templateId
                      : ndaTemplates.some((tpl) => tpl.id === selected)
                        ? selected
                        : "";
                  const nextDocumentId =
                    opt?.documentId && opt.documentId.length > 0
                      ? opt.documentId
                      : selected;
                  update({
                    ndaTemplateId: nextTemplateId,
                    ndaDocumentId: nextDocumentId,
                  });
                }}
              >
                <SelectTrigger
                  aria-label={tShare("accessRules.additionalProtections.ndaDocument")}
                  className="h-9 w-full border-border/60 bg-transparent shadow-none"
                  data-testid="security-nda-document-select"
                >
                  <SelectValue
                    placeholder={tShare(
                      "accessRules.additionalProtections.ndaDocumentPlaceholder",
                    )}
                  />
                </SelectTrigger>
                <SelectContent>
                  {ndaOptions.length === 0 ? (
                    <SelectItem value="__empty__" disabled>
                      {tShare(
                        "accessRules.additionalProtections.ndaDocumentPlaceholder",
                      )}
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
              {ndaSelectionMissing ? (
                <p className="text-xs text-destructive" data-testid="security-nda-document-error">
                  {tShare("accessRules.errors.ndaDocumentRequired")}
                </p>
              ) : null}
            </div>
          ) : null}
        </OptionRow>

        <OptionRow
          icon={DownloadIcon}
          label={t("creator.allowDownload")}
          checked={config.allowDownload}
          onCheckedChange={(checked) => update({ allowDownload: checked })}
          data-testid="security-switch-allowDownload"
        />
        <OptionRow
          icon={DropIcon}
          label={t("creator.watermark")}
          checked={config.watermarkEnabled}
          onCheckedChange={(checked) => update({ watermarkEnabled: checked })}
          data-testid="security-switch-watermarkEnabled"
        />
      </Section>

      <section className="space-y-1.5">
        <button
          type="button"
          data-testid="security-advanced-toggle"
          aria-expanded={advancedOpen}
          onClick={() => setAdvancedOpen((v) => !v)}
          className="flex w-full items-center gap-1.5 px-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/75 transition-colors hover:text-foreground"
        >
          <CaretDownIcon
            size={13}
            className={cn(
              "shrink-0 transition-transform duration-200",
              advancedOpen ? "rotate-0" : "-rotate-90",
            )}
          />
          {t("creator.sectionAdvanced")}
        </button>
        {advancedOpen ? (
          <div className="grid animate-in fade-in-0 slide-in-from-top-1 duration-200 grid-cols-1 gap-3 px-1 sm:grid-cols-2">
            <div className="space-y-1.5">
              <label className="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
                <ClockIcon size={14} />
                {t("creator.expiry")}
              </label>
              <Select
                value={String(config.expiryDays)}
                onValueChange={(value) =>
                  update({
                    expiryDays: value === "custom" ? "custom" : Number(value),
                  })
                }
              >
                <SelectTrigger
                  data-testid="security-expiry-select"
                  className="h-9 border-border/60 bg-transparent shadow-none"
                >
                  <SelectValue placeholder={t("creator.expiryPlaceholder")} />
                </SelectTrigger>
                <SelectContent side="bottom" alignItemWithTrigger={false}>
                  <SelectItem value="7">{t("creator.expiryDays.7")}</SelectItem>
                  <SelectItem value="30">{t("creator.expiryDays.30")}</SelectItem>
                  <SelectItem value="90">{t("creator.expiryDays.90")}</SelectItem>
                  <SelectItem value="custom">
                    {t("creator.expiryDays.custom")}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <label className="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
                <EyeIcon size={14} />
                {t("creator.maxViews")}
              </label>
              <Select
                value={String(config.maxViews)}
                onValueChange={(value) =>
                  update({
                    maxViews: value === "unlimited" ? "unlimited" : Number(value),
                  })
                }
              >
                <SelectTrigger
                  data-testid="security-max-views-select"
                  className="h-9 border-border/60 bg-transparent shadow-none"
                >
                  <SelectValue placeholder={t("creator.maxViewsPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="unlimited">
                    {t("creator.maxViewsOptions.unlimited")}
                  </SelectItem>
                  <SelectItem value="10">
                    {t("creator.maxViewsOptions.10")}
                  </SelectItem>
                  <SelectItem value="50">
                    {t("creator.maxViewsOptions.50")}
                  </SelectItem>
                  <SelectItem value="100">
                    {t("creator.maxViewsOptions.100")}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        ) : null}
      </section>
    </div>
  );
}

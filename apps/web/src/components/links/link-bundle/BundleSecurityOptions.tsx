import {
  createContext,
  useContext,
  useMemo,
  useState,
  type ComponentType,
  type ReactNode,
} from "react";
import { useTranslation } from "react-i18next";
import {
  EnvelopeIcon,
  ScrollIcon,
  DownloadIcon,
  DropIcon,
  ClockIcon,
  EyeIcon,
  CaretDownIcon,
  type IconWeight,
} from "@phosphor-icons/react";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
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
import { toDateTimeLocal, toRFC3339 } from "@/components/links/share/utils";
import { useSecurityOptions } from "../smart-link/useSecurityOptions";
import {
  LINK_CUSTOM_MAX_VIEWS_DEFAULT,
} from "./pipelineUtils";

interface NdaOption {
  id: string;
  title: string;
  templateId: string;
  documentId: string;
}

export type SecuritySurface = "default" | "atelier";

const SecuritySurfaceContext = createContext<SecuritySurface>("default");

interface BundleSecurityOptionsProps {
  config: PermissionConfig;
  onChange: (config: PermissionConfig) => void;
  contactSelector?: ReactNode;
  /** Shared content docs — excluded from NDA agreement picker. */
  excludeNdaDocumentIds?: string[];
  /** Visual density. `atelier` is the post-upload share dossier. */
  variant?: SecuritySurface;
  /** When false, expiry / max-views stay visible (no accordion). */
  advancedCollapsible?: boolean;
}

interface OptionRowProps {
  icon: ComponentType<{ size?: number; className?: string; weight?: IconWeight }>;
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
  const surface = useContext(SecuritySurfaceContext);
  const atelier = surface === "atelier";
  return (
    <div className={cn(atelier ? "px-0 py-2.5" : "px-1 py-2")}>
      <div className="flex items-center gap-3">
        <div
          className={cn(
            "flex size-8 shrink-0 items-center justify-center",
            atelier
              ? "rounded-md text-muted-foreground ring-1 ring-foreground/[0.08]"
              : "rounded-lg bg-foreground text-background",
          )}
        >
          <Icon size={16} weight={atelier ? "light" : "regular"} />
        </div>
        <p
          className={cn(
            "min-w-0 flex-1 font-medium tracking-[-0.01em] text-foreground",
            atelier ? "text-[13px]" : "text-[13.5px]",
          )}
        >
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
  index,
  title,
  children,
}: {
  index?: string;
  title: string;
  children: ReactNode;
}) {
  const surface = useContext(SecuritySurfaceContext);
  const atelier = surface === "atelier";
  return (
    <section className={atelier ? "space-y-1.5" : "space-y-1"}>
      <h3
        className={cn(
          "flex items-baseline gap-2.5",
          atelier ? "px-0" : "px-1",
        )}
      >
        {atelier && index ? (
          <span className="font-mono text-[10px] tracking-[0.16em] text-muted-foreground/45">
            {index}
          </span>
        ) : null}
        <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/75">
          {title}
        </span>
      </h3>
      <div className={atelier ? "divide-y divide-foreground/[0.06]" : "divide-y divide-border/40"}>
        {children}
      </div>
    </section>
  );
}

export function BundleSecurityOptions({
  config,
  onChange,
  contactSelector,
  excludeNdaDocumentIds = [],
  variant = "default",
  advancedCollapsible = true,
}: BundleSecurityOptionsProps) {
  const { t } = useTranslation("links");
  const { t: tShare } = useTranslation("linkShare");
  const atelier = variant === "atelier";
  const [advancedOpen, setAdvancedOpen] = useState(true);
  const showAdvanced = !advancedCollapsible || advancedOpen;
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
    <SecuritySurfaceContext.Provider value={variant}>
      <div className={cn(atelier ? "space-y-5" : "space-y-4")}>
      <Section index="01" title={t("creator.sectionAccessControl")}>
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

      <Section index="02" title={t("creator.sectionContentProtection")}>
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

      <section className={atelier ? "space-y-2.5" : "space-y-1.5"}>
        {advancedCollapsible ? (
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
        ) : (
          <h3 className="flex items-baseline gap-2.5">
            {atelier ? (
              <span className="font-mono text-[10px] tracking-[0.16em] text-muted-foreground/45">
                03
              </span>
            ) : null}
            <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/75">
              {t("creator.sectionAdvanced")}
            </span>
          </h3>
        )}
        {showAdvanced ? (
          <div
            className={cn(
              "grid grid-cols-1 gap-3 sm:grid-cols-2",
              !atelier && "animate-in fade-in-0 slide-in-from-top-1 duration-200 px-1",
            )}
          >
            <div
              className={cn(
                "space-y-1.5",
                atelier &&
                  "rounded-xl bg-muted/35 px-3 py-2.5 ring-1 ring-foreground/[0.05]",
              )}
            >
              <label className="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
                <ClockIcon size={14} weight={atelier ? "light" : "regular"} />
                {t("creator.expiry")}
              </label>
              <Select
                value={String(config.expiryDays)}
                onValueChange={(value) => {
                  if (value === "custom") {
                    const d = new Date();
                    d.setDate(d.getDate() + 7);
                    update({
                      expiryDays: "custom",
                      _editExpiresAt: config._editExpiresAt ?? d.toISOString(),
                    });
                    return;
                  }
                  update({
                    expiryDays: Number(value),
                    _editExpiresAt: undefined,
                  });
                }}
              >
                <SelectTrigger
                  data-testid="security-expiry-select"
                  className="h-9 border-border/60 bg-transparent shadow-none"
                >
                  <SelectValue placeholder={t("creator.expiryPlaceholder")} />
                </SelectTrigger>
                <SelectContent side="bottom" alignItemWithTrigger={false}>
                  <SelectItem value="7">{t("creator.expiryDays.7")}</SelectItem>
                  <SelectItem value="15">{t("creator.expiryDays.15")}</SelectItem>
                  <SelectItem value="30">{t("creator.expiryDays.30")}</SelectItem>
                  <SelectItem value="custom">
                    {t("creator.expiryDays.custom")}
                  </SelectItem>
                </SelectContent>
              </Select>
              {config.expiryDays === "custom" ? (
                <Input
                  type="datetime-local"
                  data-testid="security-expiry-custom-datetime"
                  aria-label={t("creator.customExpiresAt")}
                  className="h-9 border-border/60 bg-transparent shadow-none"
                  value={toDateTimeLocal(config._editExpiresAt)}
                  min={toDateTimeLocal(new Date().toISOString())}
                  onChange={(e) => {
                    const rfc = toRFC3339(e.target.value);
                    update({
                      expiryDays: "custom",
                      _editExpiresAt: rfc || undefined,
                    });
                  }}
                />
              ) : null}
            </div>
            <div
              className={cn(
                "space-y-1.5",
                atelier &&
                  "rounded-xl bg-muted/35 px-3 py-2.5 ring-1 ring-foreground/[0.05]",
              )}
            >
              <label className="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
                <EyeIcon size={14} weight={atelier ? "light" : "regular"} />
                {t("creator.maxViews")}
              </label>
              <Select
                value={config.maxViews === "custom" ? "custom" : String(config.maxViews)}
                onValueChange={(value) => {
                  if (value === "custom") {
                    const fallback =
                      typeof config.maxViews === "number"
                        ? config.maxViews
                        : config._editMaxViews;
                    update({
                      maxViews: "custom",
                      _editMaxViews: fallback ?? LINK_CUSTOM_MAX_VIEWS_DEFAULT,
                    });
                    return;
                  }
                  if (value === "unlimited") {
                    update({
                      maxViews: "unlimited",
                      _editMaxViews: undefined,
                    });
                    return;
                  }
                  update({
                    maxViews: Number(value),
                    _editMaxViews: undefined,
                  });
                }}
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
                  <SelectItem value="custom">
                    {t("creator.maxViewsOptions.custom")}
                  </SelectItem>
                </SelectContent>
              </Select>
              {config.maxViews === "custom" ? (
                <Input
                  type="text"
                  inputMode="numeric"
                  pattern="[0-9]*"
                  data-testid="security-max-views-custom-input"
                  aria-label={t("creator.customMaxViews")}
                  className="h-9 border-border/60 bg-transparent shadow-none"
                  value={config._editMaxViews ?? ""}
                  onChange={(e) => {
                    const raw = e.target.value.trim();
                    if (raw === "") {
                      update({
                        maxViews: "custom",
                        _editMaxViews: undefined,
                      });
                      return;
                    }
                    if (!/^\d+$/.test(raw)) return;
                    const n = Number.parseInt(raw, 10);
                    if (!Number.isFinite(n)) return;
                    update({
                      maxViews: "custom",
                      _editMaxViews: n,
                    });
                  }}
                />
              ) : null}
            </div>
          </div>
        ) : null}
      </section>
      </div>
    </SecuritySurfaceContext.Provider>
  );
}

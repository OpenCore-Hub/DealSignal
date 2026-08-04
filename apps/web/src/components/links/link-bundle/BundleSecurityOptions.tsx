import { useEffect, useMemo, useState } from "react";
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
import { api } from "@/lib/api";
import type { PermissionConfig } from "@/types";
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
  contactSelector?: React.ReactNode;
  /** Shared content docs — excluded from NDA agreement picker. */
  excludeNdaDocumentIds?: string[];
}

interface OptionRowProps {
  icon: React.ComponentType<{ size?: number; className?: string }>;
  label: string;
  description: string;
  checked: boolean;
  disabled?: boolean;
  onCheckedChange: (checked: boolean) => void;
  "data-testid"?: string;
}

function OptionRow({
  icon: Icon,
  label,
  description,
  checked,
  disabled,
  onCheckedChange,
  "data-testid": testId,
}: OptionRowProps) {
  return (
    <div className="group flex items-center gap-3 rounded-lg px-3 py-2.5 transition-colors hover:bg-muted/50">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted/70 text-muted-foreground transition-colors group-hover:bg-muted group-hover:text-foreground">
        <Icon size={18} />
      </div>
      <div className="min-w-0 flex-1">
        <span className="text-sm font-medium">{label}</span>
        <p className="text-xs leading-relaxed text-muted-foreground">
          {description}
        </p>
      </div>
      <Switch
        checked={checked}
        disabled={disabled}
        onCheckedChange={onCheckedChange}
        className="shrink-0"
        data-testid={testId}
      />
    </div>
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
  const [ndaTemplates, setNdaTemplates] = useState<
    { id: string; name: string; sourceDocumentId: string }[]
  >([]);
  const [ndaDocuments, setNdaDocuments] = useState<{ id: string; title: string }[]>([]);

  const { update } = useSecurityOptions(config, onChange);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const [tplRes, docRes] = await Promise.all([
          api.listNDATemplates(),
          api.getDocuments(),
        ]);
        if (cancelled) return;
        setNdaTemplates(
          (tplRes.data ?? []).map((tpl) => ({
            id: tpl.id,
            name: tpl.name,
            sourceDocumentId: tpl.source_document_id,
          })),
        );
        setNdaDocuments(
          (docRes.data ?? []).map((doc) => ({
            id: doc.id,
            title: doc.title,
          })),
        );
      } catch {
        if (!cancelled) {
          setNdaTemplates([]);
          setNdaDocuments([]);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const excludeSet = useMemo(
    () => new Set(excludeNdaDocumentIds),
    [excludeNdaDocumentIds],
  );

  const ndaOptions = useMemo((): NdaOption[] => {
    if (ndaTemplates.length > 0) {
      return ndaTemplates.map((tpl) => ({
        id: tpl.id,
        title: tpl.name,
        templateId: tpl.id,
        documentId: tpl.sourceDocumentId,
      }));
    }
    return ndaDocuments
      .filter((doc) => !excludeSet.has(doc.id))
      .map((doc) => ({
        id: doc.id,
        title: doc.title,
        templateId: "",
        documentId: doc.id,
      }));
  }, [ndaTemplates, ndaDocuments, excludeSet]);

  const selectedNdaValue = config.ndaTemplateId || config.ndaDocumentId || "";
  const ndaSelectionMissing =
    config.ndaEnabled && !config.ndaTemplateId && !config.ndaDocumentId;

  return (
    <div className="space-y-6">
      {/* ── 访问控制 ── */}
      <section>
        <h3 className="mb-1 px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {t("creator.sectionAccessControl")}
        </h3>
        <div className="divide-y divide-border/50 rounded-lg border border-border/50 bg-card/50">
          <OptionRow
            icon={EnvelopeIcon}
            label={t("creator.requireEmailVerification")}
            description={t("creator.requireEmailDesc")}
            checked={config.requireEmailVerification}
            disabled={config.ndaEnabled}
            onCheckedChange={(checked) =>
              update({
                requireEmailVerification: checked,
                contactIds: checked ? config.contactIds : [],
              })
            }
            data-testid="security-switch-requireEmailVerification"
          />
          {config.requireEmailVerification && contactSelector}
        </div>
      </section>

      {/* ── 内容保护 ── */}
      <section>
        <h3 className="mb-1 px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {t("creator.sectionContentProtection")}
        </h3>
        <div className="divide-y divide-border/50 rounded-lg border border-border/50 bg-card/50">
          <div>
            <OptionRow
              icon={ScrollIcon}
              label={tShare("accessRules.additionalProtections.requireNda")}
              description={tShare("accessRules.additionalProtections.requireNdaDescription")}
              checked={config.ndaEnabled}
              onCheckedChange={(checked) =>
                update({
                  ndaEnabled: checked,
                  ndaDocumentId: checked ? config.ndaDocumentId : "",
                  ndaTemplateId: checked ? config.ndaTemplateId : "",
                })
              }
              data-testid="security-switch-ndaEnabled"
            />
            {config.ndaEnabled && (
              <div className="space-y-2 px-3 pb-3 pl-[3.75rem]">
                <Label className="text-xs font-normal text-muted-foreground">
                  {tShare("accessRules.additionalProtections.ndaDocument")}
                </Label>
                <Select
                  value={selectedNdaValue || null}
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
                    className="w-full"
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
            )}
          </div>
          <OptionRow
            icon={DownloadIcon}
            label={t("creator.allowDownload")}
            description={t("creator.allowDownloadDesc")}
            checked={config.allowDownload}
            onCheckedChange={(checked) =>
              update({ allowDownload: checked })
            }
            data-testid="security-switch-allowDownload"
          />
          <OptionRow
            icon={DropIcon}
            label={t("creator.watermark")}
            description={t("creator.watermarkDesc")}
            checked={config.watermarkEnabled}
            onCheckedChange={(checked) =>
              update({ watermarkEnabled: checked })
            }
            data-testid="security-switch-watermarkEnabled"
          />

        </div>
      </section>

      {/* ── 高级设置 ── */}
      <section>
        <button
          type="button"
          data-testid="security-advanced-toggle"
          aria-expanded={advancedOpen}
          onClick={() => setAdvancedOpen((v) => !v)}
          className="flex w-full items-center gap-1.5 px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground transition-colors hover:text-foreground"
        >
          <CaretDownIcon
            size={14}
            className={`shrink-0 transition-transform ${advancedOpen ? "" : "-rotate-90"}`}
          />
          {t("creator.sectionAdvanced")}
        </button>
        {advancedOpen && (
          <div className="mt-3 grid grid-cols-1 gap-4 rounded-lg border border-border/50 bg-card/50 p-4 sm:grid-cols-2">
            <div className="space-y-2">
              <label className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground">
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
                <SelectTrigger data-testid="security-expiry-select">
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
            <div className="space-y-2">
              <label className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground">
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
                <SelectTrigger data-testid="security-max-views-select">
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
        )}
      </section>
    </div>
  );
}

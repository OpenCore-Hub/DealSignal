import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ndaPickerDisplayTitle, useNdaPickerSources } from "@/components/links/share";

export type RoomNdaSelection = {
  ndaTemplateId: string;
  ndaDocumentId: string;
};

interface RoomNdaAgreementPickerProps {
  value: RoomNdaSelection;
  onChange: (next: RoomNdaSelection) => void;
  disabled?: boolean;
  showError?: boolean;
}

/** Room-level NDA picker: one agreement for every invited member. */
export function RoomNdaAgreementPicker({
  value,
  onChange,
  disabled,
  showError,
}: RoomNdaAgreementPickerProps) {
  const { t } = useTranslation("dealRooms");
  const { ndaTemplates, agreementDocs, loading } = useNdaPickerSources();

  const ndaOptions = useMemo(() => {
    const fromTemplates = ndaTemplates.map((tpl) => ({
      id: tpl.id,
      title: tpl.name,
      templateId: tpl.id,
      documentId: tpl.sourceDocumentId,
    }));
    const coveredDocIds = new Set(
      fromTemplates.map((opt) => opt.documentId).filter(Boolean),
    );
    const fromDocs = agreementDocs
      .filter((doc) => !coveredDocIds.has(doc.id))
      .map((doc) => ({
        id: doc.id,
        title: doc.title,
        templateId: "",
        documentId: doc.id,
      }));
    return [...fromTemplates, ...fromDocs];
  }, [ndaTemplates, agreementDocs]);

  const selectedValue = useMemo(() => {
    if (value.ndaTemplateId) {
      const byTpl = ndaOptions.find(
        (o) => o.templateId === value.ndaTemplateId || o.id === value.ndaTemplateId,
      );
      if (byTpl) return byTpl.id;
      return value.ndaTemplateId;
    }
    if (value.ndaDocumentId) {
      const byDoc = ndaOptions.find(
        (o) => o.documentId === value.ndaDocumentId || o.id === value.ndaDocumentId,
      );
      if (byDoc) return byDoc.id;
      return value.ndaDocumentId;
    }
    return null;
  }, [value.ndaTemplateId, value.ndaDocumentId, ndaOptions]);

  const selectedLabel = useMemo(() => {
    const opt = ndaOptions.find((o) => o.id === selectedValue);
    return ndaPickerDisplayTitle(opt?.title, t("members.ndaAgreementUntitled"));
  }, [ndaOptions, selectedValue, t]);

  const missing = !value.ndaTemplateId && !value.ndaDocumentId;

  return (
    <div className="space-y-2">
      <Label htmlFor="room-nda-agreement">{t("members.ndaAgreement")}</Label>
      <p className="text-[12px] leading-snug text-muted-foreground">
        {t("members.ndaAgreementHint")}
      </p>
      <Select
        value={selectedValue}
        disabled={disabled || loading}
        onValueChange={(next) => {
          const selected = next ?? "";
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
          onChange({
            ndaTemplateId: nextTemplateId,
            ndaDocumentId: nextDocumentId,
          });
        }}
      >
        <SelectTrigger
          id="room-nda-agreement"
          aria-label={t("members.ndaAgreement")}
          className="h-9 w-full"
          data-testid="room-nda-agreement-select"
        >
          <SelectValue placeholder={t("members.ndaAgreementPlaceholder")}>
            {selectedValue ? selectedLabel : null}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {ndaOptions.length === 0 ? (
            <SelectItem value="__empty__" disabled label={t("members.ndaAgreementEmpty")}>
              {t("members.ndaAgreementEmpty")}
            </SelectItem>
          ) : (
            ndaOptions.map((opt) => {
              const title = ndaPickerDisplayTitle(opt.title, t("members.ndaAgreementUntitled"));
              return (
                <SelectItem key={opt.id} value={opt.id} label={title}>
                  {title}
                </SelectItem>
              );
            })
          )}
        </SelectContent>
      </Select>
      {showError && missing ? (
        <p className="text-xs text-destructive" data-testid="room-nda-agreement-error">
          {t("members.ndaAgreementRequired")}
        </p>
      ) : null}
    </div>
  );
}

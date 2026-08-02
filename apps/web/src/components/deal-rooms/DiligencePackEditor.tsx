import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import type { DDCoveragePackItem } from "@/types";

interface DiligencePackEditorProps {
  roomId: string;
  onPackChanged?: () => void;
}

function emptyItem(): DDCoveragePackItem {
  return {
    id: "",
    label_en: "",
    label_zh: "",
    query_en: "",
    query_zh: "",
    value_type: "",
  };
}

export function DiligencePackEditor({ roomId, onPackChanged }: DiligencePackEditorProps) {
  const { t } = useTranslation("dealRooms");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [forked, setForked] = useState(false);
  const [version, setVersion] = useState("");
  const [items, setItems] = useState<DDCoveragePackItem[]>([]);
  const [open, setOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const pack = await api.getDDCoveragePack(roomId);
      setForked(pack.forked);
      setVersion(pack.pack_version);
      setItems(pack.items.map((it) => ({ ...it, value_type: it.value_type ?? "" })));
    } catch (e) {
      if (e instanceof ApiError && e.code === "dd_coverage_disabled") {
        return;
      }
      toast.error(e instanceof Error ? e.message : t("diligence.packLoadFailed"));
    } finally {
      setLoading(false);
    }
  }, [roomId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const updateItem = (index: number, patch: Partial<DDCoveragePackItem>) => {
    setItems((prev) => prev.map((it, i) => (i === index ? { ...it, ...patch } : it)));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const pack = await api.putDDCoveragePack(
        roomId,
        items.map((it) => ({
          ...it,
          value_type: it.value_type || undefined,
        })),
      );
      setForked(pack.forked);
      setVersion(pack.pack_version);
      setItems(pack.items.map((it) => ({ ...it, value_type: it.value_type ?? "" })));
      toast.success(t("diligence.packSaved"));
      onPackChanged?.();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("diligence.packSaveFailed"));
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    setSaving(true);
    try {
      const pack = await api.resetDDCoveragePack(roomId);
      setForked(pack.forked);
      setVersion(pack.pack_version);
      setItems(pack.items.map((it) => ({ ...it, value_type: it.value_type ?? "" })));
      toast.success(t("diligence.packReset"));
      onPackChanged?.();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("diligence.packResetFailed"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card data-testid="diligence-pack-editor">
      <CardHeader className="pb-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <CardTitle className="text-base">{t("diligence.packTitle")}</CardTitle>
            <CardDescription>{t("diligence.packDescription")}</CardDescription>
          </div>
          <div className="flex items-center gap-2">
            {forked ? (
              <Badge variant="secondary" data-testid="diligence-pack-forked">
                {t("diligence.packForked", { version })}
              </Badge>
            ) : (
              <Badge variant="outline">{t("diligence.packBuiltin", { version })}</Badge>
            )}
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setOpen((v) => !v)}
              data-testid="diligence-pack-toggle"
            >
              {open ? t("diligence.packHide") : t("diligence.packEdit")}
            </Button>
          </div>
        </div>
      </CardHeader>
      {open ? (
        <CardContent className="space-y-3">
          {loading ? (
            <p className="text-sm text-muted-foreground">{t("diligence.loading")}</p>
          ) : (
            <>
              <div className="space-y-3">
                {items.map((item, index) => (
                  <div
                    key={`${item.id}-${index}`}
                    className="grid gap-2 rounded-md border border-border p-3"
                    data-testid={`diligence-pack-item-${index}`}
                  >
                    <div className="grid gap-2 sm:grid-cols-2">
                      <Input
                        value={item.id}
                        onChange={(e) => updateItem(index, { id: e.target.value })}
                        placeholder={t("diligence.packItemId")}
                        aria-label={t("diligence.packItemId")}
                      />
                      <Select
                        value={item.value_type || "none"}
                        onValueChange={(value) =>
                          updateItem(index, {
                            value_type: !value || value === "none" ? "" : value,
                          })
                        }
                      >
                        <SelectTrigger aria-label={t("diligence.packValueType")}>
                          <SelectValue placeholder={t("diligence.packValueType")} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="none">{t("diligence.packValueTypeNone")}</SelectItem>
                          <SelectItem value="percent">{t("diligence.valueType.percent")}</SelectItem>
                          <SelectItem value="money">{t("diligence.valueType.money")}</SelectItem>
                          <SelectItem value="share">{t("diligence.valueType.share")}</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="grid gap-2 sm:grid-cols-2">
                      <Input
                        value={item.label_en}
                        onChange={(e) => updateItem(index, { label_en: e.target.value })}
                        placeholder={t("diligence.packLabelEn")}
                        aria-label={t("diligence.packLabelEn")}
                      />
                      <Input
                        value={item.label_zh}
                        onChange={(e) => updateItem(index, { label_zh: e.target.value })}
                        placeholder={t("diligence.packLabelZh")}
                        aria-label={t("diligence.packLabelZh")}
                      />
                    </div>
                    <div className="grid gap-2 sm:grid-cols-2">
                      <Input
                        value={item.query_en}
                        onChange={(e) => updateItem(index, { query_en: e.target.value })}
                        placeholder={t("diligence.packQueryEn")}
                        aria-label={t("diligence.packQueryEn")}
                      />
                      <Input
                        value={item.query_zh}
                        onChange={(e) => updateItem(index, { query_zh: e.target.value })}
                        placeholder={t("diligence.packQueryZh")}
                        aria-label={t("diligence.packQueryZh")}
                      />
                    </div>
                    <div className="flex justify-end">
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        disabled={items.length <= 1}
                        onClick={() => setItems((prev) => prev.filter((_, i) => i !== index))}
                      >
                        {t("diligence.packRemoveItem")}
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setItems((prev) => [...prev, emptyItem()])}
                  data-testid="diligence-pack-add"
                >
                  {t("diligence.packAddItem")}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  disabled={saving}
                  onClick={() => void handleSave()}
                  data-testid="diligence-pack-save"
                >
                  {t("diligence.packSave")}
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  disabled={saving || !forked}
                  onClick={() => void handleReset()}
                  data-testid="diligence-pack-reset"
                >
                  {t("diligence.packResetAction")}
                </Button>
              </div>
            </>
          )}
        </CardContent>
      ) : null}
    </Card>
  );
}

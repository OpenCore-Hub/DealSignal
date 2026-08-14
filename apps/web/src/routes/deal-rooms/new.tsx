import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { motion } from "motion/react";
import { Check } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { CreateRoomFoldersDialog } from "@/components/deal-rooms/CreateRoomFoldersDialog";
import { PipelinePaper } from "@/components/links/link-bundle/PipelinePaper";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { apiErrorMessage } from "@/lib/apiErrors";
import { usageAtCap } from "@/lib/planQuota";
import {
  ensureDealRoomTemplates,
  templateFolderCount,
} from "@/lib/dealRoomTemplates";
import { dealRoomSlugFromName, normalizeDealRoomName, validateDealRoomName } from "@/lib/dealRoomName";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useUIStore } from "@/stores/uiStore";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import type { DealRoomTemplate } from "@/types";

function SectionLabel({ index, label }: { index: string; label: string }) {
  return (
    <h2 className="mb-3 flex items-center gap-2.5">
      <span className="font-mono text-[10px] font-medium tracking-[0.16em] text-muted-foreground/65">
        {index}
      </span>
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/75">
        {label}
      </span>
    </h2>
  );
}

export function NewDealRoomPage() {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const setBreadcrumbTail = useUIStore((state) => state.setBreadcrumbTail);
  const reducedMotion = useReducedMotion();
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [ndaOverride, setNdaOverride] = useState<boolean | null>(null);
  const [creating, setCreating] = useState(false);
  const [folderDialogOpen, setFolderDialogOpen] = useState(false);
  const [nameTouched, setNameTouched] = useState(false);

  const {
    data: catalog,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    const res = await api.getDealRoomTemplates();
    return res.data;
  }, [tc]);
  const templates = useMemo(
    () => (catalog == null ? [] : ensureDealRoomTemplates(catalog)),
    [catalog],
  );
  const { data: billing } = useAsyncData(() => api.getBillingInfo().catch(() => null), []);
  const roomsAtCap = billing ? usageAtCap(billing.roomsUsed, billing.roomsLimit) : false;
  const ndaPlanBlocked = billing != null && billing.ndaEnabled === false;

  const getTemplateDisplay = useCallback(
    (template: DealRoomTemplate) => ({
      name: t(`templates.${template.scenario}.name`, { defaultValue: template.name }),
      description: t(`templates.${template.scenario}.description`, {
        defaultValue: template.description,
      }),
    }),
    [t],
  );

  useEffect(() => {
    setBreadcrumbTail({ label: t("new.breadcrumb") });
    return () => setBreadcrumbTail(null);
  }, [setBreadcrumbTail, t]);

  useEffect(() => {
    if (templates && templates.length > 0 && !selectedTemplateId) {
      setSelectedTemplateId(templates[0].id);
    }
  }, [templates, selectedTemplateId]);

  const selectTemplate = (template: DealRoomTemplate) => {
    setSelectedTemplateId(template.id);
    setNdaOverride(null);
  };

  const selectedTemplate = useMemo(
    () => templates.find((tpl) => tpl.id === selectedTemplateId),
    [templates, selectedTemplateId],
  );

  const ndaEnabled = ndaPlanBlocked
    ? false
    : (ndaOverride ?? Boolean(selectedTemplate?.ndaEnabled));

  const nameIssue = validateDealRoomName(name);
  const nameValid = nameIssue === null;
  const selectedDisplay = selectedTemplate ? getTemplateDisplay(selectedTemplate) : null;
  const descriptionPlaceholder = selectedDisplay?.description || t("new.descriptionPlaceholder");

  const openFolderDialog = () => {
    if (!selectedTemplate || !nameValid) return;
    if (roomsAtCap) {
      toast.error(t("new.roomLimitReached"));
      return;
    }
    setFolderDialogOpen(true);
  };

  const handleCreate = async (folders: { name: string; path: string; description?: string }[]) => {
    if (!selectedTemplate || !nameValid) return;
    if (roomsAtCap) {
      toast.error(t("new.roomLimitReached"));
      return;
    }
    const trimmedName = normalizeDealRoomName(name);
    const baseSlug = dealRoomSlugFromName(trimmedName);
    setCreating(true);
    try {
      const payload = {
        name: trimmedName,
        description: description.trim(),
        template: selectedTemplate.scenario,
        ndaEnabled: ndaPlanBlocked ? false : ndaEnabled,
        folders: folders.map((folder, index) => ({
          name: folder.name,
          path: folder.path,
          ...(folder.description ? { description: folder.description } : {}),
          sort_order: index,
        })),
      };
      let room;
      for (let attempt = 0; attempt < 8; attempt++) {
        const slug = attempt === 0 ? baseSlug : `${baseSlug}-${attempt + 1}`;
        try {
          room = await api.createDealRoom({ ...payload, slug });
          break;
        } catch (error) {
          const taken =
            error instanceof ApiError &&
            (error.code === "duplicate_slug" || error.code === "slug_conflict");
          if (!taken || attempt === 7) throw error;
        }
      }
      if (!room) throw new Error("create room failed");
      toast.success(t("new.created"));
      setFolderDialogOpen(false);
      navigate(`/${workspaceSlug}/deal-rooms/${room.id}`);
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "dealRooms:new.createFailed" }));
    } finally {
      setCreating(false);
    }
  };

  const fieldClass =
    "h-10 border-border/60 bg-transparent shadow-none placeholder:text-[12px] placeholder:font-normal placeholder:text-foreground/28";

  return (
    <motion.div
      initial={reducedMotion ? false : { opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      className="flex w-full flex-col gap-5"
    >
      <header className="flex flex-col items-start">
        <h1 className="text-[1.375rem] font-medium leading-none tracking-[-0.045em] text-foreground">
          {t("new.title")}
        </h1>
        <p className="mt-3 max-w-[22rem] border-l border-foreground/12 pl-3 text-[12.5px] leading-[1.55] tracking-[-0.011em] text-muted-foreground">
          {t("new.subtitle")}
        </p>
      </header>

      {error ? (
        <div className="flex flex-col items-center justify-center gap-4 rounded-2xl bg-muted/30 px-8 py-16 text-center ring-1 ring-foreground/[0.05]">
          <p className="text-body text-muted-foreground">{error}</p>
          <Button onClick={refetch}>{tc("retry")}</Button>
        </div>
      ) : (
        <>
        <PipelinePaper className="w-full">
          <div className="flex flex-col lg:min-h-[calc(100dvh-14.5rem)]">
            <div className="grid flex-1 lg:grid-cols-[minmax(0,1fr)_minmax(21rem,24.5rem)]">
              <section className="flex min-h-0 flex-col px-5 py-6 sm:px-7 sm:py-7">
                <SectionLabel index="01" label={t("new.scenario")} />
                {loading ? (
                  <div className="grid flex-1 grid-cols-1 gap-2 sm:grid-cols-2">
                    {Array.from({ length: 4 }).map((_, i) => (
                      <Skeleton key={i} className="h-full min-h-[4.25rem] w-full rounded-xl" />
                    ))}
                  </div>
                ) : (
                  <div className="grid min-h-0 flex-1 grid-cols-1 gap-px overflow-hidden rounded-xl bg-foreground/[0.055] sm:grid-cols-2 sm:auto-rows-fr">
                    {templates.map((template, index) => {
                      const selected = selectedTemplateId === template.id;
                      const display = getTemplateDisplay(template);
                      return (
                        <button
                          key={template.id}
                          type="button"
                          aria-pressed={selected}
                          onClick={() => selectTemplate(template)}
                          className={cn(
                            "flex h-full min-h-[4.25rem] items-start gap-3 px-4 py-3.5 text-left",
                            "transition-colors duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
                            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset",
                            selected
                              ? "bg-foreground text-background"
                              : "bg-background hover:bg-foreground/[0.03]",
                          )}
                        >
                          <span
                            className={cn(
                              "mt-0.5 font-mono text-[10px] tracking-[0.14em]",
                              selected ? "text-background/55" : "text-muted-foreground/55",
                            )}
                          >
                            {String(index + 1).padStart(2, "0")}
                          </span>
                          <span className="min-w-0 flex-1">
                            <span className="block text-[13px] font-medium tracking-[-0.02em]">
                              {display.name}
                            </span>
                            <span
                              className={cn(
                                "mt-0.5 block text-[11px] tabular-nums",
                                selected ? "text-background/65" : "text-muted-foreground/80",
                              )}
                            >
                              {templateFolderCount(template) === 0
                                ? t("new.customFolderMeta")
                                : t("new.folderCount", {
                                    count: templateFolderCount(template),
                                  })}
                            </span>
                          </span>
                          <span
                            className={cn(
                              "mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full",
                              selected
                                ? "bg-background text-foreground"
                                : "ring-1 ring-foreground/15",
                            )}
                            aria-hidden
                          >
                            {selected ? <Check size={10} weight="bold" /> : null}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                )}
              </section>

              <section className="flex flex-col border-t border-foreground/[0.055] px-5 py-6 sm:px-7 sm:py-7 lg:border-t-0 lg:border-l">
                <SectionLabel index="02" label={t("new.basicInfo")} />
                <div className="flex min-h-0 flex-1 flex-col gap-4">
                  <div className="space-y-1.5">
                    <Label htmlFor="room-name" className="text-[12px] text-muted-foreground">
                      {t("new.name")}
                      <span className="ml-1 text-foreground/45">{t("new.nameRequired")}</span>
                    </Label>
                    <Input
                      id="room-name"
                      placeholder={t("new.namePlaceholder")}
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      onBlur={() => {
                        const next = normalizeDealRoomName(name);
                        if (next !== name) setName(next);
                        setNameTouched(true);
                      }}
                      aria-invalid={nameTouched && !nameValid}
                      aria-describedby={nameTouched && nameIssue ? "room-name-hint" : undefined}
                      className={fieldClass}
                    />
                    {nameTouched && nameIssue ? (
                      <p id="room-name-hint" className="text-[11.5px] leading-snug text-destructive">
                        {t(`new.nameError.${nameIssue}`)}
                      </p>
                    ) : null}
                  </div>
                  <div className="flex min-h-0 flex-1 flex-col space-y-1.5">
                    <Label htmlFor="room-description" className="text-[12px] text-muted-foreground">
                      {t("new.description")}
                      <span className="ml-1 text-foreground/45">{t("new.descriptionOptional")}</span>
                    </Label>
                    <Textarea
                      id="room-description"
                      placeholder={descriptionPlaceholder}
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                      rows={5}
                      className="min-h-[7rem] flex-1 resize-none border-border/60 bg-transparent shadow-none placeholder:text-[12px] placeholder:font-normal placeholder:text-foreground/28 lg:min-h-[10rem]"
                    />
                  </div>
                  <div className="flex items-center justify-between gap-6 pt-1">
                    <div className="min-w-0">
                      <p className="text-[13px] font-medium tracking-[-0.015em] text-foreground">
                        {t("new.enableNda")}
                      </p>
                      <p className="mt-0.5 text-[12px] leading-snug text-muted-foreground">
                        {ndaPlanBlocked
                          ? t("new.ndaPlanRequired")
                          : t("new.enableNdaDescription")}
                      </p>
                    </div>
                    <Switch
                      checked={ndaEnabled}
                      disabled={ndaPlanBlocked}
                      onCheckedChange={(checked) => {
                        if (checked && ndaPlanBlocked) return;
                        setNdaOverride(checked);
                      }}
                      aria-label={t("new.enableNda")}
                    />
                  </div>
                  {roomsAtCap ? (
                    <p className="text-xs text-muted-foreground" data-testid="new-room-limit-hint">
                      {t("new.roomLimitReached")}
                    </p>
                  ) : null}
                </div>
              </section>
            </div>

            <div className="flex items-center justify-center gap-8 border-t border-foreground/[0.06] bg-muted/45 px-5 py-3.5 sm:px-8">
              <Button
                variant="outline"
                onClick={() => navigate(`/${workspaceSlug}/deal-rooms`)}
                className="h-10 min-w-[6.5rem] rounded-lg bg-background px-4 text-[13px] font-medium tracking-tight"
              >
                {t("new.cancel")}
              </Button>
              <Button
                disabled={!nameValid || creating || roomsAtCap}
                onClick={openFolderDialog}
                className={cn(
                  "h-10 min-w-[8.5rem] rounded-lg px-5 text-[13px] font-medium tracking-tight",
                  "shadow-[inset_0_1px_0_rgba(255,255,255,0.12)]",
                )}
              >
                {creating ? t("new.creating") : t("new.create")}
              </Button>
            </div>
          </div>
        </PipelinePaper>
        <CreateRoomFoldersDialog
          open={folderDialogOpen}
          onOpenChange={setFolderDialogOpen}
          templateName={
            selectedTemplate
              ? getTemplateDisplay(selectedTemplate).name
              : ""
          }
          folders={selectedTemplate?.folderStructure ?? []}
          creating={creating}
          onConfirm={(folders) => {
            void handleCreate(folders);
          }}
        />
        </>
      )}
    </motion.div>
  );
}

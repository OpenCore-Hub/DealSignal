import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { motion } from "motion/react";
import {
  Plus,
  Check,
  Folder,
  FileText,
  ShieldCheck,
  GlobeHemisphereWest,
  LockKey,
  UsersThree,
  CaretLeft,
  CaretRight,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";
import { BackButton } from "@/components/common/BackButton";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { useAsyncData } from "@/hooks/useAsyncData";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import type { DealRoomTemplate } from "@/types";

const permissionIcons: Record<string, typeof GlobeHemisphereWest> = {
  public: GlobeHemisphereWest,
  standard: LockKey,
  confidential: ShieldCheck,
  collaborative: UsersThree,
};

export function NewDealRoomPage() {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const reducedMotion = useReducedMotion();
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [nda, setNda] = useState(true);
  const [creating, setCreating] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  const {
    data: templates,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    const res = await api.getDealRoomTemplates();
    return res.data;
  }, [tc]);

  const getTemplateDisplay = useCallback(
    (template: DealRoomTemplate) => ({
      name: t(`templates.${template.scenario}.name`, { defaultValue: template.name }),
      description: t(`templates.${template.scenario}.description`, {
        defaultValue: template.description,
      }),
    }),
    [t]
  );

  useEffect(() => {
    if (templates && templates.length > 0 && !selectedTemplateId) {
      const first = templates[0];
      const display = getTemplateDisplay(first);
      // eslint-disable-next-line react-hooks/set-state-in-effect -- derive initial form state from fetched templates
      setSelectedTemplateId(first.id);
      setNda(first.ndaEnabled);
      setName(display.name);
      setDescription(display.description);
    }
  }, [templates, selectedTemplateId, getTemplateDisplay]);

  const selectTemplate = (template: DealRoomTemplate, fillFields = false) => {
    const display = getTemplateDisplay(template);
    setSelectedTemplateId(template.id);
    setNda(template.ndaEnabled);
    if (fillFields || !name) setName(display.name);
    if (fillFields || !description) setDescription(display.description);
  };

  const scroll = (direction: "left" | "right") => {
    const container = scrollRef.current;
    if (!container) return;
    const cardWidth = 288;
    const gap = 16;
    const scrollAmount = direction === "left" ? -(cardWidth + gap) : cardWidth + gap;
    container.scrollBy({ left: scrollAmount, behavior: "smooth" });
  };

  const selectedTemplate = useMemo(
    () => templates?.find((tpl) => tpl.id === selectedTemplateId),
    [templates, selectedTemplateId]
  );

  const slugify = (value: string) =>
    value
      .toLowerCase()
      .trim()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "");

  const handleCreate = async () => {
    if (!selectedTemplate || !name) return;
    const slug = slugify(name) || selectedTemplate.scenario;
    if (!slug) {
      toast.error(tc("error.saveFailed"));
      return;
    }
    setCreating(true);
    try {
      const room = await api.createDealRoom({
        name,
        slug,
        description,
        template: selectedTemplate.scenario,
        ndaEnabled: nda,
      });
      toast.success(t("new.created"));
      navigate(`/${workspaceSlug}/deal-rooms/${room.id}`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("new.createFailed"));
    } finally {
      setCreating(false);
    }
  };

  return (
    <motion.div
      initial={reducedMotion ? false : { opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      className="mx-auto max-w-5xl space-y-6"
    >
      <BackButton to={`/${workspaceSlug}/deal-rooms`} label={t("detail.back")} />

      <div className="space-y-1">
        <h1 className="text-h1">{t("new.title")}</h1>
        <p className="text-body text-muted-foreground">
          {t("new.subtitle")}
        </p>
      </div>

      {error ? (
        <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
          <p className="text-body text-muted-foreground">{error}</p>
          <Button onClick={refetch}>{tc("retry")}</Button>
        </div>
      ) : loading ? (
        <div className="flex gap-4 overflow-x-auto pb-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-40 w-72 shrink-0" />
          ))}
        </div>
      ) : (
        <div className="relative">
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="absolute -left-14 top-1/2 z-10 -translate-y-1/2 rounded-full shadow-md"
            onClick={() => scroll("left")}
            aria-label={t("new.previousTemplate")}
          >
            <CaretLeft size={20} />
          </Button>
          <div
            ref={scrollRef}
            className="flex gap-4 overflow-x-auto pb-4 scroll-smooth [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
          >
            {templates?.map((template) => {
              const selected = selectedTemplateId === template.id;
              const display = getTemplateDisplay(template);
              return (
                <Card
                  key={template.id}
                  role="button"
                  tabIndex={0}
                  className={`w-72 shrink-0 cursor-pointer transition-colors hover:bg-muted/50 hover:border-muted-foreground/20 ${
                    selected ? "ring-2 ring-primary" : ""
                  }`}
                  onClick={() => selectTemplate(template, true)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      selectTemplate(template);
                    }
                  }}
                >
                  <CardContent>
                    <div className="flex items-start justify-between">
                      <p className="text-h3">{display.name}</p>
                      {selected && (
                        <div className="flex h-5 w-5 items-center justify-center rounded-full bg-primary text-primary-foreground">
                          <Check size={12} weight="bold" />
                        </div>
                      )}
                    </div>
                    <p className="text-caption mt-2 text-muted-foreground line-clamp-3">
                      {display.description}
                    </p>
                    <div className="mt-3 flex flex-wrap gap-1">
                      <Badge variant="outline" className="text-caption">
                        {t("new.folderCount", { count: template.folderStructure.length })}
                      </Badge>
                      {template.ndaEnabled && (
                        <Badge variant="secondary" className="text-caption">
                          NDA
                        </Badge>
                      )}
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="absolute -right-14 top-1/2 z-10 -translate-y-1/2 rounded-full shadow-md"
            onClick={() => scroll("right")}
            aria-label={t("new.nextTemplate")}
          >
            <CaretRight size={20} />
          </Button>
        </div>
      )}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle className="text-h2">{t("new.basicInfo")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="room-name">{t("new.name")}</Label>
              <Input
                id="room-name"
                placeholder={t("new.namePlaceholder")}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="room-description">{t("new.description")}</Label>
              <Input
                id="room-description"
                placeholder={t("new.descriptionPlaceholder")}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <div>
                <p className="text-sm font-medium">{t("new.enableNda")}</p>
                <p className="text-caption text-muted-foreground">{t("new.enableNdaDescription")}</p>
              </div>
              <Switch checked={nda} onCheckedChange={setNda} />
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="outline" onClick={() => navigate(`/${workspaceSlug}/deal-rooms`)}>
                {t("new.cancel")}
              </Button>
              <Button className="gap-1.5" disabled={!name || creating} onClick={handleCreate}>
                <Plus size={16} weight="bold" />
                {creating ? t("new.creating") : t("new.create")}
              </Button>
            </div>
          </CardContent>
        </Card>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <Folder size={20} />
                {t("new.folders")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {selectedTemplate ? (
                <ul className="space-y-2">
                  {selectedTemplate.folderStructure.map((folder, idx) => (
                    <li key={idx} className="flex items-start gap-2 text-sm">
                      <Folder size={16} className="mt-0.5 text-muted-foreground" />
                      <div>
                        <p className="font-medium">{folder.name}</p>
                        {folder.description && (
                          <p className="text-caption text-muted-foreground">{folder.description}</p>
                        )}
                      </div>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-sm text-muted-foreground">{t("detail.noTemplate")}</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <FileText size={20} />
                {t("new.recommendedFiles")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {selectedTemplate ? (
                <ul className="space-y-2">
                  {selectedTemplate.recommendedFiles.map((file, idx) => (
                    <li key={idx} className="flex items-center gap-2 text-sm">
                      <FileText size={16} className="text-muted-foreground" />
                      {file}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-sm text-muted-foreground">{t("detail.noTemplate")}</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <ShieldCheck size={20} />
                {t("new.defaultPermission")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {selectedTemplate ? (
                <div className="flex items-center gap-2 text-sm">
                  {(() => {
                    const Icon = permissionIcons[selectedTemplate.defaultPermissionLevel];
                    return <Icon size={18} className="text-muted-foreground" />;
                  })()}
                  {t(`permission.${selectedTemplate.defaultPermissionLevel}.label`)}
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">{t("detail.noTemplate")}</p>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </motion.div>
  );
}

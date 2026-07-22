import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Books, SpinnerGap } from "@phosphor-icons/react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import type {
  DealRoomDocumentItem,
  DealRoomFolder,
  DealRoomKnowledgeBase,
  DealRoomKnowledgeBaseStatus,
} from "@/types";

interface KnowledgeBasePanelProps {
  roomId: string;
  isAdmin: boolean;
  documents: DealRoomDocumentItem[];
  folders?: DealRoomFolder[];
}

type WizardMode = "create" | "rebuild" | null;

export function KnowledgeBasePanel({
  roomId,
  isAdmin,
  documents,
  folders = [],
}: KnowledgeBasePanelProps) {
  const { t } = useTranslation("dealRooms");
  const [kb, setKb] = useState<DealRoomKnowledgeBase | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<"forbidden" | "generic" | null>(null);
  const [wizard, setWizard] = useState<WizardMode>(null);
  const [selectedDocIds, setSelectedDocIds] = useState<Set<string>>(new Set());
  const [selectedFolderPaths, setSelectedFolderPaths] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);
  const [rebuildConfirmOpen, setRebuildConfirmOpen] = useState(false);

  const selectableDocs = useMemo(
    () => documents.filter((d) => d.status === "ready"),
    [documents],
  );

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setLoadError(null);
      try {
        const res = await api.getDealRoomKnowledgeBase(roomId);
        if (cancelled) return;
        setKb(res);
      } catch (e) {
        if (cancelled) return;
        if (e instanceof ApiError && e.status === 403) {
          setLoadError("forbidden");
        } else {
          setLoadError("generic");
        }
        setKb(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [roomId]);

  const openWizard = (mode: WizardMode) => {
    setSelectedDocIds(new Set(kb?.document_ids ?? []));
    setSelectedFolderPaths(new Set(kb?.folder_paths ?? []));
    setWizard(mode);
  };

  const toggleDoc = (documentId: string, checked: boolean) => {
    setSelectedDocIds((prev) => {
      const next = new Set(prev);
      if (checked) next.add(documentId);
      else next.delete(documentId);
      return next;
    });
  };

  const toggleFolder = (path: string, checked: boolean) => {
    setSelectedFolderPaths((prev) => {
      const next = new Set(prev);
      if (checked) next.add(path);
      else next.delete(path);
      return next;
    });
  };

  const runSubmit = async () => {
    if (!wizard) return;
    setBusy(true);
    try {
      const selection = {
        folder_paths: Array.from(selectedFolderPaths),
        document_ids: Array.from(selectedDocIds),
      };
      const res =
        wizard === "create"
          ? await api.createDealRoomKnowledgeBase(roomId, selection)
          : await api.rebuildDealRoomKnowledgeBase(roomId, selection);
      setKb(res);
      setWizard(null);
      setRebuildConfirmOpen(false);
    } catch (e) {
      if (e instanceof ApiError && e.code === "no_searchable_chunks") {
        toast.error(t("knowledgeBase.noSearchableChunks"));
        return;
      }
      if (e instanceof ApiError && e.code === "knowledge_base_embed_failed") {
        toast.error(t("knowledgeBase.embedFailed"));
        try {
          setKb(await api.getDealRoomKnowledgeBase(roomId));
        } catch {
          // keep prior status if refresh fails
        }
        return;
      }
      const fallback =
        wizard === "create"
          ? t("knowledgeBase.createFailed")
          : t("knowledgeBase.rebuildFailed");
      toast.error(e instanceof Error ? e.message : fallback);
    } finally {
      setBusy(false);
    }
  };

  const onPrimarySubmit = () => {
    if (wizard === "rebuild") {
      setRebuildConfirmOpen(true);
      return;
    }
    void runSubmit();
  };

  const statusText = (status: DealRoomKnowledgeBaseStatus, row: DealRoomKnowledgeBase) => {
    switch (status) {
      case "none":
        return t("knowledgeBase.status.none");
      case "building": {
        const total = row.building_document_ids?.length ?? row.embedded_count ?? 0;
        const done = row.embedded_count ?? 0;
        return t("knowledgeBase.status.building", { done, total });
      }
      case "ready":
        return t("knowledgeBase.status.ready", { count: row.embedded_count });
      case "stale":
        return t("knowledgeBase.status.stale");
      case "failed":
        return t("knowledgeBase.status.failed");
      default:
        return status;
    }
  };

  const canCreate =
    isAdmin && kb && (kb.status === "none" || kb.status === "failed") && !wizard;
  const canRebuild =
    isAdmin &&
    kb &&
    (kb.status === "ready" || kb.status === "stale" || kb.status === "failed") &&
    !wizard;

  return (
    <Card data-testid="knowledge-base-panel">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-h3">
          <Books size={20} />
          {t("knowledgeBase.title")}
        </CardTitle>
        <CardDescription>{t("knowledgeBase.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {loading ? (
          <p className="flex items-center gap-2 text-sm text-muted-foreground">
            <SpinnerGap size={16} className="animate-spin" />
            {t("common:loading")}
          </p>
        ) : loadError === "forbidden" ? (
          <p className="text-sm text-muted-foreground">{t("knowledgeBase.forbidden")}</p>
        ) : loadError === "generic" || !kb ? (
          <p className="text-sm text-muted-foreground">{t("knowledgeBase.loadFailed")}</p>
        ) : (
          <>
            <p className="text-sm" role="status">
              {statusText(kb.status, kb)}
            </p>

            {!isAdmin ? (
              <p className="text-xs text-muted-foreground">{t("knowledgeBase.forbidden")}</p>
            ) : null}

            {canCreate ? (
              <Button type="button" size="sm" onClick={() => openWizard("create")}>
                {t("knowledgeBase.create")}
              </Button>
            ) : null}

            {canRebuild ? (
              <div className="space-y-2">
                <Button type="button" size="sm" variant="outline" onClick={() => openWizard("rebuild")}>
                  {t("knowledgeBase.rebuild")}
                </Button>
                <p className="text-xs text-muted-foreground">{t("knowledgeBase.rebuildHint")}</p>
              </div>
            ) : null}

            {wizard ? (
              <div className="space-y-3 rounded-lg border p-3">
                {folders.length > 0 ? (
                  <div className="space-y-2">
                    <p className="text-sm font-medium">{t("knowledgeBase.selectFolders")}</p>
                    <ul className="space-y-2">
                      {folders.map((folder) => {
                        const checked = selectedFolderPaths.has(folder.path);
                        const id = `kb-folder-${folder.path}`;
                        return (
                          <li key={folder.path} className="flex items-center gap-2">
                            <Checkbox
                              id={id}
                              checked={checked}
                              onCheckedChange={(v) => toggleFolder(folder.path, v === true)}
                            />
                            <Label htmlFor={id} className="font-normal">
                              {folder.name}
                            </Label>
                          </li>
                        );
                      })}
                    </ul>
                  </div>
                ) : null}

                <div className="space-y-2">
                  <p className="text-sm font-medium">{t("knowledgeBase.selectDocuments")}</p>
                  {selectableDocs.length === 0 ? (
                    <p className="text-sm text-muted-foreground">{t("knowledgeBase.status.none")}</p>
                  ) : (
                    <ul className="space-y-2">
                      {selectableDocs.map((doc) => {
                        const checked = selectedDocIds.has(doc.document_id);
                        const id = `kb-doc-${doc.document_id}`;
                        return (
                          <li key={doc.document_id} className="flex items-center gap-2">
                            <Checkbox
                              id={id}
                              checked={checked}
                              onCheckedChange={(v) => toggleDoc(doc.document_id, v === true)}
                            />
                            <Label htmlFor={id} className="font-normal">
                              {doc.title}
                            </Label>
                          </li>
                        );
                      })}
                    </ul>
                  )}
                </div>

                <div className="flex flex-wrap gap-2">
                  <Button type="button" size="sm" disabled={busy} onClick={onPrimarySubmit}>
                    {busy
                      ? wizard === "create"
                        ? t("knowledgeBase.creating")
                        : t("knowledgeBase.rebuilding")
                      : wizard === "create"
                        ? t("knowledgeBase.confirmCreate")
                        : t("knowledgeBase.confirmRebuild")}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    disabled={busy}
                    onClick={() => setWizard(null)}
                  >
                    {t("knowledgeBase.cancel")}
                  </Button>
                </div>
              </div>
            ) : null}
          </>
        )}
      </CardContent>

      <Dialog open={rebuildConfirmOpen} onOpenChange={setRebuildConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("knowledgeBase.confirmRebuildTitle")}</DialogTitle>
            <DialogDescription>{t("knowledgeBase.confirmRebuildBody")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              disabled={busy}
              onClick={() => setRebuildConfirmOpen(false)}
            >
              {t("knowledgeBase.cancel")}
            </Button>
            <Button type="button" disabled={busy} onClick={() => void runSubmit()}>
              {busy ? t("knowledgeBase.rebuilding") : t("knowledgeBase.confirmRebuild")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

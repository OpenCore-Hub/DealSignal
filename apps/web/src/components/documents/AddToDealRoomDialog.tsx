import { useMemo, useState } from "react";
import { Buildings } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import { toast } from "sonner";
import type { DealRoomFolder, DealRoomFolderDocs } from "@/types";

interface AddToDealRoomDialogProps {
  documentId: string;
  documentTitle: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAdded?: () => void;
}

export function AddToDealRoomDialog({
  documentId,
  documentTitle,
  open,
  onOpenChange,
  onAdded,
}: AddToDealRoomDialogProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");

  const [selectedRoomId, setSelectedRoomId] = useState<string>("");
  const [selectedFolder, setSelectedFolder] = useState<string>("");

  const {
    data: rooms,
    loading: loadingRooms,
    error: roomsError,
  } = useAsyncData(async () => {
    if (!open) return [];
    const res = await api.getDealRooms();
    return res.data;
  }, [open]);

  const effectiveRoomId = selectedRoomId || rooms?.[0]?.id || "";

  const {
    data: details,
    loading: loadingDetails,
  } = useAsyncData(async () => {
    if (!open || !effectiveRoomId) {
      return { folders: [] as DealRoomFolder[], docs: [] as DealRoomFolderDocs[] };
    }
    const [foldersRes, docsRes] = await Promise.all([
      api.getDealRoomFolders(effectiveRoomId),
      api.getDealRoomDocuments(effectiveRoomId),
    ]);
    return { folders: foldersRes.data, docs: docsRes.data };
  }, [open, effectiveRoomId]);

  const folders = useMemo(() => details?.folders ?? [], [details?.folders]);
  const folderDocs = useMemo(() => details?.docs ?? [], [details?.docs]);

  const defaultFolder = useMemo(() => {
    return folders.find((f) => f.path === "/")?.path ?? folders[0]?.path ?? "";
  }, [folders]);

  const effectiveFolder =
    selectedFolder && folders.some((f) => f.path === selectedFolder)
      ? selectedFolder
      : defaultFolder;

  const alreadyAdded = useMemo(() => {
    return folderDocs.some((fd) =>
      fd.documents.some((d) => d.document_id === documentId)
    );
  }, [folderDocs, documentId]);

  const canAdd =
    effectiveRoomId && effectiveFolder && !alreadyAdded && !loadingDetails;

  const [adding, setAdding] = useState(false);

  const handleAdd = async () => {
    if (!canAdd) return;
    setAdding(true);
    try {
      const sortOrder =
        folderDocs.find((fd) => fd.folder === effectiveFolder)?.documents.length ?? 0;
      await api.addDealRoomDocument(effectiveRoomId, {
        document_id: documentId,
        folder_path: effectiveFolder,
        sort_order: sortOrder,
      });
      toast.success(t("documents.addedSingle"));
      onAdded?.();
      handleOpenChange(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setAdding(false);
    }
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setSelectedRoomId("");
      setSelectedFolder("");
    }
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Buildings size={20} />
            {t("documents.addToRoom")}
          </DialogTitle>
          <DialogDescription>{t("documents.addToRoomDescription", { title: documentTitle })}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {roomsError ? (
            <p className="text-sm text-destructive">{roomsError}</p>
          ) : loadingRooms ? (
            <p className="text-sm text-muted-foreground">{tc("loading")}</p>
          ) : !rooms || rooms.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("documents.noRooms")}</p>
          ) : (
            <div className="space-y-2">
              <Label htmlFor="room-select">{t("documents.selectRoom")}</Label>
              <Select value={effectiveRoomId} onValueChange={(v) => v && setSelectedRoomId(v)}>
                <SelectTrigger id="room-select" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {rooms.map((room) => (
                    <SelectItem key={room.id} value={room.id}>
                      {room.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {effectiveRoomId && (
            <div className="space-y-2">
              <Label htmlFor="folder-select">{t("documents.selectFolder")}</Label>
              {loadingDetails ? (
                <p className="text-sm text-muted-foreground">{tc("loading")}</p>
              ) : folders.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("documents.noFolders")}</p>
              ) : (
                <Select value={effectiveFolder} onValueChange={(v) => v && setSelectedFolder(v)}>
                  <SelectTrigger id="folder-select" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {folders.map((folder) => (
                      <SelectItem key={folder.path} value={folder.path}>
                        {folder.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
              {alreadyAdded && (
                <p className="text-sm text-destructive">{t("documents.alreadyAdded")}</p>
              )}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)} disabled={adding}>
            {tc("cancel")}
          </Button>
          <Button onClick={handleAdd} disabled={!canAdd || adding}>
            {adding ? t("documents.adding") : t("documents.add")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

import { useState } from "react";
import { FileText } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { DealRoomFolderTree } from "./DealRoomFolderTree";
import { DocumentPicker } from "./DocumentPicker";
import type { DealRoomFolder, DealRoomFolderDocs, Document } from "@/types";

interface DealRoomDocumentsDialogProps {
  roomId: string;
  folders: DealRoomFolder[];
  folderDocs: DealRoomFolderDocs[];
  workspaceDocuments: Document[];
  isAdmin?: boolean;
  onChanged: () => void;
  children?: React.ReactNode;
}

export function DealRoomDocumentsDialog({
  roomId,
  folders,
  folderDocs,
  workspaceDocuments,
  isAdmin = true,
  onChanged,
  children,
}: DealRoomDocumentsDialogProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const [open, setOpen] = useState(false);
  const [adding, setAdding] = useState(false);

  const allRoomDocuments = folderDocs.flatMap((fd) => fd.documents);

  const handleFolderCreate = async (name: string, parentPath?: string) => {
    try {
      await api.createDealRoomFolder(roomId, { name, parent_path: parentPath });
      toast.success(t("folders.created", { name }));
      onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("folders.createFailed"));
    }
  };

  const handleFolderRename = async (path: string, name: string) => {
    try {
      await api.renameDealRoomFolder(roomId, path, { name });
      toast.success(t("folders.renamed"));
      onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("folders.renameFailed"));
    }
  };

  const handleFolderDelete = async (path: string) => {
    try {
      await api.deleteDealRoomFolder(roomId, path);
      toast.success(t("folders.deleted"));
      onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("folders.deleteFailed"));
    }
  };

  const handleDocumentMove = async (docId: string, folderPath: string) => {
    try {
      await api.updateDealRoomDocument(roomId, docId, { folder_path: folderPath });
      toast.success(t("documents.moved"));
      onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("documents.moveFailed"));
    }
  };

  const handleDocumentReorder = async (docId: string, sortOrder: number) => {
    try {
      await api.updateDealRoomDocument(roomId, docId, { sort_order: sortOrder });
      onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("documents.reorderFailed"));
    }
  };

  const handleDocumentRemove = async (docId: string) => {
    try {
      await api.removeDealRoomDocument(roomId, docId);
      toast.success(t("documents.removed"));
      onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("documents.removeFailed"));
    }
  };

  const handleAddDocuments = async (documentIds: string[], folderPath: string) => {
    setAdding(true);
    try {
      let lastOrder =
        folderDocs.find((fd) => fd.folder === folderPath)?.documents.length ?? 0;
      for (const documentId of documentIds) {
        await api.addDealRoomDocument(roomId, {
          document_id: documentId,
          folder_path: folderPath,
          sort_order: lastOrder++,
        });
      }
      toast.success(t("documents.added", { count: documentIds.length }));
      onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setAdding(false);
    }
  };

  const handleDocumentOpen = (documentId: string) => {
    window.open(`/viewer/${documentId}`, "_blank", "noopener,noreferrer");
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={children ? (children as React.ReactElement) : (
        <Button className="gap-1.5">
          <FileText size={16} />
          {t("detail.manageDocs")}
        </Button>
      )} />
      <DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FileText size={20} />
            {t("detail.manageDocs")}
          </DialogTitle>
          <DialogDescription>{t("documents.dialogDescription")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-6 py-2">
          <DealRoomFolderTree
            roomId={roomId}
            folders={folders}
            folderDocs={folderDocs}
            isAdmin={isAdmin}
            onFolderCreate={handleFolderCreate}
            onFolderRename={handleFolderRename}
            onFolderDelete={handleFolderDelete}
            onDocumentMove={handleDocumentMove}
            onDocumentReorder={handleDocumentReorder}
            onDocumentRemove={handleDocumentRemove}
            onDocumentOpen={handleDocumentOpen}
          />

          {isAdmin && (
            <DocumentPicker
              workspaceDocuments={workspaceDocuments}
              roomDocuments={allRoomDocuments}
              folders={folders}
              onAdd={handleAddDocuments}
              disabled={adding}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

import { useMemo, useState } from "react";
import {
  Folder,
  FolderOpen,
  FileText,
  Plus,
  PencilSimple,
  Trash,
  CaretUp,
  CaretDown,
  DotsThree,
  ArrowsLeftRight,
  X,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "react-i18next";

import type { DealRoomDocumentItem, DealRoomFolder, DealRoomFolderDocs } from "@/types";

interface DealRoomFolderTreeProps {
  roomId: string;
  folders: DealRoomFolder[];
  folderDocs: DealRoomFolderDocs[];
  isAdmin?: boolean;
  onFolderCreate: (name: string, parentPath?: string) => Promise<void>;
  onFolderRename: (path: string, name: string) => Promise<void>;
  onFolderDelete: (path: string) => Promise<void>;
  onDocumentMove: (docId: string, folderPath: string) => Promise<void>;
  onDocumentReorder: (docId: string, sortOrder: number) => Promise<void>;
  onDocumentRemove: (docId: string) => Promise<void>;
  onDocumentOpen?: (docId: string) => void;
}

export function DealRoomFolderTree({
  folders,
  folderDocs,
  isAdmin = true,
  onFolderCreate,
  onFolderRename,
  onFolderDelete,
  onDocumentMove,
  onDocumentReorder,
  onDocumentRemove,
  onDocumentOpen,
}: DealRoomFolderTreeProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const [expanded, setExpanded] = useState<Set<string>>(new Set(folders.map((f) => f.path)));
  const [creatingParent, setCreatingParent] = useState<string | null>(null);
  const [newFolderName, setNewFolderName] = useState("");
  const [creating, setCreating] = useState(false);
  const [renamingFolder, setRenamingFolder] = useState<DealRoomFolder | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const [renaming, setRenaming] = useState(false);
  const [movingDoc, setMovingDoc] = useState<DealRoomDocumentItem | null>(null);
  const [targetFolder, setTargetFolder] = useState<string>("/");
  const [actingDocId, setActingDocId] = useState<string | null>(null);

  const sortedFolders = useMemo(
    () => [...folders].sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name)),
    [folders]
  );

  const docsByFolder = useMemo(() => {
    const map = new Map<string, DealRoomDocumentItem[]>();
    for (const fd of folderDocs) {
      const sorted = [...fd.documents].sort((a, b) => a.sort_order - b.sort_order);
      map.set(fd.folder, sorted);
    }
    for (const folder of folders) {
      if (!map.has(folder.path)) map.set(folder.path, []);
    }
    return map;
  }, [folderDocs, folders]);

  const toggleFolder = (path: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const handleCreate = async () => {
    if (!newFolderName.trim()) return;
    setCreating(true);
    try {
      await onFolderCreate(newFolderName.trim(), creatingParent ?? undefined);
      setNewFolderName("");
      setCreatingParent(null);
    } finally {
      setCreating(false);
    }
  };

  const startRename = (folder: DealRoomFolder) => {
    setRenamingFolder(folder);
    setRenameValue(folder.name);
  };

  const handleRename = async () => {
    if (!renamingFolder || !renameValue.trim() || renameValue.trim() === renamingFolder.name) {
      setRenamingFolder(null);
      return;
    }
    setRenaming(true);
    try {
      await onFolderRename(renamingFolder.path, renameValue.trim());
      setRenamingFolder(null);
    } finally {
      setRenaming(false);
    }
  };

  const handleDelete = async (folder: DealRoomFolder) => {
    const docs = docsByFolder.get(folder.path) ?? [];
    if (docs.length > 0) {
      alert(t("folders.deleteNotEmpty"));
      return;
    }
    if (!confirm(t("folders.deleteConfirm", { name: folder.name }))) return;
    await onFolderDelete(folder.path);
  };

  const handleMove = async () => {
    if (!movingDoc) return;
    setActingDocId(movingDoc.id);
    try {
      await onDocumentMove(movingDoc.id, targetFolder);
      setMovingDoc(null);
    } finally {
      setActingDocId(null);
    }
  };

  const handleReorder = async (doc: DealRoomDocumentItem, direction: -1 | 1) => {
    setActingDocId(doc.id);
    try {
      await onDocumentReorder(doc.id, doc.sort_order + direction);
    } finally {
      setActingDocId(null);
    }
  };

  const handleRemove = async (doc: DealRoomDocumentItem) => {
    if (!confirm(t("documents.removeConfirm", { title: doc.title }))) return;
    setActingDocId(doc.id);
    try {
      await onDocumentRemove(doc.id);
    } finally {
      setActingDocId(null);
    }
  };

  return (
    <div className="space-y-4">
      {isAdmin && (
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            className="gap-1"
            onClick={() => setCreatingParent("/")}
          >
            <Plus size={14} />
            {t("folders.newRoot")}
          </Button>
        </div>
      )}

      {creatingParent !== null && (
        <div className="flex items-center gap-2 rounded-md border border-border p-3">
          <Folder size={18} className="text-muted-foreground" />
          <Input
            value={newFolderName}
            onChange={(e) => setNewFolderName(e.target.value)}
            placeholder={t("folders.namePlaceholder")}
            className="h-8 flex-1"
            autoFocus
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleCreate();
              if (e.key === "Escape") setCreatingParent(null);
            }}
          />
          <Button size="sm" onClick={() => void handleCreate()} disabled={!newFolderName.trim() || creating}>
            {t("folders.create")}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setCreatingParent(null)}>
            <X size={14} />
          </Button>
        </div>
      )}

      <div className="space-y-1">
        {sortedFolders.map((folder) => {
          const docs = docsByFolder.get(folder.path) ?? [];
          const isExpanded = expanded.has(folder.path);
          return (
            <div key={folder.path} className="rounded-md border border-border">
              <button
                type="button"
                onClick={() => toggleFolder(folder.path)}
                className="flex w-full items-center justify-between gap-2 p-3 text-left hover:bg-muted/50"
              >
                <div className="flex items-center gap-2 min-w-0">
                  {isExpanded ? (
                    <FolderOpen size={18} className="text-primary shrink-0" />
                  ) : (
                    <Folder size={18} className="text-muted-foreground shrink-0" />
                  )}
                  <span className="truncate text-sm font-medium">{folder.name}</span>
                  {folder.description && (
                    <span className="hidden text-caption text-muted-foreground sm:inline">
                      {folder.description}
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <span className="text-caption text-muted-foreground">{docs.length}</span>
                  {isAdmin && (
                    <DropdownMenu>
                      <DropdownMenuTrigger
                      render={
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={(e) => e.stopPropagation()}
                          aria-label={t("folders.actions", { name: folder.name })}
                        >
                          <DotsThree size={16} />
                        </Button>
                      }
                    />
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          onClick={(e) => {
                            e.stopPropagation();
                            startRename(folder);
                          }}
                        >
                          <PencilSimple size={14} />
                          {t("folders.rename")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={(e) => {
                            e.stopPropagation();
                            setCreatingParent(folder.path);
                          }}
                        >
                          <Plus size={14} />
                          {t("folders.newSubfolder")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={(e) => {
                            e.stopPropagation();
                            void handleDelete(folder);
                          }}
                        >
                          <Trash size={14} />
                          {tc("delete")}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  )}
                </div>
              </button>

              {isExpanded && (
                <div className="border-t border-border bg-muted/20">
                  {docs.length === 0 ? (
                    <p className="p-3 text-sm text-muted-foreground">{t("documents.emptyFolder")}</p>
                  ) : (
                    <ul className="divide-y divide-border">
                      {docs.map((doc, idx) => (
                        <li
                          key={doc.id}
                          className="flex items-center justify-between gap-2 p-3 hover:bg-muted/30"
                        >
                          <button
                            type="button"
                            onClick={() => onDocumentOpen?.(doc.document_id)}
                            className="flex items-center gap-2 min-w-0 text-left"
                          >
                            <FileText size={16} className="text-muted-foreground shrink-0" />
                            <span className="truncate text-sm">{doc.title}</span>
                          </button>
                          {isAdmin && (
                            <div className="flex items-center gap-1 shrink-0">
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                disabled={idx === 0 || actingDocId === doc.id}
                                onClick={() => void handleReorder(doc, -1)}
                                aria-label={t("documents.moveUp")}
                              >
                                <CaretUp size={14} />
                              </Button>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                disabled={idx === docs.length - 1 || actingDocId === doc.id}
                                onClick={() => void handleReorder(doc, 1)}
                                aria-label={t("documents.moveDown")}
                              >
                                <CaretDown size={14} />
                              </Button>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                disabled={actingDocId === doc.id}
                                onClick={() => setMovingDoc(doc)}
                                aria-label={t("documents.moveTo")}
                              >
                                <ArrowsLeftRight size={14} />
                              </Button>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                disabled={actingDocId === doc.id}
                                onClick={() => void handleRemove(doc)}
                                aria-label={t("documents.remove")}
                              >
                                <Trash size={14} />
                              </Button>
                            </div>
                          )}
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>

      <Dialog open={!!renamingFolder} onOpenChange={(open) => !open && setRenamingFolder(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("folders.renameTitle")}</DialogTitle>
          </DialogHeader>
          <Input
            value={renameValue}
            onChange={(e) => setRenameValue(e.target.value)}
            placeholder={t("folders.namePlaceholder")}
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleRename();
            }}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setRenamingFolder(null)}>
              {tc("cancel")}
            </Button>
            <Button onClick={() => void handleRename()} disabled={!renameValue.trim() || renaming}>
              {tc("save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!movingDoc} onOpenChange={(open) => !open && setMovingDoc(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("documents.moveTitle")}</DialogTitle>
            <DialogDescription>{movingDoc?.title}</DialogDescription>
          </DialogHeader>
          <Select value={targetFolder} onValueChange={(v) => v && setTargetFolder(v)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {folders.map((f) => (
                <SelectItem key={f.path} value={f.path}>
                  {f.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <DialogFooter>
            <Button variant="outline" onClick={() => setMovingDoc(null)}>
              {tc("cancel")}
            </Button>
            <Button onClick={() => void handleMove()} disabled={actingDocId === movingDoc?.id}>
              {t("documents.move")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

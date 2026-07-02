import { useEffect, useMemo, useRef, useState } from "react";
import {
  Folder,
  FolderOpen,
  FileText,
  Plus,
  UploadSimple,
  PencilSimple,
  Trash,
  CaretRight,
  CaretDown,
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { DocumentPicker } from "./DocumentPicker";
import type { DealRoomDocumentItem, DealRoomFolder, DealRoomFolderDocs, Document } from "@/types";

interface DealRoomFolderTreeProps {
  roomId: string;
  folders: DealRoomFolder[];
  folderDocs: DealRoomFolderDocs[];
  workspaceDocuments?: Document[];
  roomDocuments?: DealRoomDocumentItem[];
  isAdmin?: boolean;
  onFolderCreate: (name: string, parentPath?: string) => Promise<void>;
  onFolderRename: (path: string, name: string) => Promise<void>;
  onFolderDelete: (path: string) => Promise<void>;
  onDocumentMove: (docId: string, folderPath: string) => Promise<void>;
  onDocumentReorder: (docId: string, sortOrder: number) => Promise<void>;
  onDocumentRemove: (docId: string) => Promise<void>;
  onDocumentsAdd?: (documentIds: string[], folderPath: string) => Promise<void>;
  onDocumentOpen?: (docId: string) => void;
  onFolderUpload?: (file: File, folderPath: string) => Promise<void>;
}

interface TreeNode {
  folder: DealRoomFolder;
  children: TreeNode[];
  documents: DealRoomDocumentItem[];
}

function parentPath(path: string): string | null {
  if (path === "/") return null;
  const idx = path.lastIndexOf("/");
  if (idx <= 0) return "/";
  return path.slice(0, idx);
}

function buildTree(folders: DealRoomFolder[], docsByFolder: Map<string, DealRoomDocumentItem[]>): TreeNode {
  const rootFolder = folders.find((f) => f.path === "/") ?? {
    path: "/",
    name: "Root",
    sort_order: 0,
  };
  const map = new Map<string, TreeNode>();
  map.set("/", { folder: rootFolder, children: [], documents: docsByFolder.get("/") ?? [] });

  const sorted = [...folders]
    .filter((f) => f.path !== "/")
    .sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name));

  for (const folder of sorted) {
    map.set(folder.path, { folder, children: [], documents: docsByFolder.get(folder.path) ?? [] });
  }

  for (const folder of sorted) {
    const node = map.get(folder.path)!;
    const parent = map.get(parentPath(folder.path) ?? "/");
    if (parent) {
      parent.children.push(node);
    } else {
      map.get("/")!.children.push(node);
    }
  }

  return map.get("/")!;
}

function ContextMenu({
  x,
  y,
  children,
  onClose,
}: {
  x: number;
  y: number;
  children: React.ReactNode;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) {
        onClose();
      }
    };
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("click", handleClick, true);
    window.addEventListener("scroll", onClose, true);
    window.addEventListener("resize", onClose, true);
    window.addEventListener("keydown", handleEsc);
    return () => {
      window.removeEventListener("click", handleClick, true);
      window.removeEventListener("scroll", onClose, true);
      window.removeEventListener("resize", onClose, true);
      window.removeEventListener("keydown", handleEsc);
    };
  }, [onClose]);

  return (
    <div
      ref={ref}
      className="fixed z-50 min-w-[10rem] rounded-md border border-border bg-popover p-1 shadow-md"
      style={{ left: x, top: y }}
      onContextMenu={(e) => e.preventDefault()}
    >
      {children}
    </div>
  );
}

export function DealRoomFolderTree({
  folders,
  folderDocs,
  workspaceDocuments,
  roomDocuments,
  isAdmin = true,
  onFolderCreate,
  onFolderRename,
  onFolderDelete,
  onDocumentMove,
  onDocumentReorder,
  onDocumentRemove,
  onDocumentsAdd,
  onDocumentOpen,
  onFolderUpload,
}: DealRoomFolderTreeProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const { t: td } = useTranslation("documents");

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

  const root = useMemo(() => buildTree(folders, docsByFolder), [folders, docsByFolder]);

  const [expanded, setExpanded] = useState<Set<string>>(new Set(["/", ...folders.map((f) => f.path)]));
  const [creatingParent, setCreatingParent] = useState<string | null>(null);
  const [newFolderName, setNewFolderName] = useState("");
  const [creating, setCreating] = useState(false);
  const [renamingFolder, setRenamingFolder] = useState<DealRoomFolder | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const [renaming, setRenaming] = useState(false);
  const [movingDoc, setMovingDoc] = useState<DealRoomDocumentItem | null>(null);
  const [targetFolder, setTargetFolder] = useState<string>("/");
  const [actingDocId, setActingDocId] = useState<string | null>(null);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; folder: DealRoomFolder } | null>(null);
  const [addToFolder, setAddToFolder] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [uploadingFolder, setUploadingFolder] = useState<string | null>(null);
  const folderUploadInputRef = useRef<HTMLInputElement>(null);

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
    setContextMenu(null);
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
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("folders.renameFailed"));
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
    try {
      await onFolderDelete(folder.path);
      setContextMenu(null);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("folders.deleteFailed"));
    }
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

  const handleContextMenu = (e: React.MouseEvent, folder: DealRoomFolder) => {
    e.preventDefault();
    e.stopPropagation();
    setContextMenu({ x: e.clientX, y: e.clientY, folder });
  };

  const handleAddDocuments = async (documentIds: string[], folderPath: string) => {
    if (!onDocumentsAdd) return;
    setAdding(true);
    try {
      await onDocumentsAdd(documentIds, folderPath);
      setAddToFolder(null);
    } finally {
      setAdding(false);
    }
  };

  const startFolderUpload = (folderPath: string) => {
    if (!onFolderUpload) return;
    setUploadingFolder(folderPath);
    folderUploadInputRef.current?.click();
  };

  const handleFolderUploadChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !uploadingFolder || !onFolderUpload) return;
    try {
      await onFolderUpload(file, uploadingFolder);
    } finally {
      setUploadingFolder(null);
      e.target.value = "";
    }
  };

  const renderCreateRow = (parentPath: string, depth: number) => (
    <div
      key={`create-${parentPath}`}
      className="flex items-center gap-2 rounded-md border border-border p-2"
      style={{ marginLeft: `${depth * 16 + 24}px` }}
    >
      <Folder size={16} className="text-muted-foreground" />
      <Input
        value={newFolderName}
        onChange={(e) => setNewFolderName(e.target.value)}
        placeholder={t("folders.namePlaceholder")}
        className="h-7 flex-1"
        autoFocus
        onKeyDown={(e) => {
          if (e.key === "Enter") void handleCreate();
          if (e.key === "Escape") setCreatingParent(null);
        }}
      />
      <Button size="sm" className="h-7" onClick={() => void handleCreate()} disabled={!newFolderName.trim() || creating}>
        {t("folders.create")}
      </Button>
      <Button size="sm" variant="ghost" className="h-7" onClick={() => setCreatingParent(null)}>
        <X size={14} />
      </Button>
    </div>
  );

  const renderDocument = (doc: DealRoomDocumentItem, idx: number, listLength: number, depth: number) => (
    <li
      key={doc.id}
      className="group flex items-center justify-between gap-2 rounded-md p-2 hover:bg-muted/30"
      style={{ marginLeft: `${depth * 16 + 24}px` }}
      onDoubleClick={() => onDocumentOpen?.(doc.document_id)}
      title={t("documents.doubleClickToOpen")}
    >
      <div className="flex items-center gap-2 min-w-0">
        <FileText size={16} className="text-muted-foreground shrink-0" />
        <span className="cursor-pointer truncate text-sm hover:text-primary">{doc.title}</span>
      </div>
      {isAdmin && (
        <div className="flex items-center gap-1 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={idx === 0 || actingDocId === doc.id}
            onClick={() => void handleReorder(doc, -1)}
            aria-label={t("documents.moveUp")}
          >
            <CaretRight className="rotate-[-90deg]" size={14} />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={idx === listLength - 1 || actingDocId === doc.id}
            onClick={() => void handleReorder(doc, 1)}
            aria-label={t("documents.moveDown")}
          >
            <CaretRight className="rotate-90" size={14} />
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
  );

  const renderFolder = (node: TreeNode, depth: number) => {
    const isExpanded = expanded.has(node.folder.path);
    const docs = node.documents;
    return (
      <div key={node.folder.path} className="select-none">
        <div
          role="button"
          tabIndex={0}
          aria-expanded={isExpanded}
          onClick={() => toggleFolder(node.folder.path)}
          onContextMenu={(e) => handleContextMenu(e, node.folder)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              toggleFolder(node.folder.path);
            }
          }}
          className="group flex w-full items-center justify-between gap-2 rounded-md p-2 text-left hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background outline-none"
          style={{ paddingLeft: `${depth * 16 + 8}px` }}
        >
          <div className="flex items-center gap-2 min-w-0">
            {isExpanded ? (
              <CaretDown size={16} className="text-muted-foreground shrink-0" />
            ) : (
              <CaretRight size={16} className="text-muted-foreground shrink-0" />
            )}
            {isExpanded ? (
              <FolderOpen size={18} className="text-primary shrink-0" />
            ) : (
              <Folder size={18} className="text-muted-foreground shrink-0" />
            )}
            <span className="truncate text-sm font-medium">{node.folder.name}</span>
            {node.folder.description && (
              <span className="hidden text-caption text-muted-foreground sm:inline">{node.folder.description}</span>
            )}
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <span className="text-caption text-muted-foreground">{docs.length}</span>
            {isAdmin && (
              <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={(e) => {
                    e.stopPropagation();
                    setCreatingParent(node.folder.path);
                  }}
                  aria-label={t("folders.newSubfolder")}
                >
                  <Plus size={14} />
                </Button>
                {onFolderUpload && (
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={(e) => {
                      e.stopPropagation();
                      startFolderUpload(node.folder.path);
                    }}
                    aria-label={t("folders.addFile")}
                  >
                    <UploadSimple size={14} />
                  </Button>
                )}
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={(e) => {
                    e.stopPropagation();
                    startRename(node.folder);
                  }}
                  aria-label={t("folders.rename")}
                >
                  <PencilSimple size={14} />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={(e) => {
                    e.stopPropagation();
                    void handleDelete(node.folder);
                  }}
                  aria-label={tc("delete")}
                >
                  <Trash size={14} />
                </Button>
              </div>
            )}
          </div>
        </div>

        {isExpanded && (
          <div className="py-1">
            {creatingParent === node.folder.path && renderCreateRow(node.folder.path, depth + 1)}
            {docs.length === 0 && creatingParent !== node.folder.path ? (
              <p className="py-2 text-sm text-muted-foreground" style={{ marginLeft: `${depth * 16 + 48}px` }}>
                {t("documents.emptyFolder")}
              </p>
            ) : (
              <ul className="space-y-0.5">
                {docs.map((doc, idx) => renderDocument(doc, idx, docs.length, depth + 1))}
              </ul>
            )}
            {node.children.map((child) => renderFolder(child, depth + 1))}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="space-y-2">
      {isAdmin && (
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" className="gap-1" onClick={() => setCreatingParent("/")}>
            <Plus size={14} />
            {t("folders.newRoot")}
          </Button>
        </div>
      )}

      {creatingParent === "/" && renderCreateRow("/", 0)}

      <input
        ref={folderUploadInputRef}
        type="file"
        accept={td("upload.supportedTypes")}
        data-testid="folder-upload-input"
        tabIndex={-1}
        aria-hidden
        className="absolute opacity-0 overflow-hidden w-[1px] h-[1px] p-0 m-[-1px] border-none"
        onChange={(e) => void handleFolderUploadChange(e)}
      />

      <div className="space-y-0.5">{renderFolder(root, 0)}</div>

      {contextMenu && (
        <ContextMenu x={contextMenu.x} y={contextMenu.y} onClose={() => setContextMenu(null)}>
          {isAdmin && (
            <>
              <button
                className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-left hover:bg-accent hover:text-accent-foreground"
                onClick={() => {
                  setCreatingParent(contextMenu.folder.path);
                  setContextMenu(null);
                }}
              >
                <Plus size={14} />
                {t("folders.newSubfolder")}
              </button>
              {onDocumentsAdd && workspaceDocuments && workspaceDocuments.length > 0 && (
                <button
                  className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-left hover:bg-accent hover:text-accent-foreground"
                  onClick={() => {
                    setAddToFolder(contextMenu.folder.path);
                    setContextMenu(null);
                  }}
                >
                  <FileText size={14} />
                  {t("folders.addFile")}
                </button>
              )}
              <button
                className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-left hover:bg-accent hover:text-accent-foreground"
                onClick={() => startRename(contextMenu.folder)}
              >
                <PencilSimple size={14} />
                {t("folders.rename")}
              </button>
              <div className="my-1 h-px bg-border" />
              <button
                className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-left text-destructive hover:bg-destructive/10"
                onClick={() => void handleDelete(contextMenu.folder)}
              >
                <Trash size={14} />
                {tc("delete")}
              </button>
            </>
          )}
          {!isAdmin && (
            <p className="px-2 py-1 text-sm text-muted-foreground">{t("folders.readOnly")}</p>
          )}
        </ContextMenu>
      )}

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

      <Dialog open={addToFolder !== null} onOpenChange={(open) => !open && setAddToFolder(null)}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("folders.addFile")}</DialogTitle>
            <DialogDescription>{t("documents.addFromWorkspace")}</DialogDescription>
          </DialogHeader>
          {addToFolder && workspaceDocuments && roomDocuments && (
            <DocumentPicker
              workspaceDocuments={workspaceDocuments}
              roomDocuments={roomDocuments}
              folders={folders}
              onAdd={handleAddDocuments}
              initialFolderPath={addToFolder}
              allowFolderChange={false}
              disabled={adding}
            />
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
